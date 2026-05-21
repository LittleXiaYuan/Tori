/** Lightweight dispatch-workers SDK facade over the Dispatch slice. */
import {
  DispatchClient,
  DispatchClientError,
  createDispatchClient,
  type DispatchClientOptions,
  type DispatchWorker,
  type DispatchWorkersResponse,
} from "./dispatch.js";

export type {
  DispatchClientOptions as DispatchWorkersClientOptions,
  DispatchWorker,
  DispatchWorkersResponse,
};

export { DispatchClientError as DispatchWorkersClientError };

export class DispatchWorkersClient {
  private readonly client: DispatchClient;

  constructor(options: DispatchClientOptions) {
    this.client = createDispatchClient(options);
  }

  list(): Promise<DispatchWorkersResponse> {
    return this.client.workers();
  }

  detail(id: string): Promise<DispatchWorker> {
    return this.client.worker(id);
  }
}

export function createDispatchWorkersClient(options: DispatchClientOptions): DispatchWorkersClient {
  return new DispatchWorkersClient(options);
}
