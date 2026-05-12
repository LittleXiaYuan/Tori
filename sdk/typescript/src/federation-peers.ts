/** Lightweight federation-peers SDK facade over the Federation slice. */
import {
  FederationClient,
  FederationClientError,
  createFederationClient,
  type FederationClientOptions,
  type FederationPeersResponse,
} from "./federation.js";

export type {
  FederationClientOptions as FederationPeersClientOptions,
  FederationPeersResponse,
};

export { FederationClientError as FederationPeersClientError };

export class FederationPeersClient {
  private readonly client: FederationClient;

  constructor(options: FederationClientOptions) { this.client = createFederationClient(options); }
  list(): Promise<FederationPeersResponse> { return this.client.peers(); }
}

export function createFederationPeersClient(options: FederationClientOptions): FederationPeersClient { return new FederationPeersClient(options); }
