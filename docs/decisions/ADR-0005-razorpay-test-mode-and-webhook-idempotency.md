# ADR-0005: Razorpay Test Mode and Webhook-Driven Idempotency

## Context & Problem Statement
Release 1 requires a payment gateway integration for purchasing credit packs. Real money transactions must not be enabled during development, and the integration must be resilient against duplicate webhooks, network drops, and browser callback interruptions.

## Decision
We choose **Razorpay in Test Mode** as the initial payment gateway, with **server-side order creation and webhook-driven credit fulfillment protected by an idempotency log table (`payment_webhook_events`)**.

## Rationale & Engineering Trade-offs

### Advantages of the Webhook-Driven Architecture
1. **Never Rely on Client-Side Callbacks**: Browsers can crash, close, lose network connectivity, or have ad-blockers interfere after payment completion. The server-to-server webhook from Razorpay (`payment.captured`) is the definitive, authoritative confirmation that funds were collected.
2. **Mandatory HMAC-SHA256 Cryptographic Verification**: The backend verifies the raw webhook payload against Razorpay's shared webhook secret using `crypto/hmac`. Forged webhooks are rejected with HTTP 400.
3. **Idempotency Protection against Duplicate Delivery**: Payment gateways routinely retry webhooks if network latency exceeds 2 seconds. By recording incoming `event_id` into `payment_webhook_events` with a `UNIQUE` constraint, duplicate webhooks are detected immediately and return HTTP 200 without double-granting credits.
4. **Clean Abstraction Boundary**: Razorpay SDK logic is encapsulated within `internal/payments/`. Adding Stripe or LemonSqueezy in the future requires implementing the same domain payment interface without altering the credit ledger or user modules.

### Downsides & Mitigation
- *Risk*: Webhooks require a public endpoint for local testing.
- *Mitigation*: In local development, webhooks can be tested via the Razorpay CLI, ngrok/localtunnel, or simulated via local integration tests using synthetic signed payloads.

## Status
Accepted.
