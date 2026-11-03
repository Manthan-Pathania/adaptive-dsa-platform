# Adaptive DSA Platform — Deployment Architecture & Infrastructure Guide

This document defines the production deployment architecture, resource requirements, cost estimates, and hardening measures for the Adaptive DSA Interview Preparation Platform.

---

## 1. Architectural Overview

The platform uses a cost-conscious, highly reliable single-node VPS architecture orchestrated via Docker Compose and fronted by Caddy:

```
                                  [ Internet / User Browsers ]
                                                │
                                                ▼  HTTPS (443) / HTTP (80)
┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Linux VPS (Ubuntu 24.04 LTS — 2 vCPU / 4 GB RAM)                                                │
│                                                                                                  │
│  ┌────────────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ Caddy Edge Reverse Proxy (caddy:2-alpine)                                                  │  │
│  │ - Automatic Let's Encrypt / ZeroSSL TLS certificates & HTTP->HTTPS redirection             │  │
│  │ - Compression: zstd & gzip                                                                 │  │
│  │ - Edge security headers (HSTS, CSP, X-Frame-Options, Permissions-Policy)                   │  │
│  │ - Access logging in structured JSON                                                        │  │
│  └──────────────────────────────────────┬─────────────────────────────────────────────────────┘  │
│                                         │                                                        │
│           ┌─────────────────────────────┴─────────────────────────────┐                          │
│           ▼ /api/*, /healthz, /readyz                                 ▼ /* (Static + SSR)        │
│  ┌──────────────────────────────────────────────┐    ┌────────────────────────────────────────┐  │
│  │ Go REST API & Worker (backend:8080)          │    │ Next.js Frontend (frontend:3000)       │  │
│  │ - Multi-stage statically compiled binary     │    │ - Next.js Standalone server.js bundle  │  │
│  │ - Non-root user (appuser:appgroup, UID 10001)│    │ - Non-root user (nextjs:nodejs, 1001)  │  │
│  │ - Embedded durable background worker         │    │ - Server-Side Rendering & Turbopack    │  │
│  │ - Limits: 1.0 CPU, 512 MB RAM                │    │ - Limits: 1.0 CPU, 512 MB RAM          │  │
│  └──────────────────────┬───────────────────────┘    └────────────────────────────────────────┘  │
│                         │                                                                        │
│                         ▼ PostgreSQL Protocol (5432) — Private Docker Network Only               │
│  ┌────────────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ PostgreSQL 16 Storage (postgres:16-alpine)                                                 │  │
│  │ - Strictly isolated from host network (NO exposed public ports)                           │  │
│  │ - Persistent named volume: adaptive_dsa_pgdata                                             │  │
│  │ - Migrator account (DDL) vs Application account (DML only, least privilege)                │  │
│  │ - Limits: 1.0 CPU, 768 MB RAM                                                              │  │
│  └────────────────────────────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Infrastructure Assumptions & Monthly Cost Estimates

### A. Resource Assumptions
- **Host System**: 1x Linux VPS (Ubuntu 24.04 LTS x86_64).
- **Compute / Memory Allocation**:
  - `postgres`: 768 MB RAM limit, 1.0 CPU limit.
  - `backend`: 512 MB RAM limit, 1.0 CPU limit.
  - `frontend`: 512 MB RAM limit, 1.0 CPU limit.
  - `caddy`: 256 MB RAM limit, 0.5 CPU limit.
  - **Total Container Memory Allocation**: ~2.0 GB.
  - **Host Reserve**: 2.0 GB available for Linux kernel, page cache, background backups, and buffer headroom.

### B. Provider Cost Estimates (Verified Current Pricing)

| Provider | Plan / Specifications | Estimated Monthly Cost | Notes |
| :--- | :--- | :--- | :--- |
| **Hetzner Cloud** (Recommended) | **CX22**: 2 vCPU (x86), 4 GB RAM, 40 GB NVMe SSD, 20 TB traffic | **€3.79 – €4.50 / month** (~$4.10 – $4.85) | Datacenters in Falkenstein, Nuremberg, or Helsinki. Excellent cost/performance ratio. |
| **DigitalOcean** | **Basic Droplet**: 2 vCPU, 4 GB RAM, 80 GB SSD, 4 TB traffic | **$24.00 / month** | Global locations (BLR1 India, SGP1 Singapore, NYC, FRA). |
| **Linode / Akamai** | **Shared CPU**: 2 vCPU, 4 GB RAM, 80 GB SSD, 4 TB traffic | **$24.00 / month** | Reliable worldwide locations including Mumbai. |
| **AWS Lightsail** | **Standard**: 2 vCPU, 4 GB RAM, 80 GB SSD, 4 TB traffic | **$20.00 / month** | Fixed AWS pricing, simple snapshot backups. |
| **Custom Domain** | `.com` / `.dev` / `.in` domain registration | **$10 – $14 / year** (~$1/mo) | Cloudflare Registrar / Porkbun / Namecheap. |

**Total Estimated Monthly Infrastructure Cost**: **$5.00 – $25.00 / month**.

---

## 3. Security & Isolation Architecture

1. **Private PostgreSQL Isolation**:
   - PostgreSQL is bound exclusively to the private Docker bridge network (`platform_net`).
   - Host port 5432 is **not exposed** in `docker-compose.yml`. Public database access is impossible.
2. **Non-Root Execution**:
   - Both Go binary and Next.js standalone server execute under dedicated unprivileged system users (`appuser:10001` and `nextjs:1001`).
3. **Automated TLS & Security Headers**:
   - Caddy automatically provisions and manages TLS certificates via Let's Encrypt / ZeroSSL using ACME HTTP-01 challenge.
   - Enforces `Strict-Transport-Security`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, and `Permissions-Policy`.
4. **Secrets Separation**:
   - All production secrets (`CSRF_SECRET`, `DATABASE_URL`, `GEMINI_API_KEY`, `RAZORPAY_KEY_SECRET`) are injected via environment variables or an untracked `.env` file with `chmod 600`.
   - Never baked into container images or committed to Git.
