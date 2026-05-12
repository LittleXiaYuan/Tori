/** Lightweight dispatch-control SDK facade over the Dispatch slice. */
import {
  DispatchClient,
  DispatchClientError,
  createDispatchClient,
  type DispatchClientOptions,
  type DispatchEnqueueRequest,
  type DispatchEnqueueResponse,
  type DispatchStatusResponse,
} from "./dispatch.js";

export type {
  DispatchClientOptions as DispatchControlClientOptions,
  DispatchEnqueueRequest,
  DispatchEnqueueResponse,
  DispatchStatusResponse,
};

export { DispatchClientError as DispatchControlClientError };

export class DispatchControlClient {
  private readonly client: DispatchClient;

  constructor(options: DispatchClientOptions) {
    this.client = createDispatchClient(options);
  }

  removeWorker(id: string): Promise<DispatchStatusResponse> {
    return this.client.removeWorker(id);
  }

  enqueue(request: DispatchEnqueueRequest): Promise<DispatchEnqueueResponse> {
    return this.client.enqueue(request);
  }
}

export function createDispatchControlClient(options: DispatchClientOptions): DispatchControlClient {
  return new DispatchControlClient(options);
}
