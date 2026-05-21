/** Lightweight plugin-agent-memory SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginAgentMemorySearchResponse,
  type PluginApiClientOptions,
  type PluginOkResponse,
} from "./plugin-api.js";

export type {
  PluginAgentMemorySearchResponse,
  PluginApiClientOptions as PluginAgentMemoryClientOptions,
  PluginOkResponse as PluginAgentMemoryOkResponse,
};

export { PluginApiClientError as PluginAgentMemoryClientError };

export class PluginAgentMemoryClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  search(query: string, topK?: number): Promise<PluginAgentMemorySearchResponse> {
    return this.client.agentMemorySearch(query, topK);
  }

  add(fact: string, source?: string): Promise<PluginOkResponse> {
    return this.client.agentMemoryAdd(fact, source);
  }
}

export function createPluginAgentMemoryClient(options: PluginApiClientOptions): PluginAgentMemoryClient {
  return new PluginAgentMemoryClient(options);
}
