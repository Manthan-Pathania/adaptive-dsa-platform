# ADR-0006: Spaced Repetition Revision Engine (SM-2 / FSRS Adaptation)

## Context & Problem Statement
DSA interview preparation involves high cognitive load and rapid memory decay. Candidates often solve a problem once and forget the underlying pattern weeks later.
Simple fixed calendar intervals (e.g., revise in 3 days, then 7 days) fail because they treat an easy problem solved in 10 minutes identically to a hard problem solved after looking at the editorial for 45 minutes. Furthermore, missing several study days creates a catastrophic backlog of overdue items.

## Decision
We choose an **evidence-based adaptation of the SuperMemo-2 (SM-2) / Free Spaced Repetition Scheduler (FSRS) algorithm**, tailored specifically for algorithmic problem solving, coupled with an **overdue dampening triage strategy**.

## Rationale & Engineering Trade-offs

### The DSA-Adapted Scheduling Engine
1. **Multi-Factor Input**:
   - *Outcome Weight*: `SOLVED_INDEPENDENTLY` increases memory stability factor $S$ exponentially ($I_{n} = I_{n-1} \times EF$). `SOLVED_WITH_HINTS` increases interval modestly. `SOLVED_AFTER_EDITORIAL` or `UNRESOLVED_FAILED` resets the repetition count to 0 and interval to 1–2 days.
   - *Mistake Severity*: Severe conceptual mistakes (pattern recognition failure) penalize stability more heavily than syntax or minor edge-case oversights.
   - *Time Factor*: Solving time relative to problem benchmark provides fine-grained calibration.
2. **Anti-Backlog Dampening**:
   - If a candidate misses a week, standard spaced repetition schedules produce an impossible backlog of 30+ questions.
   - Our engine computes current estimated retention $R(t) = e^{-t / S}$. It prioritizes the top $K$ questions with the highest decay rates and highest pattern leverage that fit into today's stated study time budget, smoothly rolling forward the remainder without crushing candidate morale.
3. **Deterministic & Transparent**:
   - No opaque, unexplainable machine learning model. The scheduling rules are 100% deterministic, testable in pure Go unit tests, and fully transparent to the candidate ("Why is this problem due today? Because you used hints on your last attempt 5 days ago").

### Downsides & Mitigation
- *Risk*: Algorithm requires fine-tuning of initial difficulty constants and intervals.
- *Mitigation*: Parameters ($EF$, minimum intervals, mistake penalties) are isolated in configuration structs, allowing empirical tuning based on user progress data.

## Status
Accepted.
