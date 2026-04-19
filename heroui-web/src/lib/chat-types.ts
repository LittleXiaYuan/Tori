import type { EmotionResult, StickerSuggestion } from "@/lib/api";
import type { SkillSuggestion as SkillGrowthSuggestion } from "@/lib/api-types";
import type { AgentEvent } from "@/components/execution-trace";
import type { AgentAction } from "@/components/chat-extras";
import type {
  BrowserActionArtifactSummary,
} from "@/components/browser-session-card";
import type { BrowserRequirement } from "@/components/browser-connect-card";

export interface Suggestion {
  type: "followup" | "save_skill";
  label: string;
  icon?: string;
}

export interface SandboxInfo {
  sandbox_id: string;
  stream_url?: string;
  created_at?: string;
  message?: string;
}

export interface Message {
  role: "user" | "assistant";
  content: string;
  id: string;
  emotion?: EmotionResult;
  sticker?: StickerSuggestion;
  stickers?: Record<string, StickerSuggestion>;
  skills_used?: string[];
  actions?: AgentAction[];
  traceEvents?: AgentEvent[];
  suggestions?: Suggestion[];
  images?: string[];
  reasoning?: string;
  reasoningStartMs?: number;
  reasoningEndMs?: number;
  browserSummary?: BrowserActionArtifactSummary;
  browserRequirement?: BrowserRequirement;
  skillSuggestions?: SkillGrowthSuggestion[];
  contextLayers?: string[];
  sandbox?: SandboxInfo;
  airiSynced?: boolean;
  airiEmotion?: string;
  timestamp?: number;
}
