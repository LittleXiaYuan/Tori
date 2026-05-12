/** Lightweight dispatch-worker-config SDK facade over the Dispatch slice. */
import {
  DispatchClient,
  DispatchClientError,
  createDispatchClient,
  type DispatchClientOptions,
  type DispatchWorkerConfigResponse,
} from "./dispatch.js";

export type {
  DispatchClientOptions as DispatchWorkerConfigClientOptions,
  DispatchWorkerConfigResponse,
};

export { DispatchClientError as DispatchWorkerConfigClientError };

export class DispatchWorkerConfigClient {
  private readonly client: DispatchClient;

  constructor(options: DispatchClientOptions) {
    this.client = createDispatchClient(options);
  }

  get(type?: string): Promise<DispatchWorkerConfigResponse> {
    return this.client.workerConfig(type);
  }
}

export function createDispatchWorkerConfigClient(options: DispatchClientOptions): DispatchWorkerConfigClient {
  return new DispatchWorkerConfigClient(options);
}
