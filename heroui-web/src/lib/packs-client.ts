import { fetcher } from "./api-core";
import type { PackBackendModulesResponse, PackBackendRouteAuditReport, PackCatalogReport, PackCapabilityGateReport, PackCapabilityIndexReport, PackCapabilityPlanReport, PackCapabilityPrepareReport, PackCapabilityResolveReport, PackListResponse, PackMutationResponse } from "./pack-types";

export interface PacksPruneResponse {
  removed: string[];
  kept: string[];
  errors?: string[];
  removed_count: number;
  kept_count: number;
}

export interface PacksClient {
  installed(): Promise<PackListResponse>;
  enabled(): Promise<PackListResponse>;
  catalog(options?: { capability?: string; q?: string }): Promise<PackCatalogReport>;
  capabilities(): Promise<PackCapabilityIndexReport>;
  resolveCapability(capability: string): Promise<PackCapabilityResolveReport>;
  gateCapability(capability: string): Promise<PackCapabilityGateReport>;
  planCapabilities(capabilities: string[]): Promise<PackCapabilityPlanReport>;
  prepareCapabilities(capabilities: string[]): Promise<PackCapabilityPrepareReport>;
  backendModules(): Promise<PackBackendModulesResponse>;
  backendRouteAudit(): Promise<PackBackendRouteAuditReport>;
  installLocal(manifestPath: string, source?: string, download?: boolean): Promise<PackMutationResponse>;
  installFromURL(manifestUrl: string, source?: string, download?: boolean): Promise<PackMutationResponse>;
  enable(id: string): Promise<PackMutationResponse>;
  disable(id: string): Promise<PackMutationResponse>;
  rollback(id: string): Promise<PackMutationResponse>;
  prune(): Promise<PacksPruneResponse>;
}

export function createPacksClient(): PacksClient {
  return {
    installed: () => fetcher<PackListResponse>("/v1/packs/installed"),
    enabled: () => fetcher<PackListResponse>("/v1/packs/enabled"),
    catalog: (options) => {
      const params = new URLSearchParams();
      if (options?.capability) params.set("capability", options.capability);
      if (options?.q) params.set("q", options.q);
      const query = params.toString();
      return fetcher<PackCatalogReport>(query ? `/v1/packs/catalog?${query}` : "/v1/packs/catalog");
    },
    capabilities: () => fetcher<PackCapabilityIndexReport>("/v1/packs/capabilities"),
    resolveCapability: (capability) =>
      fetcher<PackCapabilityResolveReport>(`/v1/packs/capabilities/resolve?capability=${encodeURIComponent(capability)}`),
    gateCapability: (capability) =>
      fetcher<PackCapabilityGateReport>(`/v1/packs/capabilities/gate?capability=${encodeURIComponent(capability)}`),
    planCapabilities: (capabilities) => {
      const params = capabilities.map((capability) => `capability=${encodeURIComponent(capability)}`).join("&");
      return fetcher<PackCapabilityPlanReport>(`/v1/packs/capabilities/plan?${params}`);
    },
    prepareCapabilities: (capabilities) => {
      const params = capabilities.map((capability) => `capability=${encodeURIComponent(capability)}`).join("&");
      return fetcher<PackCapabilityPrepareReport>(`/v1/packs/capabilities/prepare?${params}`);
    },
    backendModules: () => fetcher<PackBackendModulesResponse>("/v1/packs/backend-modules"),
    backendRouteAudit: () => fetcher<PackBackendRouteAuditReport>("/v1/packs/backend-route-audit"),
    installLocal: (manifestPath, source, download) =>
      fetcher<PackMutationResponse>("/v1/packs/install", {
        method: "POST",
        body: JSON.stringify({ manifest_path: manifestPath, source, download }),
      }),
    installFromURL: (manifestUrl, source, download) =>
      fetcher<PackMutationResponse>("/v1/packs/install", {
        method: "POST",
        body: JSON.stringify({ manifest_url: manifestUrl, source, download }),
      }),
    enable: (id) =>
      fetcher<PackMutationResponse>("/v1/packs/enable", { method: "POST", body: JSON.stringify({ id }) }),
    disable: (id) =>
      fetcher<PackMutationResponse>("/v1/packs/disable", { method: "POST", body: JSON.stringify({ id }) }),
    rollback: (id) =>
      fetcher<PackMutationResponse>("/v1/packs/rollback", { method: "POST", body: JSON.stringify({ id }) }),
    prune: () =>
      fetcher<PacksPruneResponse>("/v1/packs/prune", { method: "POST", body: JSON.stringify({}) }),
  };
}
