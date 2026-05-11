/** Lightweight circuit-breaker SDK facade over the providers slice. */
import {
  createProvidersClient,
  ProvidersClient,
  ProvidersClientError,
  type ProviderActionResponse,
  type ProvidersClientOptions,
} from "./providers.js";

export type {
  ProviderActionResponse as BreakerResetResponse,
  ProvidersClientOptions as BreakerClientOptions,
};

export { ProvidersClientError as BreakerClientError };

export class BreakerClient {
  private readonly client: ProvidersClient;

  constructor(options: ProvidersClientOptions) {
    this.client = createProvidersClient(options);
  }

  reset(): Promise<ProviderActionResponse> {
    return this.client.resetBreakers();
  }
}

export function createBreakerClient(options: ProvidersClientOptions): BreakerClient {
  return new BreakerClient(options);
}
