/** Lightweight graph-context SDK facade over the Graph slice. */
import {
  GraphClient,
  GraphClientError,
  createGraphClient,
  type GraphClientOptions,
  type GraphContextResponse,
} from "./graph.js";

export type {
  GraphClientOptions as GraphContextClientOptions,
  GraphContextResponse,
};

export { GraphClientError as GraphContextClientError };

export class GraphContextClient {
  private readonly client: GraphClient;
  constructor(options: GraphClientOptions) { this.client = createGraphClient(options); }
  byEntityId(entityId: string): Promise<GraphContextResponse> { return this.client.contextByEntityId(entityId); }
  byName(name: string): Promise<GraphContextResponse> { return this.client.contextByName(name); }
}

export function createGraphContextClient(options: GraphClientOptions): GraphContextClient { return new GraphContextClient(options); }
