/** Lightweight plugin-send SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginSendResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginSendClientOptions,
  PluginSendResponse,
};

export { PluginApiClientError as PluginSendClientError };

export class PluginSendClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  send(channel: string, target: string, content: string, format?: string): Promise<PluginSendResponse> {
    return this.client.send(channel, target, content, format);
  }
}

export function createPluginSendClient(options: PluginApiClientOptions): PluginSendClient {
  return new PluginSendClient(options);
}
