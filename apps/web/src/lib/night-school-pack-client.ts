import { fetcher } from "./api-core";

export interface DreamEntry {
  id: string;
  created_at: string;
  tenant_id?: string;
  thoughts_generated: number;
  explorations_run: number;
  facts_discovered: number;
  skills_suggested: number;
}

export interface DistillEntry {
  id: string;
  key: string;
  content: string;
  source?: string;
  confidence: number;
  created_at: string;
  task_id?: string;
}

export interface TraitEntry {
  id: string;
  dimension: string;
  preference: string;
  confidence: number;
  source?: string;
  created_at: string;
  updated_at: string;
  hit_count: number;
}

export interface DreamsResponse {
  recent: DreamEntry[];
}

export interface DistillResponse {
  rules: DistillEntry[];
  patterns: DistillEntry[];
  tool_insights: DistillEntry[];
}

export interface TraitsResponse {
  traits: TraitEntry[];
}

export interface NightSchoolPackClient {
  dreams(limit?: number): Promise<DreamsResponse>;
  distill(limit?: number): Promise<DistillResponse>;
  traits(limit?: number): Promise<TraitsResponse>;
  /** Forget a learned trait the user disagrees with (persists on the server). */
  forgetTrait(dimension: string, preference: string): Promise<{ ok: boolean }>;
}

function withLimit(path: string, limit?: number): string {
  if (typeof limit === "number" && limit > 0) {
    return `${path}?limit=${encodeURIComponent(limit)}`;
  }
  return path;
}

export function createNightSchoolPackClient(): NightSchoolPackClient {
  return {
    dreams: (limit) => fetcher<DreamsResponse>(withLimit("/v1/night-school/dreams", limit)),
    distill: (limit) => fetcher<DistillResponse>(withLimit("/v1/night-school/distill", limit)),
    traits: (limit) => fetcher<TraitsResponse>(withLimit("/v1/night-school/traits", limit)),
    forgetTrait: (dimension, preference) => fetcher<{ ok: boolean }>("/v1/night-school/traits/forget", {
      method: "POST",
      body: JSON.stringify({ dimension, preference }),
    }),
  };
}
