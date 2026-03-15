-- Automated Cleanup Jobs: pg_cron for periodic maintenance
-- This migration sets up automated cleanup jobs for expired memory
-- and partition maintenance using pg_cron extension.

-- ============================================================================
-- 1. Enable pg_cron extension (optional)
-- ============================================================================

-- Note: This requires pg_cron extension to be installed
-- Cloud providers (Aliyun RDS, Tencent Cloud, AWS RDS) usually support this

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'pg_cron') THEN
        CREATE EXTENSION IF NOT EXISTS pg_cron;
        RAISE NOTICE 'pg_cron extension enabled';
    ELSE
        RAISE NOTICE 'pg_cron not available, cleanup jobs must be run manually or via external scheduler';
    END IF;
END $$;

-- ============================================================================
-- 2. Schedule cleanup jobs
-- ============================================================================

DO $$
BEGIN
    -- Only schedule if pg_cron is available
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
        
        -- Job 1: Clean expired memory (daily at 3:00 AM)
        PERFORM cron.schedule(
            'cleanup-expired-memory',
            '0 3 * * *',  -- Every day at 3:00 AM
            $$
            DELETE FROM memory_short WHERE expires_at < NOW();
            DELETE FROM memory_mid WHERE expires_at < NOW() AND pinned = FALSE;
            DELETE FROM memory_long WHERE expires_at < NOW() AND pinned = FALSE;
            $$
        );
        
        -- Job 2: Partition maintenance (daily at 2:00 AM)
        -- Only if pg_partman is available
        IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_partman') THEN
            PERFORM cron.schedule(
                'partition-maintenance',
                '0 2 * * *',  -- Every day at 2:00 AM
                $$SELECT partman.run_maintenance('public.audit_log', p_analyze := false)$$
            );
        END IF;
        
        -- Job 3: Vacuum and analyze (weekly on Sunday at 4:00 AM)
        PERFORM cron.schedule(
            'weekly-vacuum',
            '0 4 * * 0',  -- Every Sunday at 4:00 AM
            $$
            VACUUM ANALYZE memory_mid;
            VACUUM ANALYZE memory_long;
            VACUUM ANALYZE audit_log;
            $$
        );
        
        RAISE NOTICE 'Cleanup jobs scheduled successfully';
    END IF;
END $$;

-- ============================================================================
-- 3. Manual cleanup functions (for environments without pg_cron)
-- ============================================================================

-- Function to clean expired memory
CREATE OR REPLACE FUNCTION cleanup_expired_memory()
RETURNS TABLE(
    short_deleted BIGINT,
    mid_deleted BIGINT,
    long_deleted BIGINT
) AS $$
DECLARE
    v_short_deleted BIGINT;
    v_mid_deleted BIGINT;
    v_long_deleted BIGINT;
BEGIN
    -- Clean short-term memory
    DELETE FROM memory_short WHERE expires_at < NOW();
    GET DIAGNOSTICS v_short_deleted = ROW_COUNT;
    
    -- Clean mid-term memory (excluding pinned)
    DELETE FROM memory_mid WHERE expires_at < NOW() AND pinned = FALSE;
    GET DIAGNOSTICS v_mid_deleted = ROW_COUNT;
    
    -- Clean long-term memory (excluding pinned)
    DELETE FROM memory_long WHERE expires_at < NOW() AND pinned = FALSE;
    GET DIAGNOSTICS v_long_deleted = ROW_COUNT;
    
    RETURN QUERY SELECT v_short_deleted, v_mid_deleted, v_long_deleted;
END;
$$ LANGUAGE plpgsql;

-- Function to get cleanup statistics
CREATE OR REPLACE FUNCTION get_memory_stats()
RETURNS TABLE(
    layer TEXT,
    total_items BIGINT,
    expired_items BIGINT,
    pinned_items BIGINT,
    avg_age_days NUMERIC
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        'short'::TEXT,
        COUNT(*)::BIGINT,
        COUNT(*) FILTER (WHERE expires_at < NOW())::BIGINT,
        0::BIGINT,
        AVG(EXTRACT(EPOCH FROM (NOW() - created_at)) / 86400)::NUMERIC
    FROM memory_short
    UNION ALL
    SELECT 
        'mid'::TEXT,
        COUNT(*)::BIGINT,
        COUNT(*) FILTER (WHERE expires_at < NOW())::BIGINT,
        COUNT(*) FILTER (WHERE pinned = TRUE)::BIGINT,
        AVG(EXTRACT(EPOCH FROM (NOW() - created_at)) / 86400)::NUMERIC
    FROM memory_mid
    UNION ALL
    SELECT 
        'long'::TEXT,
        COUNT(*)::BIGINT,
        COUNT(*) FILTER (WHERE expires_at < NOW())::BIGINT,
        COUNT(*) FILTER (WHERE pinned = TRUE)::BIGINT,
        AVG(EXTRACT(EPOCH FROM (NOW() - created_at)) / 86400)::NUMERIC
    FROM memory_long;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 4. Comments for documentation
-- ============================================================================

COMMENT ON FUNCTION cleanup_expired_memory() IS 'Manually clean expired memory items (returns deletion counts)';
COMMENT ON FUNCTION get_memory_stats() IS 'Get memory statistics including expired and pinned items';

-- ============================================================================
-- 5. Usage examples (for manual execution)
-- ============================================================================

-- Example 1: View memory statistics
-- SELECT * FROM get_memory_stats();

-- Example 2: Manually clean expired memory
-- SELECT * FROM cleanup_expired_memory();

-- Example 3: View scheduled cron jobs (if pg_cron is enabled)
-- SELECT * FROM cron.job;

-- Example 4: Manually run partition maintenance (if pg_partman is enabled)
-- SELECT partman.run_maintenance('public.audit_log');

