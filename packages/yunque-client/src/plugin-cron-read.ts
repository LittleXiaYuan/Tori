/** Lightweight plugin-cron-read SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginCronListResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginCronReadClientOptions,
  PluginCronListResponse,
};

export { PluginApiClientError as PluginCronReadClientError };

export class PluginCronReadClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  list(plugin?: string): Promise<PluginCronListResponse> {
    return this.client.cronList(plugin);
  }
}

export function createPluginCronReadClient(options: PluginApiClientOptions): PluginCronReadClient {
  return new PluginCronReadClient(options);
}
