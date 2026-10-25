# ADR-0001: Modular Monolith Architecture over Microservices

## Context & Problem Statement
The platform requires a scalable, maintainable, and observable backend supporting identity, question curation, spaced repetition scheduling, AI orchestration, credit accounting, and payment processing. 
The system is built by a backend-focused engineer targeting a low-cost production deployment on a modest VPS (< $10/month) while showcasing high engineering rigor for portfolio evaluation.
Should the system be built as distributed microservices or as a Go modular monolith?

## Decision
We choose a **Go Modular Monolith** with clean internal architectural boundaries (Hexagonal / Ports and Adapters).

## Rationale & Engineering Trade-offs

### Advantages of Modular Monolith
1. **Low Memory & Operating Cost**: A compiled Go monolith consumes ~30–50MB RSS under normal load. Multiple microservices each running their own runtime, connection pools, and monitoring agents would easily exhaust a 2GB–4GB VPS.
2. **ACID Transactions without Distributed Complexity**: Critical operations—such as atomic credit grant on payment webhook, or atomic attempt logging with progress upsert and spaced repetition reschedule—can be executed inside a single PostgreSQL ACID transaction (`BEGIN ... COMMIT`). In microservices, this would require 2-Phase Commit (2PC), Sagas, or outbox patterns with eventual consistency.
3. **Zero Inter-Service Network Latency & Failure Modes**: In-process Go function calls have nanosecond latency, whereas microservices introduce serialization overhead (JSON/gRPC), network timeouts, retries, and cascading failures.
4. **Developer Ergonomics**: A single repository, single Go module, unified test suite, and straightforward Docker Compose setup makes local development fast and frictionless.

### Downsides & Mitigation
- *Risk*: Code coupling and boundary erosion over time (e.g., identity importing billing directly).
- *Mitigation*: Strict package structure (`internal/identity`, `internal/catalog`, `internal/billing`), clear interfaces, and explicit Go constructor injection. Circular package dependencies are forbidden by the Go compiler.

## Status
Accepted.
