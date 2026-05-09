# ADR 0006: SKIP LOCKED Worker Concurrency

## Status

Accepted

## Context

Multiple workers may run at the same time. Without coordination, two workers could select and send the same delivery.

The system needs concurrency control without adding Redis, Kafka, or a separate distributed lock service.

## Decision

Use PostgreSQL row-level locking with `FOR UPDATE SKIP LOCKED` when claiming due deliveries.

The worker:

- Selects due `PENDING` or `RETRYING` deliveries.
- Locks up to 10 rows.
- Skips rows already locked by another worker.
- Marks claimed rows as `PROCESSING`.
- Commits the claim before sending HTTP requests.

## Consequences

Benefits:

- Prevents duplicate processing across concurrent workers.
- Uses PostgreSQL, which is already the source of truth.
- Keeps the worker architecture simple for the MVP.

Tradeoffs:

- Claimed rows can become stuck if a worker crashes after marking `PROCESSING`.
- Stuck processing recovery is required and runs periodically.
- Very high throughput may require a more specialized queue later.
