/** Lightweight knowledge-source-control SDK facade over knowledge source mutations. */
import {
  createKnowledgeClient,
  KnowledgeClient,
  KnowledgeClientError,
  type KnowledgeClientOptions,
  type KnowledgeDeleteResponse,
  type KnowledgeMutationResponse,
  type KnowledgeSource,
  type KnowledgeStats,
  type KnowledgeUpdateSourceRequest,
} from "./knowledge.js";

export type {
  KnowledgeClientOptions as KnowledgeSourceControlClientOptions,
  KnowledgeDeleteResponse,
  KnowledgeMutationResponse,
  KnowledgeSource,
  KnowledgeStats,
  KnowledgeUpdateSourceRequest,
};

export { KnowledgeClientError as KnowledgeSourceControlClientError };

export class KnowledgeSourceControlClient {
  private readonly client: KnowledgeClient;

  constructor(options: KnowledgeClientOptions) { this.client = createKnowledgeClient(options); }
  update(request: KnowledgeUpdateSourceRequest): Promise<KnowledgeMutationResponse> { return this.client.updateSource(request); }
  delete(id: string): Promise<KnowledgeDeleteResponse> { return this.client.deleteSource(id); }
}

export function createKnowledgeSourceControlClient(options: KnowledgeClientOptions): KnowledgeSourceControlClient { return new KnowledgeSourceControlClient(options); }
