/** Lightweight Pack Runtime SDK slice for frontend shell sync and automation scripts. */
export type PackStatus = "installed" | "enabled" | "disabled" | string;
export type PackBackendRouteSpec = { method: string; path: string; description?: string; [key: string]: unknown };
export type PackBackendManifest = { capabilities?: string[]; routes?: string[]; routeSpecs?: PackBackendRouteSpec[]; permissions?: string[]; [key: string]: unknown };
export type PackFrontendMenu = { key: string; label: string; path: string; icon?: string; order?: number; [key: string]: unknown };
export type PackFrontendRoute = { path: string; component: string; title?: string; [key: string]: unknown };
export type PackFrontendAssets = { type?: string; entry?: string; [key: string]: unknown };
export type PackFrontendManifest = { menus?: PackFrontendMenu[]; routes?: PackFrontendRoute[]; assets?: PackFrontendAssets; [key: string]: unknown };
export type PackSdkManifest = { typescript?: string; go?: string; python?: string; [key: string]: unknown };
export type PackSdkEntrypoint = { packId: string; packName: string; language: string; importPath: string };
export type PackRouteBinding = { packId: string; packName: string; path: string; component: string; title?: string; assets?: PackFrontendAssets; distribution?: PackDistributionManifest; sdk: PackSdkEntrypoint[] };
export type PackBackendRouteBinding = PackBackendRouteSpec & { packId: string; packName: string };
export type PackDistributionManifest = { manifestUrl?: string; packageUrl?: string; frontendUrl?: string; sha256?: string; sizeBytes?: number; [key: string]: unknown };
export type PackUpdateManifest = { channel?: string; rollback?: boolean; [key: string]: unknown };
export type PackManifest = { id: string; name: string; version: string; description?: string; requiresCore?: string; optional?: boolean; defaultState?: string; status?: string; backend?: PackBackendManifest; frontend?: PackFrontendManifest; sdk?: PackSdkManifest; distribution?: PackDistributionManifest; update?: PackUpdateManifest; metadata?: Record<string, string>; [key: string]: unknown };
export type PackArtifacts = { packagePath?: string; sha256?: string; sizeBytes?: number; cachedAt?: string; [key: string]: unknown };
export type InstalledPack = { manifest: PackManifest; status: PackStatus; source?: string; artifacts?: PackArtifacts; previousArtifacts?: PackArtifacts; installedAt?: string; updatedAt?: string; previousVersion?: string; [key: string]: unknown };
export type PacksListResponse = { packs: InstalledPack[]; enabled?: InstalledPack[]; count: number; [key: string]: unknown };
export type PackMutationResponse = { pack: InstalledPack; status: PackStatus; [key: string]: unknown };
export type PackCatalogEntry = { manifest_path?: string; manifest_url?: string; package_url?: string; source?: string; manifest: PackManifest; installed: boolean; enabled: boolean; status?: PackStatus; update_action: "use" | "enable" | "install" | "update" | string; downloadable: boolean; [key: string]: unknown };
export type PackReleaseCatalogEntry = { release_url: string; release_tag?: string; release_name?: string; published_at?: string; package_url: string; asset_name?: string; sha256?: string; size_bytes?: number; manifest: PackManifest; installed: boolean; enabled: boolean; status?: PackStatus; update_action: "use" | "enable" | "install" | "update" | string; downloadable: boolean; [key: string]: unknown };
export type PackReleaseCatalogReport = { generated_at: string; releases: string[]; count: number; entries: PackReleaseCatalogEntry[]; errors?: string[]; [key: string]: unknown };
export type PackCatalogSourceReport = { source: string; ok: boolean; manifest_count: number; matched_entries: number; errors?: string[]; [key: string]: unknown };
export type PackCatalogReport = { generated_at: string; sources: string[]; source_reports?: PackCatalogSourceReport[]; count: number; installed: number; enabled: number; downloadable: number; capabilities: number; capability?: string; query?: string; entries: PackCatalogEntry[]; install_hints?: PackCatalogEntry[]; enable_hints?: PackCatalogEntry[]; download_hints?: PackCatalogEntry[]; errors?: string[]; [key: string]: unknown };
export type PackBackendRouteInfo = { method?: string; methods?: string[]; path: string };
export type PackBackendModuleInfo = { pack_id: string; routes: PackBackendRouteInfo[] };
export type PackBackendModulesResponse = { modules: PackBackendModuleInfo[]; count: number; [key: string]: unknown };
export type PackCapabilityIndexEntry = { capability: string; pack_id: string; pack_name: string; pack_status: string; enabled: boolean; optional: boolean; routes?: string[]; permissions?: string[]; sdk_typescript?: string; frontend_paths?: string[]; [key: string]: unknown };
export type PackCapabilityIndexReport = { generated_at: string; packs: number; enabled_packs: number; capabilities: number; enabled_capabilities: number; entries: PackCapabilityIndexEntry[]; [key: string]: unknown };
export type PackCapabilityCogniProfileEntry = { capability: string; source_type: "pack" | "skill" | "mcp" | string; source_id: string; source_name: string; enabled: boolean; action: "use" | "enable" | string; risk: "low" | "medium" | "high" | string; token_hint: string; use_when?: string[]; constraints?: string[]; permissions?: string[]; frontend_paths?: string[]; invoke_hint?: string; [key: string]: unknown };
export type PackCapabilityCogniProfileReport = { generated_at: string; entries: PackCapabilityCogniProfileEntry[]; count: number; [key: string]: unknown };
export type PackCapabilityResolveReport = { generated_at: string; capability: string; found: boolean; enabled: boolean; action: "use" | "enable" | "install" | string; preferred?: PackCapabilityIndexEntry; entries: PackCapabilityIndexEntry[]; enabled_entries: PackCapabilityIndexEntry[]; [key: string]: unknown };
export type PackCapabilityBinding = { capability: string; packId: string; packName: string; routes: string[]; permissions: string[]; frontendPaths: string[]; sdk: PackSdkEntrypoint[] };
export type PackBackendRouteAuditEntry = { pack_id: string; pack_name?: string; pack_status?: string; enabled: boolean; status: "ok" | "missing" | "method-mismatch" | "undeclared" | "pack-not-installed" | "registry-unavailable" | string; declared: boolean; mounted: boolean; method?: string; methods?: string[]; path: string; auth?: string; description?: string; issues?: string[]; [key: string]: unknown };
export type PackBackendRouteAuditReport = { generated_at: string; packs: number; enabled_packs: number; mounted_modules: number; declared_routes: number; mounted_routes: number; ok_routes: number; missing_routes: number; method_mismatches: number; undeclared_routes: number; entries: PackBackendRouteAuditEntry[]; [key: string]: unknown };
export type PackCapabilityGateReport = { generated_at: string; capability: string; allowed: boolean; action: "use" | "enable" | "install" | string; reason?: string; resolution: PackCapabilityResolveReport; route_audit?: PackBackendRouteAuditEntry[]; [key: string]: unknown };
export type PackCapabilityPlanReport = { generated_at: string; capabilities: string[]; allowed: boolean; action: "use" | "enable" | "install" | "fix-route-audit" | string; allowed_count: number; blocked_count: number; use_count: number; enable_count: number; install_count: number; route_audit_issue_count: number; gates: PackCapabilityGateReport[]; required_packs?: PackCapabilityIndexEntry[]; enable_packs?: PackCapabilityIndexEntry[]; install_capabilities?: string[]; catalog_install_hints?: PackCatalogEntry[]; catalog_download_hints?: PackCatalogEntry[]; catalog_source_reports?: PackCatalogSourceReport[]; route_audit_issues?: PackBackendRouteAuditEntry[]; unavailable_reasons?: string[]; downloadable_pack_hints?: PackCapabilityIndexEntry[]; [key: string]: unknown };
export type PackCapabilityPrepareStep = { action: "use" | "enable" | "install" | "download" | "fix-route-audit" | string; pack_id?: string; pack_name?: string; capability?: string; manifest_path?: string; manifest_url?: string; package_url?: string; frontend_url?: string; sha256?: string; size_bytes?: number; installed: boolean; enabled: boolean; downloadable: boolean; reason?: string; catalog_entry?: PackCatalogEntry; capability_info?: PackCapabilityIndexEntry; [key: string]: unknown };
export type PackCapabilityPrepareReport = { generated_at: string; capabilities: string[]; allowed: boolean; action: "use" | "enable" | "install" | "fix-route-audit" | string; plan: PackCapabilityPlanReport; use_steps?: PackCapabilityPrepareStep[]; enable_steps?: PackCapabilityPrepareStep[]; install_steps?: PackCapabilityPrepareStep[]; download_steps?: PackCapabilityPrepareStep[]; route_audit_fix_steps?: PackCapabilityPrepareStep[]; catalog_source_reports?: PackCatalogSourceReport[]; steps: PackCapabilityPrepareStep[]; step_count: number; download_count: number; enable_count: number; install_count: number; route_audit_issue_count: number; ready_count: number; unavailable_reasons?: string[]; route_audit_issues?: PackBackendRouteAuditEntry[]; [key: string]: unknown };
export type PackPruneResponse = { removed: string[]; kept: string[]; errors?: string[]; removed_count: number; kept_count: number; [key: string]: unknown };
export type PackInstallRequest = { manifestPath?: string; manifestUrl?: string; packageUrl?: string; sha256?: string; source?: string; download?: boolean };
export type PackStudioPlanRequest = { packId?: string; pack_id?: string; manifest?: PackManifest; goal?: string };
export type PackStudioPlanReport = { generated_at: string; pack_id: string; pack_name: string; version: string; source?: string; status?: string; installed: boolean; enabled: boolean; goal: string; risk_level: "low" | "medium" | "high" | string; summary: string; capabilities?: string[]; permissions?: string[]; frontend_paths?: string[]; backend_routes?: string[]; surfaces: string[]; editable: string[]; guarded: string[]; warnings?: string[]; editable_files: string[]; diff_preview: string; audit_steps: string[]; package_steps: string[]; rollback_steps: string[]; cogni_use: string[]; xiaoyu_prompt: string; [key: string]: unknown };
export type PackStudioInspectRequest = { packagePath?: string; package_path?: string; packageUrl?: string; package_url?: string; sha256?: string; goal?: string };
export type YqpackEntryReport = { path: string; kind: string; size_bytes: number; editable: boolean; reason: string; needs_source?: boolean; [key: string]: unknown };
export type YqpackInspectReport = { generated_at: string; source: string; sha256: string; expected_sha256?: string; sha256_match: boolean; size_bytes: number; manifest: PackManifest; entries: YqpackEntryReport[]; entry_count: number; editable_count: number; guarded_count: number; warnings?: string[]; plan: PackStudioPlanReport; [key: string]: unknown };
export type PackStudioWorkspaceRequest = PackStudioInspectRequest;
export type PackStudioWorkspaceReport = { generated_at: string; workspace_path: string; workspace_id: string; package_source: string; original_sha256: string; expected_sha256?: string; sha256_match: boolean; manifest: PackManifest; inspect: YqpackInspectReport; editable_files: string[]; guarded_files: string[]; audit_commands: string[]; repack_commands: string[]; rollback_commands: string[]; next_steps: string[]; warnings?: string[]; [key: string]: unknown };
export type PackStudioPatchRequest = { workspacePath?: string; workspace_path?: string; filePath?: string; file_path?: string; content: string; reason?: string; apply?: boolean };
export type PackStudioPatchReport = { generated_at: string; workspace_path: string; file_path: string; relative_path: string; applied: boolean; reason?: string; old_sha256?: string; new_sha256: string; diff_preview: string; warnings?: string[]; next_steps: string[]; [key: string]: unknown };
export type PacksClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };
export type PackCatalogSourceSummary = { total_sources: number; ok_sources: number; error_sources: number; manifest_count: number; matched_entries: number; errors: string[]; [key: string]: unknown };
export type PackCapabilityPrepareSummary = {
  kind: "pack_capability_prepare_summary";
  generated_at: string;
  capabilities: string[];
  allowed: boolean;
  action: string;
  plan: {
    allowed_count: number;
    blocked_count: number;
    use_count: number;
    enable_count: number;
    install_count: number;
    route_audit_issue_count: number;
    required_packs: PackCapabilityIndexEntry[];
    enable_packs: PackCapabilityIndexEntry[];
    install_capabilities: string[];
    catalog_install_hints: PackCatalogEntry[];
    catalog_download_hints: PackCatalogEntry[];
  };
  prepare: {
    step_count: number;
    ready_count: number;
    enable_count: number;
    install_count: number;
    download_count: number;
    route_audit_issue_count: number;
  } | null;
  steps: PackCapabilityPrepareStep[];
  catalog_source_reports: PackCatalogSourceReport[];
  route_audit_issues: PackBackendRouteAuditEntry[];
  unavailable_reasons: string[];
};

export class PacksClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Packs request failed with HTTP ${status}`); this.name = "PacksClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

function packSdkEntrypoints(pack: InstalledPack): PackSdkEntrypoint[] { return Object.entries(pack.manifest.sdk ?? {}).filter((entry): entry is [string, string] => typeof entry[1] === "string" && entry[1].trim().length > 0).map(([language, importPath]) => ({ packId: pack.manifest.id, packName: pack.manifest.name, language, importPath })); }
function normalizeRoutePath(path: string): string { const trimmed = path.trim().replace(/\/+$/, ""); return trimmed || "/"; }
function packRouteBindings(pack: InstalledPack): PackRouteBinding[] { const routeSdk = packSdkEntrypoints(pack); return (pack.manifest.frontend?.routes ?? []).map((route) => ({ packId: pack.manifest.id, packName: pack.manifest.name, path: route.path, component: route.component, title: route.title, assets: pack.manifest.frontend?.assets, distribution: pack.manifest.distribution, sdk: routeSdk })); }
function packBackendRouteBindings(pack: InstalledPack): PackBackendRouteBinding[] { return (pack.manifest.backend?.routeSpecs ?? []).map((route) => ({ packId: pack.manifest.id, packName: pack.manifest.name, method: route.method, path: route.path, description: route.description })); }
function packFrontendPaths(pack: InstalledPack): string[] { return Array.from(new Set([...(pack.manifest.frontend?.menus ?? []).map((menu) => menu.path), ...(pack.manifest.frontend?.routes ?? []).map((route) => route.path)].filter((path): path is string => typeof path === "string" && path.trim().length > 0))).sort(); }
function packCapabilityBindings(pack: InstalledPack): PackCapabilityBinding[] { const sdk = packSdkEntrypoints(pack); const frontendPaths = packFrontendPaths(pack); return (pack.manifest.backend?.capabilities ?? []).filter((capability) => capability.trim().length > 0).map((capability) => ({ capability, packId: pack.manifest.id, packName: pack.manifest.name, routes: [...(pack.manifest.backend?.routes ?? [])], permissions: [...(pack.manifest.backend?.permissions ?? [])], frontendPaths, sdk })); }
export function summarizeCatalogSourceReports(reports: readonly PackCatalogSourceReport[] = []): PackCatalogSourceSummary { const errors = new Set<string>(); const summary: PackCatalogSourceSummary = { total_sources: reports.length, ok_sources: 0, error_sources: 0, manifest_count: 0, matched_entries: 0, errors: [] }; for (const report of reports) { if (report.ok && (report.errors?.length ?? 0) === 0) summary.ok_sources += 1; else summary.error_sources += 1; summary.manifest_count += report.manifest_count; summary.matched_entries += report.matched_entries; for (const error of report.errors ?? []) if (error.trim()) errors.add(error); } summary.errors = [...errors]; return summary; }
export function hasCatalogSourceIssues(report: { source_reports?: PackCatalogSourceReport[]; catalog_source_reports?: PackCatalogSourceReport[]; errors?: string[] }): boolean { const reports = report.source_reports ?? report.catalog_source_reports; return (reports ?? []).some((source) => !source.ok || (source.errors?.length ?? 0) > 0) || (report.errors?.length ?? 0) > 0; }
export function summarizeCapabilityPrepare(plan: PackCapabilityPlanReport, prepare?: PackCapabilityPrepareReport | null): PackCapabilityPrepareSummary { return { kind: "pack_capability_prepare_summary", generated_at: prepare?.generated_at || plan.generated_at, capabilities: prepare?.capabilities || plan.capabilities, allowed: prepare?.allowed ?? plan.allowed, action: prepare?.action || plan.action, plan: { allowed_count: plan.allowed_count, blocked_count: plan.blocked_count, use_count: plan.use_count, enable_count: plan.enable_count, install_count: plan.install_count, route_audit_issue_count: plan.route_audit_issue_count, required_packs: plan.required_packs || [], enable_packs: plan.enable_packs || [], install_capabilities: plan.install_capabilities || [], catalog_install_hints: plan.catalog_install_hints || [], catalog_download_hints: plan.catalog_download_hints || [] }, prepare: prepare ? { step_count: prepare.step_count, ready_count: prepare.ready_count, enable_count: prepare.enable_count, install_count: prepare.install_count, download_count: prepare.download_count, route_audit_issue_count: prepare.route_audit_issue_count } : null, steps: prepare?.steps || [], catalog_source_reports: prepare?.catalog_source_reports || plan.catalog_source_reports || [], route_audit_issues: prepare?.route_audit_issues || plan.route_audit_issues || [], unavailable_reasons: prepare?.unavailable_reasons || plan.unavailable_reasons || [] }; }

export class PacksClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: PacksClientOptions) { if (!options.baseUrl) throw new Error("PacksClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("PacksClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }
  installed(): Promise<PacksListResponse> { return this.json<PacksListResponse>("GET", "/v1/packs/installed"); }
  list(): Promise<PacksListResponse> { return this.json<PacksListResponse>("GET", "/v1/packs"); }
  enabled(): Promise<PacksListResponse> { return this.json<PacksListResponse>("GET", "/v1/packs/enabled"); }
  catalog(options?: { capability?: string; q?: string }): Promise<PackCatalogReport> { const params = new URLSearchParams(); if (options?.capability) params.set("capability", options.capability); if (options?.q) params.set("q", options.q); const query = params.toString(); return this.json<PackCatalogReport>("GET", query ? `/v1/packs/catalog?${query}` : "/v1/packs/catalog"); }
  releaseCatalog(releases: string[]): Promise<PackReleaseCatalogReport> { return this.json<PackReleaseCatalogReport>("POST", "/v1/packs/release-catalog", { releases }); }
  capabilities(): Promise<PackCapabilityIndexReport> { return this.json<PackCapabilityIndexReport>("GET", "/v1/packs/capabilities"); }
  resolveCapability(capability: string): Promise<PackCapabilityResolveReport> { return this.json<PackCapabilityResolveReport>("GET", `/v1/packs/capabilities/resolve?capability=${encodeURIComponent(capability)}`); }
  gateCapability(capability: string): Promise<PackCapabilityGateReport> { return this.json<PackCapabilityGateReport>("GET", `/v1/packs/capabilities/gate?capability=${encodeURIComponent(capability)}`); }
  planCapabilities(capabilities: string[]): Promise<PackCapabilityPlanReport> { const params = capabilities.map((capability) => `capability=${encodeURIComponent(capability)}`).join("&"); return this.json<PackCapabilityPlanReport>("GET", `/v1/packs/capabilities/plan?${params}`); }
  prepareCapabilities(capabilities: string[]): Promise<PackCapabilityPrepareReport> { const params = capabilities.map((capability) => `capability=${encodeURIComponent(capability)}`).join("&"); return this.json<PackCapabilityPrepareReport>("GET", `/v1/packs/capabilities/prepare?${params}`); }
  backendModules(): Promise<PackBackendModulesResponse> { return this.json<PackBackendModulesResponse>("GET", "/v1/packs/backend-modules"); }
  backendRouteAudit(): Promise<PackBackendRouteAuditReport> { return this.json<PackBackendRouteAuditReport>("GET", "/v1/packs/backend-route-audit"); }
  capabilityCogniProfile(): Promise<PackCapabilityCogniProfileReport> { return this.json<PackCapabilityCogniProfileReport>("GET", "/v1/packs/capabilities/cogni-profile"); }
  studioPlan(request: PackStudioPlanRequest): Promise<PackStudioPlanReport> { return this.json<PackStudioPlanReport>("POST", "/v1/packs/studio/plan", { pack_id: request.pack_id ?? request.packId, manifest: request.manifest, goal: request.goal }); }
  studioInspect(request: PackStudioInspectRequest): Promise<YqpackInspectReport> { return this.json<YqpackInspectReport>("POST", "/v1/packs/studio/inspect", { package_path: request.package_path ?? request.packagePath, package_url: request.package_url ?? request.packageUrl, sha256: request.sha256, goal: request.goal }); }
  studioWorkspace(request: PackStudioWorkspaceRequest): Promise<PackStudioWorkspaceReport> { return this.json<PackStudioWorkspaceReport>("POST", "/v1/packs/studio/workspace", { package_path: request.package_path ?? request.packagePath, package_url: request.package_url ?? request.packageUrl, sha256: request.sha256, goal: request.goal }); }
  studioPatch(request: PackStudioPatchRequest): Promise<PackStudioPatchReport> { return this.json<PackStudioPatchReport>("POST", "/v1/packs/studio/patch", { workspace_path: request.workspace_path ?? request.workspacePath, file_path: request.file_path ?? request.filePath, content: request.content, reason: request.reason, apply: Boolean(request.apply) }); }
  install(request: PackInstallRequest): Promise<PackMutationResponse> { return this.json<PackMutationResponse>("POST", "/v1/packs/install", { manifest_path: request.manifestPath, manifest_url: request.manifestUrl, package_url: request.packageUrl, sha256: request.sha256, source: request.source, download: request.download }); }
  enable(id: string): Promise<PackMutationResponse> { return this.mutate("/v1/packs/enable", id); }
  disable(id: string): Promise<PackMutationResponse> { return this.mutate("/v1/packs/disable", id); }
  rollback(id: string): Promise<PackMutationResponse> { return this.mutate("/v1/packs/rollback", id); }
  prune(): Promise<PackPruneResponse> { return this.json<PackPruneResponse>("POST", "/v1/packs/prune", {}); }
  async frontendSync(): Promise<{ menus: PackFrontendMenu[]; routes: PackFrontendRoute[]; sdk: PackSdkEntrypoint[]; distributions: PackDistributionManifest[]; routeBindings: PackRouteBinding[]; backendRouteBindings: PackBackendRouteBinding[]; capabilityBindings: PackCapabilityBinding[]; packs: InstalledPack[] }> {
    const response = await this.enabled();
    const packs = response.packs ?? [];
    const sdk = packs.flatMap((pack) => packSdkEntrypoints(pack));
    return {
      packs,
      menus: packs.flatMap((pack) => pack.manifest.frontend?.menus ?? []).sort((a, b) => (a.order ?? 0) - (b.order ?? 0)),
      routes: packs.flatMap((pack) => pack.manifest.frontend?.routes ?? []),
      sdk,
      distributions: packs.map((pack) => pack.manifest.distribution).filter((distribution): distribution is PackDistributionManifest => Boolean(distribution?.packageUrl || distribution?.frontendUrl)),
      routeBindings: packs.flatMap((pack) => packRouteBindings(pack)),
      backendRouteBindings: packs.flatMap((pack) => packBackendRouteBindings(pack)),
      capabilityBindings: packs.flatMap((pack) => packCapabilityBindings(pack)),
    };
  }
  async routeBinding(path: string): Promise<PackRouteBinding | undefined> { const sync = await this.frontendSync(); const normalized = normalizeRoutePath(path); return sync.routeBindings.find((route) => normalizeRoutePath(route.path) === normalized); }
  private mutate(path: string, id: string): Promise<PackMutationResponse> { return this.json<PackMutationResponse>("POST", path, { id }); }
  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> { const headers = this.authHeaders(); const init: RequestInit = { method, headers }; if (body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(body); } const response = await this.fetchImpl(new URL(`${this.baseUrl}${path}`), init); const parsed = await parseResponse(response); if (!response.ok) throw new PacksClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}
export function createPacksClient(options: PacksClientOptions): PacksClient { return new PacksClient(options); }
