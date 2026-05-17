import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/packs-sdk.json"), "utf8"));
const failures = [];
function fail(message) { failures.push(message); }
function readRepoFile(path) { const fullPath = resolve(repoRoot, path); if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; } return readFileSync(fullPath, "utf8"); }
const requiredCapabilities = ["list", "catalog", "installed", "enabled", "capabilities", "resolveCapability", "gateCapability", "planCapabilities", "prepareCapabilities", "backendModules", "backendRouteAudit", "install", "enable", "disable", "rollback", "prune", "frontendSync", "routeBinding"];
const requiredRoutes = ["GET /v1/packs", "GET /v1/packs/catalog", "GET /v1/packs/installed", "GET /v1/packs/enabled", "GET /v1/packs/capabilities", "GET /v1/packs/capabilities/resolve", "GET /v1/packs/capabilities/gate", "GET /v1/packs/capabilities/plan", "GET /v1/packs/capabilities/prepare", "GET /v1/packs/backend-modules", "GET /v1/packs/backend-route-audit", "POST /v1/packs/install", "POST /v1/packs/enable", "POST /v1/packs/disable", "POST /v1/packs/rollback", "POST /v1/packs/prune"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
for (const actual of capabilityNames) if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
const manifestRoutes = new Set(manifest.routes ?? []);
for (const route of requiredRoutes) if (!manifestRoutes.has(route)) fail(`manifest missing route: ${route}`);
for (const route of manifestRoutes) if (!requiredRoutes.includes(route)) fail(`manifest has unexpected route: ${route}`);
const gateway = readRepoFile("internal/controlplane/gateway/handlers_packs.go") + "\n" + readRepoFile("internal/controlplane/gateway/gateway.go") + "\n" + readRepoFile("internal/controlplane/gateway/gateway_setters.go");
const packContract = readRepoFile("pkg/packruntime/manifest.go") + "\n" + readRepoFile("pkg/packruntime/registry.go") + "\n" + readRepoFile("pkg/packruntime/backend.go") + "\n" + readRepoFile("packs/AUTHORING.md");
for (const route of ['"/v1/packs"', '"/v1/packs/catalog"', '"/v1/packs/installed"', '"/v1/packs/enabled"', '"/v1/packs/capabilities"', '"/v1/packs/capabilities/resolve"', '"/v1/packs/capabilities/gate"', '"/v1/packs/capabilities/plan"', '"/v1/packs/capabilities/prepare"', '"/v1/packs/backend-modules"', '"/v1/packs/backend-route-audit"', '"/v1/packs/install"', '"/v1/packs/enable"', '"/v1/packs/disable"', '"/v1/packs/rollback"']) if (!gateway.includes(route)) fail(`gateway route not found: ${route}`);
for (const handler of ["handlePacksList", "handlePackCatalog", "packCatalogReport", "PackCatalogReport", "PackCatalogEntry", "SetPackCatalogSources", "handlePacksEnabled", "handlePackCapabilities", "handlePackCapabilityResolve", "handlePackCapabilityGate", "handlePackCapabilityPlan", "handlePackCapabilityPrepare", "packCapabilityIndexReport", "packCapabilityResolveReport", "packCapabilityGateReport", "packCapabilityPlanReport", "packCapabilityPrepareReport", "CapabilityIndexReport", "CapabilityIndexEntry", "CapabilityResolveReport", "CapabilityGateReport", "CapabilityPlanReport", "CapabilityPrepareReport", "CapabilityPrepareStep", "json:\"download_steps", "json:\"package_url", "handlePackBackendModules", "handlePackBackendRouteAudit", "backendRouteAuditReport", "json:\"route_audit", "json:\"enable_packs", "json:\"install_capabilities", "handlePackInstall", "handlePackEnable", "handlePackDisable", "handlePackRollback", "RegisterBackendPack", "registerBackendPack", "requirePackRoute", "backendPackRouteInfos", "BackendRouteInfo{Method", "Methods: methods", "normalizeBackendRouteMethods", "must declare an HTTP method"]) if (!gateway.includes(handler) && !packContract.includes(handler)) fail(`pack runtime gateway contract not found: ${handler}`);
for (const token of ["Manifest", "Registry", "BackendModule", "BackendRoute", "Method  string", "Methods []string", "BackendRouteSpec", "routeSpecs", "AllowsRoute", "BackendModuleInfo", "BackendRouteInfo", "PackCatalogReport", "PackCatalogEntry", "PackCatalogSourceReport", "json:\"source_reports", "json:\"manifest_count", "json:\"matched_entries", "json:\"install_hints", "json:\"download_hints", "CapabilityIndexReport", "CapabilityIndexEntry", "CapabilityResolveReport", "CapabilityGateReport", "CapabilityPlanReport", "CapabilityPrepareReport", "CapabilityPrepareStep", "json:\"download_steps", "json:\"package_url", "json:\"enabled_capabilities", "json:\"enabled_entries", "json:\"route_audit", "json:\"enable_packs", "json:\"install_capabilities", "json:\"sdk_typescript", "BackendRouteAuditReport", "BackendRouteAuditEntry", "json:\"methods,omitempty\"", "frontend", "menus", "routes", "sdk", "DistributionManifest", "PackArtifacts", "PreviousArtifacts", "PruneArtifacts", "CacheDistribution", "InstallWithArtifacts", "packageUrl", "frontendUrl", "sha256", "rollback"]) if (!packContract.includes(token)) fail(`pack runtime contract missing token: ${token}`);
if (!packContract.includes("json:\"catalog_source_reports")) fail("pack runtime contract missing token: json:\"catalog_source_reports");
function symbolAlternatives(symbol) { const raw = symbol.split("#").pop().replace(/\(\).*$/, ""); const tail = raw.replace(/^.*\./, "").replace(/^.*::/, ""); const snake = tail.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`).replace(/^_/, ""); return [raw, tail, tail.replace(/^[A-Z]/, (c) => c.toLowerCase()), tail.replace(/^[a-z]/, (c) => c.toUpperCase()), snake].filter(Boolean); }
for (const [language, config] of Object.entries(manifest.languages ?? {})) {
  const combinedSource = (config.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const capability of requiredCapabilities) if (!config.entrypoints?.[capability]) fail(`${language} entrypoints missing required Packs capability: ${capability}`);
  for (const [capability, symbol] of Object.entries(config.entrypoints ?? {})) {
    if (!capabilityNames.has(capability)) fail(`${language} entrypoint references unknown Packs capability: ${capability}`);
    if (!symbolAlternatives(symbol).some((candidate) => combinedSource.includes(candidate))) fail(`${language} implementation missing entrypoint for ${capability}: ${symbol}`);
  }
  for (const doc of config.docs ?? []) {
    const text = readRepoFile(doc);
    if (!/Pack Runtime SDK|Packs SDK|createPacksClient|frontendSync|sync\.sdk|TypeScript pack SDK import example|backend-modules|v1\/packs/i.test(text)) fail(`${language} doc ${doc} does not mention Packs SDK helpers`);
  }
}
for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/Pack Runtime SDK|Packs SDK|frontendSync|sync\.sdk|TypeScript pack SDK import example|backend-modules|v1\/packs/i.test(text)) fail(`overview doc ${doc} does not describe Packs SDK surface`);
}
const packsSource = readRepoFile("sdk/typescript/src/packs.ts");
for (const token of ["PackSdkEntrypoint", "PackDistributionManifest", "PackArtifacts", "PackPruneResponse", "PackRouteBinding", "PackBackendRouteBinding", "PackCatalogReport", "PackCatalogEntry", "PackCatalogSourceReport", "source_reports", "catalog_source_reports", "catalog(", "PackCapabilityIndexReport", "PackCapabilityIndexEntry", "PackCapabilityResolveReport", "PackCapabilityGateReport", "PackCapabilityPlanReport", "PackCapabilityPrepareReport", "PackCapabilityPrepareStep", "PackCapabilityBinding", "capabilities()", "resolveCapability(", "gateCapability(", "planCapabilities(", "prepareCapabilities(", "capabilityBindings:", "packCapabilityBindings", "PackBackendRouteAuditReport", "backendRouteAudit()", "backendRouteBindings:", "previousArtifacts", "download?: boolean", "prune()", "routeBindings:", "routeBinding(path", "packRouteBindings", "packSdkEntrypoints", "Object.entries(pack.manifest.sdk", "importPath", "distributions:", "pack.manifest.distribution", "methods?: string[]"]) if (!packsSource.includes(token)) fail(`Packs SDK frontendSync missing SDK/distribution sync token: ${token}`);
for (const binding of manifest.packBindings ?? []) {
  const text = readRepoFile(binding);
  if (!/packManifest|sdkImport|routes|frontend/.test(text)) fail(`pack binding manifest ${binding} is incomplete`);
}
if (!manifest.languages?.typescript) fail("manifest must expose a TypeScript Packs SDK implementation");
if (failures.length) { console.error("Packs SDK manifest check failed:"); for (const failure of failures) console.error(`- ${failure}`); process.exit(1); }
console.log(`Packs SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
