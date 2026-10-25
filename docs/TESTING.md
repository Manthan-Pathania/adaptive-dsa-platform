# Testing Strategy & Quality Gates — Adaptive DSA Interview Preparation Platform

## 1. Testing Philosophy

Testing is an integral component of the development lifecycle, not an afterthought. The platform's automated test suite is designed around three non-negotiable principles:
1. **Deterministic Test Execution**: Tests must never fail due to network hiccups, third-party API downtime, or external rate limits. Live Gemini or Razorpay services are **never** invoked during standard test runs.
2. **Real PostgreSQL in Integration Tests**: Mocking database drivers hides real-world bugs (e.g., syntax errors, foreign key violations, locking bugs, type coercion differences). All database tests run against a real PostgreSQL instance.
3. **High-Value Quality Gates**: Every Pull Request must pass static analysis, race condition detection, unit tests, and integration tests before merging.

---

## 2. The Testing Pyramid

```mermaid
graph BT
    E2E["End-to-End Tests<br/>(Playwright: Critical user flows, auth, log attempt, dashboard)"]
    API["API & Handler Tests<br/>(httptest: RFC 9457 errors, auth middleware, request validation)"]
    Integration["Integration Tests<br/>(Real PostgreSQL & Redis: sqlc queries, transactions, webhooks)"]
    Unit["Unit Tests<br/>(Pure Go: SM-2 algorithm, credit ledger invariants, token estimators)"]

    Unit --> Integration
    Integration --> API
    API --> E2E
```

| Layer | Target Scope | Tooling & Frameworks | Execution Speed |
| :--- | :--- | :--- | :--- |
| **Unit Tests** | Domain entities, SM-2 spaced repetition math, credit validation, mistake classification rules. | Standard Go `testing`, `testify/assert`. | < 2 seconds |
| **Integration Tests** | `sqlc` queries, PostgreSQL constraints, database transactions, Razorpay webhook idempotency, fake AI provider. | `pgxpool`, real PostgreSQL container (or local test DB). | 5–15 seconds |
| **API Tests** | HTTP routing, authentication middleware, CSRF validation, RFC 9457 Problem Details error responses. | `net/http/httptest`, `chi/v5`. | 5–10 seconds |
| **E2E Tests** | User registration, onboarding flow, logging problem attempt, viewing revision queue, mock checkout. | Playwright (TypeScript) against Next.js + Go backend. | 30–60 seconds |

---

## 3. Critical Test Scenarios & Suites

### 3.1. Revision & Mastery Engine (Unit Tests)
- **Independence Impact**:
  - Test that solving a problem with `SOLVED_INDEPENDENTLY` yields an interval progression significantly greater than `SOLVED_WITH_HINTS`.
  - Test that `SOLVED_AFTER_EDITORIAL` resets repetition count to 0 and interval to 1 day.
- **Mistake Decay**:
  - Test that severe algorithmic mistake tags (`FAILED_PATTERN_RECOGNITION`) reduce memory stability more than syntax errors.
- **Anti-Backlog Algorithm**:
  - Verify that when a user is 14 days overdue on 20 items, the triage function selects top-priority decayed core problems and spaces the remainder without exceeding the daily study time budget.

### 3.2. Credit Ledger & Accounting Invariants (Integration Tests)
- **Positive Balance Constraint**:
  - Attempting to deduct 10 credits from an account with 5 credits must fail at the application layer with `ErrInsufficientCredits` and must be blocked at the database layer by `chk_positive_balance`.
- **Atomic Balance Consistency**:
  - Insert 50 credits (PURCHASE), deduct 10 credits (AI_USAGE), and add 5 credits (PROMO).
  - Verify that `credit_accounts.cached_balance` strictly equals `SELECT SUM(amount) FROM credit_ledger_entries WHERE user_id = $1`.
- **Concurrent Deduction Race Condition**:
  - Spawn 10 concurrent goroutines attempting to deduct 5 credits each from an initial balance of 20 credits.
  - Verify that exactly 4 succeed, 6 fail with `ErrInsufficientCredits`, and final balance is exactly 0 (no negative balance, no phantom reads).

### 3.3. Payment Processing & Razorpay Webhooks (Integration Tests)
- **Valid Webhook & Credit Grant**:
  - Simulate valid Razorpay `payment.captured` event with valid HMAC-SHA256 signature.
  - Assert HTTP 200, order marked `PAID`, transaction logged, and credit ledger entry created.
- **Tampered / Invalid Signature**:
  - Send valid payload with invalid signature header.
  - Assert HTTP 400 Bad Request, zero credit grant, error logged.
- **Idempotent Webhook Replay**:
  - Send the exact same webhook payload twice consecutively.
  - Assert both return HTTP 200, but the credit ledger contains **exactly one** grant entry.
- **Concurrent Webhook Delivery**:
  - Dispatch 5 concurrent requests with identical `event_id`.
  - Assert database unique constraint prevents duplicate rows and credits are granted exactly once.
- **Amount & Currency Mismatch**:
  - Tamper with payload amount (order expected 500 cents, payload has 100 cents).
  - Assert transaction is flagged `DISPUTED`, rejected, and zero credits granted.

### 3.4. AI Provider & Resilience Tests (Fake Provider)
- **Timeout & Graceful Degradation**:
  - Configure fake provider with simulated 5-second delay.
  - Assert application context timeout triggers at 3 seconds, returns HTTP 503, and **no user credits are deducted**.
- **Quota Limit (429 Too Many Requests)**:
  - Simulate Gemini free-tier 429 response.
  - Assert friendly RFC 9457 error message indicating rate limit, with zero credit deduction.
- **Malformed Provider JSON**:
  - Provider returns non-JSON or truncated response.
  - Assert system handles parse failure gracefully without crashing.

### 3.5. Security & Ownership Tests
- **Cross-Tenant Access**:
  - Candidate A attempts to GET/UPDATE attempt or note belonging to Candidate B.
  - Assert HTTP 404 Not Found (resource isolation).
- **Admin Privilege Escalation**:
  - Candidate attempts to access `/api/v1/admin/users`.
  - Assert HTTP 403 Forbidden.
- **Session Expiry & Revocation**:
  - User logs out; old session token is immediately invalid on subsequent requests.

---

## 4. Test Environment Setup & Execution

### Local Test Database Setup
Integration tests run against a dedicated test database to avoid polluting local development data:
```bash
# Start test Postgres instance
docker run -d --name dsa-test-db -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=dsa_test -p 5433:5432 postgres:16-alpine

# Run database migrations on test database
goose -dir migrations postgres "postgres://postgres:postgres@localhost:5433/dsa_test?sslmode=disable" up
```

### Command Execution Reference

```bash
# 1. Run pure unit tests (fast, no DB required)
go test -v -short ./internal/...

# 2. Run all tests including integration tests with race detector
go test -v -race -tags=integration ./...

# 3. Generate test coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# 4. Run static analysis & linting
golangci-lint run ./...

# 5. Run frontend type checking & tests
cd web && npm run lint && npm run typecheck
```

---

## 5. CI/CD Quality Gates (GitHub Actions Workflow)

Every Pull Request must pass the following pipeline before merging:

```mermaid
flowchart LR
    Lint["1. Lint & Format<br/>(golangci-lint, eslint)"]
    Vuln["2. Security Scan<br/>(govulncheck, npm audit)"]
    Unit["3. Unit Tests<br/>(go test -short)"]
    Integration["4. Integration Tests<br/>(PostgreSQL service container)"]
    Build["5. Build Verification<br/>(go build, next build)"]

    Lint --> Vuln
    Vuln --> Unit
    Unit --> Integration
    Integration --> Build
```

### Pull Request Acceptance Criteria
1. `golangci-lint` passes with zero warnings.
2. `govulncheck` reports zero known vulnerabilities.
3. Unit and integration tests pass with 100% success rate.
4. Go race detector (`-race`) completes cleanly with zero race warnings.
5. All database migrations apply and rollback cleanly (`goose up` followed by `goose down`).
6. TypeScript builds without type errors (`tsc --noEmit`).
