/** Lightweight memory-search SDK facade over the Memory slice. */
import {
  createMemoryClient,
  MemoryClient,
  MemoryClientError,
  type MemoryClientOptions,
  type MemoryItem,
  type MemoryLayer,
  type MemorySearchRequest,
  type MemorySearchResponse,
} from "./memory.js";

export type {
  MemoryItem,
  MemoryLayer,
  MemoryClientOptions as MemorySearchClientOptions,
  MemorySearchRequest,
  MemorySearchResponse,
};

export { MemoryClientError as MemorySearchClientError };

export class MemorySearchClient {
  private readonly client: MemoryClient;

  constructor(options: MemoryClientOptions) {
    this.client = createMemoryClient(options);
  }

  search(query: string | MemorySearchRequest, options: Omit<MemorySearchRequest, "query"> = {}): Promise<MemorySearchResponse> {
    const request = typeof query === "string" ? { ...options, query } : query;
    return this.client.search(request);
  }
}

export function createMemorySearchClient(options: MemoryClientOptions): MemorySearchClient {
  return new MemorySearchClient(options);
}
