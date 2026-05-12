/** Lightweight plugin-reload SDK facade over the Plugins slice. */
import {
  createPluginsClient,
  PluginsClient,
  PluginsClientError,
  type PluginsClientOptions,
  type PluginReloadResponse,
} from "./plugins.js";

export type {
  PluginsClientOptions as PluginReloadClientOptions,
  PluginReloadResponse,
};

export { PluginsClientError as PluginReloadClientError };

export class PluginReloadClient {
  private readonly client: PluginsClient;

  constructor(options: PluginsClientOptions) {
    this.client = createPluginsClient(options);
  }

  reload(): Promise<PluginReloadResponse> {
    return this.client.reload();
  }
}

export function createPluginReloadClient(options: PluginsClientOptions): PluginReloadClient {
  return new PluginReloadClient(options);
}
