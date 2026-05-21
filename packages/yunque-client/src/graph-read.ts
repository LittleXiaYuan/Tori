/** Lightweight graph-read SDK facade over the Graph slice. */
import {
  GraphClient,
  GraphClientError,
  createGraphClient,
  type GraphClientOptions,
  type GraphContextResponse,
  type GraphEntitiesResponse,
  type GraphEntity,
  type GraphRelation,
  type GraphRelationsResponse,
  type GraphStatsResponse,
} from "./graph.js";

export type {
  GraphClientOptions as GraphReadClientOptions,
  GraphContextResponse,
  GraphEntitiesResponse,
  GraphEntity,
  GraphRelation,
  GraphRelationsResponse,
  GraphStatsResponse,
};

export { GraphClientError as GraphReadClientError };

export class GraphReadClient {
  private readonly client: GraphClient;

  constructor(options: GraphClientOptions) {
    this.client = createGraphClient(options);
  }

  entities(query?: string): Promise<GraphEntitiesResponse> {
    return this.client.entities(query);
  }

  relations(entityId?: string): Promise<GraphRelationsResponse> {
    return this.client.relations(entityId);
  }

  contextByEntityId(entityId: string): Promise<GraphContextResponse> {
    return this.client.contextByEntityId(entityId);
  }

  contextByName(name: string): Promise<GraphContextResponse> {
    return this.client.contextByName(name);
  }

  stats(): Promise<GraphStatsResponse> {
    return this.client.stats();
  }
}

export function createGraphReadClient(options: GraphClientOptions): GraphReadClient {
  return new GraphReadClient(options);
}
