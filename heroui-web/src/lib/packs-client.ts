import { fetcher } from "./api-core";
import type { PackBackendModulesResponse, PackBackendRouteAuditReport, PackCatalogReport, PackCapabilityGateReport, PackCapabilityIndexReport, PackCapabilityPlanReport, PackCapabilityPrepareReport, PackCapabilityPrepareSummary, PackCapabilityResolveReport, PackListResponse, PackMutationResponse } from "./pack-types";

export interface PacksPruneResponse {
  removed: string[];
  kept: string[];
  errors?: string[];
  removed_count: number;
  kept_count: number;
}

export function summarizeCapabilityPrepare(plan: PackCapabilityPlanReport, prepare?: PackCapabilityPrepareReport | null): PackCapabilityPrepareSummary {
  return {
    kind: "pack_capability_prepare_summary",
    generated_at: prepare?.generated_at || plan.generated_at,
    capabilities: prepare?.capabilities || plan.capabilities,
    allowed: prepare?.allowed ?? plan.allowed,
    action: prepare?.action || plan.action,
    plan: {
      allowed_count: plan.allowed_count,
      blocked_count: plan.blocked_count,
      use_count: plan.use_count,
      enable_count: plan.enable_count,
      install_count: plan.install_count,
      route_audit_issue_count: plan.route_audit_issue_count,
      required_packs: plan.required_packs || [],
      enable_packs: plan.enable_packs || [],
      install_capabilities: plan.install_capabilities || [],
      catalog_install_hints: plan.catalog_install_hints || [],
      catalog_download_hints: plan.catalog_download_hints || [],
    },
    prepare: prepare ? {
      step_count: prepare.step_count,
      ready_count: prepare.ready_count,
      enable_count: prepare.enable_count,
      install_count: prepare.install_count,
      download_count: prepare.download_count,
      route_audit_issue_count: prepare.route_audit_issue_count,
    } : null,
    steps: prepare?.steps || [],
    catalog_source_reports: prepare?.catalog_source_reports || plan.catalog_source_reports || [],
    route_audit_issues: prepare?.route_audit_issues || plan.route_audit_issues || [],
    unavailable_reasons: prepare?.unavailable_reasons || plan.unavailable_reasons || [],
  };
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
