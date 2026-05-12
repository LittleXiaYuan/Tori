/** Lightweight runtime-queue-control SDK facade over runtime queue cancellation. */
import {
  createRuntimeClient,
  RuntimeClient,
  RuntimeClientError,
  type QueueCancelResponse,
  type RuntimeClientOptions,
} from "./runtime.js";

export type {
  QueueCancelResponse,
  RuntimeClientOptions as RuntimeQueueControlClientOptions,
};

export { RuntimeClientError as RuntimeQueueControlClientError };

export class RuntimeQueueControlClient {
  private readonly client: RuntimeClient;

  constructor(options: RuntimeClientOptions) { this.client = createRuntimeClient(options); }
  cancel(sessionId: string, taskId: string): Promise<QueueCancelResponse> { return this.client.cancelQueuedTask(sessionId, taskId); }
}

export function createRuntimeQueueControlClient(options: RuntimeClientOptions): RuntimeQueueControlClient { return new RuntimeQueueControlClient(options); }
