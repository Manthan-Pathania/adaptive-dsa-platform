# ADR-0002: Type-Safe SQL with sqlc and pgxpool over GORM

## Context & Problem Statement
The backend requires robust, performant, and type-safe database access to PostgreSQL 16.
Two primary paradigms exist in the Go ecosystem:
1. Object-Relational Mappers (ORMs) such as GORM or Ent.
2. Code generation from raw SQL queries using `sqlc` coupled with the native PostgreSQL driver `pgx/v5` (`pgxpool`).

## Decision
We choose **`sqlc` with `pgx/v5` (`pgxpool`)** for database operations.

## Rationale & Engineering Trade-offs

### Advantages of sqlc + pgx
1. **Full Control Over PostgreSQL Features**: Allows direct use of advanced PostgreSQL capabilities, such as `ON CONFLICT DO UPDATE`, CTEs, window functions, `SELECT ... FOR UPDATE`, and GIN indexing on array tags (`mistake_tags TEXT[]`). ORMs often obscure or complicate these features with awkward DSLs.
2. **Compile-Time Type Safety**: `sqlc` parses SQL queries against the exact Goose schema and generates idiomatic Go structs and query methods. If a column name changes or a type is mismatched, the build fails at compile time, eliminating runtime SQL syntax errors.
3. **Zero Reflection & Maximum Performance**: `pgx` is the fastest PostgreSQL driver for Go, using the native binary protocol. `sqlc` generates standard Go code without reflection or runtime query generation overhead.
4. **Learning & Portfolio Value**: Writing raw SQL forces a deep understanding of query planning, index selection (`EXPLAIN ANALYZE`), transaction isolation, and schema constraints—essential senior backend competencies that ORMs conceal.

### Downsides & Mitigation
- *Risk*: Requires writing boilerplate SQL for simple CRUD queries.
- *Mitigation*: The queries are explicit, maintainable, and version-controlled under `internal/database/queries/`. Tooling (`sqlc generate`) executes in milliseconds.

## Status
Accepted.
