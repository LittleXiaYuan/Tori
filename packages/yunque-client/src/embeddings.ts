/** Lightweight embeddings SDK facade over the discovery slice. */
import {
  createDiscoveryClient,
  DiscoveryClient,
  DiscoveryClientError,
  type DiscoveryClientOptions,
  type EmbeddingProvider,
  type EmbeddingProvidersResponse,
  type EmbeddingResponse,
} from "./discovery.js";

export type {
  DiscoveryClientOptions as EmbeddingsClientOptions,
  EmbeddingProvider,
  EmbeddingProvidersResponse,
  EmbeddingResponse,
};

export { DiscoveryClientError as EmbeddingsClientError };

export class EmbeddingsClient {
  private readonly client: DiscoveryClient;

  constructor(options: DiscoveryClientOptions) {
    this.client = createDiscoveryClient(options);
  }

  providers(): Promise<EmbeddingProvidersResponse> {
    return this.client.embeddingProviders();
  }

  embed(text: string, provider?: string): Promise<EmbeddingResponse> {
    return this.client.embed(text, provider);
  }
}

export function createEmbeddingsClient(options: DiscoveryClientOptions): EmbeddingsClient {
  return new EmbeddingsClient(options);
}
