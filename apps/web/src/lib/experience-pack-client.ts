import { fetcher } from "./api-core";

export interface Recommendation {
  item_id: string;
  score: number;
  reason: string;
  confidence: number;
}

export interface ItemEntry {
  id: string;
  category: string;
  tags?: string[];
  uses: number;
  successes: number;
  failures: number;
  avg_rating: number;
  last_used: string;
}

export interface ScoredLabel {
  label: string;
  score: number;
}

export interface EvaluationEntry {
  id: string;
  task_id: string;
  created_at: string;
  quality_score: number;
  goal_achieved: number;
  efficiency: number;
  reasoning?: string;
  suggestions?: string[];
  side_effects?: string[];
  should_distill: boolean;
}

export interface RecommendationsResponse {
  recommendations: Recommendation[];
  context?: string;
}

export interface ItemsResponse {
  items: ItemEntry[];
}

export interface PreferencesResponse {
  preferred_categories: ScoredLabel[];
  preferred_tags: ScoredLabel[];
  avoid_categories: ScoredLabel[];
  interaction_count: number;
}

export interface EvaluationsResponse {
  recent: EvaluationEntry[];
}

export interface ExperiencePackClient {
  recommendations(limit?: number, context?: string): Promise<RecommendationsResponse>;
  items(): Promise<ItemsResponse>;
  preferences(): Promise<PreferencesResponse>;
  evaluations(limit?: number): Promise<EvaluationsResponse>;
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const parts: string[] = [];
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === "") continue;
    parts.push(`${k}=${encodeURIComponent(v)}`);
  }
  return parts.length ? `?${parts.join("&")}` : "";
}

export function createExperiencePackClient(): ExperiencePackClient {
  return {
    recommendations: (limit, context) =>
      fetcher<RecommendationsResponse>(`/v1/experience/recommendations${buildQuery({ limit, context })}`),
    items: () => fetcher<ItemsResponse>("/v1/experience/items"),
    preferences: () => fetcher<PreferencesResponse>("/v1/experience/preferences"),
    evaluations: (limit) =>
      fetcher<EvaluationsResponse>(`/v1/experience/evaluations${buildQuery({ limit })}`),
  };
}
