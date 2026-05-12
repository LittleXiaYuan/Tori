/** Lightweight notify-channel-read SDK facade over the Notify slice. */
import {
  createNotifyClient,
  NotifyClient,
  NotifyClientError,
  type NotifyChannelsResponse,
  type NotifyClientOptions,
} from "./notify.js";

export type {
  NotifyChannelsResponse,
  NotifyClientOptions as NotifyChannelReadClientOptions,
};

export { NotifyClientError as NotifyChannelReadClientError };

export class NotifyChannelReadClient {
  private readonly client: NotifyClient;

  constructor(options: NotifyClientOptions) {
    this.client = createNotifyClient(options);
  }

  list(): Promise<NotifyChannelsResponse> {
    return this.client.channels();
  }
}

export function createNotifyChannelReadClient(options: NotifyClientOptions): NotifyChannelReadClient {
  return new NotifyChannelReadClient(options);
}
