# System Architecture — Adaptive DSA Interview Preparation Platform

## 1. Architectural Style & Philosophy

The platform is designed as a **Go Modular Monolith** coupled with a modern **Next.js frontend**. It follows clean, inward-pointing architectural boundaries (Hexagonal / Ports and Adapters).

### Core Principles
1. **Inward-Pointing Dependencies**:
   - `HTTP Handlers / Transports` $\rightarrow$ `Application Services (Use Cases)` $\rightarrow$ `Domain Entities & Invariants`.
   - Domain logic and business rules never import database drivers, HTTP frameworks, or external SDKs.
   - Application services define repository and provider interfaces; infrastructure packages implement them.
2. **Single Deployable Unit**:
   - The entire backend compiles into a single, highly efficient Go binary with minimal memory footprint (< 50MB RSS under normal load), avoiding the operational overhead, network latency, and distributed transaction complexity of microservices.
3. **Explicit Dependency Injection**:
   - Components receive their dependencies explicitly via Go struct constructors (e.g., `NewService(repo, aiProvider, logger)`).
   - No magic dependency-injection reflection frameworks are used.
4. **Authoritative PostgreSQL Source of Truth**:
   - PostgreSQL 16 is the single source of truth for all transactional data, progress state, credit ledgers, and audit logs.
   - Redis is used strictly for transient concerns: distributed rate limiting, session caching/token blacklisting, and background job queues.

---

## 2. High-Level System Architecture

```mermaid
graph TB
    subgraph Client["Client Tier"]
        Browser["Next.js Web Client<br/>(TypeScript, Tailwind CSS, shadcn/ui)"]
    end

    subgraph ReverseProxy["Edge / Reverse Proxy Tier"]
        Caddy["Caddy Server<br/>(Automatic TLS, Compression, Route Proxy)"]
    end

    subgraph BackendApp["Go Modular Monolith (/api/v1)"]
        Router["HTTP Router & Middleware<br/>(chi / stdlib, auth, slog, rate-limit, CORS)"]

        subgraph Modules["Domain & Application Modules"]
            IdentityMod["Identity & Auth"]
            CatalogMod["Question Catalog"]
            RoadmapMod["Roadmaps & Planning"]
            ProgressMod["Progress & Attempts"]
            RevisionMod["Revision & Spaced Repetition"]
            AIMod["AI Orchestration"]
            BillingMod["Credit Ledger & Billing"]
            PaymentsMod["Payments (Razorpay)"]
            AdminMod["Admin & Audit"]
        end

        subgraph Adapters["Infrastructure Adapters"]
            PGAdapter["PostgreSQL Adapter<br/>(pgxpool + sqlc)"]
            RedisAdapter["Redis Adapter<br/>(Rate limits, queues)"]
            GeminiAdapter["Gemini Client<br/>(Google GenAI SDK)"]
            RazorpayAdapter["Razorpay Client<br/>(HMAC Signature Verification)"]
        end
    end

    subgraph DataTier["Data & External Services Tier"]
        PostgresDB[(PostgreSQL 16<br/>ACID Store & Ledger)]
        RedisDB[(Redis 7<br/>Cache & Rate Limits)]
        GeminiAPI["Google Gemini API"]
        RazorpayAPI["Razorpay Payment Gateway"]
    end

    Browser -->|HTTPS| Caddy
    Caddy -->|Reverse Proxy /api/v1| Router
    Caddy -->|Reverse Proxy /| Browser

    Router --> Modules
    Modules --> Adapters
    PGAdapter --> PostgresDB
    RedisAdapter --> RedisDB
    GeminiAdapter --> GeminiAPI
    RazorpayAdapter --> RazorpayAPI
```

---

## 3. Go Project Structure (Standard Layout)

The project adheres to the idiomatic Go project layout:

```text
adaptive-dsa/
├── cmd/
│   ├── server/             # Main HTTP API application entrypoint
│   │   └── main.go
│   ├── migrate/            # CLI tool for executing Goose database migrations
│   │   └── main.go
│   └── import-questions/   # CLI tool for validated question spreadsheet import
│       └── main.go
├── internal/               # Private application code (cannot be imported externally)
│   ├── config/             # Environment variable parsing and validation
│   ├── platform/           # Shared technical infrastructure
│   │   ├── database/       # pgxpool setup, transaction helpers, health checks
│   │   ├── redis/          # Redis connection and rate-limit helpers
│   │   ├── logger/         # log/slog initialization with request correlation
│   │   ├── httperr/        # RFC 9457 Problem Details error response builders
│   │   └── middleware/     # Auth, session, request ID, CORS, recovery, rate-limit
│   ├── identity/           # User registration, Argon2id passwords, sessions, RBAC
│   │   ├── domain.go       # User, Session, Role entities
│   │   ├── service.go      # Register, Login, RevokeSession use cases
│   │   ├── repository.go   # Storage interface definition
│   │   └── handler.go      # HTTP handlers (/api/v1/auth/*)
│   ├── catalog/            # Curated questions, patterns, company evidence
│   ├── roadmap/            # Roadmaps, stage progression, personalization logic
│   ├── progress/           # User attempts, canonical question progress, notes
│   ├── revision/           # SM-2 / spaced repetition engine, overdue scheduling
│   ├── ai/                 # Gemini provider adapter, prompt templates, token counting
│   ├── billing/            # Credit accounts, double-entry immutable ledger
│   ├── payments/           # Razorpay order generation, webhook signature verification
│   ├── admin/              # User management, catalog edits, audit logs
│   └── database/           # sqlc generated code and SQL queries
│       ├── queries/        # Raw parameterized SQL queries
│       │   ├── users.sql
│       │   ├── catalog.sql
│       │   ├── progress.sql
│       │   ├── ledger.sql
│       │   └── payments.sql
│       └── generated/      # sqlc compiled Go models and query functions
├── migrations/             # Goose SQL migration files (00001_init.sql, etc.)
├── pkg/                    # Reusable public packages (if applicable)
├── docs/                   # Architecture, specifications, and ADRs
├── web/                    # Next.js frontend application
│   ├── src/
│   │   ├── app/            # Next.js App Router pages and layouts
│   │   ├── components/     # UI components (shadcn/ui, modals, charts)
│   │   ├── lib/            # Typed API client, auth session context
│   │   └── types/          # Shared TypeScript interfaces
├── Dockerfile              # Multi-stage production build for Go server
├── docker-compose.yml      # Local dev environment (Go, Postgres, Redis, Caddy)
└── Makefile                # Common engineering tasks (build, test, lint, migrate)
```

---

## 4. Module Boundaries & Interaction Contracts

Each module maintains strict encapsulation. Cross-module calls occur either through explicit application service interfaces or database transactions orchestrated by a coordinator service.

| Module | Core Responsibility | External Dependencies | Invariants Enforced |
| :--- | :--- | :--- | :--- |
| **`identity`** | User credentials, Argon2id hashing, server sessions, RBAC. | PostgreSQL, Redis (session cache). | Passwords hashed with Argon2id; session tokens cryptographically random 32-byte strings; email unique. |
| **`catalog`** | Canonical questions, patterns, subpatterns, company evidence. | PostgreSQL. | Canonical question identity is global; URLs/slugs are unique; company tags require source metadata. |
| **`roadmap`** | Master and personalized study roadmaps. | Catalog module. | Roadmaps reference canonical questions; removing a roadmap item never deletes user progress. |
| **`progress`** | User attempts, current question progress, notes, bookmarks. | Catalog module, PostgreSQL. | Exactly **one** `user_question_progress` row per `(user_id, question_id)`. Attempts are immutable logs. |
| **`revision`** | Spaced repetition engine, dynamic intervals, overdue triage. | Progress module, PostgreSQL. | Overdue items triaged by retention decay; intervals adapt to independent solving outcome. |
| **`ai`** | Pedagogical hints, approach reviews, prompt sanitization. | Gemini API, Billing module. | Request payload bounded; system prompt isolated; zero credit charge on provider timeout/error. |
| **`billing`** | Credit accounts, credit packs, double-entry immutable ledger. | PostgreSQL. | All balances represented as integers; every balance modification backed by an immutable ledger row. |
| **`payments`** | Razorpay order creation, webhook verification, idempotency. | Billing module, Razorpay API. | Webhook HMAC-SHA256 signature verified; idempotency key prevents duplicate credit grants; atomic execution. |
| **`admin`** | User moderation, catalog management, audit logging. | All modules, PostgreSQL. | All admin mutations record immutable audit log with actor ID, target ID, and payload delta. |

---

## 5. Critical Request Flows

### A. Logging a Practice Attempt & Updating Revision Schedule

```mermaid
sequenceDiagram
    autonumber
    actor Candidate as Candidate (Browser)
    participant Handler as Progress Handler
    participant Svc as Progress Service
    participant RevSvc as Revision Engine
    participant DB as PostgreSQL (Transaction)

    Candidate->>Handler: POST /api/v1/questions/{id}/attempts<br/>{outcome: SOLVED_INDEPENDENTLY, time: 25m, mistakes: [...]}
    Handler->>Svc: RecordAttempt(ctx, cmd)
    Svc->>DB: BEGIN Transaction
    Svc->>DB: INSERT INTO question_attempts (...)
    Svc->>DB: UPSERT INTO user_question_progress (...)<br/>(Update status, solve_count, last_solved_at)
    Svc->>RevSvc: CalculateNextReview(history, outcome, time)
    RevSvc-->>Svc: NextReviewSchedule(interval_days, stability, next_due)
    Svc->>DB: UPSERT INTO revision_schedules (...)<br/>(Store updated stability, next_review_due)
    Svc->>DB: COMMIT Transaction
    Svc-->>Handler: AttemptRecordedResponse
    Handler-->>Candidate: HTTP 201 Created (JSON Problem Details on err)
```

### B. Progressive AI Hint Request with Credit Reservation

```mermaid
sequenceDiagram
    autonumber
    actor Candidate as Candidate (Browser)
    participant AIHandler as AI Handler
    participant AISvc as AI Service
    participant BillingSvc as Billing Service
    participant DB as PostgreSQL
    participant Gemini as Google Gemini API

    Candidate->>AIHandler: POST /api/v1/ai/hints<br/>{question_id: "...", hint_level: 2}
    AIHandler->>AISvc: RequestHint(ctx, req)
    AISvc->>BillingSvc: CheckAndReserveCredits(ctx, user_id, required_credits: 5)
    BillingSvc->>DB: SELECT cached_balance FROM credit_accounts WHERE user_id = $1 FOR UPDATE
    alt Insufficient Credits
        BillingSvc-->>AISvc: ErrInsufficientCredits
        AISvc-->>AIHandler: ErrInsufficientCredits
        AIHandler-->>Candidate: HTTP 402 Payment Required
    else Sufficient Credits
        BillingSvc-->>AISvc: ReservationOK
        AISvc->>Gemini: GenerateContent(Prompt with Context & Level)
        alt Gemini Error / Timeout
            Gemini-->>AISvc: Error (504 Timeout)
            AISvc->>BillingSvc: CancelReservation(user_id)
            AISvc-->>AIHandler: ErrAIProviderUnavailable
            AIHandler-->>Candidate: HTTP 503 Service Unavailable (Zero credits charged)
        else Gemini Success
            Gemini-->>AISvc: Hint Content
            AISvc->>BillingSvc: CommitCreditDeduction(ctx, user_id, credits: 5, ref: "ai_hint")
            BillingSvc->>DB: INSERT INTO credit_ledger_entries (-5, "USAGE", ...)<br/>UPDATE credit_accounts SET cached_balance = cached_balance - 5
            AISvc->>DB: INSERT INTO ai_requests (tokens, latency, status: SUCCESS)
            AISvc-->>AIHandler: HintResponse
            AIHandler-->>Candidate: HTTP 200 OK {hint: "...", remaining_credits: 45}
        end
    end
```

### C. Razorpay Webhook Processing with Idempotency & Atomic Credit Grant

```mermaid
sequenceDiagram
    autonumber
    participant Razorpay as Razorpay Webhook Engine
    participant PayHandler as Payment Webhook Handler
    participant PaySvc as Payment Service
    participant DB as PostgreSQL (Serializable / Row-Locked)

    Razorpay->>PayHandler: POST /api/v1/payments/webhook<br/>Header: X-Razorpay-Signature, Body: payload
    PayHandler->>PaySvc: ProcessWebhook(ctx, signature, raw_payload)
    PaySvc->>PaySvc: VerifyHMAC(raw_payload, secret, signature)
    alt Invalid Signature
        PaySvc-->>PayHandler: ErrInvalidSignature
        PayHandler-->>Razorpay: HTTP 400 Bad Request
    else Signature Valid
        PaySvc->>DB: BEGIN Transaction
        PaySvc->>DB: INSERT INTO payment_webhook_events (event_id, ...)<br/>ON CONFLICT (event_id) DO NOTHING RETURNING id
        alt Duplicate Event (id is nil)
            PaySvc->>DB: ROLLBACK
            PaySvc-->>PayHandler: EventAlreadyProcessed
            PayHandler-->>Razorpay: HTTP 200 OK (Acknowledge duplicate)
        else New Event
            PaySvc->>DB: SELECT * FROM payment_orders WHERE provider_order_id = $1 FOR UPDATE
            PaySvc->>DB: UPDATE payment_orders SET status = 'PAID'
            PaySvc->>DB: INSERT INTO payment_transactions (...)
            PaySvc->>DB: INSERT INTO credit_ledger_entries (user_id, +amount, 'PURCHASE', ...)
            PaySvc->>DB: UPDATE credit_accounts SET cached_balance = cached_balance + pack_credits
            PaySvc->>DB: UPDATE payment_webhook_events SET status = 'PROCESSED'
            PaySvc->>DB: COMMIT Transaction
            PaySvc-->>PayHandler: ProcessedSuccessfully
            PayHandler-->>Razorpay: HTTP 200 OK
        end
    end
```

---

## 6. Cross-Cutting Technical Concerns

### Structured Logging (`log/slog`)
- Log entries are output as JSON in production and structured text in local development.
- Middleware injects a unique `request_id` (UUIDv7 or ULID) into the Go `context.Context` and every log line.
- Sensitive fields (passwords, session cookies, payment signatures, card data, AI API keys) are **strictly masked or omitted**.

### Error Handling: RFC 9457 Problem Details
All API errors return a standard JSON payload:
```json
{
  "type": "https://api.adaptivedsa.dev/errors/insufficient-credits",
  "title": "Insufficient Credits",
  "status": 402,
  "detail": "Your balance of 2 credits is insufficient for this operation (requires 5 credits).",
  "instance": "/api/v1/ai/hints",
  "code": "INSUFFICIENT_CREDITS"
}
```

### Context Propagation & Graceful Shutdown
- Every HTTP request passes `r.Context()` directly to database queries and external HTTP calls.
- Database queries enforce bounded timeouts (e.g., 3s default, 15s for heavy report queries).
- On `SIGINT`/`SIGTERM`, the Go server stops accepting new connections, allows in-flight requests 10 seconds to finish, closes the `pgxpool` connection pool cleanly, and flushes log buffers.

### Database Connection Pooling (`pgxpool`)
Configured to prevent connection exhaustion while ensuring high concurrency:
- `MaxConns`: 25 (calibrated for modest VPS with 2–4 CPU cores).
- `MinConns`: 5 (keeps warm connections ready).
- `MaxConnLifetime`: 30 minutes.
- `MaxConnIdleTime`: 5 minutes.
- `HealthCheckPeriod`: 1 minute.

---

## 7. Frontend Architecture (Next.js & TypeScript)

- **Framework**: Next.js 14+ (App Router).
- **Styling**: Tailwind CSS with shadcn/ui components (Radix UI primitives).
- **Architecture**:
  - *Server Components (RSC)*: Used for static landing pages, catalog search views, and read-only roadmap displays.
  - *Client Components (`"use client"`)*: Used for interactive attempt-logging modals, real-time stopwatch timers, hint reveal accordions, and checkout triggers.
  - *Typed API Client*: Generated or hand-crafted TypeScript SDK that guarantees 1:1 typing with Go API models and handles cookie-based authentication automatically.
