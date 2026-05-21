/** Lightweight graph-stats SDK facade over the Graph slice. */
import {
  GraphClient,
  GraphClientError,
  createGraphClient,
  type GraphClientOptions,
  type GraphStatsResponse,
} from "./graph.js";

export type {
  GraphClientOptions as GraphStatsClientOptions,
  GraphStatsResponse,
};

export { GraphClientError as GraphStatsClientError };

export class GraphStatsClient {
  private readonly client: GraphClient;
  constructor(options: GraphClientOptions) { this.client = createGraphClient(options); }
  stats(): Promise<GraphStatsResponse> { return this.client.stats(); }
}

export function createGraphStatsClient(options: GraphClientOptions): GraphStatsClient { return new GraphStatsClient(options); }
