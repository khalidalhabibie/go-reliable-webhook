package event

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const uniqueViolationCode = "23505"

type Repository interface {
	CreateWithDeliveries(ctx context.Context, evt Event) (Event, bool, error)
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateWithDeliveries(ctx context.Context, evt Event) (Event, bool, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return Event{}, false, fmt.Errorf("begin event transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if evt.IdempotencyKey != nil {
		existing, found, err := getEventByIdempotencyKey(ctx, tx, *evt.IdempotencyKey, true)
		if err != nil {
			return Event{}, false, err
		}
		if found {
			if err := tx.Commit(); err != nil {
				return Event{}, false, fmt.Errorf("commit existing event transaction: %w", err)
			}
			return existing, true, nil
		}
	}

	subscriberIDs, err := activeSubscriberIDs(ctx, tx, evt.EventType)
	if err != nil {
		return Event{}, false, err
	}

	evt.DeliveryCount = len(subscriberIDs)
	if evt.DeliveryCount == 0 {
		evt.Status = StatusFailed
	} else {
		evt.Status = StatusPending
	}

	created, err := insertEvent(ctx, tx, evt)
	if err != nil {
		if evt.IdempotencyKey != nil && isUniqueViolation(err) {
			_ = tx.Rollback()
			existing, found, getErr := getEventAfterConflict(ctx, r.db, *evt.IdempotencyKey)
			if getErr != nil {
				return Event{}, false, getErr
			}
			if found {
				return existing, true, nil
			}
		}
		return Event{}, false, err
	}

	for _, subscriberID := range subscriberIDs {
		if err := insertDelivery(ctx, tx, created.ID, subscriberID, created.CreatedAt); err != nil {
			return Event{}, false, err
		}
	}
	created.DeliveryCount = len(subscriberIDs)

	if err := tx.Commit(); err != nil {
		return Event{}, false, fmt.Errorf("commit event transaction: %w", err)
	}

	return created, false, nil
}

func getEventAfterConflict(ctx context.Context, db *sql.DB, idempotencyKey string) (Event, bool, error) {
	existing, found, err := getEventByIdempotencyKey(ctx, db, idempotencyKey, false)
	if err != nil {
		return Event{}, false, err
	}
	return existing, found, nil
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func getEventByIdempotencyKey(ctx context.Context, q queryer, idempotencyKey string, lock bool) (Event, bool, error) {
	query := `
		WITH event_row AS (
			SELECT id, event_type, payload, status, idempotency_key, payload_fingerprint, created_at, updated_at
			FROM webhook_events
			WHERE idempotency_key = $1
		)
		SELECT e.id, e.event_type, e.payload, e.status, e.idempotency_key, e.payload_fingerprint,
			e.created_at, e.updated_at,
			(SELECT COUNT(*) FROM webhook_deliveries d WHERE d.event_id = e.id) AS delivery_count
		FROM event_row e`
	if lock {
		query = `
			WITH event_row AS (
				SELECT id, event_type, payload, status, idempotency_key, payload_fingerprint, created_at, updated_at
				FROM webhook_events
				WHERE idempotency_key = $1
				FOR UPDATE
			)
			SELECT e.id, e.event_type, e.payload, e.status, e.idempotency_key, e.payload_fingerprint,
				e.created_at, e.updated_at,
				(SELECT COUNT(*) FROM webhook_deliveries d WHERE d.event_id = e.id) AS delivery_count
			FROM event_row e`
	}

	evt, err := scanEvent(q.QueryRowContext(ctx, query, idempotencyKey))
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, false, nil
	}
	if err != nil {
		return Event{}, false, fmt.Errorf("get event by idempotency key: %w", err)
	}

	return evt, true, nil
}

func activeSubscriberIDs(ctx context.Context, tx *sql.Tx, eventType string) ([]string, error) {
	const query = `
		SELECT id
		FROM webhook_subscribers
		WHERE event_type = $1 AND status = 'ACTIVE'
		ORDER BY created_at ASC`

	rows, err := tx.QueryContext(ctx, query, eventType)
	if err != nil {
		return nil, fmt.Errorf("find active subscribers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan active subscriber: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("active subscriber rows: %w", err)
	}

	return ids, nil
}

func insertEvent(ctx context.Context, tx *sql.Tx, evt Event) (Event, error) {
	const query = `
		INSERT INTO webhook_events (
			id, event_type, payload, status, idempotency_key, payload_fingerprint, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, event_type, payload, status, idempotency_key, payload_fingerprint, created_at, updated_at, 0`

	created, err := scanEvent(tx.QueryRowContext(
		ctx,
		query,
		evt.ID,
		evt.EventType,
		evt.Payload,
		evt.Status,
		evt.IdempotencyKey,
		evt.PayloadFingerprint,
		evt.CreatedAt,
		evt.UpdatedAt,
	))
	if err != nil {
		return Event{}, fmt.Errorf("insert event: %w", err)
	}

	return created, nil
}

func insertDelivery(ctx context.Context, tx *sql.Tx, eventID string, subscriberID string, now time.Time) error {
	const query = `
		INSERT INTO webhook_deliveries (
			id, event_id, subscriber_id, status, attempt_count, max_attempt, replay_count, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, 0, 5, 0, $5, $6)`

	if _, err := tx.ExecContext(ctx, query, uuid.NewString(), eventID, subscriberID, DeliveryStatusPending, now, now); err != nil {
		return fmt.Errorf("insert delivery: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEvent(row scanner) (Event, error) {
	var evt Event
	var idempotencyKey sql.NullString

	err := row.Scan(
		&evt.ID,
		&evt.EventType,
		&evt.Payload,
		&evt.Status,
		&idempotencyKey,
		&evt.PayloadFingerprint,
		&evt.CreatedAt,
		&evt.UpdatedAt,
		&evt.DeliveryCount,
	)
	if err != nil {
		return Event{}, err
	}
	if idempotencyKey.Valid {
		evt.IdempotencyKey = &idempotencyKey.String
	}

	return evt, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}
