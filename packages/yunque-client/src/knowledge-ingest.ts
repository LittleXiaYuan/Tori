/** Lightweight knowledge-ingest SDK facade over the Knowledge slice. */
import {
  createKnowledgeClient,
  KnowledgeClient,
  KnowledgeClientError,
  type KnowledgeClientOptions,
  type KnowledgeIngestRequest,
  type KnowledgeMutationResponse,
  type KnowledgeSource,
  type KnowledgeStats,
} from "./knowledge.js";

export type {
  KnowledgeClientOptions as KnowledgeIngestClientOptions,
  KnowledgeIngestRequest,
  KnowledgeMutationResponse,
  KnowledgeSource,
  KnowledgeStats,
};

export { KnowledgeClientError as KnowledgeIngestClientError };

export type KnowledgeIngestTextOptions = Omit<KnowledgeIngestRequest, "content">;

export class KnowledgeIngestClient {
  private readonly client: KnowledgeClient;

  constructor(options: KnowledgeClientOptions) {
    this.client = createKnowledgeClient(options);
  }

  ingest(request: KnowledgeIngestRequest): Promise<KnowledgeMutationResponse> {
    return this.client.ingest(request);
  }

  ingestText(content: string, options: KnowledgeIngestTextOptions = {}): Promise<KnowledgeMutationResponse> {
    return this.client.ingest({ ...options, content });
  }
}

export function createKnowledgeIngestClient(options: KnowledgeClientOptions): KnowledgeIngestClient {
  return new KnowledgeIngestClient(options);
}
