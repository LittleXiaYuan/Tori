/** Lightweight plugin-memory-write SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginOkResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginMemoryWriteClientOptions,
  PluginOkResponse as PluginMemoryWriteOkResponse,
};

export { PluginApiClientError as PluginMemoryWriteClientError };

export class PluginMemoryWriteClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  set(key: string, value: string): Promise<PluginOkResponse> {
    return this.client.memorySet(key, value);
  }

  delete(key: string): Promise<PluginOkResponse> {
    return this.client.memoryDelete(key);
  }
}

export function createPluginMemoryWriteClient(options: PluginApiClientOptions): PluginMemoryWriteClient {
  return new PluginMemoryWriteClient(options);
}
