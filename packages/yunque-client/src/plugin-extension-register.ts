/** Lightweight plugin-extension-register SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginExtensionRegisterResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginExtensionRegisterClientOptions,
  PluginExtensionRegisterResponse,
};

export { PluginApiClientError as PluginExtensionRegisterClientError };

export class PluginExtensionRegisterClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  provider(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> {
    return this.client.registerProvider(config);
  }

  channel(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> {
    return this.client.registerChannel(config);
  }

  search(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> {
    return this.client.registerSearch(config);
  }

  guardrail(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> {
    return this.client.registerGuardrail(config);
  }

  embedding(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> {
    return this.client.registerEmbedding(config);
  }

  speech(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> {
    return this.client.registerSpeech(config);
  }
}

export function createPluginExtensionRegisterClient(options: PluginApiClientOptions): PluginExtensionRegisterClient {
  return new PluginExtensionRegisterClient(options);
}
