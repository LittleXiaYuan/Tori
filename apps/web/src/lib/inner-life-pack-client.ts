import { fetcher } from "./api-core";

export interface CuriosityQuestion {
  question: string;
  category: string;
  priority: number;
  context?: string;
  related_to?: string[];
}

export interface TimelineEntry {
  id: string;
  kind: string;
  actor: string;
  created_at: string;
  payload?: Record<string, unknown>;
}

export interface CuriosityResponse {
  pending: CuriosityQuestion[];
  recent: TimelineEntry[];
}

export interface ReflectionResponse {
  recent: TimelineEntry[];
}

export interface DreamingResponse {
  recent: TimelineEntry[];
}

export interface InnerLifePackClient {
  curiosity(limit?: number): Promise<CuriosityResponse>;
  reflection(limit?: number): Promise<ReflectionResponse>;
  dreaming(limit?: number): Promise<DreamingResponse>;
}

function withLimit(path: string, limit?: number): string {
  if (typeof limit === "number" && limit > 0) {
    return `${path}?limit=${encodeURIComponent(limit)}`;
  }
  return path;
}

export function createInnerLifePackClient(): InnerLifePackClient {
  return {
    curiosity: (limit) => fetcher<CuriosityResponse>(withLimit("/v1/inner-life/curiosity", limit)),
    reflection: (limit) => fetcher<ReflectionResponse>(withLimit("/v1/inner-life/reflection", limit)),
    dreaming: (limit) => fetcher<DreamingResponse>(withLimit("/v1/inner-life/dreaming", limit)),
  };
}
