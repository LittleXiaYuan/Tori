/** Lightweight plugin-file-save SDK facade over the Plugins slice. */
import {
  createPluginsClient,
  PluginsClient,
  PluginsClientError,
  type PluginFileSaveResponse,
  type PluginsClientOptions,
} from "./plugins.js";

export type {
  PluginFileSaveResponse,
  PluginsClientOptions as PluginFileSaveClientOptions,
};

export { PluginsClientError as PluginFileSaveClientError };

export class PluginFileSaveClient {
  private readonly client: PluginsClient;

  constructor(options: PluginsClientOptions) {
    this.client = createPluginsClient(options);
  }

  saveFile(name: string, file: string, content: string, plugin?: string): Promise<PluginFileSaveResponse> {
    return this.client.saveFile(name, file, content, plugin);
  }
}

export function createPluginFileSaveClient(options: PluginsClientOptions): PluginFileSaveClient {
  return new PluginFileSaveClient(options);
}
