/** Lightweight plugin-files SDK facade over the Plugins slice. */
import {
  createPluginsClient,
  PluginsClient,
  PluginsClientError,
  type PluginFile,
  type PluginFilesResponse,
  type PluginFileSaveResponse,
  type PluginsClientOptions,
} from "./plugins.js";

export type {
  PluginFile,
  PluginFilesResponse,
  PluginFileSaveResponse,
  PluginsClientOptions as PluginFilesClientOptions,
};

export { PluginsClientError as PluginFilesClientError };

export class PluginFilesClient {
  private readonly client: PluginsClient;

  constructor(options: PluginsClientOptions) {
    this.client = createPluginsClient(options);
  }

  files(name: string): Promise<PluginFilesResponse> {
    return this.client.files(name);
  }

  saveFile(name: string, file: string, content: string, plugin?: string): Promise<PluginFileSaveResponse> {
    return this.client.saveFile(name, file, content, plugin);
  }
}

export function createPluginFilesClient(options: PluginsClientOptions): PluginFilesClient {
  return new PluginFilesClient(options);
}
