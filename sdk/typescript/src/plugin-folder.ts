/** Lightweight plugin-folder SDK facade over the Plugins slice. */
import {
  createPluginsClient,
  PluginsClient,
  PluginsClientError,
  type PluginOpenFolderResponse,
  type PluginsClientOptions,
} from "./plugins.js";

export type {
  PluginOpenFolderResponse,
  PluginsClientOptions as PluginFolderClientOptions,
};

export { PluginsClientError as PluginFolderClientError };

export class PluginFolderClient {
  private readonly client: PluginsClient;

  constructor(options: PluginsClientOptions) {
    this.client = createPluginsClient(options);
  }

  openFolder(name?: string): Promise<PluginOpenFolderResponse> {
    return this.client.openFolder(name);
  }
}

export function createPluginFolderClient(options: PluginsClientOptions): PluginFolderClient {
  return new PluginFolderClient(options);
}
