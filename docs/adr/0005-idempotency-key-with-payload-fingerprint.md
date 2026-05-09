# ADR 0005: Idempotency Key With Payload Fingerprint

## Status

Accepted

## Context

Clients may retry event creation if they hit a timeout or network error. Without idempotency, the same business event could create duplicate webhook events and duplicate deliveries.

At the same time, accidental reuse of the same idempotency key for different payloads should be rejected.

## Decision

Support an optional `Idempotency-Key` header on `POST /api/v1/events`.

For each event:

- Normalize the JSON payload.
- Store a SHA-256 `payload_fingerprint`.
- Store the idempotency key when provided.

Behavior:

- New key: create the event and deliveries.
- Same key and same payload fingerprint: return the existing event response.
- Same key and different payload fingerprint: return `409 Conflict`.

## Consequences

Benefits:

- Client retries do not create duplicate events.
- Key reuse with a different payload is detected.
- Fingerprints are smaller and easier to compare than full payloads.

Tradeoffs:

- Payload normalization must remain stable.
- The current behavior is scoped to event creation only.
- Idempotency retention is currently tied to event retention.
