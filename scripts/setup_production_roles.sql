-- ==============================================================================
-- Adaptive DSA Platform - PostgreSQL Production Role Hardening
-- Implements Principle of Least Privilege & Append-Only Record Protection
-- ==============================================================================

-- 1. Migrator Role (Executes Goose versioned DDL migrations)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'adaptive_dsa_migrator') THEN
        CREATE ROLE adaptive_dsa_migrator WITH LOGIN PASSWORD 'REPLACE_WITH_STRONG_MIGRATOR_PASSWORD';
    END IF;
END
$$;

GRANT CONNECT ON DATABASE adaptive_dsa_prod TO adaptive_dsa_migrator;
GRANT ALL ON SCHEMA public TO adaptive_dsa_migrator;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO adaptive_dsa_migrator;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO adaptive_dsa_migrator;

-- 2. Application Runtime Role (DML only, runtime operations)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'adaptive_dsa_app') THEN
        CREATE ROLE adaptive_dsa_app WITH LOGIN PASSWORD 'REPLACE_WITH_STRONG_APP_PASSWORD';
    END IF;
END
$$;

GRANT CONNECT ON DATABASE adaptive_dsa_prod TO adaptive_dsa_app;
GRANT USAGE ON SCHEMA public TO adaptive_dsa_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO adaptive_dsa_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO adaptive_dsa_app;

-- 3. Immutability Hardening: Enforce Append-Only Protections
-- The application runtime account is explicitly prohibited from updating, deleting,
-- or truncating immutable financial ledger entries and tamper-evident audit logs.
REVOKE UPDATE, DELETE, TRUNCATE ON TABLE credit_ledger_entries FROM adaptive_dsa_app;
REVOKE UPDATE, DELETE, TRUNCATE ON TABLE audit_logs FROM adaptive_dsa_app;
REVOKE UPDATE, DELETE, TRUNCATE ON TABLE payment_webhook_events FROM adaptive_dsa_app;

-- Ensure future migrations inherit correct permissions
ALTER DEFAULT PRIVILEGES FOR ROLE adaptive_dsa_migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO adaptive_dsa_app;
ALTER DEFAULT PRIVILEGES FOR ROLE adaptive_dsa_migrator IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO adaptive_dsa_app;
