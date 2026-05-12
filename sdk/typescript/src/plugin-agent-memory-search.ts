/** Lightweight plugin-agent-memory-search SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginAgentMemorySearchResponse,
  type PluginApiClientOptions,
} from "./plugin-api.js";

export type {
  PluginAgentMemorySearchResponse,
  PluginApiClientOptions as PluginAgentMemorySearchClientOptions,
};

export { PluginApiClientError as PluginAgentMemorySearchClientError };

export class PluginAgentMemorySearchClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  search(query: string, topK?: number): Promise<PluginAgentMemorySearchResponse> {
    return this.client.agentMemorySearch(query, topK);
  }
}

export function createPluginAgentMemorySearchClient(options: PluginApiClientOptions): PluginAgentMemorySearchClient {
  return new PluginAgentMemorySearchClient(options);
}
