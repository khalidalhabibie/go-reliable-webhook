# ADR 0001: Use PostgreSQL as Source of Truth

## Status

Accepted

## Context

Webhook delivery needs durable state. Events, deliveries, attempts, retries, replay actions, and crash recovery must survive application restarts.

For this MVP, adding Kafka, Redis, or another queue would increase operational complexity before the project needs it.

## Decision

Use PostgreSQL as the source of truth for:

- Subscribers.
- Events.
- Deliveries.
- Attempt logs.
- Retry scheduling.
- Worker locking state.
- Manual replay state.

## Consequences

Benefits:

- Durable state is centralized.
- Transactions can atomically create events and deliveries.
- Attempt logs are queryable for debugging.
- Worker restart recovery can be implemented with database state.

Tradeoffs:

- PostgreSQL polling has limits at very high throughput.
- Careful indexing is needed for worker polling.
- Long-running transactions must be avoided.

This is a production-minded choice for the MVP, not a claim that PostgreSQL polling is perfect for every scale.
