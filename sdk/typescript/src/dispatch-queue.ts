/** Lightweight dispatch-queue SDK facade over the Dispatch slice. */
import {
  DispatchClient,
  DispatchClientError,
  createDispatchClient,
  type DispatchClientOptions,
  type DispatchQueueResponse,
} from "./dispatch.js";

export type {
  DispatchClientOptions as DispatchQueueClientOptions,
  DispatchQueueResponse,
};

export { DispatchClientError as DispatchQueueClientError };

export class DispatchQueueClient {
  private readonly client: DispatchClient;

  constructor(options: DispatchClientOptions) {
    this.client = createDispatchClient(options);
  }

  queue(): Promise<DispatchQueueResponse> {
    return this.client.queue();
  }
}

export function createDispatchQueueClient(options: DispatchClientOptions): DispatchQueueClient {
  return new DispatchQueueClient(options);
}
