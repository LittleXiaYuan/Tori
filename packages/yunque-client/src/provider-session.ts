/** Lightweight provider-session SDK facade over the Providers slice. */
import {
  createProvidersClient,
  ProvidersClient,
  ProvidersClientError,
  type ProviderActionResponse,
  type ProviderSessionOverrideRequest,
  type ProvidersClientOptions,
} from "./providers.js";

export type {
  ProviderActionResponse,
  ProviderSessionOverrideRequest,
  ProvidersClientOptions as ProviderSessionClientOptions,
};

export { ProvidersClientError as ProviderSessionClientError };

export class ProviderSessionClient {
  private readonly client: ProvidersClient;

  constructor(options: ProvidersClientOptions) { this.client = createProvidersClient(options); }
  setSessionProvider(request: ProviderSessionOverrideRequest): Promise<ProviderActionResponse> { return this.client.setSessionProvider(request); }
  clearSessionProvider(sessionId: string): Promise<ProviderActionResponse> { return this.client.clearSessionProvider(sessionId); }
}

export function createProviderSessionClient(options: ProvidersClientOptions): ProviderSessionClient { return new ProviderSessionClient(options); }
