/** Lightweight knowledge-source-read SDK facade over knowledge source reads. */
import {
  createKnowledgeClient,
  KnowledgeClient,
  KnowledgeClientError,
  type KnowledgeClientOptions,
  type KnowledgeSource,
  type KnowledgeSourcesResponse,
  type KnowledgeStats,
} from "./knowledge.js";

export type {
  KnowledgeClientOptions as KnowledgeSourceReadClientOptions,
  KnowledgeSource,
  KnowledgeSourcesResponse,
  KnowledgeStats,
};

export { KnowledgeClientError as KnowledgeSourceReadClientError };

export class KnowledgeSourceReadClient {
  private readonly client: KnowledgeClient;

  constructor(options: KnowledgeClientOptions) { this.client = createKnowledgeClient(options); }
  stats(): Promise<KnowledgeStats> { return this.client.stats(); }
  list(): Promise<KnowledgeSourcesResponse> { return this.client.sources(); }
}

export function createKnowledgeSourceReadClient(options: KnowledgeClientOptions): KnowledgeSourceReadClient { return new KnowledgeSourceReadClient(options); }
