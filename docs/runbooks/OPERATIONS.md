# Adaptive DSA Platform — Operations, Runbooks & Disaster Recovery Guide

This operational guide provides step-by-step procedures for incident response, database backup/restore, secret rotation, credit reconciliation, and rollback.

---

## 1. Operational Diagnostics & Monitoring

### A. Health & Readiness Probes
- **Liveness Probe**: `GET /healthz`
  - Responds HTTP 200 with process uptime and RFC 3339 timestamp.
  - Used by container orchestrators to detect deadlocks.
- **Readiness Probe**: `GET /readyz`
  - Verifies live PostgreSQL connectivity, query execution, latency, and connection pool saturation.
  - Responds HTTP 200 when healthy:
    ```json
    {
      "status": "ready",
      "timestamp": "2026-10-25T11:20:00Z",
      "checks": {
        "database": {
          "available": true,
          "latency_ms": 1.45,
          "total_conns": 5,
          "idle_conns": 4,
          "acquired_conns": 1,
          "max_conns": 20
        }
      }
    }
    ```
  - Responds HTTP 503 `{"status":"unavailable"}` if database is unreachable.

### B. Administrative Metrics Endpoint
- **Endpoint**: `GET /api/v1/admin/metrics`
- **Authentication**: Requires valid session cookie with `role = 'ADMIN'`.
- **Payload**:
  - `app_env`: Current environment (`development`, `production`).
  - `uptime_seconds`: Server runtime.
  - `runtime`: Go runtime metrics (`goroutines`, `heap_alloc_bytes`, `heap_sys_bytes`, `num_gc`).
  - `database`: PostgreSQL connection pool status and latency.
  - `background_jobs`: Live queue counts (`PENDING`, `RUNNING`, `COMPLETED`, `FAILED`).

---

## 2. Disaster Recovery, Backup & Restore Procedures

### A. Objectives & Guarantees
- **Recovery Point Objective (RPO)**: **1 Hour** (with automated hourly `pg_dump`) or **< 1 Minute** (when continuous WAL archiving to S3/Cloud Storage is enabled).
- **Recovery Time Objective (RTO)**: **< 15 Minutes** (full container redeployment and database restore from compressed dump).

### B. Backup Scheduling & Retention
Automate daily backups using cron on the Linux host (`crontab -e`):
```cron
# Run daily database backup at 02:00 AM UTC
0 2 * * * /app/scripts/backup_postgres.sh /var/backups/adaptive_dsa >> /var/log/adaptive_dsa_backup.log 2>&1
```
- **Local Retention**: Automatically retains the last 7 daily backup dumps (`.dump` and `.sha256`), purging older files.
- **Off-Site Replication**: When `AWS_S3_BACKUP_BUCKET` is set, backups are mirrored to S3-compatible cloud storage.

### C. Restoring from a Backup
To restore into a clean or disaster-recovery instance:
```bash
# 1. Verify archive checksum
sha256sum -c /var/backups/adaptive_dsa/adaptive_dsa_prod_20261025_020000.dump.sha256

# 2. Execute restore script
./scripts/restore_postgres.sh /var/backups/adaptive_dsa/adaptive_dsa_prod_20261025_020000.dump adaptive_dsa_prod
```

---

## 3. Financial Immutability & Credit Reconciliation

The platform uses a double-entry accounting pattern. `credit_accounts.cached_balance` is a performance optimization; `credit_ledger_entries` is the immutable source of truth.

### Operational Reconciliation Audit
Run the CLI reconciliation command to verify exact mathematical alignment across all users:
```bash
# Run audit via CLI
docker compose exec backend /app/admin reconcile-credits

# Or locally:
go run ./cmd/admin reconcile-credits
```
**Expected Output**:
```
==================================================
Credit Balance Reconciliation Audit
==================================================
--------------------------------------------------
RECONCILIATION PASSED: All N credit accounts in exact alignment with immutable ledger.
==================================================
```
If any discrepancy is found, the tool reports the specific user ID, email, cached balance, ledger sum, and delta.

---

## 4. Incident Response Runbooks

### Incident 1: `/readyz` Returns HTTP 503 (Database Unreachable)
1. Check PostgreSQL container status:
   ```bash
   docker compose ps postgres
   docker compose logs --tail=50 postgres
   ```
2. Check disk space on the host:
   ```bash
   df -h
   ```
3. Test PostgreSQL connectivity directly from the host:
   ```bash
   docker compose exec postgres pg_isready -U dsa_prod_user -d adaptive_dsa_prod
   ```

### Incident 2: Background Jobs Stranded in RUNNING State
If the worker container crashed abruptly, jobs in progress may remain in `RUNNING`.
- The repository's CTE claiming query automatically reclaims jobs that have been in `RUNNING` with `updated_at < NOW() - INTERVAL '5 minutes'`.
- To inspect currently stuck jobs manually:
  ```sql
  SELECT id, job_type, retry_count, updated_at, last_error 
  FROM background_jobs 
  WHERE status = 'RUNNING' AND updated_at < NOW() - INTERVAL '5 minutes';
  ```
- To reset stuck jobs back to `PENDING` immediately:
  ```sql
  UPDATE background_jobs 
  SET status = 'PENDING', scheduled_at = NOW(), updated_at = NOW() 
  WHERE status = 'RUNNING' AND updated_at < NOW() - INTERVAL '5 minutes';
  ```

---

## 5. Secret Rotation Procedures

### A. Rotating `CSRF_SECRET`
1. Generate a new cryptographically secure 32-byte secret:
   ```bash
   openssl rand -hex 32
   ```
2. Update `CSRF_SECRET` in `.env`.
3. Restart the backend container: `docker compose restart backend`.
4. *Impact*: Active browser sessions will receive a new CSRF token on their next GET request.

### B. Rotating PostgreSQL Passwords
1. Connect to PostgreSQL as superuser:
   ```bash
   docker compose exec postgres psql -U postgres
   ```
2. Change password for application user:
   ```sql
   ALTER USER adaptive_dsa_app WITH PASSWORD 'NEW_STRONG_PASSWORD';
   ```
3. Update `DATABASE_URL` in `.env`.
4. Restart backend: `docker compose restart backend`.

### C. Rotating Google Gemini API Key
1. Generate new API key in Google AI Studio.
2. Update `GEMINI_API_KEY` in `.env`.
3. Restart backend: `docker compose restart backend`. Zero database downtime.

### D. Rotating Razorpay Webhook Secret
1. Generate new secret in Razorpay Dashboard -> Settings -> Webhooks.
2. Update `RAZORPAY_WEBHOOK_SECRET` in `.env`.
3. Restart backend: `docker compose restart backend`.

---

## 6. Rollback Procedures

### A. Application Rollback
If a newly deployed container image exhibits regressions:
```bash
# 1. Update image tags in docker-compose.yml to previous known-good tag
# 2. Re-deploy containers with zero downtime
docker compose up -d --no-deps backend frontend
```

### B. Migration Rollback
If a schema migration must be reverted:
```bash
# Roll back the most recent Goose migration step
docker compose run --rm migrate down
```
All migrations in `migrations/` are written with strict reversibility (`-- +goose Down`).
