/** Lightweight plugin-file-read SDK facade over the Plugins slice. */
import {
  createPluginsClient,
  PluginsClient,
  PluginsClientError,
  type PluginFile,
  type PluginFilesResponse,
  type PluginsClientOptions,
} from "./plugins.js";

export type {
  PluginFile,
  PluginFilesResponse,
  PluginsClientOptions as PluginFileReadClientOptions,
};

export { PluginsClientError as PluginFileReadClientError };

export class PluginFileReadClient {
  private readonly client: PluginsClient;

  constructor(options: PluginsClientOptions) {
    this.client = createPluginsClient(options);
  }

  files(name: string): Promise<PluginFilesResponse> {
    return this.client.files(name);
  }
}

export function createPluginFileReadClient(options: PluginsClientOptions): PluginFileReadClient {
  return new PluginFileReadClient(options);
}
