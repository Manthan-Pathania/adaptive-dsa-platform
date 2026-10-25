# Adaptive DSA Interview Preparation Platform

An evidence-based, adaptive interview preparation platform designed for serious software engineering candidates targeting Tier-1/FAANG companies (Google, Amazon, Meta).

The platform replaces passive checklists with an active learning feedback loop:
$$\text{Plan} \longrightarrow \text{Practise} \longrightarrow \text{Review} \longrightarrow \text{Evaluate} \longrightarrow \text{Adapt} \longrightarrow \text{Repeat}$$

---

## Current Status: Phase 2 (Identity & Security) Complete

Phase 2 establishes the core authentication, session management, role-based authorization, and administrative security foundations:
- **Password Security**: Argon2id (RFC 9106) hashing ($64\text{ MB}$ memory, $3$ iterations, $2$ parallelism, $16$-byte crypto salt) with constant-time verification.
- **Server-Side Sessions**: Opaque 32-byte cryptographically random tokens stored as SHA-256 hashes in PostgreSQL; cookies issued with `HttpOnly`, `SameSite=Lax`, and configurable `Secure`.
- **Session Rotation**: Session re-issuance upon login and privilege change; server-side invalidation upon logout.
- **CSRF Defense**: Session-bound HMAC-SHA256 Anti-CSRF protection via `X-CSRF-Token` header for state-changing operations.
- **Rate Limiting**: Thread-safe in-memory sliding window rate limiter on authentication endpoints.
- **Authorization & RBAC**: Type-safe middleware (`Authenticate`, `RequireAuth`, `RequireRole(RoleAdmin)`).
- **Secure Administrator Provisioning**: Operator CLI tool (`cmd/admin`) with zero browser exposure and permanent audit trail logging.
- **Security Audit Logging**: Append-only `audit_logs` table tracking critical identity actions with redaction of all credentials and tokens.

---

## Getting Started & Local Development

### 1. Prerequisites
- **Go**: Version 1.23 or newer (`go version`).
- **Docker & Docker Compose**: For local PostgreSQL and Redis services.
- **Git**: Version control.

### 2. Configuration Setup
Copy the environment template to create your local `.env`:
```bash
cp .env.example .env
```
Key settings in `.env`:
```dotenv
APP_ENV=development
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
DATABASE_URL=postgres://dsa_user:dsa_password@localhost:5432/dsa_db?sslmode=disable
LOG_LEVEL=info
LOG_FORMAT=text
```
*(Note: Real `.env` files are git-ignored and must never be committed).*

### 3. Start Local Infrastructure
Start PostgreSQL 16 and Redis 7 containers:
```bash
docker compose up -d
```
Verify container health:
```bash
docker compose ps
```

### 4. Run Database Migrations
Apply Goose schema migrations:
```bash
# Apply all pending migrations
go run ./cmd/migrate up

# Check migration status
go run ./cmd/migrate status

# Roll back the most recent migration (if needed)
go run ./cmd/migrate down
```

### 5. Generate Type-Safe SQL (sqlc)
Queries are defined in `internal/database/queries/*.sql` and compiled against `migrations/*.sql`:
```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate
```
Generated code outputs to `internal/database/generated/`.

### 6. Start the HTTP Server
Run the backend application:
```bash
go run ./cmd/server
```

### 7. Verify Health Endpoints
- **Liveness probe** (`/healthz`):
  ```bash
  curl -i http://localhost:8080/healthz
  # Responds HTTP 200 OK: {"status":"healthy","uptime_seconds":12,"timestamp":"..."}
  ```
- **Readiness probe** (`/readyz`):
  ```bash
  curl -i http://localhost:8080/readyz
  # When database is healthy: HTTP 200 OK {"status":"ready","checks":{"database":{"available":true,...}}}
  # When database is unreachable: HTTP 503 Service Unavailable {"status":"unavailable",...}
  ```
- **API Ping probe** (`/api/v1/ping`):
  ```bash
  curl -i http://localhost:8080/api/v1/ping
  # Responds HTTP 200 OK: {"message":"pong"}
  ```

### 8. Phase 2 Identity Endpoints

The following authentication and session management endpoints are available:

- **Register Candidate** (`POST /api/v1/auth/register`):
  ```bash
  curl -i -X POST http://localhost:8080/api/v1/auth/register \
    -H "Content-Type: application/json" \
    -d '{"email":"candidate@example.com","password":"StrongPassword123!","full_name":"Jane Doe"}'
  ```
  Returns `201 Created` with user profile, session cookie, and `csrf_token` response field.

- **Login** (`POST /api/v1/auth/login`):
  ```bash
  curl -i -X POST http://localhost:8080/api/v1/auth/login \
    -c cookies.txt \
    -H "Content-Type: application/json" \
    -d '{"email":"candidate@example.com","password":"StrongPassword123!"}'
  ```
  Sets `session_token` cookie and returns `csrf_token`.

- **Current User Profile** (`GET /api/v1/auth/me`):
  ```bash
  curl -i http://localhost:8080/api/v1/auth/me \
    -b cookies.txt
  ```
  Returns `200 OK` with user details and fresh `csrf_token`.

- **Retrieve CSRF Token** (`GET /api/v1/auth/csrf`):
  ```bash
  curl -i -c cookies.txt http://localhost:8080/api/v1/auth/csrf
  ```
  Returns `200 OK` with `csrf_token` in response body and sets a `csrf_token` cookie for pre-authentication Double-Submit verification.

- **Logout** (`POST /api/v1/auth/logout`):
  ```bash
  curl -i -X POST http://localhost:8080/api/v1/auth/logout \
    -b cookies.txt \
    -H "X-CSRF-Token: <csrf-token-from-login>"
  ```
  Invalidates session server-side within an atomic database transaction and clears cookies.

### 9. Secure Administrator Provisioning

Administrator accounts cannot be registered via public APIs. To provision an operator or administrator, run the operator CLI:
```bash
go run ./cmd/admin create \
  --email admin@adaptive-dsa.com \
  --name "Platform Admin"
```
Or with the compiled binary:
```bash
./bin/admin.exe create --email admin@adaptive-dsa.com --name "Platform Admin"
```

**Security Design**:
- **Zero Secret Exposure**: The command intentionally rejects any `--password` command-line argument to protect credentials from process monitoring tools (`ps`, Task Manager) and shell history files.
- **Interactive Masked Entry**: In an interactive terminal, the operator is prompted twice for the password without terminal echo (`term.ReadPassword`) with constant-time equality validation.
- **Scripted Stdin Pipe**: In non-interactive CI or automation environments, the password can be piped directly via standard input (e.g. `"MyStrongPass123!" | ./bin/admin.exe create --email ... --name ...`).
- **Atomic Persistence**: Creation of the administrator user and the mandatory `ADMIN_USER_PROVISIONED` audit log record execute within an atomic PostgreSQL transaction (`ExecTx`). If audit log insertion fails, user creation is automatically rolled back.
- **Audit Immutability**: Immutability of the `audit_logs` table is enforced at the application layer via append-only queries without any update or deletion statements.

---

## Question Bank Importer CLI (Phase 3)

The platform provides a dedicated, production-safe CLI tool for importing curated DSA spreadsheets into the PostgreSQL question bank.

### Usage

```bash
# 1. Run a Dry Run validation (validates schema, duplicates, and constraints without database writes)
go run ./cmd/import-questions --file ./testdata/sample_questions.xlsx --dry-run

# Or using the compiled binary
./bin/import-questions.exe --file path/to/questions.xlsx --dry-run

# 2. Perform a Real Transactional Import
go run ./cmd/import-questions --file ./testdata/sample_questions.xlsx --commit

# 3. Production Import (requires explicit --confirm-prod safeguard when APP_ENV=production)
./bin/import-questions.exe --file path/to/questions.xlsx --commit --confirm-prod
```

### Key Capabilities
- **Strict Contract Validation**: Validates all required columns per [`docs/QUESTION_BANK_IMPORT.md`](file:///docs/QUESTION_BANK_IMPORT.md) (Topic, Pattern, Question Name, LeetCode / Practice URL, Difficulty, Core Techniques, Time/Space Complexity).
- **Offline Dry Run**: Can validate spreadsheets against the catalog contract even when PostgreSQL is offline.
- **Idempotent Upsert**: Questions are identified deterministically by URL and slug. Repeated imports update existing metadata without duplicating records or resetting user progress.
- **Atomic Rollback**: Real imports run in an isolated transaction. If any database constraint fails, the entire batch is rolled back cleanly.
- **Production Safeguard**: When running with `APP_ENV=production`, the command requires `--confirm-prod` to prevent accidental execution.

---

## Preparation Engine & Progress API (Phase 3)

### Catalog Endpoints (Public / Authenticated)
- `GET /api/v1/topics` — List all DSA topics and their sub-patterns.
- `GET /api/v1/questions?topic_id={uuid}&difficulty=MEDIUM&search=binary` — Filter questions with pagination.
- `GET /api/v1/questions/{id}` — Retrieve detailed question specifications, patterns, and prerequisites.
- `GET /api/v1/roadmaps` — List all curated learning roadmaps (e.g., Blind 75, NeetCode 150).
- `GET /api/v1/roadmaps/{id}` — Retrieve a roadmap and its ordered list of questions.

### User Progress & Spaced Repetition (Authenticated)
- `POST /api/v1/questions/{id}/attempts` — Record a problem-solving attempt (outcome, solving time, quality rating 0–5, notes). Computes SuperMemo SM-2 interval.
- `GET /api/v1/questions/{id}/progress` — Retrieve user's canonical progress and attempt history for a question.
- `GET /api/v1/progress` — Retrieve user's global mastery stats across topics and difficulties.
- `GET /api/v1/revisions/due` — Retrieve questions scheduled for spaced revision today.

---

## Testing & Quality Gates

Run all automated unit tests:
```bash
# Run all unit tests
go test -v ./...

# Run static analysis
go vet ./...

# Check formatting
gofmt -s -l .

# Run tests with HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

---

## Project Structure

```text
adaptive-dsa/
├── cmd/
│   ├── server/             # Main HTTP application entrypoint
│   │   └── main.go
│   ├── migrate/            # Goose migration runner CLI
│   │   └── main.go
│   ├── admin/              # Operator administrator provisioning CLI
│   │   └── main.go
│   └── import-questions/   # Phase 3 Question Bank Excel Importer CLI
│       └── main.go
├── internal/
│   ├── config/             # Typed, validated environment configuration
│   ├── platform/           # Shared technical infrastructure
│   │   ├── database/       # pgxpool setup, health checking, connection bounds
│   │   ├── httperr/        # RFC 9457 Problem Details error formatting
│   │   ├── logger/         # log/slog structured logging with request ID context
│   │   └── middleware/     # Request ID, HTTP logger, panic recoverer
│   ├── identity/           # Phase 2 Domain, Argon2id, Sessions, CSRF, RBAC, Handlers
│   ├── catalog/            # Phase 3 Question Catalog, Topics, Roadmaps, Excel Importer
│   │   ├── domain.go       # Core entities, URL normalization, slugification
│   │   ├── repository.go   # Catalog repository & pgxpool / in-memory implementations
│   │   ├── service.go      # Catalog domain logic and retrieval orchestration
│   │   ├── handler.go      # HTTP handlers for /api/v1/questions, /topics, /roadmaps
│   │   └── importer/       # Excelize parser, contract validator, batch loader
│   ├── progress/           # Phase 3 Attempts, Progress, SuperMemo SM-2 Scheduling
│   │   ├── domain.go       # Progress models, attempt enums, SM-2 algorithm
│   │   ├── repository.go   # Progress & attempt persistence (pgxpool / in-memory)
│   │   ├── service.go      # Atomic attempt logging and revision calculation
│   │   └── handler.go      # HTTP handlers for /api/v1/progress, /attempts, /revisions
│   ├── server/             # HTTP server setup, router, route mounting
│   └── database/           # sqlc query files & generated Go code
│       ├── queries/
│       │   ├── heartbeat.sql
│       │   ├── identity.sql
│       │   ├── catalog.sql     # Phase 3 Catalog SQL queries
│       │   └── progress.sql    # Phase 3 Progress & SM-2 SQL queries
│       └── generated/      # sqlc type-safe compiled database code
├── migrations/             # Goose SQL migration scripts
│   ├── 00001_initial_infrastructure.sql
│   ├── 00002_identity_and_sessions.sql
│   └── 00003_core_preparation_engine.sql
├── testdata/               # Excel question bank contract fixture
│   └── sample_questions.xlsx
├── docs/                   # Architecture, specifications, and ADRs
│   ├── PRODUCT_SPEC.md
│   ├── ARCHITECTURE.md
│   ├── DATABASE_DESIGN.md
│   ├── SECURITY.md
│   ├── TESTING.md
│   ├── DEPLOYMENT.md
│   ├── IMPLEMENTATION_PLAN.md
│   ├── AI_AND_CREDIT_MODEL.md
│   ├── QUESTION_BANK_IMPORT.md
│   └── decisions/          # Architecture Decision Records (ADRs 0001–0006)
├── .github/workflows/      # GitHub Actions CI quality gates
│   └── ci.yml
├── .env.example            # Environment configuration template
├── .gitignore              # Excludes secrets, binaries, test artifacts
├── docker-compose.yml      # Local PostgreSQL 16 and Redis 7 definitions
├── Makefile                # Common engineering tasks
├── sqlc.yaml               # sqlc PostgreSQL code generation configuration
├── go.mod                  # Go module definition (Go 1.23+)
└── go.sum                  # Cryptographic checksums
```

---

## Platform Notes & Verification (Phase 3)
1. **Spreadsheet Status**: The operator's proprietary 600–700 question spreadsheet is external to the repository. The importer runs against the documented specification in [`docs/QUESTION_BANK_IMPORT.md`](file:///docs/QUESTION_BANK_IMPORT.md) and the reference fixture `testdata/sample_questions.xlsx`.
2. **Local Environment & CI**: The local git repository currently has no remote origin configured, and Docker Desktop is not installed on the local Windows host. All unit tests, contract validations, SM-2 tests, and HTTP server tests pass 100% locally. Full PostgreSQL integration tests run in GitHub Actions once a remote origin is configured.
3. **Out of Scope for Phase 3**: Frontend UI, React/Next.js components, AI tutoring with Gemini, AI credit transactions, payment processing (Razorpay), and production cloud deployments are scheduled for Phases 4 through 6.
