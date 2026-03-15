-- Add new memory fields for the rewritten memory system

-- Mid-term: category, access tracking
ALTER TABLE memory_mid ADD COLUMN IF NOT EXISTS category TEXT DEFAULT '';
ALTER TABLE memory_mid ADD COLUMN IF NOT EXISTS access_cnt INTEGER DEFAULT 0;
ALTER TABLE memory_mid ADD COLUMN IF NOT EXISTS last_access TIMESTAMPTZ DEFAULT NOW();

-- Long-term: category, access tracking
ALTER TABLE memory_long ADD COLUMN IF NOT EXISTS category TEXT DEFAULT '';
ALTER TABLE memory_long ADD COLUMN IF NOT EXISTS access_cnt INTEGER DEFAULT 0;
ALTER TABLE memory_long ADD COLUMN IF NOT EXISTS last_access TIMESTAMPTZ DEFAULT NOW();

-- Short-term: add session_key for sliding window (was using 'key' as session key)
ALTER TABLE memory_short ADD COLUMN IF NOT EXISTS session_key TEXT DEFAULT '';

-- Tenant config (JSON blob for per-tenant settings)
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS config JSONB DEFAULT '{}';

-- Full-text search index for memory_mid and memory_long (better than ILIKE)
CREATE INDEX IF NOT EXISTS idx_memory_mid_value_trgm ON memory_mid USING gin (value gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_memory_long_value_trgm ON memory_long USING gin (value gin_trgm_ops);

-- Enable pg_trgm for fuzzy text search (if not already)
CREATE EXTENSION IF NOT EXISTS pg_trgm;
