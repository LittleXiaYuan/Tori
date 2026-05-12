/** Lightweight plugin-extensions-list SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginExtensionsResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginExtensionsListClientOptions,
  PluginExtensionsResponse,
};

export { PluginApiClientError as PluginExtensionsListClientError };

export class PluginExtensionsListClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  list(): Promise<PluginExtensionsResponse> {
    return this.client.extensions();
  }
}

export function createPluginExtensionsListClient(options: PluginApiClientOptions): PluginExtensionsListClient {
  return new PluginExtensionsListClient(options);
}
