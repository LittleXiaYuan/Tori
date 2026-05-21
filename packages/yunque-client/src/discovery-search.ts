/** Lightweight discovery-search SDK facade over the Discovery slice. */
import {
  DiscoveryClient,
  DiscoveryClientError,
  createDiscoveryClient,
  type DiscoveryClientOptions,
  type SearchProvidersResponse,
  type SearchResponse,
  type SearchResult,
} from "./discovery.js";

export type {
  DiscoveryClientOptions as DiscoverySearchClientOptions,
  SearchProvidersResponse,
  SearchResponse,
  SearchResult,
};

export { DiscoveryClientError as DiscoverySearchClientError };

export class DiscoverySearchClient {
  private readonly client: DiscoveryClient;

  constructor(options: DiscoveryClientOptions) { this.client = createDiscoveryClient(options); }
  search(q: string, options: { limit?: number; provider?: string } = {}): Promise<SearchResponse> { return this.client.search(q, options); }
  searchProviders(): Promise<SearchProvidersResponse> { return this.client.searchProviders(); }
}

export function createDiscoverySearchClient(options: DiscoveryClientOptions): DiscoverySearchClient { return new DiscoverySearchClient(options); }
