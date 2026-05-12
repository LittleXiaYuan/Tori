/** Lightweight plugin-create SDK facade over the Plugins slice. */
import {
  createPluginsClient,
  PluginsClient,
  PluginsClientError,
  type PluginCreateRequest,
  type PluginCreateResponse,
  type PluginSkillManifest,
  type PluginsClientOptions,
} from "./plugins.js";

export type {
  PluginCreateRequest,
  PluginCreateResponse,
  PluginSkillManifest,
  PluginsClientOptions as PluginCreateClientOptions,
};

export { PluginsClientError as PluginCreateClientError };

export class PluginCreateClient {
  private readonly client: PluginsClient;

  constructor(options: PluginsClientOptions) {
    this.client = createPluginsClient(options);
  }

  create(request: PluginCreateRequest): Promise<PluginCreateResponse> {
    return this.client.create(request);
  }
}

export function createPluginCreateClient(options: PluginsClientOptions): PluginCreateClient {
  return new PluginCreateClient(options);
}
