DROP INDEX IF EXISTS idx_webhook_delivery_attempts_delivery_id;
DROP INDEX IF EXISTS idx_webhook_deliveries_subscriber_id;
DROP INDEX IF EXISTS idx_webhook_deliveries_event_id;
DROP INDEX IF EXISTS idx_webhook_deliveries_status_next_retry_at;
DROP INDEX IF EXISTS idx_webhook_events_idempotency_key;
DROP INDEX IF EXISTS idx_webhook_subscribers_event_type_status;

DROP TABLE IF EXISTS webhook_delivery_attempts;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS webhook_subscribers;
