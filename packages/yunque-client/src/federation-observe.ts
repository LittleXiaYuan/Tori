/** Lightweight federation-observe SDK facade over the Federation slice. */
import {
  FederationClient,
  FederationClientError,
  createFederationClient,
  type FederationBridgeStatsResponse,
  type FederationCapabilitiesResponse,
  type FederationClientOptions,
  type FederationPeersResponse,
  type FederationStatsResponse,
} from "./federation.js";

export type {
  FederationBridgeStatsResponse,
  FederationCapabilitiesResponse,
  FederationClientOptions as FederationObserveClientOptions,
  FederationPeersResponse,
  FederationStatsResponse,
};

export { FederationClientError as FederationObserveClientError };

export class FederationObserveClient {
  private readonly client: FederationClient;

  constructor(options: FederationClientOptions) {
    this.client = createFederationClient(options);
  }

  peers(): Promise<FederationPeersResponse> {
    return this.client.peers();
  }

  stats(): Promise<FederationStatsResponse> {
    return this.client.stats();
  }

  capabilities(): Promise<FederationCapabilitiesResponse> {
    return this.client.capabilities();
  }

  bridgeStats(): Promise<FederationBridgeStatsResponse> {
    return this.client.bridgeStats();
  }
}

export function createFederationObserveClient(options: FederationClientOptions): FederationObserveClient {
  return new FederationObserveClient(options);
}
