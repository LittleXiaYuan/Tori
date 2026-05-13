import { fetcher } from "./api-core";
import type { PackBackendModulesResponse, PackListResponse, PackMutationResponse } from "./api-types";

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
  backendModules(): Promise<PackBackendModulesResponse>;
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
    backendModules: () => fetcher<PackBackendModulesResponse>("/v1/packs/backend-modules"),
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
