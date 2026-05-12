/** Lightweight runtime-queue-read SDK facade over runtime queue reads. */
import {
  createRuntimeClient,
  RuntimeClient,
  RuntimeClientError,
  type QueueOverviewResponse,
  type QueueSessionResponse,
  type QueueTask,
  type RuntimeClientOptions,
} from "./runtime.js";

export type {
  QueueOverviewResponse,
  QueueSessionResponse,
  QueueTask,
  RuntimeClientOptions as RuntimeQueueReadClientOptions,
};

export { RuntimeClientError as RuntimeQueueReadClientError };

export class RuntimeQueueReadClient {
  private readonly client: RuntimeClient;

  constructor(options: RuntimeClientOptions) { this.client = createRuntimeClient(options); }
  overview(): Promise<QueueOverviewResponse> { return this.client.queues(); }
  session(sessionId: string): Promise<QueueSessionResponse> { return this.client.sessionQueue(sessionId); }
}

export function createRuntimeQueueReadClient(options: RuntimeClientOptions): RuntimeQueueReadClient { return new RuntimeQueueReadClient(options); }
