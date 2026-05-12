/** Lightweight system-ops SDK facade over the System slice. */
import {
  SystemClient,
  SystemClientError,
  createSystemClient,
  type SystemCacheStatsResponse,
  type SystemClientOptions,
  type SystemInfoResponse,
  type SystemMetricsResponse,
  type SystemModulesResponse,
  type SystemSBOMResponse,
  type SystemStatsResponse,
} from "./system.js";

export type {
  SystemCacheStatsResponse,
  SystemClientOptions as SystemOpsClientOptions,
  SystemInfoResponse,
  SystemMetricsResponse,
  SystemModulesResponse,
  SystemSBOMResponse,
  SystemStatsResponse,
};

export { SystemClientError as SystemOpsClientError };

export class SystemOpsClient {
  private readonly client: SystemClient;

  constructor(options: SystemClientOptions) {
    this.client = createSystemClient(options);
  }

  systemInfo(): Promise<SystemInfoResponse> {
    return this.client.systemInfo();
  }

  systemStats(): Promise<SystemStatsResponse> {
    return this.client.systemStats();
  }

  metrics(): Promise<SystemMetricsResponse> {
    return this.client.metrics();
  }

  metricsPrometheus(): Promise<string> {
    return this.client.metricsPrometheus();
  }

  cacheStats(): Promise<SystemCacheStatsResponse> {
    return this.client.cacheStats();
  }

  modules(): Promise<SystemModulesResponse> {
    return this.client.modules();
  }

  sbom(): Promise<SystemSBOMResponse> {
    return this.client.sbom();
  }
}

export function createSystemOpsClient(options: SystemClientOptions): SystemOpsClient {
  return new SystemOpsClient(options);
}
