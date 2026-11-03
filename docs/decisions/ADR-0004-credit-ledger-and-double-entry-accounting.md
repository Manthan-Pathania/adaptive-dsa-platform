# ADR-0004: Append-Only Immutable Event Ledger with Row-Locked Balance Cache for Credits and Billing

## Context & Problem Statement
The platform meters AI capabilities (progressive hints, code reviews, concept breakdowns) using a utility credit token system, which candidates purchase via payment packs.
How should credit balances and financial state be modeled in the database?

Common pitfalls include storing a single mutable integer column (`users.credits = credits - 1`), which leaves no audit trail, makes error reconciliation impossible, and is prone to lost updates under concurrency.

Conversely, a full multi-account double-entry bookkeeping system (with debits, credits, systemic asset/liability/equity accounts) introduces substantial schema overhead and synthetic counter-accounts (e.g., system mint and burn reserve accounts) for an internal metered utility token that is not a multi-party fiat currency.

## Decision
We choose an **append-only immutable signed event ledger (`credit_ledger_entries`) paired with an atomic, row-locked materialized balance cache (`credit_accounts`)**.

We explicitly document this as a **Signed Immutable Event Ledger with Row-Locked Materialized Balance**, rather than mislabeling it as multi-account double-entry bookkeeping.

## Accounting Invariants & Architecture

### 1. Mathematical Auditability
Every credit movement is an immutable row recording:
- `amount`: Signed integer delta (positive for grants/purchases/refunds, negative for AI usage deductions).
- `entry_type`: Explicit taxonomy (`PURCHASE`, `AI_USAGE`, `PROMOTIONAL_GRANT`, `REFUND`, `ADMIN_ADJUSTMENT`).
- `reference_type` & `reference_id`: Contextual link to external payment orders (`payment_orders.id`), AI completions (`ai_requests.id`), or admin actors (`users.id`).
- `description`: Plain-English audit explanation.
- `created_at`: UTC timestamp.

### 2. Atomic Row-Locked Deductions & Concurrency Protection
Balance changes and ledger insertions execute within a single database transaction:
```sql
-- 1. Acquire exclusive row lock to serialize concurrent operations for this user
SELECT cached_balance FROM credit_accounts WHERE user_id = $1 FOR UPDATE;

-- 2. Verify cached_balance >= required_credits (prevent negative balance underflow)

-- 3. Update cached balance
UPDATE credit_accounts SET cached_balance = cached_balance + $2, updated_at = NOW() WHERE user_id = $1;

-- 4. Insert immutable audit record
INSERT INTO credit_ledger_entries (user_id, amount, entry_type, reference_type, reference_id, description)
VALUES ($1, $2, $3, $4, $5, $6);
```
Under concurrent load (verified up to 20 parallel requests), row-level locking ensures zero lost updates, zero negative balances, and deterministic serialization.

### 3. Reconciliation & Consistency Verification
The system enforces the mathematical reconciliation invariant:
$$\text{cached\_balance} \equiv \sum_{i} \text{ledger\_entry}_i.\text{amount}$$
At any time, an audit job or API call (`ReconcileBalance`) computes:
```sql
SELECT COALESCE(SUM(amount), 0) FROM credit_ledger_entries WHERE user_id = $1;
```
If `cached_balance != sum`, the system flags an inconsistency.

### 4. Integer Units
All balances and ledger amounts are strictly stored as integers, completely eliminating IEEE-754 floating-point rounding errors.

## Trade-off Analysis: Event Ledger vs. Multi-Account Double-Entry

| Dimension | Signed Immutable Event Ledger (Selected) | Multi-Account Double-Entry (Rejected) |
| :--- | :--- | :--- |
| **Domain Fit** | Native model for in-app metered utility credits (mint, spend, refund) | Over-engineered for utility credits; designed for multi-party fiat banking |
| **Audit Trail** | 100% complete — every delta is recorded immutably with timestamp and ref ID | 100% complete — balanced dual debit/credit entries |
| **Complexity** | 2 tables (`credit_accounts`, `credit_ledger_entries`) | 4+ tables (Chart of Accounts, Journal Entries, Postings, Balances) |
| **Counter-Accounts** | None required — single user-centric ledger | Requires synthetic accounts (`System:MintPool`, `System:UsageBurnPool`, `System:Liabilities`) |
| **Query Performance** | Fast $O(1)$ balance check on `credit_accounts.cached_balance` | Requires aggregation over ledger postings or complex dual-account caches |
| **Reconciliation** | Trivial: `cached_balance == SUM(amount)` | Standard zero-sum check: $\sum \text{Debits} - \sum \text{Credits} = 0$ |

## Status
Accepted. Accurately reconciled and implemented in Go (`internal/credits`) with PostgreSQL 16 row-locking integration tests.
