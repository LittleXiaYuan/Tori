/** Lightweight knowledge-search SDK facade over the Knowledge slice. */
import {
  createKnowledgeClient,
  KnowledgeClient,
  KnowledgeClientError,
  type KnowledgeChunk,
  type KnowledgeClientOptions,
  type KnowledgeSearchRequest,
  type KnowledgeSearchResponse,
} from "./knowledge.js";

export type {
  KnowledgeChunk,
  KnowledgeClientOptions as KnowledgeSearchClientOptions,
  KnowledgeSearchRequest,
  KnowledgeSearchResponse,
};

export { KnowledgeClientError as KnowledgeSearchClientError };

export class KnowledgeSearchClient {
  private readonly client: KnowledgeClient;

  constructor(options: KnowledgeClientOptions) {
    this.client = createKnowledgeClient(options);
  }

  search(query: string | KnowledgeSearchRequest, options: Omit<KnowledgeSearchRequest, "query"> = {}): Promise<KnowledgeSearchResponse> {
    const request = typeof query === "string" ? { ...options, query } : query;
    return this.client.search(request);
  }
}

export function createKnowledgeSearchClient(options: KnowledgeClientOptions): KnowledgeSearchClient {
  return new KnowledgeSearchClient(options);
}
