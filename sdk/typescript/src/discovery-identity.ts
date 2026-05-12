/** Lightweight discovery-identity SDK facade over the Discovery slice. */
import {
  DiscoveryClient,
  DiscoveryClientError,
  createDiscoveryClient,
  type DiscoveryClientOptions,
  type IdentityProfile,
  type IdentityProfilesResponse,
  type ResolveIdentityRequest,
} from "./discovery.js";

export type {
  DiscoveryClientOptions as DiscoveryIdentityClientOptions,
  IdentityProfile,
  IdentityProfilesResponse,
  ResolveIdentityRequest,
};

export { DiscoveryClientError as DiscoveryIdentityClientError };

export class DiscoveryIdentityClient {
  private readonly client: DiscoveryClient;

  constructor(options: DiscoveryClientOptions) { this.client = createDiscoveryClient(options); }
  resolveIdentity(request: ResolveIdentityRequest): Promise<IdentityProfile> { return this.client.resolveIdentity(request); }
  identityProfiles(): Promise<IdentityProfilesResponse> { return this.client.identityProfiles(); }
}

export function createDiscoveryIdentityClient(options: DiscoveryClientOptions): DiscoveryIdentityClient { return new DiscoveryIdentityClient(options); }
