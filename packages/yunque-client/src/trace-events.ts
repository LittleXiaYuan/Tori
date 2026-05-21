/** Lightweight trace-events SDK facade over the Trace slice. */
import {
  createTraceClient,
  TraceClient,
  TraceClientError,
  type TraceByIDResponse,
  type TraceClientOptions,
  type TraceEvent,
  type TraceEventsResponse,
  type TraceQueryOptions,
  type TraceRecentOptions,
} from "./trace.js";

export type {
  TraceByIDResponse,
  TraceClientOptions as TraceEventsClientOptions,
  TraceEvent,
  TraceEventsResponse,
  TraceQueryOptions,
  TraceRecentOptions,
};

export { TraceClientError as TraceEventsClientError };

export class TraceEventsClient {
  private readonly client: TraceClient;

  constructor(options: TraceClientOptions) {
    this.client = createTraceClient(options);
  }

  recent(options?: TraceRecentOptions): Promise<TraceEventsResponse> {
    return this.client.recent(options);
  }

  byTraceId(traceId: string, options?: TraceQueryOptions): Promise<TraceByIDResponse> {
    return this.client.byTraceId(traceId, options);
  }
}

export function createTraceEventsClient(options: TraceClientOptions): TraceEventsClient {
  return new TraceEventsClient(options);
}
