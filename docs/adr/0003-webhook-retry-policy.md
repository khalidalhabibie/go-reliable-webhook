# ADR 0003: Webhook Retry Policy

## Status

Accepted

## Context

Webhook receivers can fail for transient or permanent reasons. Retrying every failure can create unnecessary traffic and hide integration problems. Not retrying transient failures makes the system unreliable.

## Decision

Retry only failures that are likely transient:

- Timeout or network error: retry.
- HTTP `429`: retry.
- HTTP `500-599`: retry.
- HTTP `400`, `401`, `403`, `404`: do not retry.
- HTTP `200-299`: success.

Use this backoff schedule:

- Attempt 1: immediate.
- Attempt 2: 1 minute.
- Attempt 3: 5 minutes.
- Attempt 4: 15 minutes.
- Attempt 5: 30 minutes.

When retry attempts are exhausted, mark the delivery `DEAD`. For non-retryable failures, mark it `FAILED`.

## Consequences

Benefits:

- Transient receiver issues get another chance.
- Permanent client-side errors do not create unnecessary retries.
- Operators can manually replay `FAILED` or `DEAD` deliveries.

Tradeoffs:

- The schedule is fixed for now.
- Different subscribers cannot yet customize retry behavior.
- More advanced jitter and rate limiting can be added later.
