# Copilot Review Instructions

Review this repository as a senior backend engineer.

Prioritize production risks over formatting or style preferences. Keep feedback practical and focused on issues that could affect correctness, reliability, security, or operability.

Focus especially on:

- Worker reliability.
- Retry safety.
- Transaction boundaries.
- Concurrency and locking.
- Idempotency.
- Security.
- Observability.
- Test quality.

For each finding, include:

- File path.
- Risk level.
- Production impact.
- Minimal fix.

Avoid broad rewrites. Prefer small, targeted fixes that preserve the existing architecture.
