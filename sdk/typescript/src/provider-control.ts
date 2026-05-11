/** Lightweight provider-control SDK facade over the Providers slice. */
import {
  createProvidersClient,
  ProvidersClient,
  ProvidersClientError,
  type ExecProviderResponse,
  type ProviderActionResponse,
  type ProviderMode,
  type ProviderModeResponse,
  type ProviderSessionOverrideRequest,
  type ProvidersClientOptions,
} from "./providers.js";

export type {
  ExecProviderResponse,
  ProviderActionResponse,
  ProviderMode,
  ProviderModeResponse,
  ProviderSessionOverrideRequest,
  ProvidersClientOptions as ProviderControlClientOptions,
};

export { ProvidersClientError as ProviderControlClientError };

export class ProviderControlClient {
  private readonly client: ProvidersClient;

  constructor(options: ProvidersClientOptions) {
    this.client = createProvidersClient(options);
  }

  enable(id: string): Promise<ProviderActionResponse> {
    return this.client.enableProvider(id);
  }

  disable(id: string): Promise<ProviderActionResponse> {
    return this.client.disableProvider(id);
  }

  switchModel(id: string, model: string): Promise<ProviderActionResponse> {
    return this.client.switchModel(id, model);
  }

  setSessionProvider(request: ProviderSessionOverrideRequest): Promise<ProviderActionResponse> {
    return this.client.setSessionProvider(request);
  }

  clearSessionProvider(sessionId: string): Promise<ProviderActionResponse> {
    return this.client.clearSessionProvider(sessionId);
  }

  setMode(mode: ProviderMode): Promise<ProviderModeResponse> {
    return this.client.setMode(mode);
  }

  setExecProvider(providerId: string): Promise<ExecProviderResponse> {
    return this.client.setExecProvider(providerId);
  }
}

export function createProviderControlClient(options: ProvidersClientOptions): ProviderControlClient {
  return new ProviderControlClient(options);
}
