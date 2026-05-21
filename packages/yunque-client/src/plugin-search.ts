/** Lightweight plugin-search SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginSearchResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginSearchClientOptions,
  PluginSearchResponse,
};

export { PluginApiClientError as PluginSearchClientError };

export class PluginSearchClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  search(query: string, limit?: number): Promise<PluginSearchResponse> {
    return this.client.search(query, limit);
  }
}

export function createPluginSearchClient(options: PluginApiClientOptions): PluginSearchClient {
  return new PluginSearchClient(options);
}
