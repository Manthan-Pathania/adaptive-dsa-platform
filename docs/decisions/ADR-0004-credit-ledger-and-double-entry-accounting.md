# ADR-0004: Double-Entry Immutable Ledger for Credits and Billing

## Context & Problem Statement
The platform meters AI capabilities (progressive hints, code reviews, concept breakdowns) using a credit system, which candidates purchase via payment packs.
How should credit balances and financial state be modeled in the database?
Common pitfalls include storing a single mutable integer column (`users.credits = credits - 1`), which leaves no audit trail, makes error reconciliation impossible, and is prone to lost updates under concurrency.

## Decision
We choose an **append-only immutable credit ledger (`credit_ledger_entries`) paired with a row-locked materialized balance cache (`credit_accounts`)**.

## Rationale & Engineering Trade-offs

### Advantages of the Double-Entry Ledger Pattern
1. **Mathematical Auditability**: Every single credit movement is an immutable row recording:
   - `amount` (integer delta, positive or negative).
   - `entry_type` (`PURCHASE`, `AI_USAGE`, `PROMOTIONAL_GRANT`, `REFUND`, `ADMIN_ADJUSTMENT`).
   - `reference_type` & `reference_id` (linking to specific payment orders or AI requests).
2. **Protection Against Race Conditions**: Balance deductions acquire an exclusive row lock (`SELECT cached_balance FROM credit_accounts WHERE user_id = $1 FOR UPDATE`). This serializes concurrent AI requests for the same user, preventing negative balances or double-spending.
3. **Reconciliation & Error Recovery**: If any data anomaly or application bug occurs, the true balance can always be recomputed from scratch via `SELECT SUM(amount) FROM credit_ledger_entries WHERE user_id = $1`.
4. **Integer Financial Units**: Balances and transaction amounts are strictly modeled as integers (integer credits, integer cents), completely eliminating floating-point IEEE-754 rounding errors.

### Downsides & Mitigation
- *Risk*: `credit_ledger_entries` grows with every AI interaction.
- *Mitigation*: The table is indexed by `(user_id, created_at DESC)`. Because active balance checks query the cached balance in `credit_accounts` directly, ledger reads are confined to user history and reconciliation jobs.

## Status
Accepted.
