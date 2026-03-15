-- Audit Log Partitioning: Monthly partitions with automatic management
-- This migration creates a partitioned audit_log table and sets up
-- automatic partition creation and cleanup using pg_partman.

-- ============================================================================
-- 1. Create partitioned audit_log table
-- ============================================================================

CREATE TABLE IF NOT EXISTS audit_log (
    id          BIGSERIAL,
    seq         BIGINT NOT NULL,
    timestamp   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    type        TEXT NOT NULL,
    actor       TEXT,
    action      TEXT NOT NULL,
    detail      TEXT,
    prev_hash   TEXT,
    hash        TEXT NOT NULL UNIQUE,
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_log_type ON audit_log(type);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log(actor);
CREATE INDEX IF NOT EXISTS idx_audit_log_seq ON audit_log(seq);

-- ============================================================================
-- 2. Create initial partitions (current month + next 2 months)
-- ============================================================================

-- Current month (2026-03)
CREATE TABLE IF NOT EXISTS audit_log_2026_03 PARTITION OF audit_log
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

-- Next month (2026-04)
CREATE TABLE IF NOT EXISTS audit_log_2026_04 PARTITION OF audit_log
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

-- Month after (2026-05)
CREATE TABLE IF NOT EXISTS audit_log_2026_05 PARTITION OF audit_log
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

-- ============================================================================
-- 3. Setup pg_partman for automatic partition management (optional)
-- ============================================================================

-- Note: This requires pg_partman extension to be installed
-- If pg_partman is not available, partitions must be created manually

DO $$
BEGIN
    -- Check if pg_partman is available
    IF EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'pg_partman') THEN
        -- Create extension if not exists
        CREATE EXTENSION IF NOT EXISTS pg_partman SCHEMA partman;
        
        -- Configure automatic partition management
        PERFORM partman.create_parent(
            p_parent_table := 'public.audit_log',
            p_control := 'timestamp',
            p_type := 'native',
            p_interval := '1 month',
            p_premake := 3  -- Create 3 months ahead
        );
        
        -- Configure retention (keep 12 months, drop old partitions)
        UPDATE partman.part_config 
        SET retention = '12 months', 
            retention_keep_table = false,
            retention_keep_index = false
        WHERE parent_table = 'public.audit_log';
        
        RAISE NOTICE 'pg_partman configured for audit_log';
    ELSE
        RAISE NOTICE 'pg_partman not available, partitions must be created manually';
    END IF;
END $$;

-- ============================================================================
-- 4. Comments for documentation
-- ============================================================================

COMMENT ON TABLE audit_log IS 'Merkle audit chain with monthly partitions (retains 12 months)';
COMMENT ON COLUMN audit_log.seq IS 'Sequence number for chain ordering';
COMMENT ON COLUMN audit_log.prev_hash IS 'Hash of previous record (Merkle chain)';
COMMENT ON COLUMN audit_log.hash IS 'SHA256 hash of this record';

