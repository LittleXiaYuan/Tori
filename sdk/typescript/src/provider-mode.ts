/** Lightweight provider-mode SDK facade over the Providers slice. */
import {
  createProvidersClient,
  ProvidersClient,
  ProvidersClientError,
  type ExecProviderResponse,
  type ProviderMode,
  type ProviderModeResponse,
  type ProvidersClientOptions,
} from "./providers.js";

export type {
  ExecProviderResponse,
  ProviderMode,
  ProviderModeResponse,
  ProvidersClientOptions as ProviderModeClientOptions,
};

export { ProvidersClientError as ProviderModeClientError };

export class ProviderModeClient {
  private readonly client: ProvidersClient;

  constructor(options: ProvidersClientOptions) { this.client = createProvidersClient(options); }
  getMode(): Promise<ProviderModeResponse> { return this.client.getMode(); }
  setMode(mode: ProviderMode): Promise<ProviderModeResponse> { return this.client.setMode(mode); }
  getExecProvider(): Promise<ExecProviderResponse> { return this.client.getExecProvider(); }
  setExecProvider(providerId: string): Promise<ExecProviderResponse> { return this.client.setExecProvider(providerId); }
}

export function createProviderModeClient(options: ProvidersClientOptions): ProviderModeClient { return new ProviderModeClient(options); }
