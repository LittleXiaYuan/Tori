/** Lightweight provider-health SDK facade over the Providers slice. */
import {
  createProvidersClient,
  ProvidersClient,
  ProvidersClientError,
  type ExecProviderResponse,
  type ProviderModeResponse,
  type ProvidersClientOptions,
  type ProvidersResponse,
  type ProviderTestResponse,
} from "./providers.js";

export type {
  ExecProviderResponse,
  ProviderModeResponse,
  ProvidersClientOptions as ProviderHealthClientOptions,
  ProvidersResponse,
  ProviderTestResponse,
};

export { ProvidersClientError as ProviderHealthClientError };

export class ProviderHealthClient {
  private readonly client: ProvidersClient;

  constructor(options: ProvidersClientOptions) {
    this.client = createProvidersClient(options);
  }

  list(): Promise<ProvidersResponse> {
    return this.client.listProviders();
  }

  test(id: string): Promise<ProviderTestResponse> {
    return this.client.testProvider(id);
  }

  mode(): Promise<ProviderModeResponse> {
    return this.client.getMode();
  }

  exec(): Promise<ExecProviderResponse> {
    return this.client.getExecProvider();
  }
}

export function createProviderHealthClient(options: ProvidersClientOptions): ProviderHealthClient {
  return new ProviderHealthClient(options);
}
