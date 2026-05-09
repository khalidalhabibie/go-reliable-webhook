# go-reliable-webhook

`go-reliable-webhook` is a production-minded, not production-perfect, reliable webhook delivery system built with Go, Fiber, and PostgreSQL.

The project focuses on the core backend mechanics behind webhook delivery:

- Register webhook subscribers.
- Create events with idempotency protection.
- Generate delivery records for matching subscribers.
- Send signed HTTP webhook requests from a database-backed worker.
- Retry transient failures.
- Store attempt logs for debugging.
- Support manual replay for failed deliveries.

## Why Webhook Delivery Is Not Just HTTP POST

A simple HTTP POST is easy until the receiver is slow, offline, rate-limiting, or returns a temporary 500. A reliable webhook system needs durable state around that HTTP call:

- The original event must be stored before delivery starts.
- Each subscriber delivery must survive process restarts.
- Failed attempts need logs for debugging.
- Retries must be bounded and scheduled.
- Duplicate client requests should not create duplicate events.
- Multiple workers must not send the same delivery at the same time.
- Receivers need a way to verify that a payload came from this system and was not tampered with.

This project uses PostgreSQL as the source of truth for those concerns.

## Features

- Fiber HTTP API.
- PostgreSQL migrations.
- Subscriber management.
- Event creation with optional `Idempotency-Key`.
- Payload fingerprinting with SHA-256 over normalized JSON.
- Delivery creation per active matching subscriber.
- Database-backed worker with `FOR UPDATE SKIP LOCKED`.
- HMAC-SHA256 webhook signatures.
- Retry policy for network errors, timeouts, HTTP 429, and HTTP 5xx.
- Attempt logs for every send attempt.
- Delivery visibility endpoints.
- Manual replay for `FAILED` and `DEAD` deliveries.
- Recovery for stuck `PROCESSING` deliveries.

## Architecture Overview

The API writes durable records to PostgreSQL. The worker polls PostgreSQL for pending work and sends webhook requests asynchronously.

High-level flow:

1. A client registers a subscriber for an `event_type`.
2. A client creates an event.
3. The API stores the event and creates one delivery per active subscriber in one transaction.
4. The worker claims due deliveries using `SELECT FOR UPDATE SKIP LOCKED`.
5. The worker marks a delivery as `PROCESSING`, sends the HTTP POST, stores an attempt log, and updates the delivery status.
6. Failed retryable deliveries become `RETRYING` with `next_retry_at`.
7. Operators can replay terminal failed deliveries through the API.

Main packages:

- `cmd/api`: application entrypoint and route wiring.
- `internal/subscriber`: subscriber API, service, and repository.
- `internal/event`: event creation and idempotency logic.
- `internal/delivery`: delivery visibility, sender, worker, retry decisions, and replay.
- `internal/platform`: config, database, logging, and HTTP response helpers.

## Database Tables

`webhook_subscribers`

Stores subscriber endpoint configuration:

- `id`
- `name`
- `url`
- `event_type`
- `secret`
- `status`
- `created_at`
- `updated_at`

`webhook_events`

Stores incoming events:

- `id`
- `event_type`
- `payload`
- `status`
- `idempotency_key`
- `payload_fingerprint`
- `created_at`
- `updated_at`

`webhook_deliveries`

Stores delivery state per event/subscriber pair:

- `id`
- `event_id`
- `subscriber_id`
- `status`
- `attempt_count`
- `max_attempt`
- `next_retry_at`
- `last_attempt_at`
- `replay_count`
- `locked_at`
- `created_at`
- `updated_at`

`webhook_delivery_attempts`

Stores one row per send attempt:

- `id`
- `delivery_id`
- `attempt_number`
- `request_url`
- `request_headers`
- `request_body`
- `response_status_code`
- `response_body`
- `error_message`
- `duration_ms`
- `created_at`

## Retry Policy

The sender retries failures that are likely transient:

- Timeout or network error: retry.
- HTTP `429`: retry.
- HTTP `500-599`: retry.
- HTTP `400`, `401`, `403`, `404`: do not retry.
- HTTP `200-299`: success.

Backoff schedule:

- Attempt 1: immediate.
- Attempt 2: 1 minute.
- Attempt 3: 5 minutes.
- Attempt 4: 15 minutes.
- Attempt 5: 30 minutes.

When retries are exhausted, the delivery becomes `DEAD`. Non-retryable failures become `FAILED`.

## Idempotency Behavior

`POST /api/v1/events` supports an optional `Idempotency-Key` header.

- New key: create the event and deliveries.
- Same key with the same normalized payload: return the existing event response.
- Same key with a different payload: return `409 Conflict`.

The payload fingerprint is a SHA-256 hash of normalized JSON. This prevents duplicate event creation during client retries while still rejecting accidental key reuse for different payloads.

## Signature Verification

Webhook requests include:

- `X-Webhook-Event-Id`
- `X-Webhook-Delivery-Id`
- `X-Webhook-Timestamp`
- `X-Webhook-Signature`

Signature format:

```text
sha256=<hex_signature>
```

Signature payload:

```text
timestamp + "." + raw_request_body
```

The signature is HMAC-SHA256 using the subscriber secret. Receivers should compute the same HMAC and compare it using constant-time comparison.

## Worker Crash Recovery

`PROCESSING` is a temporary state. If a worker crashes after claiming a delivery but before writing the final status, the delivery could otherwise remain stuck.

The worker periodically recovers stuck processing rows:

- Find `PROCESSING` deliveries where `locked_at` is older than 10 minutes.
- Change them to `RETRYING`.
- Set `next_retry_at` to now.
- Do not increment `attempt_count`.
- Do not delete attempt logs.

This keeps delivery state recoverable after process crashes while preserving debugging history.

## How To Run Locally

Start PostgreSQL:

```sh
docker compose up -d postgres
```

Set required environment variables:

```sh
export APP_ENV=development
export PORT=8080
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/reliable_webhook?sslmode=disable'
export HTTP_CLIENT_TIMEOUT_SECONDS=10
```

Run migrations:

```sh
make migrate-up
```

Run the API and worker:

```sh
make run
```

Check health:

```sh
curl http://localhost:8080/api/v1/health
```

Run tests:

```sh
make test
```

## Example API Requests

Create a subscriber:

```sh
curl -X POST http://localhost:8080/api/v1/subscribers \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Billing Service",
    "url": "https://example.com/webhooks",
    "event_type": "payment.succeeded"
  }'
```

List subscribers:

```sh
curl http://localhost:8080/api/v1/subscribers
```

Deactivate a subscriber:

```sh
curl -X PATCH http://localhost:8080/api/v1/subscribers/<subscriber_id>/deactivate
```

Create an event:

```sh
curl -X POST http://localhost:8080/api/v1/events \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: payment-pay_123-succeeded' \
  -d '{
    "event_type": "payment.succeeded",
    "payload": {
      "payment_id": "pay_123",
      "amount": 150000,
      "currency": "IDR"
    }
  }'
```

List deliveries:

```sh
curl 'http://localhost:8080/api/v1/deliveries?status=FAILED&page=1&size=20'
```

Get delivery detail with attempt logs:

```sh
curl http://localhost:8080/api/v1/deliveries/<delivery_id>
```

Replay a failed delivery:

```sh
curl -X POST http://localhost:8080/api/v1/deliveries/<delivery_id>/replay
```

## Future Improvements

- Graceful worker shutdown with in-flight attempt tracking.
- Dedicated worker process mode separate from the API process.
- Metrics for delivery latency, attempts, success rate, and dead deliveries.
- Admin authentication and authorization.
- Subscriber secret rotation.
- Stronger payload validation and event schemas.
- Dead-letter review workflow.
- More complete integration tests with PostgreSQL.
- Request signing timestamp tolerance guidance for webhook receivers.
