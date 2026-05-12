/** Lightweight plugin-cron SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginCronAddResponse,
  type PluginCronListResponse,
  type PluginOkResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginCronClientOptions,
  PluginCronAddResponse,
  PluginCronListResponse,
  PluginOkResponse as PluginCronOkResponse,
};

export { PluginApiClientError as PluginCronClientError };

export class PluginCronClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  add(name: string, expression: string, message: string): Promise<PluginCronAddResponse> {
    return this.client.cronAdd(name, expression, message);
  }

  remove(id: string): Promise<PluginOkResponse> {
    return this.client.cronRemove(id);
  }

  list(plugin?: string): Promise<PluginCronListResponse> {
    return this.client.cronList(plugin);
  }
}

export function createPluginCronClient(options: PluginApiClientOptions): PluginCronClient {
  return new PluginCronClient(options);
}
