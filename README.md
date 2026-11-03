# Adaptive DSA Interview Preparation Platform

An evidence-based, adaptive interview preparation platform designed for software engineering candidates targeting Tier-1/FAANG companies (Google, Amazon, Meta).

The platform replaces static checklists with an active adaptive learning feedback loop:
$$\text{Plan} \longrightarrow \text{Practise} \longrightarrow \text{Review} \longrightarrow \text{Evaluate} \longrightarrow \text{Adapt} \longrightarrow \text{Repeat}$$

---

## Release 1 Status: Complete & Verified

Release 1 provides a complete end-to-end preparation workspace running locally via Docker Compose on Windows, WSL 2, and Docker Desktop.

### Key Capabilities

1. **Curated Master Question Bank (§ 3)**:
   - 539 active, quality-controlled questions imported from `FAANG_DSA_Master_Plan_v2.xlsx`.
   - Categorized by topic, algorithmic pattern, subpattern, difficulty, canonical LeetCode ID, and verified company tags.
2. **Unified Admin Command Center (§ 4)**:
   - Web interface at `/admin` for bulk spreadsheet import, dry-run validation, duplicate detection, question CRUD, and soft-archival.
3. **Structured Candidate Onboarding (§ 5)**:
   - 6-step wizard at `/onboarding` guiding candidates through goal setting, target companies, timelines, daily availability, and optional diagnostic assessments.
4. **Master & Company-Curated Roadmaps (§ 6)**:
   - 5 pre-seeded curricula in PostgreSQL: Master DSA (29 Patterns), Blind 75, Google, Meta, and Amazon tracks at `/roadmaps`.
5. **Canonical Question Identity & Shared Progress (§ 7)**:
   - Candidate progress is keyed strictly on `(user_id, question_id)`. Solving a problem in one roadmap instantly reflects across all roadmaps containing that problem.
6. **Practice Workspace & Historical Attempts (§ 8)**:
   - Focused practice workspace at `/questions/[id]` with attempt timers, outcome tracking (`SOLVED_INDEPENDENTLY`, `HINTS_USED`, `EDITORIAL_USED`, `FAILED`), mistake tags, personal notes, and code snippets.
7. **Daily Preparation Dashboard (§ 9, § 10)**:
   - Actionable central workspace at `/dashboard` answering what to study today, which revisions are due, where weaknesses lie, and goal progress.
8. **SM-2 Spaced Repetition Engine (§ 11)**:
   - Automated review scheduling based on attempt outcome, time spent, and ease factor. Dedicated revision queue at `/revisions`.
9. **Adaptive Stages & Evidence-Based Mastery (§ 12, § 13)**:
   - Stage 1 (Foundations), Stage 2 (Strengthening), Stage 3 (Consolidation). Mastery requires independent derivation and successful spaced revision.
10. **AI Coach & Progressive Hints (§ 14)**:
    - 4-level progressive hint ladder, approach review, and concept explanations. Zero charge on failed requests. Supported mock AI mode for offline local evaluation.
11. **Immutable Credit Ledger & Wallet (§ 15)**:
    - Double-entry signed ledger. Every new candidate receives **50 welcome credits** atomically upon registration.
12. **Razorpay Mock Payment Gateway (§ 16)**:
    - Fully functional mock checkout flow for Starter (50cr / ₹99), Pro (150cr / ₹249), and Master (400cr / ₹599) packages with idempotent fulfillment.
13. **Data Portability (§ 19)**:
    - Complete JSON export of candidate attempts, progress records, and SM-2 schedules via `/dashboard`.

---

## Local Evaluation Quickstart (Docker Compose)

The application runs locally using Docker Compose without requiring external cloud accounts or paid third-party API keys.

### 1. Start the Complete Stack
From the project root:
```powershell
docker compose -f docker-compose.local.yml up -d
```

### 2. Verify Container Health
```powershell
docker compose -f docker-compose.local.yml ps
```
Expected output:
| Name | Service | Status | Ports |
| :--- | :--- | :--- | :--- |
| `adaptive_dsa_local_pg` | PostgreSQL 16 | Up (healthy) | `0.0.0.0:5432->5432` |
| `adaptive_dsa_local_backend` | Go Monolith API | Up (healthy) | `0.0.0.0:8080->8080` |
| `adaptive_dsa_local_frontend` | Next.js 16 Web | Up (healthy) | `0.0.0.0:3000->3000` |
| `adaptive_dsa_local_caddy` | Caddy 2 Proxy | Up | `0.0.0.0:80->80` |

### 3. Open the Web Application
Open your browser to:
- **Frontend Direct**: `http://localhost:3000`
- **Edge Reverse Proxy**: `http://localhost`

---

## Default Credentials & Admin Access

### Pre-Seeded Accounts
- **Administrator**:
  - Email: `admin@adaptivedsa.local`
  - Password: `AdminPassword123!`
  - Access: Full access to `/admin` (Spreadsheet Importer, Question Bank Management, Audited Credit Grants).
- **Candidate Account**:
  - Navigate to `http://localhost:3000/register`.
  - Enter name, email, and password.
  - Automatically redirected to `/onboarding` with **50 welcome credits** credited to your wallet.

### Manual Admin Provisioning CLI
To provision additional administrators securely:
```powershell
# Interactive (prompts for masked password without terminal echo)
go run ./cmd/admin create --email newadmin@adaptivedsa.local --name "Security Officer"

# Non-interactive / scripted pipe
"AdminPassword123!" | go run ./cmd/admin create --email newadmin@adaptivedsa.local --name "Security Officer"
```

---

## Authoritative AI Pricing Contract

All pricing and balances are strictly enforced by the backend (`internal/ai/domain.go`):

| AI Operation | Authoritative Credit Cost | Description |
| :--- | :--- | :--- |
| **Level 1: Intuition Nudge** | **2 Credits** | High-level mental model nudge without algorithmic specifics. |
| **Level 2: Structural Pattern** | **2 Credits** | Identifies relevant algorithmic pattern (e.g. Fast & Slow Pointers). |
| **Level 3: Loop Invariant** | **4 Credits** | Key mathematical invariant or core state transition condition. |
| **Level 4: Pseudocode Walkthrough** | **4 Credits** | Step-by-step algorithmic breakdown without giving exact syntax. |
| **Approach Review** | **5 Credits** | Analyzes candidate's reasoning, edge cases, and time/space complexity. |
| **Concept Explanation** | **3 Credits** | Deep dive into pattern principles adapted to the candidate's background. |

*Safety Invariant: If an AI provider call fails or times out, exactly 0 credits are deducted.*

---

## Mock Payment Packages

When testing locally (`PAYMENT_PROVIDER=mock`), real credit cards are not charged. You can test the full Razorpay modal simulation directly from `/credits`:

| Package Name | Credits Granted | Authoritative Server Price | Local Test Action |
| :--- | :--- | :--- | :--- |
| **Starter Pack** | 50 Credits | ₹99 (`9900` paise) | Click "Purchase" -> instant mock verification |
| **Pro Pack** | 150 Credits | ₹249 (`24900` paise) | Click "Purchase" -> instant mock verification |
| **Master Pack** | 400 Credits | ₹599 (`59900` paise) | Click "Purchase" -> instant mock verification |

---

## Key UI Routes

| Route | Description | Access Control |
| :--- | :--- | :--- |
| `/` | Landing page explaining the active preparation loop | Public |
| `/register` | New candidate registration (auto-grants 50 credits) | Public |
| `/login` | Candidate / Administrator login | Public |
| `/onboarding` | 6-step personalized preparation setup wizard | Candidate |
| `/dashboard` | Daily prep dashboard with tasks, revisions, and streaks | Candidate |
| `/questions` | Question catalog with pattern, topic, and difficulty filters | Candidate |
| `/questions/[id]` | Problem practice workspace with timer, notes, and AI tutor | Candidate |
| `/roadmaps` | Curated roadmaps explorer (Master 29 Patterns, Blind 75, FAANG) | Candidate |
| `/roadmaps/[slug]` | Interactive roadmap curriculum and stage breakdown | Candidate |
| `/revisions` | SM-2 spaced repetition queue with urgency indicators | Candidate |
| `/credits` | AI credit wallet, transaction history, and package checkout | Candidate |
| `/admin` | Bulk spreadsheet import, audit logs, and question management | Admin Only |

---

## Automated Verification & Quality Assurance

### Run Frontend Linting & Build
```powershell
cd web
npm run lint    # Confirms 0 errors and 0 warnings
npm run build   # Validates TypeScript and generates all 15 routes
cd ..
```

### Run Go Unit & Integration Tests
```powershell
go test ./...
```

### Run End-to-End Acceptance Journey Suite
To run the automated 10-gate candidate journey verification script against the running Docker stack:
```powershell
node scratch/scratch_test_e2e.mjs
```
The test verifies:
1. Registration & session cookie generation
2. 50 welcome credits grant
3. Profile onboarding status
4. Onboarding wizard goal persistence
5. Roadmaps exploration & curriculum inspection
6. Problem practice & attempt logging
7. SM-2 spaced repetition queue scheduling
8. AI progressive hints & credit deduction
9. Mock payment checkout & instant fulfillment
10. Data portability JSON export
