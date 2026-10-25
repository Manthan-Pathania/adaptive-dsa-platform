# ADR-0003: Server-Side Sessions with HttpOnly Cookies over Stateless JWTs

## Context & Problem Statement
User authentication requires a secure, manageable mechanism for verifying identity across HTTP requests between the Next.js frontend and Go backend.
Common approaches:
1. Stateless JSON Web Tokens (JWT) stored in browser `localStorage`.
2. Stateful server-side sessions stored in PostgreSQL / Redis and passed via secure `HttpOnly` cookies.

## Decision
We choose **stateful server-side sessions stored in PostgreSQL** using cryptographically secure random session tokens delivered via `HttpOnly`, `Secure`, `SameSite=Lax` cookies.

## Rationale & Engineering Trade-offs

### Advantages of Stateful Sessions
1. **Instant Revocation Capability**: Stateless JWTs cannot be revoked without maintaining a distributed token blacklist, defeating their stateless nature. With server-side sessions, user logout, password resets, account suspensions, or security breaches immediately delete the session row from PostgreSQL, instantly invalidating access.
2. **Protection Against XSS Token Theft**: Storing JWTs in `localStorage` or `sessionStorage` exposes tokens to malicious JavaScript or compromised third-party NPM packages. `HttpOnly` cookies cannot be accessed or stolen by client-side JavaScript.
3. **Auditability**: The platform can inspect active sessions per user (IP address, user agent, last active timestamp) and allow candidates to "Log out from all devices".
4. **Minimal Overhead**: With PostgreSQL connection pooling (`pgxpool`) and indexing on `session_token_hash`, session lookup takes sub-millisecond time. If needed, a Redis cache can be placed in front with zero schema changes.

### Downsides & Mitigation
- *Risk*: Cross-Site Request Forgery (CSRF) is a consideration with cookie-based authentication.
- *Mitigation*: Cookies are configured with `SameSite=Lax`, and state-changing HTTP endpoints require a custom header check (`X-Requested-With`) or anti-CSRF token.

## Status
Accepted.
