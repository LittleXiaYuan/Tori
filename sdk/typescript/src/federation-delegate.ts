/** Lightweight federation-delegate SDK facade over the Federation slice. */
import {
  FederationClient,
  FederationClientError,
  createFederationClient,
  type FederationClientOptions,
  type FederationDelegatePayload,
  type FederationDelegateResponse,
  type FederationDiscoverRequest,
  type FederationDiscoverResponse,
} from "./federation.js";

export type {
  FederationClientOptions as FederationDelegateClientOptions,
  FederationDelegatePayload,
  FederationDelegateResponse,
  FederationDiscoverRequest,
  FederationDiscoverResponse,
};

export { FederationClientError as FederationDelegateClientError };

export class FederationDelegateClient {
  private readonly client: FederationClient;

  constructor(options: FederationClientOptions) {
    this.client = createFederationClient(options);
  }

  discover(body: FederationDiscoverRequest): Promise<FederationDiscoverResponse> {
    return this.client.discover(body);
  }

  delegate(body: FederationDelegatePayload): Promise<FederationDelegateResponse> {
    return this.client.delegate(body);
  }
}

export function createFederationDelegateClient(options: FederationClientOptions): FederationDelegateClient {
  return new FederationDelegateClient(options);
}
