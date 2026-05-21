/** Lightweight reverie-control SDK facade over the Reverie slice. */
import {
  ReverieClient,
  ReverieClientError,
  createReverieClient,
  type ReverieClientOptions,
  type ReverieConfig,
  type ReverieConfigResponse,
  type ReverieDeleteResponse,
  type ReverieThinkRequest,
  type ReverieThinkResponse,
} from "./reverie.js";

export type {
  ReverieClientOptions as ReverieControlClientOptions,
  ReverieConfig,
  ReverieConfigResponse,
  ReverieDeleteResponse,
  ReverieThinkRequest,
  ReverieThinkResponse,
};

export { ReverieClientError as ReverieControlClientError };

export class ReverieControlClient {
  private readonly client: ReverieClient;

  constructor(options: ReverieClientOptions) {
    this.client = createReverieClient(options);
  }

  updateConfig(body: ReverieConfig): Promise<ReverieConfigResponse> {
    return this.client.updateConfig(body);
  }

  think(body?: ReverieThinkRequest): Promise<ReverieThinkResponse> {
    return this.client.think(body);
  }

  deleteThought(id: string): Promise<ReverieDeleteResponse> {
    return this.client.deleteThought(id);
  }
}

export function createReverieControlClient(options: ReverieClientOptions): ReverieControlClient {
  return new ReverieControlClient(options);
}
