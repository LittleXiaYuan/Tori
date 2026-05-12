/** Lightweight trace-recent SDK facade over recent execution/audit trace events. */
import {
  createTraceClient,
  TraceClient,
  TraceClientError,
  type TraceClientOptions,
  type TraceEvent,
  type TraceEventsResponse,
  type TraceRecentOptions,
} from "./trace.js";

export type {
  TraceClientOptions as TraceRecentClientOptions,
  TraceEvent,
  TraceEventsResponse,
  TraceRecentOptions,
};

export { TraceClientError as TraceRecentClientError };

export class TraceRecentClient {
  private readonly client: TraceClient;

  constructor(options: TraceClientOptions) { this.client = createTraceClient(options); }
  recent(options?: TraceRecentOptions): Promise<TraceEventsResponse> { return this.client.recent(options); }
}

export function createTraceRecentClient(options: TraceClientOptions): TraceRecentClient { return new TraceRecentClient(options); }
