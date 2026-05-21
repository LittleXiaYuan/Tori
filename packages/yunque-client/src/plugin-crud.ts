/** Lightweight plugin-crud SDK facade over the Plugins slice. */
import {
  createPluginsClient,
  PluginsClient,
  PluginsClientError,
  type PluginCreateRequest,
  type PluginCreateResponse,
  type PluginDeleteResponse,
  type PluginSkillManifest,
  type PluginsClientOptions,
} from "./plugins.js";

export type {
  PluginCreateRequest,
  PluginCreateResponse,
  PluginDeleteResponse,
  PluginSkillManifest,
  PluginsClientOptions as PluginCrudClientOptions,
};

export { PluginsClientError as PluginCrudClientError };

export class PluginCrudClient {
  private readonly client: PluginsClient;

  constructor(options: PluginsClientOptions) {
    this.client = createPluginsClient(options);
  }

  create(request: PluginCreateRequest): Promise<PluginCreateResponse> {
    return this.client.create(request);
  }

  delete(name: string): Promise<PluginDeleteResponse> {
    return this.client.delete(name);
  }
}

export function createPluginCrudClient(options: PluginsClientOptions): PluginCrudClient {
  return new PluginCrudClient(options);
}
