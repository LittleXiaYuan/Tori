import type { EmotionResult, StickerSuggestion } from "@/lib/api";
import type { SkillSuggestion as SkillGrowthSuggestion } from "@/lib/api-types";
import type { AgentEvent } from "@/components/execution-trace";
import type { AgentAction } from "@/components/chat-extras";
import type {
  BrowserActionArtifactSummary,
} from "@/components/browser-session-card";
import type { BrowserRequirement } from "@/components/browser-connect-card";
import type { MemoryAccess, ReflectionEvent, ContextLayer } from "@/components/cognitive-status-bar";

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

export interface ChatShareFile {
  path: string;
  name: string;
  size?: number;
}

export interface ChatSharePayload {
  title: string;
  message?: string;
  files?: ChatShareFile[];
}

export interface ChatShareReceipt {
  id: string;
  status: "sent" | "failed";
  channelId: string;
  channelName: string;
  channelType: string;
  targetTitle: string;
  sentAt: number;
  shareCode?: string;
  error?: string;
}

export interface Message {
  role: "user" | "assistant";
  content: string;
  id: string;
  model?: string;
  providerId?: string;
  files?: ChatShareFile[];
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
  cognitiveMemories?: MemoryAccess[];
  cognitiveReflections?: ReflectionEvent[];
  cognitiveContextLayers?: ContextLayer[];
  shareReceipts?: ChatShareReceipt[];
}
