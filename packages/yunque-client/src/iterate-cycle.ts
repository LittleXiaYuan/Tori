/** Lightweight iterate-cycle SDK facade over the Iterate slice. */
import {
  IterateClient,
  IterateClientError,
  createIterateClient,
  type IterateClientOptions,
  type IterateStatusResponse,
  type IterateTriggerResponse,
} from "./iterate.js";

export type {
  IterateClientOptions as IterateCycleClientOptions,
  IterateStatusResponse,
  IterateTriggerResponse,
};

export { IterateClientError as IterateCycleClientError };

export class IterateCycleClient {
  private readonly client: IterateClient;

  constructor(options: IterateClientOptions) {
    this.client = createIterateClient(options);
  }

  trigger(): Promise<IterateTriggerResponse> {
    return this.client.trigger();
  }

  status(): Promise<IterateStatusResponse> {
    return this.client.status();
  }
}

export function createIterateCycleClient(options: IterateClientOptions): IterateCycleClient {
  return new IterateCycleClient(options);
}
