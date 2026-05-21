/** Lightweight plugin-delete SDK facade over the Plugins slice. */
import {
  createPluginsClient,
  PluginsClient,
  PluginsClientError,
  type PluginDeleteResponse,
  type PluginsClientOptions,
} from "./plugins.js";

export type {
  PluginDeleteResponse,
  PluginsClientOptions as PluginDeleteClientOptions,
};

export { PluginsClientError as PluginDeleteClientError };

export class PluginDeleteClient {
  private readonly client: PluginsClient;

  constructor(options: PluginsClientOptions) {
    this.client = createPluginsClient(options);
  }

  delete(name: string): Promise<PluginDeleteResponse> {
    return this.client.delete(name);
  }
}

export function createPluginDeleteClient(options: PluginsClientOptions): PluginDeleteClient {
  return new PluginDeleteClient(options);
}
