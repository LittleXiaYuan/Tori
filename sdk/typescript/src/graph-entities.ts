/** Lightweight graph-entities SDK facade over the Graph slice. */
import {
  GraphClient,
  GraphClientError,
  createGraphClient,
  type GraphClientOptions,
  type GraphEntitiesResponse,
  type GraphEntity,
} from "./graph.js";

export type {
  GraphClientOptions as GraphEntitiesClientOptions,
  GraphEntitiesResponse,
  GraphEntity,
};

export { GraphClientError as GraphEntitiesClientError };

export class GraphEntitiesClient {
  private readonly client: GraphClient;
  constructor(options: GraphClientOptions) { this.client = createGraphClient(options); }
  entities(query?: string): Promise<GraphEntitiesResponse> { return this.client.entities(query); }
}

export function createGraphEntitiesClient(options: GraphClientOptions): GraphEntitiesClient { return new GraphEntitiesClient(options); }
