/** Lightweight web-search SDK facade over the discovery slice. */
import {
  createDiscoveryClient,
  DiscoveryClient,
  DiscoveryClientError,
  type DiscoveryClientOptions,
  type SearchProvidersResponse,
  type SearchResponse,
  type SearchResult,
} from "./discovery.js";

export type {
  DiscoveryClientOptions as SearchClientOptions,
  SearchProvidersResponse,
  SearchResponse,
  SearchResult,
};

export { DiscoveryClientError as SearchClientError };

export class SearchClient {
  private readonly client: DiscoveryClient;

  constructor(options: DiscoveryClientOptions) {
    this.client = createDiscoveryClient(options);
  }

  search(q: string, options: { limit?: number; provider?: string } = {}): Promise<SearchResponse> {
    return this.client.search(q, options);
  }

  providers(): Promise<SearchProvidersResponse> {
    return this.client.searchProviders();
  }
}

export function createSearchClient(options: DiscoveryClientOptions): SearchClient {
  return new SearchClient(options);
}
