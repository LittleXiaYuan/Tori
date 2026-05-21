/** Lightweight plugin-memory SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginMemoryListResponse,
  type PluginMemorySearchResponse,
  type PluginMemoryValueResponse,
  type PluginOkResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginMemoryClientOptions,
  PluginMemoryListResponse,
  PluginMemorySearchResponse,
  PluginMemoryValueResponse,
  PluginOkResponse as PluginMemoryOkResponse,
};

export { PluginApiClientError as PluginMemoryClientError };

export class PluginMemoryClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  get(key: string): Promise<PluginMemoryValueResponse> {
    return this.client.memoryGet(key);
  }

  set(key: string, value: string): Promise<PluginOkResponse> {
    return this.client.memorySet(key, value);
  }

  delete(key: string): Promise<PluginOkResponse> {
    return this.client.memoryDelete(key);
  }

  list(prefix?: string): Promise<PluginMemoryListResponse> {
    return this.client.memoryList(prefix);
  }

  search(query: string, limit?: number): Promise<PluginMemorySearchResponse> {
    return this.client.memorySearch(query, limit);
  }
}

export function createPluginMemoryClient(options: PluginApiClientOptions): PluginMemoryClient {
  return new PluginMemoryClient(options);
}
