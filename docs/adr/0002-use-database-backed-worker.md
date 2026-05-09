# ADR 0002: Use Database-Backed Worker

## Status

Accepted

## Context

The system must deliver webhooks asynchronously. Sending webhooks inside the event creation request would make the API slow and fragile because external receivers can be unavailable or slow.

## Decision

Use a simple worker loop that polls `webhook_deliveries` for due records and processes them in batches.

The worker:

- Claims due deliveries.
- Marks them `PROCESSING`.
- Sends the webhook request.
- Stores an attempt log.
- Updates delivery status and retry schedule.

## Consequences

Benefits:

- API responses stay fast.
- Delivery work survives process restarts.
- Failed attempts can be retried later.
- Operational state remains visible in PostgreSQL.

Tradeoffs:

- Polling introduces a small delay.
- The API process currently starts the worker, which is simple but not ideal for all deployments.
- A future deployment may split API and worker into separate processes.
