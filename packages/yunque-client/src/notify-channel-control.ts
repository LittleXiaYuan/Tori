/** Lightweight notify-channel-control SDK facade over the Notify slice. */
import {
  createNotifyClient,
  NotifyClient,
  NotifyClientError,
  type NotifyChannel,
  type NotifyClientOptions,
  type NotifyOkResponse,
  type NotifyToggleRequest,
} from "./notify.js";

export type {
  NotifyChannel,
  NotifyClientOptions as NotifyChannelControlClientOptions,
  NotifyOkResponse,
  NotifyToggleRequest,
};

export { NotifyClientError as NotifyChannelControlClientError };

export class NotifyChannelControlClient {
  private readonly client: NotifyClient;

  constructor(options: NotifyClientOptions) {
    this.client = createNotifyClient(options);
  }

  add(channel: NotifyChannel): Promise<NotifyOkResponse> {
    return this.client.addChannel(channel);
  }

  remove(id: string): Promise<NotifyOkResponse> {
    return this.client.removeChannel(id);
  }

  toggle(request: NotifyToggleRequest): Promise<NotifyOkResponse> {
    return this.client.toggleChannel(request);
  }

  test(id: string): Promise<NotifyOkResponse> {
    return this.client.testChannel(id);
  }
}

export function createNotifyChannelControlClient(options: NotifyClientOptions): NotifyChannelControlClient {
  return new NotifyChannelControlClient(options);
}
