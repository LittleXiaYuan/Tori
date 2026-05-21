/** Lightweight plugin-control SDK facade over the Plugins slice. */
import {
  createPluginsClient,
  PluginsClient,
  PluginsClientError,
  type PluginOpenFolderResponse,
  type PluginReloadResponse,
  type PluginToggleResponse,
  type PluginUIResponse,
  type PluginsClientOptions,
} from "./plugins.js";

export type {
  PluginOpenFolderResponse,
  PluginReloadResponse,
  PluginToggleResponse,
  PluginUIResponse,
  PluginsClientOptions as PluginControlClientOptions,
};

export { PluginsClientError as PluginControlClientError };

export class PluginControlClient {
  private readonly client: PluginsClient;

  constructor(options: PluginsClientOptions) {
    this.client = createPluginsClient(options);
  }

  toggle(name: string, enabled: boolean): Promise<PluginToggleResponse> {
    return this.client.toggle(name, enabled);
  }

  ui(): Promise<PluginUIResponse> {
    return this.client.ui();
  }

  reload(): Promise<PluginReloadResponse> {
    return this.client.reload();
  }

  openFolder(name?: string): Promise<PluginOpenFolderResponse> {
    return this.client.openFolder(name);
  }
}

export function createPluginControlClient(options: PluginsClientOptions): PluginControlClient {
  return new PluginControlClient(options);
}
