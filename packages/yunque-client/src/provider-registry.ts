/** Lightweight provider-registry SDK facade over the Providers slice. */
import {
  createProvidersClient,
  ProvidersClient,
  ProvidersClientError,
  type LocalDiscoverRequest,
  type LocalRegisterRequest,
  type ProviderActionResponse,
  type ProviderConfig,
  type ProviderPresetsResponse,
  type ProvidersClientOptions,
  type ToriDiscoverResponse,
} from "./providers.js";

export type {
  LocalDiscoverRequest,
  LocalRegisterRequest,
  ProviderActionResponse,
  ProviderConfig,
  ProviderPresetsResponse,
  ProvidersClientOptions as ProviderRegistryClientOptions,
  ToriDiscoverResponse,
};

export { ProvidersClientError as ProviderRegistryClientError };

export class ProviderRegistryClient {
  private readonly client: ProvidersClient;

  constructor(options: ProvidersClientOptions) {
    this.client = createProvidersClient(options);
  }

  presets(): Promise<ProviderPresetsResponse> {
    return this.client.presets();
  }

  register(config: ProviderConfig): Promise<ProviderActionResponse> {
    return this.client.registerProvider(config);
  }

  delete(id: string): Promise<ProviderActionResponse> {
    return this.client.deleteProvider(id);
  }

  discoverLocal(request: LocalDiscoverRequest): Promise<Record<string, unknown>> {
    return this.client.discoverLocal(request);
  }

  registerLocal(request: LocalRegisterRequest): Promise<ProviderActionResponse> {
    return this.client.registerLocal(request);
  }

  discoverTori(options?: { autoRegister?: boolean }): Promise<ToriDiscoverResponse> {
    return this.client.discoverTori(options);
  }
}

export function createProviderRegistryClient(options: ProvidersClientOptions): ProviderRegistryClient {
  return new ProviderRegistryClient(options);
}
