/** Lightweight plugin-memory-read SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginMemoryListResponse,
  type PluginMemorySearchResponse,
  type PluginMemoryValueResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginMemoryReadClientOptions,
  PluginMemoryListResponse,
  PluginMemorySearchResponse,
  PluginMemoryValueResponse,
};

export { PluginApiClientError as PluginMemoryReadClientError };

export class PluginMemoryReadClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  get(key: string): Promise<PluginMemoryValueResponse> {
    return this.client.memoryGet(key);
  }

  list(prefix?: string): Promise<PluginMemoryListResponse> {
    return this.client.memoryList(prefix);
  }

  search(query: string, limit?: number): Promise<PluginMemorySearchResponse> {
    return this.client.memorySearch(query, limit);
  }
}

export function createPluginMemoryReadClient(options: PluginApiClientOptions): PluginMemoryReadClient {
  return new PluginMemoryReadClient(options);
}
