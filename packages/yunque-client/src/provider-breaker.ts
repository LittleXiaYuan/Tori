/** Lightweight provider-breaker SDK facade over the Providers slice. */
import {
  createProvidersClient,
  ProvidersClient,
  ProvidersClientError,
  type ProviderActionResponse,
  type ProvidersClientOptions,
} from "./providers.js";

export type {
  ProviderActionResponse,
  ProvidersClientOptions as ProviderBreakerClientOptions,
};

export { ProvidersClientError as ProviderBreakerClientError };

export class ProviderBreakerClient {
  private readonly client: ProvidersClient;

  constructor(options: ProvidersClientOptions) { this.client = createProvidersClient(options); }
  reset(): Promise<ProviderActionResponse> { return this.client.resetBreakers(); }
}

export function createProviderBreakerClient(options: ProvidersClientOptions): ProviderBreakerClient { return new ProviderBreakerClient(options); }
