/** Lightweight plugin-cron-control SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginCronAddResponse,
  type PluginOkResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginCronControlClientOptions,
  PluginCronAddResponse,
  PluginOkResponse as PluginCronControlOkResponse,
};

export { PluginApiClientError as PluginCronControlClientError };

export class PluginCronControlClient {
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
}

export function createPluginCronControlClient(options: PluginApiClientOptions): PluginCronControlClient {
  return new PluginCronControlClient(options);
}
