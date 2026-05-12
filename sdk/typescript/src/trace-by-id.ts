/** Lightweight trace-by-id SDK facade over trace id event reads. */
import {
  createTraceClient,
  TraceClient,
  TraceClientError,
  type TraceByIDResponse,
  type TraceClientOptions,
  type TraceEvent,
  type TraceQueryOptions,
} from "./trace.js";

export type {
  TraceByIDResponse,
  TraceClientOptions as TraceByIdClientOptions,
  TraceEvent,
  TraceQueryOptions,
};

export { TraceClientError as TraceByIdClientError };

export class TraceByIdClient {
  private readonly client: TraceClient;

  constructor(options: TraceClientOptions) { this.client = createTraceClient(options); }
  get(traceId: string, options?: TraceQueryOptions): Promise<TraceByIDResponse> { return this.client.byTraceId(traceId, options); }
}

export function createTraceByIdClient(options: TraceClientOptions): TraceByIdClient { return new TraceByIdClient(options); }
