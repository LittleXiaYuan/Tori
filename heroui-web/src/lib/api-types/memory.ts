// ══════════════════════════════════════════════════════════════════════════
// Memory / Knowledge / Graph Types
// ══════════════════════════════════════════════════════════════════════════
// The agent's persistent and semi-persistent state — RAG knowledge base
// chunks/sources, the entity-relation graph, and unified memory search hits.

// --- Knowledge Base ---

export interface KBChunk {
  id: string;
  source_id: string;
  content: string;
  index: number;
  metadata?: Record<string, string>;
}

export interface KBSource {
  id: string;
  name: string;
  type: string;
  path?: string;
  trigger?: string;
  chunk_count: number;
  added_at: string;
}

export interface KBImportTreeNode {
  title: string;
  url?: string;
  path?: string;
  children?: KBImportTreeNode[];
}

export interface KBStats {
  sources: number;
  chunks: number;
  total_chars: number;
}

// --- Memory search ---

export interface MemorySearchResult {
  id: string;
  content: string;
  score: number;
  metadata?: Record<string, string>;
  created_at: string;
}

// --- Knowledge Graph ---

export interface GraphEntity {
  id: string;
  name: string;
  type: string;
  properties?: Record<string, string>;
  mentions?: number;
  created_at: string;
}

export interface GraphRelation {
  id: string;
  from_id: string;
  to_id: string;
  type: string;
  weight?: number;
  context?: string;
  created_at?: string;
}

export interface GraphStats {
  entities: number;
  relations: number;
  entity_types: Record<string, number>;
  relation_types: Record<string, number>;
}
