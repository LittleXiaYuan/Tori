/** Lightweight knowledge-sources SDK facade over the Knowledge slice. */
import {
  createKnowledgeClient,
  KnowledgeClient,
  KnowledgeClientError,
  type KnowledgeClientOptions,
  type KnowledgeDeleteResponse,
  type KnowledgeMutationResponse,
  type KnowledgeSource,
  type KnowledgeSourcesResponse,
  type KnowledgeStats,
  type KnowledgeUpdateSourceRequest,
} from "./knowledge.js";

export type {
  KnowledgeClientOptions as KnowledgeSourcesClientOptions,
  KnowledgeDeleteResponse,
  KnowledgeMutationResponse,
  KnowledgeSource,
  KnowledgeSourcesResponse,
  KnowledgeStats,
  KnowledgeUpdateSourceRequest,
};

export { KnowledgeClientError as KnowledgeSourcesClientError };

export class KnowledgeSourcesClient {
  private readonly client: KnowledgeClient;

  constructor(options: KnowledgeClientOptions) {
    this.client = createKnowledgeClient(options);
  }

  stats(): Promise<KnowledgeStats> {
    return this.client.stats();
  }

  list(): Promise<KnowledgeSourcesResponse> {
    return this.client.sources();
  }

  update(request: KnowledgeUpdateSourceRequest): Promise<KnowledgeMutationResponse> {
    return this.client.updateSource(request);
  }

  delete(id: string): Promise<KnowledgeDeleteResponse> {
    return this.client.deleteSource(id);
  }
}

export function createKnowledgeSourcesClient(options: KnowledgeClientOptions): KnowledgeSourcesClient {
  return new KnowledgeSourcesClient(options);
}
