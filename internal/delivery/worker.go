package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

const (
	DeliveryStatusPending    = "PENDING"
	DeliveryStatusProcessing = "PROCESSING"
	DeliveryStatusRetrying   = "RETRYING"
	DeliveryStatusSuccess    = "SUCCESS"
	DeliveryStatusFailed     = "FAILED"
	DeliveryStatusDead       = "DEAD"

	workerBatchSize    = 10
	workerPollInterval = 5 * time.Second
	processingLockTTL  = 10 * time.Minute
)

var ErrStaleDeliveryLock = errors.New("stale delivery lock")

type WebhookSender interface {
	Send(ctx context.Context, req SendRequest) SendResult
}

type Worker struct {
	db     *sql.DB
	sender WebhookSender
	log    *slog.Logger
	now    func() time.Time
}

type claimedDelivery struct {
	ID               string
	EventID          string
	SubscriberID     string
	SubscriberURL    string
	SubscriberSecret string
	Payload          []byte
	AttemptCount     int
	MaxAttempt       int
	LockToken        string
}

type deliveryDecision struct {
	Status      string
	NextRetryAt *time.Time
}

func NewWorker(db *sql.DB, sender WebhookSender, log *slog.Logger) *Worker {
	return &Worker{
		db:     db,
		sender: sender,
		log:    log,
		now:    time.Now,
	}
}

func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(workerPollInterval)
	defer ticker.Stop()

	for {
		if err := w.ProcessBatch(ctx); err != nil {
			w.log.Error("delivery worker batch failed", "error", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) ProcessBatch(ctx context.Context) error {
	if err := w.recoverStaleProcessing(ctx); err != nil {
		return err
	}

	deliveries, err := w.claimBatch(ctx)
	if err != nil {
		return err
	}

	for _, item := range deliveries {
		result := w.sender.Send(ctx, SendRequest{
			DeliveryID:       item.ID,
			EventID:          item.EventID,
			SubscriberURL:    item.SubscriberURL,
			SubscriberSecret: item.SubscriberSecret,
			Payload:          item.Payload,
		})
		if ctx.Err() != nil && result.ErrorMessage != nil {
			result.ShouldRetry = true
			result.RetryImmediately = true
		}

		recordCtx := ctx
		var cancelRecord context.CancelFunc
		if ctx.Err() != nil {
			recordCtx, cancelRecord = context.WithTimeout(context.Background(), 5*time.Second)
		}

		if err := w.recordAttempt(recordCtx, item, result); err != nil {
			if cancelRecord != nil {
				cancelRecord()
			}
			if errors.Is(err, ErrStaleDeliveryLock) {
				w.log.Info("stale_delivery_lock", "delivery_id", item.ID)
				continue
			}
			w.log.Error("failed to record delivery attempt", "delivery_id", item.ID, "error", err)
		}
		if cancelRecord != nil {
			cancelRecord()
		}
	}

	return nil
}

func (w *Worker) recoverStaleProcessing(ctx context.Context) error {
	const query = `
		UPDATE webhook_deliveries
		SET status = $1,
			next_retry_at = NOW(),
			locked_at = NULL,
			lock_token = NULL,
			updated_at = NOW()
		WHERE status = $2
			AND locked_at IS NOT NULL
			AND locked_at <= NOW() - ($3::interval)`

	result, err := w.db.ExecContext(
		ctx,
		query,
		DeliveryStatusRetrying,
		DeliveryStatusProcessing,
		fmt.Sprintf("%d seconds", int(processingLockTTL.Seconds())),
	)
	if err != nil {
		return fmt.Errorf("recover stale processing deliveries: %w", err)
	}

	recovered, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read recovered delivery count: %w", err)
	}
	if recovered > 0 {
		w.log.Info("delivery_processing_recovered", "count", recovered)
	}

	return nil
}

func (w *Worker) claimBatch(ctx context.Context) ([]claimedDelivery, error) {
	tx, err := w.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin claim delivery transaction: %w", err)
	}
	defer tx.Rollback()

	lockToken := uuid.NewString()
	const query = `
		WITH candidates AS (
			SELECT id
			FROM webhook_deliveries
			WHERE status IN ($1, $2)
				AND (next_retry_at IS NULL OR next_retry_at <= NOW())
			ORDER BY created_at ASC
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		UPDATE webhook_deliveries d
		SET status = $4,
			locked_at = NOW(),
			lock_token = $5,
			updated_at = NOW()
		FROM candidates c, webhook_events e, webhook_subscribers s
		WHERE d.id = c.id
			AND e.id = d.event_id
			AND s.id = d.subscriber_id
		RETURNING d.id, d.event_id, d.subscriber_id, s.url, s.secret, e.payload,
			d.attempt_count, d.max_attempt, d.lock_token`

	rows, err := tx.QueryContext(
		ctx,
		query,
		DeliveryStatusPending,
		DeliveryStatusRetrying,
		workerBatchSize,
		DeliveryStatusProcessing,
		lockToken,
	)
	if err != nil {
		return nil, fmt.Errorf("claim deliveries: %w", err)
	}
	defer rows.Close()

	deliveries := make([]claimedDelivery, 0, workerBatchSize)
	for rows.Next() {
		var item claimedDelivery
		if err := rows.Scan(
			&item.ID,
			&item.EventID,
			&item.SubscriberID,
			&item.SubscriberURL,
			&item.SubscriberSecret,
			&item.Payload,
			&item.AttemptCount,
			&item.MaxAttempt,
			&item.LockToken,
		); err != nil {
			return nil, fmt.Errorf("scan claimed delivery: %w", err)
		}
		deliveries = append(deliveries, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claimed delivery rows: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claim delivery transaction: %w", err)
	}

	return deliveries, nil
}

func (w *Worker) recordAttempt(ctx context.Context, item claimedDelivery, result SendResult) error {
	tx, err := w.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin record attempt transaction: %w", err)
	}
	defer tx.Rollback()

	now := w.now().UTC()
	attemptNumber := item.AttemptCount + 1
	decision := decideDeliveryStatus(result, attemptNumber, item.MaxAttempt, now)
	requestHeaders, err := requestHeadersJSON(item)
	if err != nil {
		return err
	}

	const insertAttempt = `
		INSERT INTO webhook_delivery_attempts (
			id, delivery_id, attempt_number, request_url, request_headers, request_body,
			response_status_code, response_body, error_message, duration_ms, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	if _, err := tx.ExecContext(
		ctx,
		insertAttempt,
		uuid.NewString(),
		item.ID,
		attemptNumber,
		item.SubscriberURL,
		requestHeaders,
		item.Payload,
		nullableInt(result.StatusCode),
		nullableString(result.ResponseBody),
		nullableString(result.ErrorMessage),
		result.DurationMS,
		now,
	); err != nil {
		return fmt.Errorf("insert delivery attempt: %w", err)
	}

	const updateDelivery = `
		UPDATE webhook_deliveries
		SET status = $1,
			attempt_count = $2,
			next_retry_at = $3,
			last_attempt_at = $4,
			locked_at = NULL,
			lock_token = NULL,
			updated_at = $5
		WHERE id = $6
			AND status = $7
			AND lock_token = $8`

	updateResult, err := tx.ExecContext(
		ctx,
		updateDelivery,
		decision.Status,
		attemptNumber,
		decision.NextRetryAt,
		now,
		now,
		item.ID,
		DeliveryStatusProcessing,
		item.LockToken,
	)
	if err != nil {
		return fmt.Errorf("update delivery after attempt: %w", err)
	}
	updated, err := updateResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated delivery count: %w", err)
	}
	if updated == 0 {
		return ErrStaleDeliveryLock
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit record attempt transaction: %w", err)
	}

	return nil
}

func decideDeliveryStatus(result SendResult, attemptNumber int, maxAttempt int, now time.Time) deliveryDecision {
	if result.ErrorMessage == nil && result.StatusCode != nil && *result.StatusCode >= 200 && *result.StatusCode <= 299 {
		return deliveryDecision{Status: DeliveryStatusSuccess}
	}

	if result.ShouldRetry {
		if attemptNumber < maxAttempt {
			nextRetryAt := now
			if !result.RetryImmediately {
				nextRetryAt = now.Add(backoffDuration(attemptNumber + 1))
			}
			return deliveryDecision{Status: DeliveryStatusRetrying, NextRetryAt: &nextRetryAt}
		}
		return deliveryDecision{Status: DeliveryStatusDead}
	}

	return deliveryDecision{Status: DeliveryStatusFailed}
}

func backoffDuration(nextAttemptNumber int) time.Duration {
	switch nextAttemptNumber {
	case 1:
		return 0
	case 2:
		return time.Minute
	case 3:
		return 5 * time.Minute
	case 4:
		return 15 * time.Minute
	default:
		return 30 * time.Minute
	}
}

func requestHeadersJSON(item claimedDelivery) ([]byte, error) {
	headers := map[string]string{
		"Content-Type":          "application/json",
		"X-Webhook-Event-Id":    item.EventID,
		"X-Webhook-Delivery-Id": item.ID,
		"X-Webhook-Signature":   "[redacted]",
		"X-Webhook-Timestamp":   "[generated]",
	}

	value, err := json.Marshal(headers)
	if err != nil {
		return nil, fmt.Errorf("marshal delivery attempt request headers: %w", err)
	}
	return value, nil
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
