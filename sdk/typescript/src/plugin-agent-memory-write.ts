/** Lightweight plugin-agent-memory-write SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginOkResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginAgentMemoryWriteClientOptions,
  PluginOkResponse as PluginAgentMemoryWriteOkResponse,
};

export { PluginApiClientError as PluginAgentMemoryWriteClientError };

export class PluginAgentMemoryWriteClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  add(fact: string, source?: string): Promise<PluginOkResponse> {
    return this.client.agentMemoryAdd(fact, source);
  }
}

export function createPluginAgentMemoryWriteClient(options: PluginApiClientOptions): PluginAgentMemoryWriteClient {
  return new PluginAgentMemoryWriteClient(options);
}
