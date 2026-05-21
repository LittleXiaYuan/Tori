/** Lightweight graph-write SDK facade over the Graph slice. */
import {
  GraphClient,
  GraphClientError,
  createGraphClient,
  type GraphClientOptions,
  type GraphDeleteEntityResponse,
  type GraphEntity,
  type GraphRelation,
} from "./graph.js";

export type {
  GraphClientOptions as GraphWriteClientOptions,
  GraphDeleteEntityResponse,
  GraphEntity,
  GraphRelation,
};

export { GraphClientError as GraphWriteClientError };

export class GraphWriteClient {
  private readonly client: GraphClient;

  constructor(options: GraphClientOptions) {
    this.client = createGraphClient(options);
  }

  putEntity(entity: GraphEntity): Promise<GraphEntity> {
    return this.client.putEntity(entity);
  }

  deleteEntity(id: string): Promise<GraphDeleteEntityResponse> {
    return this.client.deleteEntity(id);
  }

  putRelation(relation: GraphRelation): Promise<GraphRelation> {
    return this.client.putRelation(relation);
  }
}

export function createGraphWriteClient(options: GraphClientOptions): GraphWriteClient {
  return new GraphWriteClient(options);
}
