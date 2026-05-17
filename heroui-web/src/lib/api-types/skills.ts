// ══════════════════════════════════════════════════════════════════════════
// Skills / Plugins / SkillHub / Market / Iterate Types
// ══════════════════════════════════════════════════════════════════════════
// Everything that lives in the "what can the agent do" surface — from
// statically registered skills, to dynamically generated ones, to the
// remote SkillHub marketplace and the self-iteration proposal pipeline.

export interface SkillInfo {
  name: string;
  description: string;
  parameters: Record<string, unknown>;
  category?: string;
  usage_total?: number;
  success_rate?: number;
}

export interface SkillCategory {
  id: string;
  name: string;
  description: string;
}

export interface DynamicSkillDef {
  name: string;
  description: string;
  parameters: Record<string, unknown>;
  instruction: string;
  composed_of: string[];
  source: string;
  approval_status: string; // "draft" | "approved"
}

export interface PluginInfo {
  name: string;
  description: string;
  enabled: boolean;
  skill_count: number;
}

export interface PluginMeta {
  name: string;
  description: string;
  enabled: boolean;
  skill_count: number;
  source: "builtin" | "script";
  language?: string;
}

export interface PluginFile {
  name: string;
  content: string;
  size: number;
}

export interface PluginUITab {
  key: string;
  label: string;
  label_en?: string;
  icon: string;
  description?: string;
  plugin: string;
}

// --- Pack Runtime ---
// The backend pack registry is the source of truth for optional capability
// packs. Frontend shells should render menus/routes from enabled packs instead
// of hardcoding every new feature into the main app.
export type PackStatus = "installed" | "enabled" | "disabled" | string;

export interface PackBackendRouteSpec {
  method: string;
  path: string;
  description?: string;
}

export interface PackBackendManifest {
  capabilities?: string[];
  routes?: string[];
  routeSpecs?: PackBackendRouteSpec[];
  permissions?: string[];
}

export interface PackFrontendMenu {
  key: string;
  label: string;
  path: string;
  icon?: string;
  order?: number;
}

export interface PackFrontendRoute {
  path: string;
  component: string;
  title?: string;
}

export interface PackFrontendAssets {
  type?: string;
  entry?: string;
}

export interface PackFrontendManifest {
  menus?: PackFrontendMenu[];
  routes?: PackFrontendRoute[];
  assets?: PackFrontendAssets;
}

export interface PackSDKManifest {
  typescript?: string;
  go?: string;
  python?: string;
}

export interface PackDistributionManifest {
  manifestUrl?: string;
  packageUrl?: string;
  frontendUrl?: string;
  sha256?: string;
  sizeBytes?: number;
}

export interface PackUpdateManifest {
  channel?: string;
  rollback: boolean;
}

export interface PackManifest {
  id: string;
  name: string;
  version: string;
  description?: string;
  requiresCore?: string;
  optional: boolean;
  defaultState?: "enabled" | "disabled" | string;
  backend: PackBackendManifest;
  frontend: PackFrontendManifest;
  sdk?: PackSDKManifest;
  distribution?: PackDistributionManifest;
  update?: PackUpdateManifest;
  metadata?: Record<string, string>;
}

export interface PackArtifacts {
  packagePath?: string;
  sha256?: string;
  sizeBytes?: number;
  cachedAt?: string;
}

export interface InstalledPack {
  manifest: PackManifest;
  status: "enabled" | "disabled" | "rolled_back" | string;
  source: string;
  artifacts?: PackArtifacts;
  previousArtifacts?: PackArtifacts;
  installedAt?: string;
  updatedAt?: string;
  previousVersion?: string;
}

export interface PackListResponse {
  packs: InstalledPack[];
  enabled?: InstalledPack[];
  count: number;
}

export interface PackMutationResponse {
  pack: InstalledPack;
  status: InstalledPack["status"];
}

export interface PackBackendRouteInfo {
  method?: string;
  methods?: string[];
  path: string;
  auth?: string;
}

export interface PackBackendModuleInfo {
  pack_id: string;
  routes: PackBackendRouteInfo[];
}

export interface PackBackendModulesResponse {
  modules: PackBackendModuleInfo[];
  count: number;
}

export interface PackCapabilityIndexEntry {
  capability: string;
  pack_id: string;
  pack_name: string;
  pack_status: string;
  enabled: boolean;
  optional: boolean;
  routes?: string[];
  permissions?: string[];
  sdk_typescript?: string;
  frontend_paths?: string[];
}

export interface PackCapabilityIndexReport {
  generated_at: string;
  packs: number;
  enabled_packs: number;
  capabilities: number;
  enabled_capabilities: number;
  entries: PackCapabilityIndexEntry[];
}

export interface PackCapabilityResolveReport {
  generated_at: string;
  capability: string;
  found: boolean;
  enabled: boolean;
  action: "use" | "enable" | "install" | string;
  preferred?: PackCapabilityIndexEntry;
  entries: PackCapabilityIndexEntry[];
  enabled_entries: PackCapabilityIndexEntry[];
}

export interface PackCapabilityGateReport {
  generated_at: string;
  capability: string;
  allowed: boolean;
  action: "use" | "enable" | "install" | string;
  reason?: string;
  resolution: PackCapabilityResolveReport;
  route_audit?: PackBackendRouteAuditEntry[];
}

export interface PackCapabilityPlanReport {
  generated_at: string;
  capabilities: string[];
  allowed: boolean;
  action: "use" | "enable" | "install" | "fix-route-audit" | string;
  allowed_count: number;
  blocked_count: number;
  use_count: number;
  enable_count: number;
  install_count: number;
  route_audit_issue_count: number;
  gates: PackCapabilityGateReport[];
  required_packs?: PackCapabilityIndexEntry[];
  enable_packs?: PackCapabilityIndexEntry[];
    install_capabilities?: string[];
    catalog_install_hints?: PackCatalogEntry[];
    catalog_download_hints?: PackCatalogEntry[];
    catalog_source_reports?: PackCatalogSourceReport[];
    route_audit_issues?: PackBackendRouteAuditEntry[];
    unavailable_reasons?: string[];
    downloadable_pack_hints?: PackCapabilityIndexEntry[];
}

export interface PackCapabilityPrepareStep {
  action: "use" | "enable" | "install" | "download" | "fix-route-audit" | string;
  pack_id?: string;
  pack_name?: string;
  capability?: string;
  manifest_path?: string;
  manifest_url?: string;
  package_url?: string;
  frontend_url?: string;
  sha256?: string;
  size_bytes?: number;
  installed: boolean;
  enabled: boolean;
  downloadable: boolean;
  reason?: string;
  catalog_entry?: PackCatalogEntry;
  capability_info?: PackCapabilityIndexEntry;
}

export interface PackCapabilityPrepareReport {
  generated_at: string;
  capabilities: string[];
  allowed: boolean;
  action: "use" | "enable" | "install" | "fix-route-audit" | string;
  plan: PackCapabilityPlanReport;
  use_steps?: PackCapabilityPrepareStep[];
  enable_steps?: PackCapabilityPrepareStep[];
    install_steps?: PackCapabilityPrepareStep[];
    download_steps?: PackCapabilityPrepareStep[];
    route_audit_fix_steps?: PackCapabilityPrepareStep[];
    catalog_source_reports?: PackCatalogSourceReport[];
    steps: PackCapabilityPrepareStep[];
  step_count: number;
  download_count: number;
  enable_count: number;
  install_count: number;
  route_audit_issue_count: number;
  ready_count: number;
  unavailable_reasons?: string[];
  route_audit_issues?: PackBackendRouteAuditEntry[];
}

export interface PackCatalogEntry {
  manifest_path?: string;
  source?: string;
  manifest: PackManifest;
  installed: boolean;
  enabled: boolean;
  status?: PackStatus;
  update_action: "use" | "enable" | "install" | "update" | string;
  downloadable: boolean;
}

export interface PackCatalogSourceReport {
  source: string;
  ok: boolean;
  manifest_count: number;
  matched_entries: number;
  errors?: string[];
}

export interface PackCatalogReport {
  generated_at: string;
  sources: string[];
  source_reports?: PackCatalogSourceReport[];
  count: number;
  installed: number;
  enabled: number;
  downloadable: number;
  capabilities: number;
  capability?: string;
  query?: string;
  entries: PackCatalogEntry[];
  install_hints?: PackCatalogEntry[];
  enable_hints?: PackCatalogEntry[];
  download_hints?: PackCatalogEntry[];
  errors?: string[];
}

export interface PackBackendRouteAuditEntry {
  pack_id: string;
  pack_name?: string;
  pack_status?: string;
  enabled: boolean;
  status: "ok" | "missing" | "method-mismatch" | "undeclared" | "pack-not-installed" | "registry-unavailable" | string;
  declared: boolean;
  mounted: boolean;
  method?: string;
  methods?: string[];
  path: string;
  auth?: string;
  description?: string;
  issues?: string[];
}

export interface PackBackendRouteAuditReport {
  generated_at: string;
  packs: number;
  enabled_packs: number;
  mounted_modules: number;
  declared_routes: number;
  mounted_routes: number;
  ok_routes: number;
  missing_routes: number;
  method_mismatches: number;
  undeclared_routes: number;
  entries: PackBackendRouteAuditEntry[];
}

// --- SkillHub ---

export interface SkillHubItem {
  name: string;
  description: string;
  version: string;
  author: string;
  rating: number;
  source: string; // "local" | "clawhub"
  installed: boolean;
}

export interface SkillHubInstalledItem {
  slug: string;
  name: string;
  version: string;
  description: string;
  source: string;
  security_score: number;
  installed_at: string;
  updated_at: string;
  enabled: boolean;
}

export interface AuditFinding {
  layer: string;
  severity: number;
  rule: string;
  detail: string;
}

export interface AuditReport {
  slug: string;
  score: number;
  passed: boolean;
  auto_approve: boolean;
  findings: AuditFinding[];
  static_score: number;
  perm_score: number;
  sandbox_score: number;
}

export interface SkillHubDetail {
  slug: string;
  name: string;
  description: string;
  version: string;
  author: string;
  rating: number;
  rating_count: number;
  installs: number;
  category: string;
  tags: string[];
  license: string;
  installed: boolean;
  source: string;
  permissions?: string[];
  security_score: number;
  audit_report?: AuditReport;
  content?: string;
  installed_at?: string;
  updated_at?: string;
}

export interface SkillUpdateInfo {
  slug: string;
  name: string;
  current_version: string;
  latest_version: string;
  has_update: boolean;
}

export interface SkillVersionInfo {
  version: string;
  installed_at?: string;
  current: boolean;
}

export interface SkillPolicy {
  min_score: number;
  trusted_authors: string[];
  blocked_authors: string[];
  allowed_slugs: string[];
  blocked_slugs: string[];
  max_perm_level: string;
  require_audit: boolean;
  auto_approve_min: number;
}

export interface PolicyCheckResult {
  allowed: boolean;
  reason?: string;
  auto_approve?: boolean;
}

export interface MarketAnalyticsSkill {
  slug: string;
  name: string;
  author: string;
  version: string;
  installs: number;
  rating: number;
  security_score: number;
  enabled: boolean;
}

export interface MarketAnalytics {
  total_skills: number;
  installed_count: number;
  total_installs: number;
  avg_score: number;
  categories: Record<string, number>;
  top_installed: MarketAnalyticsSkill[];
  top_rated: MarketAnalyticsSkill[];
  security_stats: Record<string, number>;
}

// --- SkillGrow ---

export interface SkillGrowPattern {
  pattern: string;
  count: number;
  suggestion: string;
  first_seen: string;
  last_seen: string;
}

export interface SkillSuggestion {
  name: string;
  description: string;
  trigger: string;
  confidence: number;
}

// --- Iterate (self-improvement proposals) ---

export interface IterateProposal {
  id: string;
  type: string;
  title: string;
  description: string;
  status: "pending" | "approved" | "rejected" | "applied";
  created_at: string;
}

export interface IterateStatus {
  enabled: boolean;
  running: boolean;
  last_run?: string;
  proposals_pending: number;
  token_budget: number;
  tokens_used: number;
}
