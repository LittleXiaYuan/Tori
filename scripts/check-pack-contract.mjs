import { existsSync, readdirSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { join, relative, resolve, sep } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const failures = [];

function fail(message) {
  failures.push(message);
}

function readText(path) {
  const fullPath = resolve(repoRoot, path);
  if (!existsSync(fullPath)) {
    fail(`missing file: ${path}`);
    return "";
  }
  return readFileSync(fullPath, "utf8");
}

function readJSON(path) {
  const text = readText(path);
  if (!text) return undefined;
  try {
    return JSON.parse(text);
  } catch (error) {
    fail(`invalid json: ${path}: ${error.message}`);
    return undefined;
  }
}

function walk(dir) {
  const fullDir = resolve(repoRoot, dir);
  if (!existsSync(fullDir)) return [];
  const out = [];
  for (const entry of readdirSync(fullDir, { withFileTypes: true })) {
    const fullPath = join(fullDir, entry.name);
    const rel = relative(repoRoot, fullPath).split(sep).join("/");
    if (entry.isDirectory()) out.push(...walk(rel));
    else out.push(rel);
  }
  return out;
}

const authoring = readText("packs/AUTHORING.md");
const packRuntimeBlueprint = readText("doc/PACK-RUNTIME-BLUEPRINT.md");
const englishGuide = readText("docs/guide/pack-runtime.md") + "\n" + readText("docs/guide/pack-runtime-state.md");
const chineseGuide = readText("docs/zh/guide/pack-runtime.md") + "\n" + readText("docs/zh/guide/pack-runtime-state.md");
const docsConfig = readText("docs/.vitepress/config.ts");
const scaffoldScript = readText("scripts/scaffold-pack.mjs");
const scaffoldCheck = readText("scripts/check-pack-scaffold.mjs");
const completionCheck = readText("scripts/check-pack-runtime-completion.mjs");
for (const token of [
  "packs/examples/wasm-plugin-pack",
  "yunque.pack.wasm-plugin",
  "pack-shell-before-runtime-hosts",
  "yunque-client/wasm-plugin",
  "WASM Plugin Pack shell 闭环",
  "Host ABI 权限强执行",
]) {
  if (!packRuntimeBlueprint.includes(token)) fail(`PACK-RUNTIME-BLUEPRINT.md missing WASM Plugin token: ${token}`);
}
for (const token of [
  "packs/examples/skill-anomaly-pack",
  "yunque.pack.skill-anomaly",
  "pack-shell-before-audit-hook",
  "yunque-client/skill-anomaly",
  "Skill Anomaly Pack shell 闭环",
  "Trust Score 惩罚",
]) {
  if (!packRuntimeBlueprint.includes(token)) fail(`PACK-RUNTIME-BLUEPRINT.md missing Skill Anomaly token: ${token}`);
}
for (const token of [
  "Pack Authoring Contract",
  "packruntime.BackendModule",
  "GatewayConfig.BackendPacks",
  "RegisterBackendPack",
  "/v1/packs/enabled",
  "frontend.menus",
  "frontend.routes",
  "sdk.typescript",
  "scripts/scaffold-pack.mjs",
]) {
  if (!authoring.includes(token)) fail(`AUTHORING.md missing token: ${token}`);
}

for (const [name, text] of [["docs/guide/pack-runtime.md", englishGuide], ["docs/zh/guide/pack-runtime.md", chineseGuide]]) {
  for (const token of ["Pack Runtime", "packruntime.BackendModule", "frontend.menus", "sdk.typescript", "check-pack-contract.mjs", "scaffold-pack.mjs"]) {
    if (!text.includes(token)) fail(`${name} missing token: ${token}`);
  }
}

for (const token of ["packs/examples", "internal/packs", "heroui-web/src/app/packs", "update: { channel: \"stable\", rollback: true }", "sdk: { typescript: sdk }", "routeSpecs", "routeMethod", "--method", "distribution:", "packageUrl", "frontendUrl", "sha256", "RegisterBackendPack", "--dry-run", "--json", "dryRun", "jsonOutput"]) {
  if (!scaffoldScript.includes(token)) fail(`scaffold-pack.mjs missing token: ${token}`);
}
for (const token of ["verifier-pack", "--dry-run", "--json", "manifest.backend.routeSpecs", "manifest.frontend.menus", "manifest.frontend.routes", "manifest.sdk.typescript", "manifest.distribution.packageUrl", "manifest.distribution.frontendUrl", "manifest.distribution.sha256", "manifest.update.rollback"]) {
  if (!scaffoldCheck.includes(token)) fail(`check-pack-scaffold.mjs missing token: ${token}`);
}
for (const token of ["Pack Runtime completion audit", "RegisterBackendPack", "frontendSync()", "PruneArtifacts", "TypeScript packs SDK", "backup-pack 示例包"]) {
  if (!completionCheck.includes(token)) fail(`check-pack-runtime-completion.mjs missing token: ${token}`);
}
const scaffoldCheckResult = spawnSync(process.execPath, ["scripts/check-pack-scaffold.mjs"], { cwd: repoRoot, encoding: "utf8" });
if (scaffoldCheckResult.status !== 0) {
  fail(`check-pack-scaffold.mjs failed: ${scaffoldCheckResult.stderr || scaffoldCheckResult.stdout}`);
}
if (!process.env.PACK_COMPLETION_AUDIT_CHILD) {
  const completionCheckResult = spawnSync(process.execPath, ["scripts/check-pack-runtime-completion.mjs"], { cwd: repoRoot, encoding: "utf8", env: { ...process.env, PACK_COMPLETION_AUDIT_CHILD: "1" } });
  if (completionCheckResult.status !== 0) {
    fail(`check-pack-runtime-completion.mjs failed: ${completionCheckResult.stderr || completionCheckResult.stdout}`);
  }
}

if (!docsConfig.includes("/guide/pack-runtime") || !docsConfig.includes("/zh/guide/pack-runtime") || !docsConfig.includes("/guide/pack-runtime-state") || !docsConfig.includes("/zh/guide/pack-runtime-state")) {
  fail("docs vitepress config must expose Pack Runtime guide and state pages in both locales");
}

const gatewaySource = readText("internal/controlplane/gateway/handlers_packs.go")
  + "\n"
  + readText("internal/controlplane/gateway/gateway.go")
  + "\n"
  + readText("cmd/agent/init_tasks.go")
  + "\n"
  + readText("internal/controlplane/gateway/gateway_setters.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_packs_test.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_cogni.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_browser_pack.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_rpa_replay_pack_test.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_cogni_experience_test.go")
  + "\n"
  + readText("internal/packs/lora/handler.go")
  + "\n"
  + readText("internal/packs/cognikernel/handler.go")
  + "\n"
  + readText("internal/packs/browserintent/handler.go")
  + "\n"
  + readText("internal/packs/rpareplay/handler.go")
  + "\n"
  + readText("internal/packs/sbomdrift/handler.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_sbom_drift_pack_test.go")
  + "\n"
  + readText("internal/packs/skillanomaly/handler.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_skill_anomaly_pack_test.go")
  + "\n"
  + readText("internal/packs/wasmplugin/handler.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_wasm_plugin_pack_test.go");
for (const token of ["BackendPacks", "RegisterBackendPack", "registerBackendPack", "requirePackRoute", "backendPackAuth", "BackendRouteAuthPassthrough", "backendPackRoutes", "backendPackRouteInfos", "BackendRouteInfo{Method", "Methods: methods", "normalizeBackendRouteMethods", "must declare an HTTP method", "handlePackBackendModules", "handlePackPrune", "/v1/packs/prune", "Download     bool", "CacheDistribution", "PruneArtifacts", "InstallWithArtifacts", "route conflict", "TestRegisterBackendPackMountsModuleAfterGatewayConstruction", "TestRegisterBackendPackIsIdempotentForSamePackRoute", "TestRegisterBackendPackPanicsOnRouteConflict", "TestPackBackendModulesExposeMountedRoutes", "TestBackendPackMultiMethodRouteInfoAndGate", "TestBackendPackPassthroughAuthRouteKeepsPackGate", "expected mounted route method to be preserved", "expected downloaded artifacts to be recorded", "ensureBuiltinPacks", "loadBuiltinPackManifest", "packs/examples/lora-pack/pack.json", "packs/examples/cogni-kernel-pack/pack.json", "packs/examples/browser-intent-pack/pack.json", "packs/examples/rpa-replay-pack/pack.json", "packs/examples/sbom-drift-pack/pack.json", "packs/examples/skill-anomaly-pack/pack.json", "packs/examples/wasm-plugin-pack/pack.json", "backuppack.DefaultHandler()", "lorapack.NewHandler", "cognikernelpack.NewHandler", "browserintentpack.NewHandler", "rpareplaypack.New", "cfg.DataPath(\"rpa-replay\")", "sbomdriftpack.New", "cfg.DataPath(\"sbom-drift\")", "skillanomalypack.New", "cfg.DataPath(\"skill-anomaly\")", "HandleCogniKernelPack", "HandleBrowserIntentPack", "BackendPacks: []packruntime.BackendModule"]) {
  if (!gatewaySource.includes(token)) fail(`gateway pack registration missing token: ${token}`);
}
if (/must be called before Gateway routes are registered/.test(gatewaySource)) {
  fail("RegisterBackendPack must remain usable after Gateway construction");
}

const backendContract = readText("pkg/packruntime/backend.go") + "\n" + readText("pkg/packruntime/registry.go");
for (const token of ["type BackendRoute", "Method  string", "Methods []string", "Path    string", "Auth    BackendRouteAuthMode", "type BackendRouteAuthMode", "BackendRouteAuthPassthrough", "type BackendRouteInfo", "Method  string", "Methods []string", "json:\"methods,omitempty\"", "json:\"auth,omitempty\"", "type BackendModuleInfo", "type BackendModule", "PackID() string", "Routes() []BackendRoute"]) {
  if (!backendContract.includes(token)) fail(`packruntime backend contract missing token: ${token}`);
}

const packFiles = [
  ...walk("packs/examples").filter((path) => path.endsWith("/pack.json")),
  ...walk("packs/templates").filter((path) => path.endsWith("/pack.json")),
].sort();

if (packFiles.length === 0) {
  fail("no pack manifests found under packs/examples or packs/templates");
}

for (const path of packFiles) {
  const manifest = readJSON(path);
  if (!manifest) continue;
  if (!manifest.id || !/^yunque\.pack\.[a-z0-9][a-z0-9.-]*$/.test(manifest.id)) {
    fail(`${path}: id must use yunque.pack.<name> format`);
  }
  if (!manifest.name) fail(`${path}: name is required`);
  if (!manifest.version) fail(`${path}: version is required`);
  if (manifest.defaultState && !["enabled", "disabled"].includes(manifest.defaultState)) {
    fail(`${path}: defaultState must be enabled or disabled`);
  }
  const routes = manifest.backend?.routes ?? [];
  if (!Array.isArray(routes) || routes.length === 0) fail(`${path}: backend.routes must not be empty`);
  for (const route of routes) {
    if (typeof route !== "string" || !route.startsWith("/")) fail(`${path}: invalid backend route: ${route}`);
  }
  const routeSpecs = manifest.backend?.routeSpecs ?? [];
  if (!Array.isArray(routeSpecs) || routeSpecs.length === 0) fail(`${path}: backend.routeSpecs must not be empty`);
  const backendRouteSet = new Set(routes);
  for (const [index, route] of routeSpecs.entries()) {
    if (!route?.method || typeof route.method !== "string") fail(`${path}: backend.routeSpecs[${index}].method is required`);
    if (route?.method && route.method !== route.method.toUpperCase()) fail(`${path}: backend.routeSpecs[${index}].method must be uppercase`);
    if (!route?.path || typeof route.path !== "string" || !route.path.startsWith("/")) fail(`${path}: backend.routeSpecs[${index}].path must start with /`);
    if (route?.path && !backendRouteSet.has(route.path)) fail(`${path}: backend.routeSpecs[${index}].path must also be present in backend.routes`);
  }
  const menus = manifest.frontend?.menus ?? [];
  if (!Array.isArray(menus) || menus.length === 0) fail(`${path}: frontend.menus must not be empty`);
  const frontendRoutes = manifest.frontend?.routes ?? [];
  const frontendRoutePaths = new Set(Array.isArray(frontendRoutes) ? frontendRoutes.map((route) => route?.path).filter(Boolean) : []);
  for (const [index, menu] of menus.entries()) {
    for (const key of ["key", "label", "path"]) {
      if (!menu?.[key]) fail(`${path}: frontend.menus[${index}].${key} is required`);
    }
    if (menu?.path && !String(menu.path).startsWith("/packs/")) {
      fail(`${path}: frontend.menus[${index}].path must live under /packs/ to keep pack UI out of the main shell`);
    }
    if (menu?.path && !frontendRoutePaths.has(menu.path)) {
      fail(`${path}: frontend.menus[${index}].path must have a matching frontend.routes entry`);
    }
  }
  if (!Array.isArray(frontendRoutes) || frontendRoutes.length === 0) fail(`${path}: frontend.routes must not be empty`);
  for (const [index, route] of frontendRoutes.entries()) {
    if (!route?.path) fail(`${path}: frontend.routes[${index}].path is required`);
    if (route?.path && !String(route.path).startsWith("/packs/")) {
      fail(`${path}: frontend.routes[${index}].path must live under /packs/ to keep pack UI out of the main shell`);
    }
    if (!route?.component) fail(`${path}: frontend.routes[${index}].component is required`);
  }
  if (!manifest.sdk?.typescript) fail(`${path}: sdk.typescript is required for lightweight external callers`);
  if (!manifest.distribution?.packageUrl) fail(`${path}: distribution.packageUrl is required for downloadable incremental packs`);
  if (!manifest.distribution?.frontendUrl) fail(`${path}: distribution.frontendUrl is required for frontend synchronized updates`);
  if (!manifest.distribution?.sha256) fail(`${path}: distribution.sha256 is required for package verification`);
  if (manifest.update?.rollback !== true) fail(`${path}: update.rollback should be true for reversible pack updates`);
}

const backupManifest = readJSON("packs/examples/backup-pack/pack.json");
const backupSource = readText("internal/packs/backup/handler.go");
if (backupManifest) {
  if (!backupSource.includes(`const PackID = "${backupManifest.id}"`)) {
    fail("backup pack handler PackID must match packs/examples/backup-pack/pack.json");
  }
  for (const route of backupManifest.backend?.routes ?? []) {
    if (!backupSource.includes(route)) fail(`backup handler missing route declared in manifest: ${route}`);
  }
}

const loraManifest = readJSON("packs/examples/lora-pack/pack.json");
const loraSource = readText("internal/packs/lora/handler.go");
const loraPage = readText("heroui-web/src/app/packs/lora/page.tsx");
const legacyLoraPage = readText("heroui-web/src/app/lora/page.tsx");
if (loraManifest) {
  if (!loraSource.includes(`const PackID = "${loraManifest.id}"`)) {
    fail("lora pack handler PackID must match packs/examples/lora-pack/pack.json");
  }
  for (const route of loraManifest.backend?.routes ?? []) {
    if (!loraSource.includes(route)) fail(`lora handler missing route declared in manifest: ${route}`);
  }
  for (const method of ["http.MethodGet", "http.MethodPost", "http.MethodPut", "http.MethodPatch"]) {
    if (!loraSource.includes(method)) fail(`lora handler missing method gate declaration: ${method}`);
  }
}
if (loraPage.includes('from "@/lib/api"') || loraPage.includes("api.getLoRA") || !loraPage.includes("createLoRAPackClient")) {
  fail("lora pack page must use lora-pack-client instead of monolithic api object");
}
if (!legacyLoraPage.includes('redirect("/packs/lora")')) {
  fail("legacy /lora page should redirect to /packs/lora");
}

const cogniKernelManifest = readJSON("packs/examples/cogni-kernel-pack/pack.json");
const cogniKernelSource = readText("internal/packs/cognikernel/handler.go");
const cogniGatewayBridge = readText("internal/controlplane/gateway/handlers_cogni.go");
const cogniKernelPage = readText("heroui-web/src/app/packs/cognis/page.tsx");
const legacyCogniPage = readText("heroui-web/src/app/cognis/page.tsx");
const cogniKernelClient = readText("heroui-web/src/lib/cogni-kernel-pack-client.ts");
if (cogniKernelManifest) {
  if (!cogniKernelSource.includes(`const PackID = "${cogniKernelManifest.id}"`)) {
    fail("Cogni Kernel pack handler PackID must match packs/examples/cogni-kernel-pack/pack.json");
  }
  for (const route of cogniKernelManifest.backend?.routes ?? []) {
    if (!cogniKernelSource.includes(route)) fail(`Cogni Kernel handler missing route declared in manifest: ${route}`);
  }
  for (const method of ["http.MethodGet", "http.MethodPost", "http.MethodDelete"]) {
    if (!cogniKernelSource.includes(method)) fail(`Cogni Kernel handler missing method gate declaration: ${method}`);
  }
  if (cogniKernelManifest.frontend?.menus?.[0]?.path !== "/packs/cognis") fail("Cogni Kernel menu path must stay under /packs/cognis");
  if (cogniKernelManifest.sdk?.typescript !== "yunque-client/cognis") fail("Cogni Kernel SDK import must stay yunque-client/cognis");
}
if (!cogniGatewayBridge.includes("HandleCogniKernelPack") || !cogniGatewayBridge.includes("g.handleCognis(w, r)")) {
  fail("Cogni Kernel Gateway bridge must delegate through HandleCogniKernelPack");
}
if (cogniKernelPage.includes('from "@/lib/api"') || cogniKernelPage.includes("api.listCognis") || !cogniKernelPage.includes("createCogniKernelPackClient")) {
  fail("Cogni Kernel pack page must use cogni-kernel-pack-client instead of monolithic api object");
}
if (!legacyCogniPage.includes('redirect("/packs/cognis")')) {
  fail("legacy /cognis page should redirect to /packs/cognis");
}
for (const token of ["createCogniKernelPackClient", "/v1/cognis", "/v1/cognis/reload", "/v1/cognis/alerts", "/v1/cognis/export", 'method: "DELETE"']) {
  if (!cogniKernelClient.includes(token)) fail(`cogni-kernel-pack-client missing token: ${token}`);
}
const hardcodedCogniShell = [
  "heroui-web/src/components/sidebar.tsx",
  "heroui-web/src/lib/nav-items.tsx",
  "heroui-web/src/components/layout/account-rail.tsx",
  "heroui-web/src/components/command-palette.tsx",
].map(readText).join("\n");
if (hardcodedCogniShell.includes('href: "/cognis"') || hardcodedCogniShell.includes("nav-cognis")) {
  fail("Cogni Kernel must not remain a hard-coded main-shell nav item; use /v1/packs/enabled pack sync");
}

const browserIntentManifest = readJSON("packs/examples/browser-intent-pack/pack.json");
const browserIntentSource = readText("internal/packs/browserintent/handler.go");
const browserIntentBridge = readText("internal/controlplane/gateway/handlers_browser_pack.go");
const browserIntentPage = readText("heroui-web/src/app/packs/browser/page.tsx");
const legacyBrowserPage = readText("heroui-web/src/app/browser/page.tsx");
const browserIntentClient = readText("heroui-web/src/lib/browser-intent-pack-client.ts");
if (browserIntentManifest) {
  if (!browserIntentSource.includes(`const PackID = "${browserIntentManifest.id}"`)) {
    fail("Browser Intent pack handler PackID must match packs/examples/browser-intent-pack/pack.json");
  }
  for (const route of browserIntentManifest.backend?.routes ?? []) {
    if (!browserIntentSource.includes(route)) fail(`Browser Intent handler missing route declared in manifest: ${route}`);
  }
  for (const method of ["http.MethodGet", "http.MethodPost"]) {
    if (!browserIntentSource.includes(method)) fail(`Browser Intent handler missing method gate declaration: ${method}`);
  }
  if (!browserIntentSource.includes("BackendRouteAuthPassthrough")) fail("Browser Intent session route must declare passthrough auth for extension grant bridge");
  if (browserIntentManifest.frontend?.menus?.[0]?.path !== "/packs/browser") fail("Browser Intent menu path must stay under /packs/browser");
  if (browserIntentManifest.sdk?.typescript !== "yunque-client/browser") fail("Browser Intent SDK import must stay yunque-client/browser");
}
if (!browserIntentBridge.includes("HandleBrowserIntentPack") || !browserIntentBridge.includes("HandleBrowserIntentSession") || !browserIntentBridge.includes("requireBrowserSessionAuth")) {
  fail("Browser Intent Gateway bridge must delegate through HandleBrowserIntentPack and preserve extension session auth");
}
if (browserIntentPage.includes('from "@/lib/api"') || browserIntentPage.includes("api.browser") || !browserIntentPage.includes("createBrowserIntentPackClient")) {
  fail("Browser Intent pack page must use browser-intent-pack-client instead of monolithic api object");
}
const apiSource = readText("heroui-web/src/lib/api.ts");
for (const token of ["browserNavigate:", "browserScreenshot:", "browserOcr:", "browserStatus:", "browserConfig:", "browserExtStatus:", "browserExtAction:", "browserExtScenarios:", "browserExtRunScenario:"]) {
  if (apiSource.includes(token)) fail(`monolithic api.ts still exposes Browser Intent method: ${token}`);
}
if (!legacyBrowserPage.includes('redirect("/packs/browser")')) {
  fail("legacy /browser page should redirect to /packs/browser");
}
for (const token of ["createBrowserIntentPackClient", "/v1/browser/status", "/v1/browser/ocr", "/api/browser/ext/session", "/api/browser/ext/scenarios/run", 'method: "POST"']) {
  if (!browserIntentClient.includes(token)) fail(`browser-intent-pack-client missing token: ${token}`);
}
const hardcodedBrowserShell = [
  "heroui-web/src/components/sidebar.tsx",
  "heroui-web/src/lib/nav-items.tsx",
  "heroui-web/src/components/command-palette.tsx",
].map(readText).join("\n");
if (hardcodedBrowserShell.includes('href: "/browser"') || hardcodedBrowserShell.includes("nav-browser")) {
  fail("Browser Intent must not remain a hard-coded main-shell nav item; use /v1/packs/enabled pack sync");
}


const rpaReplayManifest = readJSON("packs/examples/rpa-replay-pack/pack.json");
const rpaReplaySource = readText("internal/packs/rpareplay/handler.go");
const rpaReplayGateTest = readText("internal/controlplane/gateway/handlers_rpa_replay_pack_test.go");
const rpaReplayPage = readText("heroui-web/src/app/packs/rpa-replay/page.tsx");
const rpaReplayClient = readText("heroui-web/src/lib/rpa-replay-pack-client.ts");
const rpaReplayClientTest = readText("heroui-web/src/lib/__tests__/rpa-replay-pack-client.test.ts");
const rpaReplaySdk = readText("sdk/typescript/src/rpa-replay.ts") + "\n" + readText("sdk/typescript/src/rpa-replay.test.ts");
if (rpaReplayManifest) {
  if (!rpaReplaySource.includes(`const PackID = "${rpaReplayManifest.id}"`)) {
    fail("RPA Replay pack handler PackID must match packs/examples/rpa-replay-pack/pack.json");
  }
  for (const route of rpaReplayManifest.backend?.routes ?? []) {
    if (!rpaReplaySource.includes(route)) fail(`RPA Replay handler missing route declared in manifest: ${route}`);
  }
  for (const method of ["http.MethodGet", "http.MethodPost"]) {
    if (!rpaReplaySource.includes(method)) fail(`RPA Replay handler missing method gate declaration: ${method}`);
  }
  if (rpaReplayManifest.frontend?.menus?.[0]?.path !== "/packs/rpa-replay") fail("RPA Replay menu path must stay under /packs/rpa-replay");
  if (rpaReplayManifest.sdk?.typescript !== "yunque-client/rpa-replay") fail("RPA Replay SDK import must stay yunque-client/rpa-replay");
  if (rpaReplayManifest.defaultState !== "disabled") fail("RPA Replay pack must remain default disabled before executor readiness");
}
if (rpaReplayPage.includes('from "@/lib/api"') || rpaReplayPage.includes("api.rpa") || !rpaReplayPage.includes("createRPAReplayPackClient")) {
  fail("RPA Replay pack page must use rpa-replay-pack-client instead of monolithic api object");
}
for (const token of ["createRPAReplayPackClient", "/v1/rpa-replay/status", "/v1/rpa-replay/replay", "/v1/rpa-replay/evidence/", 'method: "POST"']) {
  if (!rpaReplayClient.includes(token)) fail(`rpa-replay-pack-client missing token: ${token}`);
}
if (!gatewaySource.includes('cfg.DataPath("rpa-replay")')) {
  fail("RPA Replay runtime store must be wired through the configured data directory");
}
for (const token of ["TestRPAReplay", "StatusNotFound", "StatusMethodNotAllowed", "/v1/rpa-replay/replay"]) {
  if (!rpaReplayGateTest.includes(token)) fail(`RPA Replay gateway gate test missing token: ${token}`);
}
for (const token of ["createRPAReplayClient", "RPAReplayClientError", "/v1/rpa-replay/status", "/v1/rpa-replay/evidence/"]) {
  if (!rpaReplaySdk.includes(token)) fail(`RPA Replay TypeScript SDK missing token: ${token}`);
}
for (const token of ["/v1/rpa-replay/status", "/v1/rpa-replay/replay", "/v1/rpa-replay/evidence/export-report"]) {
  if (!rpaReplayClientTest.includes(token)) fail(`RPA Replay frontend client test missing token: ${token}`);
}
for (const token of ["rpaReplayStatus:", "createRPAReplayTrace:", "rpaReplay:", "rpaReplayEvidence:"]) {
  if (apiSource.includes(token)) fail(`monolithic api.ts still exposes RPA Replay method: ${token}`);
}


const sbomDriftManifest = readJSON("packs/examples/sbom-drift-pack/pack.json");
const sbomDriftSource = readText("internal/packs/sbomdrift/handler.go");
const sbomDriftGateTest = readText("internal/controlplane/gateway/handlers_sbom_drift_pack_test.go");
const sbomDriftPage = readText("heroui-web/src/app/packs/sbom-drift/page.tsx");
const sbomDriftClient = readText("heroui-web/src/lib/sbom-drift-pack-client.ts");
const sbomDriftClientTest = readText("heroui-web/src/lib/__tests__/sbom-drift-pack-client.test.ts");
const sbomDriftSdk = readText("sdk/typescript/src/sbom-drift.ts") + "\n" + readText("sdk/typescript/src/sbom-drift.test.ts");
if (sbomDriftManifest) {
  if (!sbomDriftSource.includes(`const PackID = "${sbomDriftManifest.id}"`)) {
    fail("SBOM Drift pack handler PackID must match packs/examples/sbom-drift-pack/pack.json");
  }
  for (const route of sbomDriftManifest.backend?.routes ?? []) {
    if (!sbomDriftSource.includes(route)) fail(`SBOM Drift handler missing route declared in manifest: ${route}`);
  }
  for (const method of ["http.MethodGet", "http.MethodPost"]) {
    if (!sbomDriftSource.includes(method)) fail(`SBOM Drift handler missing method gate declaration: ${method}`);
  }
  if (sbomDriftManifest.frontend?.menus?.[0]?.path !== "/packs/sbom-drift") fail("SBOM Drift menu path must stay under /packs/sbom-drift");
  if (sbomDriftManifest.frontend?.routes?.[0]?.component !== "sbom/SBOMDriftPackPage") fail("SBOM Drift frontend route component drifted");
  if (sbomDriftManifest.sdk?.typescript !== "yunque-client/sbom-drift") fail("SBOM Drift SDK import must stay yunque-client/sbom-drift");
  if (sbomDriftManifest.defaultState !== "disabled") fail("SBOM Drift pack must remain default disabled before CI scanner readiness");
  if (sbomDriftManifest.metadata?.stage !== "pack-shell-before-ci") fail("SBOM Drift pack stage must remain pack-shell-before-ci");
}
if (sbomDriftPage.includes('from "@/lib/api"') || sbomDriftPage.includes("api.sbom") || !sbomDriftPage.includes("createSBOMDriftPackClient")) {
  fail("SBOM Drift pack page must use sbom-drift-pack-client instead of monolithic api object");
}
for (const token of ["createSBOMDriftPackClient", "/v1/sbom-drift/status", "/v1/sbom-drift/diff", "/v1/sbom-drift/evidence/", 'method: "POST"']) {
  if (!sbomDriftClient.includes(token)) fail(`sbom-drift-pack-client missing token: ${token}`);
}
if (!gatewaySource.includes('cfg.DataPath("sbom-drift")')) {
  fail("SBOM Drift runtime store must be wired through the configured data directory");
}
for (const token of ["TestSBOMDrift", "StatusNotFound", "StatusMethodNotAllowed", "/v1/sbom-drift/diff"]) {
  if (!sbomDriftGateTest.includes(token)) fail(`SBOM Drift gateway gate test missing token: ${token}`);
}
for (const token of ["createSBOMDriftClient", "SBOMDriftClientError", "/v1/sbom-drift/status", "/v1/sbom-drift/evidence/"]) {
  if (!sbomDriftSdk.includes(token)) fail(`SBOM Drift TypeScript SDK missing token: ${token}`);
}
for (const token of ["/v1/sbom-drift/status", "/v1/sbom-drift/diff", "/v1/sbom-drift/evidence/baseline"]) {
  if (!sbomDriftClientTest.includes(token)) fail(`SBOM Drift frontend client test missing token: ${token}`);
}
for (const token of ["sbomDriftStatus:", "createSBOMDriftSnapshot:", "sbomDriftDiff:", "sbomDriftEvidence:"]) {
  if (apiSource.includes(token)) fail(`monolithic api.ts still exposes SBOM Drift method: ${token}`);
}


const skillAnomalyManifest = readJSON("packs/examples/skill-anomaly-pack/pack.json");
const skillAnomalySource = readText("internal/packs/skillanomaly/handler.go");
const skillAnomalyGateTest = readText("internal/controlplane/gateway/handlers_skill_anomaly_pack_test.go");
const skillAnomalyPage = readText("heroui-web/src/app/packs/skill-anomaly/page.tsx");
const skillAnomalyClient = readText("heroui-web/src/lib/skill-anomaly-pack-client.ts");
const skillAnomalyClientTest = readText("heroui-web/src/lib/__tests__/skill-anomaly-pack-client.test.ts");
const skillAnomalySdk = readText("sdk/typescript/src/skill-anomaly.ts") + "\n" + readText("sdk/typescript/src/skill-anomaly.test.ts");
if (skillAnomalyManifest) {
  if (!skillAnomalySource.includes(`const PackID = "${skillAnomalyManifest.id}"`)) {
    fail("Skill Anomaly pack handler PackID must match packs/examples/skill-anomaly-pack/pack.json");
  }
  for (const route of skillAnomalyManifest.backend?.routes ?? []) {
    if (!skillAnomalySource.includes(route)) fail(`Skill Anomaly handler missing route declared in manifest: ${route}`);
  }
  for (const method of ["http.MethodGet", "http.MethodPost"]) {
    if (!skillAnomalySource.includes(method)) fail(`Skill Anomaly handler missing method gate declaration: ${method}`);
  }
  if (skillAnomalyManifest.frontend?.menus?.[0]?.path !== "/packs/skill-anomaly") fail("Skill Anomaly menu path must stay under /packs/skill-anomaly");
  if (skillAnomalyManifest.frontend?.routes?.[0]?.component !== "security/SkillAnomalyPackPage") fail("Skill Anomaly frontend route component drifted");
  if (skillAnomalyManifest.sdk?.typescript !== "yunque-client/skill-anomaly") fail("Skill Anomaly SDK import must stay yunque-client/skill-anomaly");
  if (skillAnomalyManifest.defaultState !== "disabled") fail("Skill Anomaly pack must remain default disabled before audit hook readiness");
  if (skillAnomalyManifest.metadata?.stage !== "pack-shell-before-audit-hook") fail("Skill Anomaly pack stage must remain pack-shell-before-audit-hook");
}
if (skillAnomalyPage.includes('from "@/lib/api"') || skillAnomalyPage.includes("api.skillAnomaly") || !skillAnomalyPage.includes("createSkillAnomalyPackClient")) {
  fail("Skill Anomaly pack page must use skill-anomaly-pack-client instead of monolithic api object");
}
for (const token of ["createSkillAnomalyPackClient", "/v1/skill-anomaly/status", "/v1/skill-anomaly/detect", "/v1/skill-anomaly/evidence/", 'method: "POST"']) {
  if (!skillAnomalyClient.includes(token)) fail(`skill-anomaly-pack-client missing token: ${token}`);
}
if (!gatewaySource.includes('cfg.DataPath("skill-anomaly")')) {
  fail("Skill Anomaly runtime store must be wired through the configured data directory");
}
for (const token of ["TestSkillAnomaly", "StatusNotFound", "StatusMethodNotAllowed", "/v1/skill-anomaly/detect"]) {
  if (!skillAnomalyGateTest.includes(token)) fail(`Skill Anomaly gateway gate test missing token: ${token}`);
}
for (const token of ["createSkillAnomalyClient", "SkillAnomalyClientError", "/v1/skill-anomaly/status", "/v1/skill-anomaly/evidence/"]) {
  if (!skillAnomalySdk.includes(token)) fail(`Skill Anomaly TypeScript SDK missing token: ${token}`);
}
for (const token of ["/v1/skill-anomaly/status", "/v1/skill-anomaly/detect", "/v1/skill-anomaly/evidence/text_processing"]) {
  if (!skillAnomalyClientTest.includes(token)) fail(`Skill Anomaly frontend client test missing token: ${token}`);
}
for (const token of ["skillAnomalyStatus:", "createSkillAnomalyEvent:", "skillAnomalyDetect:", "skillAnomalyEvidence:"]) {
  if (apiSource.includes(token)) fail(`monolithic api.ts still exposes Skill Anomaly method: ${token}`);
}


const wasmPluginManifest = readJSON("packs/examples/wasm-plugin-pack/pack.json");
const wasmPluginSource = readText("internal/packs/wasmplugin/handler.go");
const wasmPluginGateTest = readText("internal/controlplane/gateway/handlers_wasm_plugin_pack_test.go");
const wasmPluginPage = readText("heroui-web/src/app/packs/wasm-plugin/page.tsx");
const wasmPluginClient = readText("heroui-web/src/lib/wasm-plugin-pack-client.ts");
const wasmPluginClientTest = readText("heroui-web/src/lib/__tests__/wasm-plugin-pack-client.test.ts");
const wasmPluginSdk = readText("sdk/typescript/src/wasm-plugin.ts") + "\n" + readText("sdk/typescript/src/wasm-plugin.test.ts");
if (wasmPluginManifest) {
  if (!wasmPluginSource.includes(`const PackID = "${wasmPluginManifest.id}"`)) {
    fail("WASM Plugin pack handler PackID must match packs/examples/wasm-plugin-pack/pack.json");
  }
  for (const route of wasmPluginManifest.backend?.routes ?? []) {
    if (!wasmPluginSource.includes(route)) fail(`WASM Plugin handler missing route declared in manifest: ${route}`);
  }
  for (const method of ["http.MethodGet", "http.MethodPost"]) {
    if (!wasmPluginSource.includes(method)) fail(`WASM Plugin handler missing method gate declaration: ${method}`);
  }
  if (wasmPluginManifest.frontend?.menus?.[0]?.path !== "/packs/wasm-plugin") fail("WASM Plugin menu path must stay under /packs/wasm-plugin");
  if (wasmPluginManifest.frontend?.routes?.[0]?.component !== "wasm/WASMPluginPackPage") fail("WASM Plugin frontend route component drifted");
  if (wasmPluginManifest.sdk?.typescript !== "yunque-client/wasm-plugin") fail("WASM Plugin SDK import must stay yunque-client/wasm-plugin");
  if (wasmPluginManifest.defaultState !== "disabled") fail("WASM Plugin pack must remain default disabled because it is a high-risk execution surface");
  if (wasmPluginManifest.metadata?.stage !== "pack-shell-before-runtime-hosts") fail("WASM Plugin pack stage must remain pack-shell-before-runtime-hosts");
}
if (wasmPluginPage.includes('from "@/lib/api"') || wasmPluginPage.includes("api.wasm") || !wasmPluginPage.includes("createWASMPluginPackClient")) {
  fail("WASM Plugin pack page must use wasm-plugin-pack-client instead of monolithic api object");
}
for (const token of ["createWASMPluginPackClient", "/v1/wasm-plugin/status", "/v1/wasm-plugin/execute", "/v1/wasm-plugin/evidence/", 'method: "POST"']) {
  if (!wasmPluginClient.includes(token)) fail(`wasm-plugin-pack-client missing token: ${token}`);
}
if (!gatewaySource.includes('cfg.DataPath("wasm-plugin")')) {
  fail("WASM Plugin runtime store must be wired through the configured data directory");
}
for (const token of ["TestWASMPlugin", "StatusNotFound", "StatusMethodNotAllowed", "/v1/wasm-plugin/execute"]) {
  if (!wasmPluginGateTest.includes(token)) fail(`WASM Plugin gateway gate test missing token: ${token}`);
}
for (const token of ["createWASMPluginClient", "WASMPluginClientError", "/v1/wasm-plugin/status", "/v1/wasm-plugin/evidence/"]) {
  if (!wasmPluginSdk.includes(token)) fail(`WASM Plugin TypeScript SDK missing token: ${token}`);
}
for (const token of ["/v1/wasm-plugin/status", "/v1/wasm-plugin/execute", "/v1/wasm-plugin/evidence/calculator"]) {
  if (!wasmPluginClientTest.includes(token)) fail(`WASM Plugin frontend client test missing token: ${token}`);
}
for (const token of ["normalizeModulePath", "validateModulePath", "module_path must not contain traversal segments"]) {
  if (!wasmPluginSource.includes(token)) fail(`WASM Plugin handler missing module path safety token: ${token}`);
}
for (const token of ["wasmPluginStatus:", "createWASMPlugin:", "wasmPluginExecute:", "wasmPluginEvidence:"]) {
  if (apiSource.includes(token)) fail(`monolithic api.ts still exposes WASM Plugin method: ${token}`);
}

if (failures.length > 0) {
  console.error("Pack contract check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Pack contract ok: ${packFiles.length} manifests verified, backend module registry contract documented`);
