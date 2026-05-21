/** Lightweight federation-stats SDK facade over the Federation slice. */
import {
  FederationClient,
  FederationClientError,
  createFederationClient,
  type FederationBridgeStatsResponse,
  type FederationClientOptions,
  type FederationStatsResponse,
} from "./federation.js";

export type {
  FederationBridgeStatsResponse,
  FederationClientOptions as FederationStatsClientOptions,
  FederationStatsResponse,
};

export { FederationClientError as FederationStatsClientError };

export class FederationStatsClient {
  private readonly client: FederationClient;

  constructor(options: FederationClientOptions) { this.client = createFederationClient(options); }
  stats(): Promise<FederationStatsResponse> { return this.client.stats(); }
  bridge(): Promise<FederationBridgeStatsResponse> { return this.client.bridgeStats(); }
}

export function createFederationStatsClient(options: FederationClientOptions): FederationStatsClient { return new FederationStatsClient(options); }
