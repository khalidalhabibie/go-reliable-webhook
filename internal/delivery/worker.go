package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
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
	processingLockTTL  = 5 * time.Minute
)

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

		if err := w.recordAttempt(ctx, item, result); err != nil {
			w.log.Error("failed to record delivery attempt", "delivery_id", item.ID, "error", err)
		}
	}

	return nil
}

func (w *Worker) recoverStaleProcessing(ctx context.Context) error {
	const query = `
		UPDATE webhook_deliveries
		SET status = $1,
			next_retry_at = NULL,
			locked_at = NULL,
			updated_at = NOW()
		WHERE status = $2
			AND locked_at IS NOT NULL
			AND locked_at <= NOW() - ($3::interval)`

	if _, err := w.db.ExecContext(
		ctx,
		query,
		DeliveryStatusRetrying,
		DeliveryStatusProcessing,
		fmt.Sprintf("%d seconds", int(processingLockTTL.Seconds())),
	); err != nil {
		return fmt.Errorf("recover stale processing deliveries: %w", err)
	}

	return nil
}

func (w *Worker) claimBatch(ctx context.Context) ([]claimedDelivery, error) {
	tx, err := w.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin claim delivery transaction: %w", err)
	}
	defer tx.Rollback()

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
			updated_at = NOW()
		FROM candidates c, webhook_events e, webhook_subscribers s
		WHERE d.id = c.id
			AND e.id = d.event_id
			AND s.id = d.subscriber_id
		RETURNING d.id, d.event_id, d.subscriber_id, s.url, s.secret, e.payload,
			d.attempt_count, d.max_attempt`

	rows, err := tx.QueryContext(
		ctx,
		query,
		DeliveryStatusPending,
		DeliveryStatusRetrying,
		workerBatchSize,
		DeliveryStatusProcessing,
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
			updated_at = $5
		WHERE id = $6`

	if _, err := tx.ExecContext(
		ctx,
		updateDelivery,
		decision.Status,
		attemptNumber,
		decision.NextRetryAt,
		now,
		now,
		item.ID,
	); err != nil {
		return fmt.Errorf("update delivery after attempt: %w", err)
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
			nextRetryAt := now.Add(backoffDuration(attemptNumber + 1))
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
