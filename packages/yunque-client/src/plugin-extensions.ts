/** Lightweight plugin-extensions SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginExtensionRegisterResponse,
  type PluginExtensionsResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginExtensionsClientOptions,
  PluginExtensionRegisterResponse,
  PluginExtensionsResponse,
};

export { PluginApiClientError as PluginExtensionsClientError };

export class PluginExtensionsClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  registerProvider(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> {
    return this.client.registerProvider(config);
  }

  registerChannel(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> {
    return this.client.registerChannel(config);
  }

  registerSearch(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> {
    return this.client.registerSearch(config);
  }

  registerGuardrail(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> {
    return this.client.registerGuardrail(config);
  }

  registerEmbedding(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> {
    return this.client.registerEmbedding(config);
  }

  registerSpeech(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> {
    return this.client.registerSpeech(config);
  }

  list(): Promise<PluginExtensionsResponse> {
    return this.client.extensions();
  }
}

export function createPluginExtensionsClient(options: PluginApiClientOptions): PluginExtensionsClient {
  return new PluginExtensionsClient(options);
}
