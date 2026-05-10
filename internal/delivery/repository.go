package delivery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("delivery not found")

type Repository interface {
	List(ctx context.Context, filters ListFilters) ([]Delivery, error)
	GetByID(ctx context.Context, id string) (Delivery, error)
	Replay(ctx context.Context, id string) (Delivery, error)
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) List(ctx context.Context, filters ListFilters) ([]Delivery, error) {
	query := `
		SELECT id, event_id, subscriber_id, status, attempt_count, max_attempt,
			next_retry_at, last_attempt_at, replay_count, created_at, updated_at
		FROM webhook_deliveries`

	args := make([]any, 0, 5)
	conditions := make([]string, 0, 3)

	if filters.Status != "" {
		args = append(args, filters.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if filters.EventID != "" {
		args = append(args, filters.EventID)
		conditions = append(conditions, fmt.Sprintf("event_id = $%d", len(args)))
	}
	if filters.SubscriberID != "" {
		args = append(args, filters.SubscriberID)
		conditions = append(conditions, fmt.Sprintf("subscriber_id = $%d", len(args)))
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	offset := (filters.Page - 1) * filters.Size
	args = append(args, filters.Size, offset)
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list deliveries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	deliveries := make([]Delivery, 0)
	for rows.Next() {
		item, err := scanDelivery(rows)
		if err != nil {
			return nil, fmt.Errorf("scan delivery: %w", err)
		}
		deliveries = append(deliveries, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("delivery rows: %w", err)
	}

	return deliveries, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (Delivery, error) {
	const query = `
		SELECT id, event_id, subscriber_id, status, attempt_count, max_attempt,
			next_retry_at, last_attempt_at, replay_count, created_at, updated_at
		FROM webhook_deliveries
		WHERE id = $1`

	item, err := scanDelivery(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Delivery{}, ErrNotFound
	}
	if err != nil {
		return Delivery{}, fmt.Errorf("get delivery: %w", err)
	}

	attempts, err := r.listAttempts(ctx, id)
	if err != nil {
		return Delivery{}, err
	}
	item.Attempts = attempts

	return item, nil
}

func (r *PostgresRepository) Replay(ctx context.Context, id string) (Delivery, error) {
	const query = `
		UPDATE webhook_deliveries
		SET status = $2,
			next_retry_at = NOW(),
			replay_count = replay_count + 1,
			locked_at = NULL,
			updated_at = NOW()
		WHERE id = $1
			AND status IN ($3, $4)
		RETURNING id, event_id, subscriber_id, status, attempt_count, max_attempt,
			next_retry_at, last_attempt_at, replay_count, created_at, updated_at`

	item, err := scanDelivery(r.db.QueryRowContext(
		ctx,
		query,
		id,
		DeliveryStatusPending,
		DeliveryStatusFailed,
		DeliveryStatusDead,
	))
	if errors.Is(err, sql.ErrNoRows) {
		exists, existsErr := r.exists(ctx, id)
		if existsErr != nil {
			return Delivery{}, existsErr
		}
		if !exists {
			return Delivery{}, ErrNotFound
		}
		return Delivery{}, ErrNotReplayable
	}
	if err != nil {
		return Delivery{}, fmt.Errorf("replay delivery: %w", err)
	}

	return item, nil
}

func (r *PostgresRepository) exists(ctx context.Context, id string) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM webhook_deliveries WHERE id = $1)`

	var exists bool
	if err := r.db.QueryRowContext(ctx, query, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("check delivery exists: %w", err)
	}

	return exists, nil
}

func (r *PostgresRepository) listAttempts(ctx context.Context, deliveryID string) ([]Attempt, error) {
	const query = `
		SELECT id, delivery_id, attempt_number, request_url, request_headers, request_body,
			response_status_code, response_body, error_message, duration_ms, created_at
		FROM webhook_delivery_attempts
		WHERE delivery_id = $1
		ORDER BY attempt_number ASC, created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, deliveryID)
	if err != nil {
		return nil, fmt.Errorf("list delivery attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	attempts := make([]Attempt, 0)
	for rows.Next() {
		attempt, err := scanAttempt(rows)
		if err != nil {
			return nil, fmt.Errorf("scan delivery attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("delivery attempt rows: %w", err)
	}

	return attempts, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDelivery(row scanner) (Delivery, error) {
	var item Delivery
	var nextRetryAt sql.NullTime
	var lastAttemptAt sql.NullTime

	err := row.Scan(
		&item.ID,
		&item.EventID,
		&item.SubscriberID,
		&item.Status,
		&item.AttemptCount,
		&item.MaxAttempt,
		&nextRetryAt,
		&lastAttemptAt,
		&item.ReplayCount,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Delivery{}, err
	}
	if nextRetryAt.Valid {
		item.NextRetryAt = &nextRetryAt.Time
	}
	if lastAttemptAt.Valid {
		item.LastAttemptAt = &lastAttemptAt.Time
	}

	return item, nil
}

func scanAttempt(row scanner) (Attempt, error) {
	var attempt Attempt
	var requestHeaders []byte
	var requestBody []byte
	var responseStatusCode sql.NullInt64
	var responseBody sql.NullString
	var errorMessage sql.NullString
	var durationMS sql.NullInt64

	err := row.Scan(
		&attempt.ID,
		&attempt.DeliveryID,
		&attempt.AttemptNumber,
		&attempt.RequestURL,
		&requestHeaders,
		&requestBody,
		&responseStatusCode,
		&responseBody,
		&errorMessage,
		&durationMS,
		&attempt.CreatedAt,
	)
	if err != nil {
		return Attempt{}, err
	}
	attempt.RequestHeaders = requestHeaders
	attempt.RequestBody = requestBody
	if responseStatusCode.Valid {
		value := int(responseStatusCode.Int64)
		attempt.ResponseStatusCode = &value
	}
	if responseBody.Valid {
		attempt.ResponseBody = &responseBody.String
	}
	if errorMessage.Valid {
		attempt.ErrorMessage = &errorMessage.String
	}
	if durationMS.Valid {
		value := int(durationMS.Int64)
		attempt.DurationMS = &value
	}

	return attempt, nil
}
