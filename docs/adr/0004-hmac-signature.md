# ADR 0004: HMAC Signature

## Status

Accepted

## Context

Webhook receivers need a way to verify that a request came from this system and that the payload was not modified in transit.

## Decision

Sign webhook requests with HMAC-SHA256 using the subscriber secret.

Headers:

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

## Consequences

Benefits:

- Receivers can verify payload integrity.
- Each subscriber has its own secret.
- The raw request body is signed, avoiding ambiguity from JSON reformatting after signing.

Tradeoffs:

- Receivers must preserve the raw request body for verification.
- Timestamp tolerance and replay protection should be documented more fully before broad external use.
- Secret rotation is not implemented yet.
