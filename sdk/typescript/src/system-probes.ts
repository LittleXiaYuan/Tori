/** Lightweight system-probes SDK facade over the System slice. */
import {
  SystemClient,
  SystemClientError,
  createSystemClient,
  type SystemClientOptions,
  type SystemCognitiveHealthResponse,
  type SystemHealthResponse,
  type SystemReadinessResponse,
  type SystemVersionResponse,
} from "./system.js";

export type {
  SystemClientOptions as SystemProbesClientOptions,
  SystemCognitiveHealthResponse,
  SystemHealthResponse,
  SystemReadinessResponse,
  SystemVersionResponse,
};

export { SystemClientError as SystemProbesClientError };

export class SystemProbesClient {
  private readonly client: SystemClient;

  constructor(options: SystemClientOptions) {
    this.client = createSystemClient(options);
  }

  health(): Promise<SystemHealthResponse> {
    return this.client.health();
  }

  livez(): Promise<SystemHealthResponse> {
    return this.client.livez();
  }

  readyz(): Promise<SystemReadinessResponse> {
    return this.client.readyz();
  }

  cognitiveHealth(): Promise<SystemCognitiveHealthResponse> {
    return this.client.cognitiveHealth();
  }

  version(): Promise<SystemVersionResponse> {
    return this.client.version();
  }
}

export function createSystemProbesClient(options: SystemClientOptions): SystemProbesClient {
  return new SystemProbesClient(options);
}
