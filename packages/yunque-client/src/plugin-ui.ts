/** Lightweight plugin-ui SDK facade over the Plugins slice. */
import {
  createPluginsClient,
  PluginsClient,
  PluginsClientError,
  type PluginsClientOptions,
  type PluginUIResponse,
} from "./plugins.js";

export type {
  PluginsClientOptions as PluginUIClientOptions,
  PluginUIResponse,
};

export { PluginsClientError as PluginUIClientError };

export class PluginUIClient {
  private readonly client: PluginsClient;

  constructor(options: PluginsClientOptions) {
    this.client = createPluginsClient(options);
  }

  ui(): Promise<PluginUIResponse> {
    return this.client.ui();
  }
}

export function createPluginUIClient(options: PluginsClientOptions): PluginUIClient {
  return new PluginUIClient(options);
}
