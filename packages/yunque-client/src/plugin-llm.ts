/** Lightweight plugin-llm SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginLLMMessage,
  type PluginLLMRequest,
  type PluginLLMResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginLLMClientOptions,
  PluginLLMMessage,
  PluginLLMRequest,
  PluginLLMResponse,
};

export { PluginApiClientError as PluginLLMClientError };

export class PluginLLMClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  complete(request: PluginLLMRequest): Promise<PluginLLMResponse> {
    return this.client.llm(request);
  }
}

export function createPluginLLMClient(options: PluginApiClientOptions): PluginLLMClient {
  return new PluginLLMClient(options);
}
