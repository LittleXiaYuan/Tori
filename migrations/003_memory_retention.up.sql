-- Memory Retention: Add TTL and Pinning support
-- This migration adds expires_at and pinned fields to memory tables
-- and creates triggers to enforce capacity limits per tenant.

-- ============================================================================
-- 1. Add expires_at and pinned fields
-- ============================================================================

-- Mid-term memory: 90 days TTL by default
ALTER TABLE memory_mid 
ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ DEFAULT NOW() + INTERVAL '90 days',
ADD COLUMN IF NOT EXISTS pinned BOOLEAN DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_memory_mid_expires ON memory_mid(expires_at);
CREATE INDEX IF NOT EXISTS idx_memory_mid_pinned ON memory_mid(pinned);

-- Long-term memory: 365 days TTL by default
ALTER TABLE memory_long 
ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ DEFAULT NOW() + INTERVAL '365 days',
ADD COLUMN IF NOT EXISTS pinned BOOLEAN DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_memory_long_expires ON memory_long(expires_at);
CREATE INDEX IF NOT EXISTS idx_memory_long_pinned ON memory_long(pinned);

-- ============================================================================
-- 2. Capacity limit triggers (LRU with pinned protection)
-- ============================================================================

-- Mid-term: max 1000 items per tenant (excluding pinned)
CREATE OR REPLACE FUNCTION enforce_mid_capacity_limit()
RETURNS TRIGGER AS $$
BEGIN
    -- Delete oldest unpinned items if over limit
    DELETE FROM memory_mid
    WHERE tenant_id = NEW.tenant_id
    AND pinned = FALSE
    AND id NOT IN (
        SELECT id FROM memory_mid
        WHERE tenant_id = NEW.tenant_id
        ORDER BY 
            pinned DESC,           -- Pinned items first
            last_access DESC,      -- Then by last access
            created_at DESC        -- Then by creation time
        LIMIT 1000
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER mid_capacity_trigger
AFTER INSERT ON memory_mid
FOR EACH ROW EXECUTE FUNCTION enforce_mid_capacity_limit();

-- Long-term: max 5000 items per tenant (excluding pinned)
CREATE OR REPLACE FUNCTION enforce_long_capacity_limit()
RETURNS TRIGGER AS $$
BEGIN
    -- Delete oldest unpinned items if over limit
    DELETE FROM memory_long
    WHERE tenant_id = NEW.tenant_id
    AND pinned = FALSE
    AND id NOT IN (
        SELECT id FROM memory_long
        WHERE tenant_id = NEW.tenant_id
        ORDER BY 
            pinned DESC,           -- Pinned items first
            last_access DESC,      -- Then by last access
            created_at DESC        -- Then by creation time
        LIMIT 5000
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER long_capacity_trigger
AFTER INSERT ON memory_long
FOR EACH ROW EXECUTE FUNCTION enforce_long_capacity_limit();

-- ============================================================================
-- 3. Update existing records with default expires_at
-- ============================================================================

-- Set expires_at for existing mid-term records (90 days from now)
UPDATE memory_mid 
SET expires_at = NOW() + INTERVAL '90 days'
WHERE expires_at IS NULL;

-- Set expires_at for existing long-term records (365 days from now)
UPDATE memory_long 
SET expires_at = NOW() + INTERVAL '365 days'
WHERE expires_at IS NULL;

-- ============================================================================
-- 4. Comments for documentation
-- ============================================================================

COMMENT ON COLUMN memory_mid.expires_at IS 'Expiration timestamp (default 90 days from creation)';
COMMENT ON COLUMN memory_mid.pinned IS 'If true, this item will not be auto-deleted by capacity limits or TTL';
COMMENT ON COLUMN memory_long.expires_at IS 'Expiration timestamp (default 365 days from creation)';
COMMENT ON COLUMN memory_long.pinned IS 'If true, this item will not be auto-deleted by capacity limits or TTL';

COMMENT ON FUNCTION enforce_mid_capacity_limit() IS 'Enforces max 1000 mid-term memory items per tenant (LRU, excluding pinned)';
COMMENT ON FUNCTION enforce_long_capacity_limit() IS 'Enforces max 5000 long-term memory items per tenant (LRU, excluding pinned)';

