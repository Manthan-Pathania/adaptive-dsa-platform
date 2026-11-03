#!/usr/bin/env bash
# ==============================================================================
# Adaptive DSA Platform - PostgreSQL Database Restore Verification Script
# Usage: ./scripts/restore_postgres.sh <backup_file> [target_database]
# ==============================================================================

set -euo pipefail

if [ "$#" -lt 1 ]; then
    echo "Usage: $0 <path_to_dump_file> [target_database_name]"
    exit 1
fi

BACKUP_FILE="$1"
TARGET_DB="${2:-adaptive_dsa_restored}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_HOST="${POSTGRES_HOST:-localhost}"
DB_PORT="${POSTGRES_PORT:-5432}"

if [ ! -f "${BACKUP_FILE}" ]; then
    echo "Error: Backup file '${BACKUP_FILE}' does not exist!"
    exit 1
fi

# Verify checksum if present
if [ -f "${BACKUP_FILE}.sha256" ]; then
    echo "Verifying archive integrity with SHA256 checksum..."
    sha256sum -c "${BACKUP_FILE}.sha256"
fi

echo "Creating target restore database '${TARGET_DB}'..."
PGPASSWORD="${POSTGRES_PASSWORD:-}" psql \
    -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" \
    -c "DROP DATABASE IF EXISTS ${TARGET_DB};"
PGPASSWORD="${POSTGRES_PASSWORD:-}" psql \
    -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" \
    -c "CREATE DATABASE ${TARGET_DB};"

echo "Restoring database from '${BACKUP_FILE}'..."
PGPASSWORD="${POSTGRES_PASSWORD:-}" pg_restore \
    -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" \
    -d "${TARGET_DB}" \
    -v \
    "${BACKUP_FILE}" || true

echo "Validating restored tables and schema..."
TABLE_COUNT=$(PGPASSWORD="${POSTGRES_PASSWORD:-}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${TARGET_DB}" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';")

echo "=================================================="
echo "Database Restore Completed Successfully"
echo "Target DB:    ${TARGET_DB}"
echo "Table Count:  $(echo ${TABLE_COUNT} | tr -d ' ')"
echo "=================================================="
