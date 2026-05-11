/** Lightweight task-trace SDK facade over the Trace slice. */
import {
  createTraceClient,
  TraceClient,
  TraceClientError,
  type TraceByTaskResponse,
  type TraceClientOptions,
  type TraceEvent,
  type TraceQueryOptions,
} from "./trace.js";

export type {
  TraceByTaskResponse as TaskTraceResponse,
  TraceClientOptions as TaskTraceClientOptions,
  TraceEvent,
  TraceQueryOptions as TaskTraceQueryOptions,
};

export { TraceClientError as TaskTraceClientError };

export class TaskTraceClient {
  private readonly client: TraceClient;

  constructor(options: TraceClientOptions) {
    this.client = createTraceClient(options);
  }

  get(taskId: string, options?: TraceQueryOptions): Promise<TraceByTaskResponse> {
    return this.client.byTaskId(taskId, options);
  }
}

export function createTaskTraceClient(options: TraceClientOptions): TaskTraceClient {
  return new TaskTraceClient(options);
}
