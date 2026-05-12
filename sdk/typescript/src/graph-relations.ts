/** Lightweight graph-relations SDK facade over the Graph slice. */
import {
  GraphClient,
  GraphClientError,
  createGraphClient,
  type GraphClientOptions,
  type GraphRelation,
  type GraphRelationsResponse,
} from "./graph.js";

export type {
  GraphClientOptions as GraphRelationsClientOptions,
  GraphRelation,
  GraphRelationsResponse,
};

export { GraphClientError as GraphRelationsClientError };

export class GraphRelationsClient {
  private readonly client: GraphClient;
  constructor(options: GraphClientOptions) { this.client = createGraphClient(options); }
  relations(entityId?: string): Promise<GraphRelationsResponse> { return this.client.relations(entityId); }
}

export function createGraphRelationsClient(options: GraphClientOptions): GraphRelationsClient { return new GraphRelationsClient(options); }
