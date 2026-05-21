/** Lightweight discovery-embeddings SDK facade over the Discovery slice. */
import {
  DiscoveryClient,
  DiscoveryClientError,
  createDiscoveryClient,
  type DiscoveryClientOptions,
  type EmbeddingProvider,
  type EmbeddingProvidersResponse,
  type EmbeddingResponse,
} from "./discovery.js";

export type {
  DiscoveryClientOptions as DiscoveryEmbeddingsClientOptions,
  EmbeddingProvider,
  EmbeddingProvidersResponse,
  EmbeddingResponse,
};

export { DiscoveryClientError as DiscoveryEmbeddingsClientError };

export class DiscoveryEmbeddingsClient {
  private readonly client: DiscoveryClient;

  constructor(options: DiscoveryClientOptions) { this.client = createDiscoveryClient(options); }
  embeddingProviders(): Promise<EmbeddingProvidersResponse> { return this.client.embeddingProviders(); }
  embed(text: string, provider?: string): Promise<EmbeddingResponse> { return this.client.embed(text, provider); }
}

export function createDiscoveryEmbeddingsClient(options: DiscoveryClientOptions): DiscoveryEmbeddingsClient { return new DiscoveryEmbeddingsClient(options); }
