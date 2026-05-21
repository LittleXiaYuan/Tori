/** Lightweight dispatch-read SDK facade over the Dispatch slice. */
import {
  DispatchClient,
  DispatchClientError,
  createDispatchClient,
  type DispatchClientOptions,
  type DispatchQueueResponse,
  type DispatchWorker,
  type DispatchWorkerConfigResponse,
  type DispatchWorkersResponse,
} from "./dispatch.js";

export type {
  DispatchClientOptions as DispatchReadClientOptions,
  DispatchQueueResponse,
  DispatchWorker,
  DispatchWorkerConfigResponse,
  DispatchWorkersResponse,
};

export { DispatchClientError as DispatchReadClientError };

export class DispatchReadClient {
  private readonly client: DispatchClient;

  constructor(options: DispatchClientOptions) {
    this.client = createDispatchClient(options);
  }

  workers(): Promise<DispatchWorkersResponse> {
    return this.client.workers();
  }

  worker(id: string): Promise<DispatchWorker> {
    return this.client.worker(id);
  }

  queue(): Promise<DispatchQueueResponse> {
    return this.client.queue();
  }

  workerConfig(type?: string): Promise<DispatchWorkerConfigResponse> {
    return this.client.workerConfig(type);
  }
}

export function createDispatchReadClient(options: DispatchClientOptions): DispatchReadClient {
  return new DispatchReadClient(options);
}
