// ══════════════════════════════════════════════════════════════════════════
// Chat / Persona / Emotion / Reverie Types
// ══════════════════════════════════════════════════════════════════════════
// Conversation-facing state: what the user sees and feels in chat — emotion,
// stickers, persona personality / presets / modes / memory blocks, and the
// reverie (idle thought) loop.

export interface EmotionResult {
  emotion: string;
  confidence: number;
  source: string;
}

export interface StickerSuggestion {
  package_id: string;
  sticker_id: string;
  platform: string;
  emotion: string;
  file_id?: string;
  set_name?: string;
  cdnurl?: string;
  emoji?: string;
}

export interface ChatResponse {
  reply: string;
  skills_used: string[];
  steps: number;
  emotion?: EmotionResult;
  sticker_suggestion?: StickerSuggestion;
  sticker_suggestions?: Record<string, StickerSuggestion>;
  plan?: Array<{
    id: number;
    action: string;
    skill: string;
    status: string;
    result?: string;
    error?: string;
  }>;
}

// --- Persona / Presets ---

export interface PersonaSkill {
  name: string;
  description: string;
  content: string;
  enabled: boolean;
}

export interface PresetInfo {
  id: string;
  name: string;
  description: string;
  tone: string;
  style: string;
  greeting: string;
  system_note: string;
  features?: Record<string, boolean>;
}

export interface EmotionHistoryEntry {
  timestamp: string;
  session_id: string;
  emotion: string;
  confidence: number;
  source: string;
  trigger?: string;
  created_at?: string;
}

export interface PersonaMemoryBlock {
  id: string;
  content: string;
  label: string;
  max_chars: number;
  read_only: boolean;
  created_at: string;
  updated_at: string;
  version: number;
}

export interface PersonaMemoryEditRequest {
  id: string;
  label: string;
  content: string; // empty means delete
}

export interface PersonaMode {
  id: string;
  name: string;
  description: string;
  active: boolean;
}

// --- Reverie ---

export interface ReverieAction {
  type: string;    // "write_memory" | "create_task" | "update_profile"
  key: string;
  value: string;
}

export interface ActionRecord {
  thought_id: string;
  action: ReverieAction;
  success: boolean;
  error?: string;
  at: string;
}

export interface ReverieThought {
  id: string;
  content: string;
  category: string;
  significance: number;
  trigger: string;
  timestamp: string;
  delivered: boolean;
  delivered_at?: string;
  actions?: ReverieAction[];
}

export interface ReverieStats {
  total_thoughts: number;
  delivered: number;
  avg_significance: number;
  categories: Record<string, number>;
  last_thought_at?: string;
  uptime_seconds: number;
}

export interface ReverieConfig {
  enabled: boolean;
  interval_minutes: number;
  min_significance: number;
  quiet_start: number;
  quiet_end: number;
}
