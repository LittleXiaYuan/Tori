/** Lightweight plugin-catalog SDK facade over the Plugins slice. */
import {
  createPluginsClient,
  PluginsClient,
  PluginsClientError,
  type PluginManifest,
  type PluginSkillManifest,
  type PluginsClientOptions,
  type PluginsListResponse,
} from "./plugins.js";

export type {
  PluginManifest,
  PluginSkillManifest,
  PluginsClientOptions as PluginCatalogClientOptions,
  PluginsListResponse,
};

export { PluginsClientError as PluginCatalogClientError };

export class PluginCatalogClient {
  private readonly client: PluginsClient;

  constructor(options: PluginsClientOptions) {
    this.client = createPluginsClient(options);
  }

  list(): Promise<PluginsListResponse> {
    return this.client.list();
  }
}

export function createPluginCatalogClient(options: PluginsClientOptions): PluginCatalogClient {
  return new PluginCatalogClient(options);
}
