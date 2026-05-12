/** Lightweight federation-capabilities SDK facade over the Federation slice. */
import {
  FederationClient,
  FederationClientError,
  createFederationClient,
  type FederationCapabilitiesResponse,
  type FederationCapabilityPayload,
  type FederationClientOptions,
  type FederationStatusResponse,
} from "./federation.js";

export type {
  FederationCapabilitiesResponse,
  FederationCapabilityPayload,
  FederationClientOptions as FederationCapabilitiesClientOptions,
  FederationStatusResponse,
};

export { FederationClientError as FederationCapabilitiesClientError };

export class FederationCapabilitiesClient {
  private readonly client: FederationClient;

  constructor(options: FederationClientOptions) { this.client = createFederationClient(options); }
  get(): Promise<FederationCapabilitiesResponse> { return this.client.capabilities(); }
  update(body: FederationCapabilityPayload): Promise<FederationStatusResponse> { return this.client.updateCapabilities(body); }
  broadcast(): Promise<FederationStatusResponse> { return this.client.broadcast(); }
}

export function createFederationCapabilitiesClient(options: FederationClientOptions): FederationCapabilitiesClient { return new FederationCapabilitiesClient(options); }
