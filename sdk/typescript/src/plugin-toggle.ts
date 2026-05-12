/** Lightweight plugin-toggle SDK facade over the Plugins slice. */
import {
  createPluginsClient,
  PluginsClient,
  PluginsClientError,
  type PluginsClientOptions,
  type PluginToggleResponse,
} from "./plugins.js";

export type {
  PluginsClientOptions as PluginToggleClientOptions,
  PluginToggleResponse,
};

export { PluginsClientError as PluginToggleClientError };

export class PluginToggleClient {
  private readonly client: PluginsClient;

  constructor(options: PluginsClientOptions) {
    this.client = createPluginsClient(options);
  }

  toggle(name: string, enabled: boolean): Promise<PluginToggleResponse> {
    return this.client.toggle(name, enabled);
  }
}

export function createPluginToggleClient(options: PluginsClientOptions): PluginToggleClient {
  return new PluginToggleClient(options);
}
