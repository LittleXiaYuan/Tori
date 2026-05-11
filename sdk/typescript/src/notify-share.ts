/** Lightweight notify-share SDK facade over the Notify slice. */
import {
  createNotifyClient,
  NotifyClient,
  NotifyClientError,
  type NotifyClientOptions,
  type NotifyShareFile,
  type NotifyShareRequest,
  type NotifyShareResponse,
} from "./notify.js";

export type {
  NotifyClientOptions as NotifyShareClientOptions,
  NotifyShareFile,
  NotifyShareRequest,
  NotifyShareResponse,
};

export { NotifyClientError as NotifyShareClientError };

export class NotifyShareClient {
  private readonly client: NotifyClient;

  constructor(options: NotifyClientOptions) {
    this.client = createNotifyClient(options);
  }

  send(request: NotifyShareRequest): Promise<NotifyShareResponse> {
    return this.client.share(request);
  }
}

export function createNotifyShareClient(options: NotifyClientOptions): NotifyShareClient {
  return new NotifyShareClient(options);
}
