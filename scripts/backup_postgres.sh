#!/usr/bin/env bash
# ==============================================================================
# Adaptive DSA Platform - Automated PostgreSQL Backup Script
# Usage: ./scripts/backup_postgres.sh [backup_dir]
# ==============================================================================

set -euo pipefail

BACKUP_DIR="${1:-/var/backups/adaptive_dsa}"
TIMESTAMP="$(date +'%Y%m%d_%H%M%S')"
DATABASE_NAME="${POSTGRES_DB:-adaptive_dsa_prod}"
DB_USER="${POSTGRES_USER:-adaptive_dsa_migrator}"
DB_HOST="${POSTGRES_HOST:-localhost}"
DB_PORT="${POSTGRES_PORT:-5432}"

BACKUP_FILE="${BACKUP_DIR}/${DATABASE_NAME}_${TIMESTAMP}.dump"
CHECKSUM_FILE="${BACKUP_FILE}.sha256"

mkdir -p "${BACKUP_DIR}"

echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] Starting database backup for '${DATABASE_NAME}'..."

# Execute compressed custom-format pg_dump
PGPASSWORD="${POSTGRES_PASSWORD:-}" pg_dump \
    -h "${DB_HOST}" \
    -p "${DB_PORT}" \
    -U "${DB_USER}" \
    -Fc \
    -b -v \
    -f "${BACKUP_FILE}" \
    "${DATABASE_NAME}"

# Generate cryptographic checksum
sha256sum "${BACKUP_FILE}" > "${CHECKSUM_FILE}"
BACKUP_SIZE="$(du -h "${BACKUP_FILE}" | cut -f1)"

echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] Backup completed successfully: ${BACKUP_FILE} (${BACKUP_SIZE})"

# Enforce local retention: Retain last 7 daily backups
echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] Enforcing local retention policy (keeping last 7 days)..."
find "${BACKUP_DIR}" -type f -name "${DATABASE_NAME}_*.dump*" -mtime +7 -delete

# Off-site S3 sync hook (if AWS_S3_BACKUP_BUCKET is configured)
if [ -n "${AWS_S3_BACKUP_BUCKET:-}" ]; then
    echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] Syncing backup to remote S3 bucket: ${AWS_S3_BACKUP_BUCKET}..."
    aws s3 cp "${BACKUP_FILE}" "s3://${AWS_S3_BACKUP_BUCKET}/backups/${DATABASE_NAME}/${DATABASE_NAME}_${TIMESTAMP}.dump"
    aws s3 cp "${CHECKSUM_FILE}" "s3://${AWS_S3_BACKUP_BUCKET}/backups/${DATABASE_NAME}/${DATABASE_NAME}_${TIMESTAMP}.dump.sha256"
fi

echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] Backup workflow finalized."
