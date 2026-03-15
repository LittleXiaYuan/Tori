-- Yunque Agent Schema
-- Requires: PostgreSQL 15+ with pgvector extension

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Tenants
CREATE TABLE IF NOT EXISTS tenants (
    id          TEXT PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    name        TEXT NOT NULL,
    api_key     TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_tenants_api_key ON tenants(api_key);

-- Short-term memory (session context, auto-expire)
CREATE TABLE IF NOT EXISTS memory_short (
    id          TEXT PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id),
    key         TEXT NOT NULL,
    value       TEXT NOT NULL,
    source      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL,
    UNIQUE(tenant_id, key)
);
CREATE INDEX idx_memory_short_tenant ON memory_short(tenant_id);
CREATE INDEX idx_memory_short_expires ON memory_short(expires_at);

-- Mid-term memory (extracted facts, deduplication by hash)
CREATE TABLE IF NOT EXISTS memory_mid (
    id          TEXT PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id),
    key         TEXT NOT NULL,
    value       TEXT NOT NULL,
    source      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, key)
);
CREATE INDEX idx_memory_mid_tenant ON memory_mid(tenant_id);

-- Long-term memory (knowledge base with vector embeddings)
CREATE TABLE IF NOT EXISTS memory_long (
    id          TEXT PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id),
    key         TEXT NOT NULL,
    value       TEXT NOT NULL,
    source      TEXT,
    embedding   vector(1536),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, key)
);
CREATE INDEX idx_memory_long_tenant ON memory_long(tenant_id);
CREATE INDEX idx_memory_long_embedding ON memory_long USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- Conversations
CREATE TABLE IF NOT EXISTS conversations (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_conversations_tenant ON conversations(tenant_id);

-- Conversation messages
CREATE TABLE IF NOT EXISTS conversation_messages (
    id              SERIAL PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL,
    content         TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_conv_messages_conv ON conversation_messages(conversation_id);

-- Scheduler jobs
CREATE TABLE IF NOT EXISTS scheduler_jobs (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    tenant_id   TEXT,
    prompt      TEXT,
    interval_ms BIGINT NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT true,
    next_run    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
