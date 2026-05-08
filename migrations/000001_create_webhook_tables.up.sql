CREATE TABLE webhook_subscribers (
    id UUID PRIMARY KEY,
    name VARCHAR NOT NULL,
    url TEXT NOT NULL,
    event_type VARCHAR NOT NULL,
    secret TEXT NOT NULL,
    status VARCHAR NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CONSTRAINT webhook_subscribers_status_check CHECK (status IN ('ACTIVE', 'INACTIVE'))
);

CREATE TABLE webhook_events (
    id UUID PRIMARY KEY,
    event_type VARCHAR NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR NOT NULL,
    idempotency_key VARCHAR NULL,
    payload_fingerprint VARCHAR NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CONSTRAINT webhook_events_status_check CHECK (status IN ('PENDING', 'COMPLETED', 'FAILED'))
);

CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES webhook_events(id),
    subscriber_id UUID NOT NULL REFERENCES webhook_subscribers(id),
    status VARCHAR NOT NULL,
    attempt_count INT NOT NULL DEFAULT 0,
    max_attempt INT NOT NULL DEFAULT 5,
    next_retry_at TIMESTAMP NULL,
    last_attempt_at TIMESTAMP NULL,
    replay_count INT NOT NULL DEFAULT 0,
    locked_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CONSTRAINT webhook_deliveries_status_check CHECK (status IN ('PENDING', 'PROCESSING', 'SUCCEEDED', 'FAILED')),
    CONSTRAINT webhook_deliveries_attempt_count_check CHECK (attempt_count >= 0),
    CONSTRAINT webhook_deliveries_max_attempt_check CHECK (max_attempt > 0),
    CONSTRAINT webhook_deliveries_replay_count_check CHECK (replay_count >= 0)
);

CREATE TABLE webhook_delivery_attempts (
    id UUID PRIMARY KEY,
    delivery_id UUID NOT NULL REFERENCES webhook_deliveries(id),
    attempt_number INT NOT NULL,
    request_url TEXT NOT NULL,
    request_headers JSONB NULL,
    request_body JSONB NOT NULL,
    response_status_code INT NULL,
    response_body TEXT NULL,
    error_message TEXT NULL,
    duration_ms INT NULL,
    created_at TIMESTAMP NOT NULL,
    CONSTRAINT webhook_delivery_attempts_attempt_number_check CHECK (attempt_number > 0),
    CONSTRAINT webhook_delivery_attempts_duration_ms_check CHECK (duration_ms IS NULL OR duration_ms >= 0)
);

CREATE INDEX idx_webhook_subscribers_event_type_status
    ON webhook_subscribers(event_type, status);

CREATE UNIQUE INDEX idx_webhook_events_idempotency_key
    ON webhook_events(idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX idx_webhook_deliveries_status_next_retry_at
    ON webhook_deliveries(status, next_retry_at);

CREATE INDEX idx_webhook_deliveries_event_id
    ON webhook_deliveries(event_id);

CREATE INDEX idx_webhook_deliveries_subscriber_id
    ON webhook_deliveries(subscriber_id);

CREATE INDEX idx_webhook_delivery_attempts_delivery_id
    ON webhook_delivery_attempts(delivery_id);
