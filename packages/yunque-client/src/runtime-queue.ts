/** Lightweight runtime-queue SDK facade over the Runtime slice. */
import {
  createRuntimeClient,
  RuntimeClient,
  RuntimeClientError,
  type QueueCancelResponse,
  type QueueOverviewResponse,
  type QueueSessionResponse,
  type QueueTask,
  type RuntimeClientOptions,
} from "./runtime.js";

export type {
  QueueCancelResponse,
  QueueOverviewResponse,
  QueueSessionResponse,
  QueueTask,
  RuntimeClientOptions as RuntimeQueueClientOptions,
};

export { RuntimeClientError as RuntimeQueueClientError };

export class RuntimeQueueClient {
  private readonly client: RuntimeClient;

  constructor(options: RuntimeClientOptions) {
    this.client = createRuntimeClient(options);
  }

  overview(): Promise<QueueOverviewResponse> {
    return this.client.queues();
  }

  session(sessionId: string): Promise<QueueSessionResponse> {
    return this.client.sessionQueue(sessionId);
  }

  cancel(sessionId: string, taskId: string): Promise<QueueCancelResponse> {
    return this.client.cancelQueuedTask(sessionId, taskId);
  }
}

export function createRuntimeQueueClient(options: RuntimeClientOptions): RuntimeQueueClient {
  return new RuntimeQueueClient(options);
}
