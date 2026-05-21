/** Lightweight notify-channels SDK facade over the Notify slice. */
import {
  createNotifyClient,
  NotifyClient,
  NotifyClientError,
  type NotifyChannel,
  type NotifyChannelsResponse,
  type NotifyClientOptions,
  type NotifyOkResponse,
  type NotifyToggleRequest,
} from "./notify.js";

export type {
  NotifyChannel,
  NotifyChannelsResponse,
  NotifyClientOptions as NotifyChannelsClientOptions,
  NotifyOkResponse,
  NotifyToggleRequest,
};

export { NotifyClientError as NotifyChannelsClientError };

export class NotifyChannelsClient {
  private readonly client: NotifyClient;

  constructor(options: NotifyClientOptions) {
    this.client = createNotifyClient(options);
  }

  list(): Promise<NotifyChannelsResponse> {
    return this.client.channels();
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

export function createNotifyChannelsClient(options: NotifyClientOptions): NotifyChannelsClient {
  return new NotifyChannelsClient(options);
}
