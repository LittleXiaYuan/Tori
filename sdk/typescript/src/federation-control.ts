/** Lightweight federation-control SDK facade over the Federation slice. */
import {
  FederationClient,
  FederationClientError,
  createFederationClient,
  type FederationCapabilityPayload,
  type FederationClientOptions,
  type FederationStatusResponse,
} from "./federation.js";

export type {
  FederationCapabilityPayload,
  FederationClientOptions as FederationControlClientOptions,
  FederationStatusResponse,
};

export { FederationClientError as FederationControlClientError };

export class FederationControlClient {
  private readonly client: FederationClient;

  constructor(options: FederationClientOptions) {
    this.client = createFederationClient(options);
  }

  updateCapabilities(body: FederationCapabilityPayload): Promise<FederationStatusResponse> {
    return this.client.updateCapabilities(body);
  }

  broadcast(): Promise<FederationStatusResponse> {
    return this.client.broadcast();
  }
}

export function createFederationControlClient(options: FederationClientOptions): FederationControlClient {
  return new FederationControlClient(options);
}
