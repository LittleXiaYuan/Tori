/** Lightweight identity SDK facade over the discovery slice. */
import {
  createDiscoveryClient,
  DiscoveryClient,
  DiscoveryClientError,
  type DiscoveryClientOptions,
  type IdentityProfile,
  type IdentityProfilesResponse,
  type ResolveIdentityRequest,
} from "./discovery.js";

export type {
  DiscoveryClientOptions as IdentityClientOptions,
  IdentityProfile,
  IdentityProfilesResponse,
  ResolveIdentityRequest,
};

export { DiscoveryClientError as IdentityClientError };

export class IdentityClient {
  private readonly client: DiscoveryClient;

  constructor(options: DiscoveryClientOptions) {
    this.client = createDiscoveryClient(options);
  }

  resolve(request: ResolveIdentityRequest): Promise<IdentityProfile> {
    return this.client.resolveIdentity(request);
  }

  profiles(): Promise<IdentityProfilesResponse> {
    return this.client.identityProfiles();
  }
}

export function createIdentityClient(options: DiscoveryClientOptions): IdentityClient {
  return new IdentityClient(options);
}
