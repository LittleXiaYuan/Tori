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
  "packs/examples/chaos-probe-pack",
  "yunque.pack.chaos-probe",
  "pack-shell-before-scheduler",
  "yunque-client/chaos-probe",
  "Chaos Probe Pack shell 闭环",
  "/v1/chaos-probe/scheduler/plan",
  "/v1/chaos-probe/degrade-state/engine/plan",
  "scheduler-plan.json",
  "degrade-engine-plan.json",
  "Prometheus 指标",
  "降级状态机",
]) {
  if (!packRuntimeBlueprint.includes(token)) fail(`PACK-RUNTIME-BLUEPRINT.md missing Chaos Probe token: ${token}`);
}
for (const token of [
  "packs/examples/wasm-plugin-pack",
  "yunque.pack.wasm-plugin",
  "pack-shell-before-runtime-hosts",
  "yunque-client/wasm-plugin",
  "WASM Plugin Pack shell 闭环",
  "Host ABI 权限强执行",
  "远程签名包安装计划",
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
  "packs/examples/guardrail-fuzzer-pack",
  "yunque.pack.guardrail-fuzzer",
  "pack-shell-before-ci-fuzz",
  "yunque-client/guardrail-fuzzer",
  "Guardrail Fuzzer Pack shell 闭环",
  "CI 定时 fuzz",
  "/v1/guardrail-fuzzer/ci-gate/plan",
  "/v1/guardrail-fuzzer/native-corpus/plan",
  "ci-gate-plan.json",
  "native-corpus-plan.json",
  "Go native fuzz corpus sync",
  "规则写回",
]) {
  if (!packRuntimeBlueprint.includes(token)) fail(`PACK-RUNTIME-BLUEPRINT.md missing Guardrail Fuzzer token: ${token}`);
}
for (const token of [
  "packs/examples/memory-time-travel-pack",
  "yunque.pack.memory-time-travel",
  "pack-shell-before-ledger-kv-history",
  "yunque-client/memory-time-travel",
  "Memory Time Travel Pack shell 闭环",
  "Ledger KV kv_history",
  "Merkle audit-chain",
  "Memory Persister write-back",
  "KV audit proof-link schema",
  "audit proof-link preview",
  "approved rollback write-back plan",
  "/v1/memory-time-travel/rollback/approved-plan",
  "approved-rollback-plan.json",
  "/v1/memory-time-travel/kv-history/native-plan",
  "/v1/memory-time-travel/kv-history/migration-preview",
  "/v1/memory-time-travel/kv-history/dual-read/parity",
  "/v1/memory-time-travel/kv-history/cutover/readiness",
  "native-kv-history-plan.json",
  "kv-history-migration-plan.json",
  "kv-history-migration-preview.json",
  "kv-history-dual-read-parity.json",
  "kv-history-cutover-readiness.json",
  "/v1/memory-time-travel/audit/links",
  "/v1/memory-time-travel/audit/links/preview",
  "/v1/memory-time-travel/audit/links/writeback-plan",
  "/v1/memory-time-travel/audit/links/writeback/store",
  "/v1/memory-time-travel/audit/links/writeback/executor/plan",
  "audit-link-preview.json",
  "audit-link-writeback-plan.json",
  "audit-link-writeback-store.json",
  "audit-link-writeback-record.json",
  "audit-link-writeback-executor-plan.json",
  "audit-link-executor-handoff-plan.json",
  "audit-link-executor-audit-plan.json",
  "kv_audit_link_preview",
  "kv_audit_link_writeback_plan",
  "kv_audit_link_writeback_store",
  "kv_audit_link_writeback_records",
  "kv_audit_link_writeback_executor_plan",
  "audit_link_executor_handoff_plan",
  "audit_link_executor_audit_plan",
  "kv_audit_link_writeback_plan_ready",
  "kv_audit_link_writeback_store_ready",
  "writes_audit_link_writeback_store",
  "kv_audit_link_writeback_executor_plan_ready",
  "executor_input_contract_ready",
  "audit_proof_link_executor_ready",
  "consumes_audit_link_writeback_store",
  "audit_append_plan_ready",
  "writes_audit_chain",
  "memory.audit.links.writeback_plan",
  "memory.audit.links.writeback_store",
  "memory.audit.links.writeback_executor_plan",
]) {
  if (!packRuntimeBlueprint.includes(token)) fail(`PACK-RUNTIME-BLUEPRINT.md missing Memory Time Travel token: ${token}`);
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
  + readText("internal/packs/chaosprobe/handler.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_chaos_probe_pack_test.go")
  + "\n"
  + readText("internal/packs/cognitivecanary/handler.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_cognitive_canary_pack_test.go")
  + "\n"
  + readText("internal/packs/rpareplay/handler.go")
  + "\n"
  + readText("internal/packs/sbomdrift/handler.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_sbom_drift_pack_test.go")
  + "\n"
  + readText("internal/packs/guardrailfuzzer/handler.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_guardrail_fuzzer_pack_test.go")
  + "\n"
  + readText("internal/packs/memorytimetravel/handler.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_memory_time_travel_pack_test.go")
  + "\n"
  + readText("internal/packs/skillanomaly/handler.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_skill_anomaly_pack_test.go")
  + "\n"
  + readText("internal/packs/wasmplugin/handler.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_wasm_plugin_pack_test.go");
for (const token of ["BackendPacks", "RegisterBackendPack", "registerBackendPack", "requirePackRoute", "backendPackAuth", "BackendRouteAuthPassthrough", "backendPackRoutes", "backendPackRouteInfos", "BackendRouteInfo{Method", "Methods: methods", "normalizeBackendRouteMethods", "must declare an HTTP method", "handlePackBackendModules", "handlePackPrune", "/v1/packs/prune", "Download     bool", "CacheDistribution", "PruneArtifacts", "InstallWithArtifacts", "route conflict", "TestRegisterBackendPackMountsModuleAfterGatewayConstruction", "TestRegisterBackendPackIsIdempotentForSamePackRoute", "TestRegisterBackendPackPanicsOnRouteConflict", "TestPackBackendModulesExposeMountedRoutes", "TestBackendPackMultiMethodRouteInfoAndGate", "TestBackendPackPassthroughAuthRouteKeepsPackGate", "expected mounted route method to be preserved", "expected downloaded artifacts to be recorded", "ensureBuiltinPacks", "loadBuiltinPackManifest", "packs/examples/lora-pack/pack.json", "packs/examples/cogni-kernel-pack/pack.json", "packs/examples/browser-intent-pack/pack.json", "packs/examples/chaos-probe-pack/pack.json", "packs/examples/cognitive-canary-pack/pack.json", "packs/examples/guardrail-fuzzer-pack/pack.json", "packs/examples/memory-time-travel-pack/pack.json", "packs/examples/rpa-replay-pack/pack.json", "packs/examples/sbom-drift-pack/pack.json", "packs/examples/skill-anomaly-pack/pack.json", "packs/examples/wasm-plugin-pack/pack.json", "backuppack.DefaultHandler()", "lorapack.NewHandler", "cognikernelpack.NewHandler", "browserintentpack.NewHandler", "chaosprobepack.New", "cfg.DataPath(\"chaos-probe\")", "cognitivecanarypack.New", "cfg.DataPath(\"cognitive-canary\")", "guardrailfuzzerpack.New", "cfg.DataPath(\"guardrail-fuzzer\")", "memorytimetravelpack.New", "cfg.DataPath(\"memory-time-travel\")", "rpareplaypack.New", "cfg.DataPath(\"rpa-replay\")", "sbomdriftpack.New", "cfg.DataPath(\"sbom-drift\")", "skillanomalypack.New", "cfg.DataPath(\"skill-anomaly\")", "HandleCogniKernelPack", "HandleBrowserIntentPack", "BackendPacks: []packruntime.BackendModule"]) {
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

const apiSource = readText("heroui-web/src/lib/api.ts");

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


const chaosProbeManifest = readJSON("packs/examples/chaos-probe-pack/pack.json");
const chaosProbeSource = readText("internal/packs/chaosprobe/handler.go");
const chaosProbeGateTest = readText("internal/controlplane/gateway/handlers_chaos_probe_pack_test.go");
const chaosProbePage = readText("heroui-web/src/app/packs/chaos-probe/page.tsx");
const chaosProbeClient = readText("heroui-web/src/lib/chaos-probe-pack-client.ts");
const chaosProbeClientTest = readText("heroui-web/src/lib/__tests__/chaos-probe-pack-client.test.ts");
const chaosProbeSdk = readText("sdk/typescript/src/chaos-probe.ts") + "\n" + readText("sdk/typescript/src/chaos-probe.test.ts");
if (chaosProbeManifest) {
  if (!chaosProbeSource.includes(`const PackID = "${chaosProbeManifest.id}"`)) {
    fail("Chaos Probe pack handler PackID must match packs/examples/chaos-probe-pack/pack.json");
  }
  for (const route of chaosProbeManifest.backend?.routes ?? []) {
    if (!chaosProbeSource.includes(route)) fail(`Chaos Probe handler missing route declared in manifest: ${route}`);
  }
  for (const method of ["http.MethodGet", "http.MethodPost"]) {
    if (!chaosProbeSource.includes(method)) fail(`Chaos Probe handler missing method gate declaration: ${method}`);
  }
  if (chaosProbeManifest.frontend?.menus?.[0]?.path !== "/packs/chaos-probe") fail("Chaos Probe menu path must stay under /packs/chaos-probe");
  if (chaosProbeManifest.frontend?.routes?.[0]?.component !== "ops/ChaosProbePackPage") fail("Chaos Probe frontend route component drifted");
  if (chaosProbeManifest.sdk?.typescript !== "yunque-client/chaos-probe") fail("Chaos Probe SDK import must stay yunque-client/chaos-probe");
  if (chaosProbeManifest.defaultState !== "disabled") fail("Chaos Probe pack must remain default disabled before scheduler and degrade write-back readiness");
  if (chaosProbeManifest.metadata?.stage !== "pack-shell-before-scheduler") fail("Chaos Probe pack stage must remain pack-shell-before-scheduler");
  if (chaosProbeManifest.metadata?.blueprint !== "doc/CHAOS-PROBE.md") fail("Chaos Probe pack blueprint pointer drifted");
}
if (chaosProbePage.includes('from "@/lib/api"') || chaosProbePage.includes("api.chaosProbe") || !chaosProbePage.includes("createChaosProbePackClient")) {
  fail("Chaos Probe pack page must use chaos-probe-pack-client instead of monolithic api object");
}
for (const token of ["createChaosProbePackClient", "/v1/chaos-probe/status", "/v1/chaos-probe/probes", "/v1/chaos-probe/run", "/v1/chaos-probe/scheduler/plan", "/v1/chaos-probe/degrade-state/writeback", "/v1/chaos-probe/degrade-state/engine/plan", "/v1/chaos-probe/reports", "/v1/chaos-probe/evidence/", "schedulerPlan", "writeDegradeState", "degradeEnginePlan", "scheduler_plan_ready", "metrics_plan_ready", "prometheus_ready", "degrade_writeback_plan_ready", "degrade_state_store_ready", "degrade_engine_plan_ready", "audit_append_plan_ready", "merkle_append_ready", "consumes_degrade_state_store", "writes_runtime_degrade_state", "runtime_degrade_state_ready", "alert_writeback_plan_ready", 'method: "POST"']) {
  if (!chaosProbeClient.includes(token)) fail(`chaos-probe-pack-client missing token: ${token}`);
}
if (!gatewaySource.includes('cfg.DataPath("chaos-probe")')) {
  fail("Chaos Probe runtime store must be wired through the configured data directory");
}
for (const token of ["TestChaosProbe", "StatusMethodNotAllowed", "/v1/chaos-probe/run", "/v1/chaos-probe/scheduler/plan", "/v1/chaos-probe/degrade-state/writeback", "/v1/chaos-probe/degrade-state/engine/plan"]) {
  if (!chaosProbeGateTest.includes(token)) fail(`Chaos Probe gateway gate test missing token: ${token}`);
}
for (const token of ["createChaosProbeClient", "ChaosProbeClientError", "/v1/chaos-probe/status", "/v1/chaos-probe/scheduler/plan", "/v1/chaos-probe/degrade-state/writeback", "/v1/chaos-probe/degrade-state/engine/plan", "/v1/chaos-probe/evidence/", "schedulerPlan", "writeDegradeState", "degradeEnginePlan"]) {
  if (!chaosProbeSdk.includes(token)) fail(`Chaos Probe TypeScript SDK missing token: ${token}`);
}
for (const token of ["/v1/chaos-probe/status", "/v1/chaos-probe/run", "/v1/chaos-probe/scheduler/plan", "/v1/chaos-probe/degrade-state/writeback", "/v1/chaos-probe/degrade-state/engine/plan", "/v1/chaos-probe/evidence/chaos-1"]) {
  if (!chaosProbeClientTest.includes(token)) fail(`Chaos Probe frontend client test missing token: ${token}`);
}
for (const token of [
  "json-chaos-probe-evidence",
  "safe_probe_ready",
  "scheduler_plan_ready",
  "scheduler_ready",
  "metrics_plan_ready",
  "prometheus_ready",
  "degrade_writeback_plan_ready",
  "degrade_writeback_ready",
  "degrade_state_store_ready",
  "writes_degrade_state_store",
  "degrade_engine_plan_ready",
  "audit_append_plan_ready",
  "merkle_append_ready",
  "consumes_degrade_state_store",
  "writes_runtime_degrade_state",
  "runtime_degrade_state_ready",
  "degrade_engine_ready",
  "alert_writeback_plan_ready",
  "alert_writeback_ready",
  "chaos.scheduler.plan",
  "chaos.metrics.plan",
  "chaos.degrade.plan",
  "chaos.degrade_state.writeback",
  "chaos.degrade_state.engine.plan",
  "chaos.audit.append.plan",
  "chaos.alert.writeback.plan",
  "scheduler-plan.json",
  "metrics-plan.json",
  "degrade-writeback-plan.json",
  "degrade-state-store.json",
  "degrade-state-record.json",
  "degrade-engine-plan.json",
  "runtime-degrade-handoff-plan.json",
  "audit-append-plan.json",
]) {
  if (!chaosProbeSource.includes(token)) fail(`Chaos Probe handler missing ops resilience shell token: ${token}`);
}
for (const token of ["chaosProbeStatus:", "chaosProbeRun:", "chaosProbeEvidence:"]) {
  if (apiSource.includes(token)) fail(`monolithic api.ts still exposes Chaos Probe method: ${token}`);
}


const cognitiveCanaryManifest = readJSON("packs/examples/cognitive-canary-pack/pack.json");
const cognitiveCanarySource = readText("internal/packs/cognitivecanary/handler.go");
const cognitiveCanaryGateTest = readText("internal/controlplane/gateway/handlers_cognitive_canary_pack_test.go");
const cognitiveCanaryPage = readText("heroui-web/src/app/packs/cognitive-canary/page.tsx");
const cognitiveCanaryClient = readText("heroui-web/src/lib/cognitive-canary-pack-client.ts");
const cognitiveCanaryClientTest = readText("heroui-web/src/lib/__tests__/cognitive-canary-pack-client.test.ts");
const cognitiveCanarySdk = readText("sdk/typescript/src/cognitive-canary.ts") + "\n" + readText("sdk/typescript/src/cognitive-canary.test.ts");
if (cognitiveCanaryManifest) {
  if (!cognitiveCanarySource.includes(`const PackID = "${cognitiveCanaryManifest.id}"`)) {
    fail("Cognitive Canary pack handler PackID must match packs/examples/cognitive-canary-pack/pack.json");
  }
  for (const route of cognitiveCanaryManifest.backend?.routes ?? []) {
    if (!cognitiveCanarySource.includes(route)) fail(`Cognitive Canary handler missing route declared in manifest: ${route}`);
  }
  for (const method of ["http.MethodGet", "http.MethodPost"]) {
    if (!cognitiveCanarySource.includes(method)) fail(`Cognitive Canary handler missing method gate declaration: ${method}`);
  }
  if (cognitiveCanaryManifest.frontend?.menus?.[0]?.path !== "/packs/cognitive-canary") fail("Cognitive Canary menu path must stay under /packs/cognitive-canary");
  if (cognitiveCanaryManifest.frontend?.routes?.[0]?.component !== "ops/CognitiveCanaryPackPage") fail("Cognitive Canary frontend route component drifted");
  if (cognitiveCanaryManifest.sdk?.typescript !== "yunque-client/cognitive-canary") fail("Cognitive Canary SDK import must stay yunque-client/cognitive-canary");
  if (cognitiveCanaryManifest.defaultState !== "disabled") fail("Cognitive Canary pack must remain default disabled before shadow traffic and auto rollback readiness");
  if (cognitiveCanaryManifest.metadata?.stage !== "pack-shell-before-shadow-traffic") fail("Cognitive Canary pack stage must remain pack-shell-before-shadow-traffic");
  if (cognitiveCanaryManifest.metadata?.blueprint !== "doc/COGNITIVE-CANARY.md") fail("Cognitive Canary pack blueprint pointer drifted");
  if (!(cognitiveCanaryManifest.backend?.capabilities ?? []).includes("cognitive_canary.response_collector.plan")) fail("Cognitive Canary manifest must declare response collector plan capability");
  if (!(cognitiveCanaryManifest.backend?.capabilities ?? []).includes("cognitive_canary.response_collector.writeback")) fail("Cognitive Canary manifest must declare response collector writeback capability");
  if (!(cognitiveCanaryManifest.backend?.capabilities ?? []).includes("cognitive_canary.response_collector.pipeline.plan")) fail("Cognitive Canary manifest must declare response collector pipeline plan capability");
}
if (cognitiveCanaryPage.includes('from "@/lib/api"') || cognitiveCanaryPage.includes("api.cognitiveCanary") || !cognitiveCanaryPage.includes("createCognitiveCanaryPackClient")) {
  fail("Cognitive Canary pack page must use cognitive-canary-pack-client instead of monolithic api object");
}
for (const token of ["createCognitiveCanaryPackClient", "/v1/cognitive-canary/status", "/v1/cognitive-canary/scenarios", "/v1/cognitive-canary/evaluate", "/v1/cognitive-canary/shadow/plan", "/v1/cognitive-canary/response-collector/writeback", "/v1/cognitive-canary/response-collector/pipeline/plan", "/v1/cognitive-canary/reports", "/v1/cognitive-canary/evidence/", "shadowPlan", "responseCollectorWriteback", "responseCollectorPipelinePlan", "shadow_plan_ready", "response_collector_plan_ready", "response_collector_writeback_ready", "writes_response_collector_store", "response_collector_pipeline_plan_ready", "consumes_response_collector_store", "response_collector_pipeline_ready", "response_collectors", "response_collector_summary", "response_collector_store", "response_collector_records", "response_collector_pipeline_plan", "artifact_sha256", "writes_files", "judge_plan_ready", "metrics_plan_ready", "prometheus_ready", "auto_rollback_plan_ready", 'method: "POST"']) {
  if (!cognitiveCanaryClient.includes(token)) fail(`cognitive-canary-pack-client missing token: ${token}`);
}
if (!gatewaySource.includes('cfg.DataPath("cognitive-canary")')) {
  fail("Cognitive Canary runtime store must be wired through the configured data directory");
}
for (const token of ["TestCognitiveCanary", "StatusMethodNotAllowed", "/v1/cognitive-canary/evaluate", "/v1/cognitive-canary/shadow/plan", "/v1/cognitive-canary/response-collector/writeback", "/v1/cognitive-canary/response-collector/pipeline/plan"]) {
  if (!cognitiveCanaryGateTest.includes(token)) fail(`Cognitive Canary gateway gate test missing token: ${token}`);
}
for (const token of ["createCognitiveCanaryClient", "CognitiveCanaryClientError", "/v1/cognitive-canary/status", "/v1/cognitive-canary/shadow/plan", "/v1/cognitive-canary/response-collector/writeback", "/v1/cognitive-canary/response-collector/pipeline/plan", "/v1/cognitive-canary/evidence/", "shadowPlan", "responseCollectorWriteback", "responseCollectorPipelinePlan", "CognitiveCanaryResponseCollectorPlan", "CognitiveCanaryResponseCollectorWritebackReport", "CognitiveCanaryResponseCollectorPipelinePlan", "response_collector_plan_ready", "response_collector_writeback_ready", "writes_response_collector_store", "response_collector_pipeline_plan_ready", "consumes_response_collector_store", "response_collectors", "response_collector_store", "artifact_sha256", "writes_files"]) {
  if (!cognitiveCanarySdk.includes(token)) fail(`Cognitive Canary TypeScript SDK missing token: ${token}`);
}
for (const token of ["/v1/cognitive-canary/status", "/v1/cognitive-canary/evaluate", "/v1/cognitive-canary/shadow/plan", "/v1/cognitive-canary/response-collector/writeback", "/v1/cognitive-canary/response-collector/pipeline/plan", "/v1/cognitive-canary/evidence/canary-1", "response_collector_summary", "response-collector-plan.json", "response-collector-store.json", "response-collector-pipeline-plan.json", "writes_response_collector_store", "consumes_response_collector_store"]) {
  if (!cognitiveCanaryClientTest.includes(token)) fail(`Cognitive Canary frontend client test missing token: ${token}`);
}
for (const token of ["json-cognitive-canary-evidence", "shadow_plan_ready", "shadow_traffic_ready", "judge_plan_ready", "judge_pipeline_ready", "response_collector_plan_ready", "response_collector_store_ready", "response_collector_writeback_ready", "writes_response_collector_store", "response_collector_pipeline_plan_ready", "consumes_response_collector_store", "response_collector_pipeline_ready", "response_collector_ready", "canary.response_collector.plan", "canary.response_collector.writeback", "canary.response_collector.pipeline.plan", "response_collectors", "response_collector_summary", "response_collector_store", "response_collector_records", "response_collector_pipeline_plan", "artifact_sha256", "writes_files", "metrics_plan_ready", "prometheus_ready", "quality_sli_ready", "auto_rollback_plan_ready", "auto_rollback_ready", "canary.shadow.plan", "canary.judge.plan", "canary.metrics.plan", "canary.rollback.plan", "shadow-plan.json", "response-collector-plan.json", "response-collector-store.json", "response-collector-record.json", "response-collector-pipeline-plan.json", "response-collector-handoff-plan.json", "judge-plan.json", "metrics-plan.json", "rollback-plan.json"]) {
  if (!cognitiveCanarySource.includes(token)) fail(`Cognitive Canary handler missing cognitive quality shell token: ${token}`);
}
for (const token of ["cognitiveCanaryStatus:", "cognitiveCanaryEvaluate:", "cognitiveCanaryEvidence:"]) {
  if (apiSource.includes(token)) fail(`monolithic api.ts still exposes Cognitive Canary method: ${token}`);
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
  for (const capability of ["sbom.cyclonedx.export", "sbom.ci_gate.plan", "sbom.ci_baseline.writeback", "sbom.govulncheck.plan"]) {
    if (!sbomDriftManifest.backend?.capabilities?.includes(capability)) fail(`SBOM Drift manifest missing capability: ${capability}`);
  }
}
if (sbomDriftPage.includes('from "@/lib/api"') || sbomDriftPage.includes("api.sbom") || !sbomDriftPage.includes("createSBOMDriftPackClient")) {
  fail("SBOM Drift pack page must use sbom-drift-pack-client instead of monolithic api object");
}
for (const token of ["createSBOMDriftPackClient", "/v1/sbom-drift/status", "/v1/sbom-drift/diff", "/v1/sbom-drift/cyclonedx/", "/v1/sbom-drift/ci-gate/plan", "/v1/sbom-drift/ci-gate/baseline/writeback", "/v1/sbom-drift/evidence/", "govulncheck_plan_ready", "govulncheck_plan", "writes_files", "writes_ci_baseline_store", 'method: "POST"']) {
  if (!sbomDriftClient.includes(token)) fail(`sbom-drift-pack-client missing token: ${token}`);
}
if (!gatewaySource.includes('cfg.DataPath("sbom-drift")')) {
  fail("SBOM Drift runtime store must be wired through the configured data directory");
}
for (const token of ["TestSBOMDrift", "StatusNotFound", "StatusMethodNotAllowed", "/v1/sbom-drift/diff", "/v1/sbom-drift/ci-gate/baseline/writeback"]) {
  if (!sbomDriftGateTest.includes(token)) fail(`SBOM Drift gateway gate test missing token: ${token}`);
}
for (const token of ["createSBOMDriftClient", "SBOMDriftClientError", "/v1/sbom-drift/status", "/v1/sbom-drift/cyclonedx/", "/v1/sbom-drift/ci-gate/plan", "/v1/sbom-drift/ci-gate/baseline/writeback", "/v1/sbom-drift/evidence/", "SBOMDriftGovulncheckPlan", "govulncheck_plan_ready", "govulncheck_plan", "govulncheck-report.json", "writes_files", "writes_ci_baseline_store"]) {
  if (!sbomDriftSdk.includes(token)) fail(`SBOM Drift TypeScript SDK missing token: ${token}`);
}
for (const token of ["/v1/sbom-drift/status", "/v1/sbom-drift/diff", "/v1/sbom-drift/cyclonedx/baseline", "/v1/sbom-drift/ci-gate/plan", "/v1/sbom-drift/ci-gate/baseline/writeback", "/v1/sbom-drift/evidence/baseline"]) {
  if (!sbomDriftClientTest.includes(token)) fail(`SBOM Drift frontend client test missing token: ${token}`);
}
for (const token of ["govulncheck_plan_ready", "govulncheck_ready", "govulncheck-report.json", "writes_files", "govulncheck-plan.json"]) {
  if (!sbomDriftSource.includes(token)) fail(`SBOM Drift handler missing govulncheck plan token: ${token}`);
}
for (const token of ["CIBaselineWriteback", "ci-baseline-store.json", "ci-baseline-record.json", "ci_baseline_writeback_ready", "writes_ci_baseline_store", "writes_ci_workflow", "executes_govulncheck", "blocks_release"]) {
  if (!sbomDriftSource.includes(token)) fail(`SBOM Drift handler missing CI baseline writeback token: ${token}`);
}
for (const token of ["sbomDriftStatus:", "createSBOMDriftSnapshot:", "sbomDriftDiff:", "sbomDriftEvidence:"]) {
  if (apiSource.includes(token)) fail(`monolithic api.ts still exposes SBOM Drift method: ${token}`);
}


const guardrailFuzzerManifest = readJSON("packs/examples/guardrail-fuzzer-pack/pack.json");
const guardrailFuzzerSource = readText("internal/packs/guardrailfuzzer/handler.go");
const guardrailFuzzerGateTest = readText("internal/controlplane/gateway/handlers_guardrail_fuzzer_pack_test.go");
const guardrailFuzzerPage = readText("heroui-web/src/app/packs/guardrail-fuzzer/page.tsx");
const guardrailFuzzerClient = readText("heroui-web/src/lib/guardrail-fuzzer-pack-client.ts");
const guardrailFuzzerClientTest = readText("heroui-web/src/lib/__tests__/guardrail-fuzzer-pack-client.test.ts");
const guardrailFuzzerSdk = readText("sdk/typescript/src/guardrail-fuzzer.ts") + "\n" + readText("sdk/typescript/src/guardrail-fuzzer.test.ts");
if (guardrailFuzzerManifest) {
  if (!guardrailFuzzerSource.includes(`const PackID = "${guardrailFuzzerManifest.id}"`)) {
    fail("Guardrail Fuzzer pack handler PackID must match packs/examples/guardrail-fuzzer-pack/pack.json");
  }
  for (const route of guardrailFuzzerManifest.backend?.routes ?? []) {
    if (!guardrailFuzzerSource.includes(route)) fail(`Guardrail Fuzzer handler missing route declared in manifest: ${route}`);
  }
  for (const method of ["http.MethodGet", "http.MethodPost"]) {
    if (!guardrailFuzzerSource.includes(method)) fail(`Guardrail Fuzzer handler missing method gate declaration: ${method}`);
  }
  if (guardrailFuzzerManifest.frontend?.menus?.[0]?.path !== "/packs/guardrail-fuzzer") fail("Guardrail Fuzzer menu path must stay under /packs/guardrail-fuzzer");
  if (guardrailFuzzerManifest.frontend?.routes?.[0]?.component !== "security/GuardrailFuzzerPackPage") fail("Guardrail Fuzzer frontend route component drifted");
  if (guardrailFuzzerManifest.sdk?.typescript !== "yunque-client/guardrail-fuzzer") fail("Guardrail Fuzzer SDK import must stay yunque-client/guardrail-fuzzer");
  if (guardrailFuzzerManifest.defaultState !== "disabled") fail("Guardrail Fuzzer pack must remain default disabled before CI fuzz gates are wired");
  if (guardrailFuzzerManifest.metadata?.stage !== "pack-shell-before-ci-fuzz") fail("Guardrail Fuzzer pack stage must remain pack-shell-before-ci-fuzz");
  if (guardrailFuzzerManifest.metadata?.blueprint !== "doc/GUARDRAIL-FUZZER.md") fail("Guardrail Fuzzer pack blueprint pointer drifted");
  if (!(guardrailFuzzerManifest.backend?.capabilities ?? []).includes("guardrail_fuzzer.native_corpus.manifest_preview")) fail("Guardrail Fuzzer manifest missing native corpus manifest preview capability");
}
if (guardrailFuzzerPage.includes('from "@/lib/api"') || guardrailFuzzerPage.includes("api.guardrailFuzzer") || !guardrailFuzzerPage.includes("createGuardrailFuzzerPackClient")) {
  fail("Guardrail Fuzzer pack page must use guardrail-fuzzer-pack-client instead of monolithic api object");
}
for (const token of ["createGuardrailFuzzerPackClient", "/v1/guardrail-fuzzer/status", "/v1/guardrail-fuzzer/run", "/v1/guardrail-fuzzer/ci-gate/plan", "/v1/guardrail-fuzzer/native-corpus/plan", "/v1/guardrail-fuzzer/evidence/", "ciGatePlan", "nativeCorpusPlan", "ci_gate_plan_ready", "rule_writeback_plan_ready", "alert_plan_ready", "alert_ready", "native_corpus_plan_ready", "native_corpus_sync_ready", "go_native_fuzz_plan_ready", "go_native_fuzz_ready", "corpus_manifest", "sync_summary", "content_sha256", "writes_files", 'method: "POST"']) {
  if (!guardrailFuzzerClient.includes(token)) fail(`guardrail-fuzzer-pack-client missing token: ${token}`);
}
if (!gatewaySource.includes('cfg.DataPath("guardrail-fuzzer")')) {
  fail("Guardrail Fuzzer runtime store must be wired through the configured data directory");
}
for (const token of ["TestGuardrailFuzzer", "StatusMethodNotAllowed", "/v1/guardrail-fuzzer/run", "/v1/guardrail-fuzzer/ci-gate/plan", "/v1/guardrail-fuzzer/native-corpus/plan"]) {
  if (!guardrailFuzzerGateTest.includes(token)) fail(`Guardrail Fuzzer gateway gate test missing token: ${token}`);
}
for (const token of ["createGuardrailFuzzerClient", "GuardrailFuzzerClientError", "/v1/guardrail-fuzzer/status", "/v1/guardrail-fuzzer/ci-gate/plan", "/v1/guardrail-fuzzer/native-corpus/plan", "/v1/guardrail-fuzzer/evidence/", "ciGatePlan", "nativeCorpusPlan", "corpus_manifest", "sync_summary", "content_sha256", "writes_files"]) {
  if (!guardrailFuzzerSdk.includes(token)) fail(`Guardrail Fuzzer TypeScript SDK missing token: ${token}`);
}
for (const token of ["/v1/guardrail-fuzzer/status", "/v1/guardrail-fuzzer/run", "/v1/guardrail-fuzzer/ci-gate/plan", "/v1/guardrail-fuzzer/native-corpus/plan", "/v1/guardrail-fuzzer/evidence/fuzz-1", "corpus_manifest", "sync_summary", "content_sha256", "writes_files"]) {
  if (!guardrailFuzzerClientTest.includes(token)) fail(`Guardrail Fuzzer frontend client test missing token: ${token}`);
}
for (const token of ["json-guardrail-fuzzer-evidence", "buildRuleCandidates", "base64_wrap", "double_url_encode", "ci_gate_plan_ready", "ci_gate_ready", "rule_writeback_plan_ready", "rule_writeback_ready", "alert_plan_ready", "alert_ready", "native_corpus_plan_ready", "native_corpus_sync_ready", "go_native_fuzz_plan_ready", "go_native_fuzz_ready", "guardrail.ci_gate.plan", "guardrail.rule_writeback.plan", "guardrail.alert.plan", "guardrail.native_corpus.plan", "guardrail.go_native_fuzz.plan", "guardrail.native_corpus.manifest_preview", "ci-gate-plan.json", "rule-writeback-plan.json", "alert-plan.json", "native-corpus-plan.json", "go-native-fuzz-plan.json", "native-corpus-manifest.json", "native-corpus-sync-preview.json", "corpus_manifest", "sync_summary", "content_sha256", "writes_files"]) {
  if (!guardrailFuzzerSource.includes(token)) fail(`Guardrail Fuzzer handler missing fuzzer shell token: ${token}`);
}
for (const token of ["guardrailFuzzerStatus:", "guardrailFuzzerRun:", "guardrailFuzzerEvidence:"]) {
  if (apiSource.includes(token)) fail(`monolithic api.ts still exposes Guardrail Fuzzer method: ${token}`);
}


const memoryTimeTravelManifest = readJSON("packs/examples/memory-time-travel-pack/pack.json");
const memoryTimeTravelSource = readText("internal/packs/memorytimetravel/handler.go");
const memoryTimeTravelGateTest = readText("internal/controlplane/gateway/handlers_memory_time_travel_pack_test.go");
const memoryTimeTravelPage = readText("heroui-web/src/app/packs/memory-time-travel/page.tsx");
const memoryTimeTravelClient = readText("heroui-web/src/lib/memory-time-travel-pack-client.ts");
const memoryTimeTravelClientTest = readText("heroui-web/src/lib/__tests__/memory-time-travel-pack-client.test.ts");
const temporalKVSource = readText("internal/ledger/temporal_kv.go") + "\n" + readText("internal/ledger/temporal_kv_test.go");
const ledgerPersisterSource = readText("internal/ledger/ledger_persister.go") + "\n" + readText("internal/ledger/ledger_persister_test.go");
const memoryTimeTravelSdk = readText("sdk/typescript/src/memory-time-travel.ts") + "\n" + readText("sdk/typescript/src/memory-time-travel.test.ts");
if (memoryTimeTravelManifest) {
  if (!memoryTimeTravelSource.includes(`const PackID = "${memoryTimeTravelManifest.id}"`)) {
    fail("Memory Time Travel pack handler PackID must match packs/examples/memory-time-travel-pack/pack.json");
  }
  for (const route of memoryTimeTravelManifest.backend?.routes ?? []) {
    if (!memoryTimeTravelSource.includes(route)) fail(`Memory Time Travel handler missing route declared in manifest: ${route}`);
  }
  for (const method of ["http.MethodGet", "http.MethodPost"]) {
    if (!memoryTimeTravelSource.includes(method)) fail(`Memory Time Travel handler missing method gate declaration: ${method}`);
  }
  if (memoryTimeTravelManifest.frontend?.menus?.[0]?.path !== "/packs/memory-time-travel") fail("Memory Time Travel menu path must stay under /packs/memory-time-travel");
  if (memoryTimeTravelManifest.frontend?.routes?.[0]?.component !== "memory/MemoryTimeTravelPackPage") fail("Memory Time Travel frontend route component drifted");
  if (memoryTimeTravelManifest.sdk?.typescript !== "yunque-client/memory-time-travel") fail("Memory Time Travel SDK import must stay yunque-client/memory-time-travel");
  if (memoryTimeTravelManifest.defaultState !== "disabled") fail("Memory Time Travel pack must remain default disabled before Ledger KV history and Memory Persister write-back readiness");
  if (memoryTimeTravelManifest.metadata?.stage !== "pack-shell-before-ledger-kv-history") fail("Memory Time Travel pack stage must remain pack-shell-before-ledger-kv-history");
  if (memoryTimeTravelManifest.metadata?.blueprint !== "doc/MEMORY-TIME-TRAVEL.md") fail("Memory Time Travel pack blueprint pointer drifted");
}
if (memoryTimeTravelPage.includes('from "@/lib/api"') || memoryTimeTravelPage.includes("api.memoryTimeTravel") || !memoryTimeTravelPage.includes("createMemoryTimeTravelPackClient")) {
  fail("Memory Time Travel pack page must use memory-time-travel-pack-client instead of monolithic api object");
}
for (const token of ["createMemoryTimeTravelPackClient", "/v1/memory-time-travel/status", "/v1/memory-time-travel/snapshots", "/v1/memory-time-travel/snapshot-at", "/v1/memory-time-travel/diff", "/v1/memory-time-travel/rollback-plan", "/v1/memory-time-travel/rollback/approved-plan", "/v1/memory-time-travel/retention/plan", "/v1/memory-time-travel/retention/prune-plan", "/v1/memory-time-travel/kv-history/native-plan", "/v1/memory-time-travel/kv-history/migration-preview", "/v1/memory-time-travel/kv-history/dual-read/parity", "/v1/memory-time-travel/kv-history/cutover/plan", "/v1/memory-time-travel/kv-history/cutover/readiness", "/v1/memory-time-travel/audit/links", "/v1/memory-time-travel/audit/links/preview", "/v1/memory-time-travel/audit/links/writeback-plan", "/v1/memory-time-travel/audit/links/writeback/store", "/v1/memory-time-travel/audit/links/writeback/executor/plan", "/v1/memory-time-travel/audit/verify", "/v1/memory-time-travel/evidence/", "auditLinksWritebackPlan", "auditLinksWritebackStore", "auditLinksWritebackExecutorPlan", 'method: "POST"']) {
  if (!memoryTimeTravelClient.includes(token)) fail(`memory-time-travel-pack-client missing token: ${token}`);
}
if (!gatewaySource.includes('cfg.DataPath("memory-time-travel")')) {
  fail("Memory Time Travel runtime store must be wired through the configured data directory");
}
for (const token of ["TestMemoryTimeTravel", "StatusMethodNotAllowed", "/v1/memory-time-travel/diff", "/v1/memory-time-travel/rollback/approved-plan", "/v1/memory-time-travel/kv-history/native-plan", "/v1/memory-time-travel/kv-history/migration-preview", "/v1/memory-time-travel/kv-history/dual-read/parity", "/v1/memory-time-travel/kv-history/cutover/plan", "/v1/memory-time-travel/kv-history/cutover/readiness", "/v1/memory-time-travel/audit/links/preview", "/v1/memory-time-travel/audit/links/writeback-plan", "/v1/memory-time-travel/audit/links/writeback/store", "/v1/memory-time-travel/audit/links/writeback/executor/plan"]) {
  if (!memoryTimeTravelGateTest.includes(token)) fail(`Memory Time Travel gateway gate test missing token: ${token}`);
}
for (const token of ["createMemoryTimeTravelClient", "MemoryTimeTravelClientError", "/v1/memory-time-travel/status", "/v1/memory-time-travel/rollback/approved-plan", "/v1/memory-time-travel/retention/plan", "/v1/memory-time-travel/retention/prune-plan", "/v1/memory-time-travel/kv-history/native-plan", "/v1/memory-time-travel/kv-history/migration-preview", "/v1/memory-time-travel/kv-history/dual-read/parity", "/v1/memory-time-travel/kv-history/cutover/plan", "/v1/memory-time-travel/kv-history/cutover/readiness", "/v1/memory-time-travel/audit/links", "/v1/memory-time-travel/audit/links/preview", "/v1/memory-time-travel/audit/links/writeback-plan", "/v1/memory-time-travel/audit/links/writeback/store", "/v1/memory-time-travel/audit/links/writeback/executor/plan", "/v1/memory-time-travel/audit/verify", "/v1/memory-time-travel/evidence/", "approvedRollbackPlan", "approved_rollback_plan", "rollback_writeback_plan", "approval_request_plan", "retentionPlan", "retentionPrunePlan", "nativeKVHistoryPlan", "nativeKVHistoryMigrationPreview", "kvHistoryDualReadParity", "kvHistoryCutoverPlan", "kvHistoryCutoverReadiness", "auditLinksPreview", "native_kv_history_plan", "kv_history_migration_plan", "kv_history_index_plan", "kv_history_migration_preview", "kv_history_dual_read_parity", "native_kv_history_preview_ready", "kv_history_cutover_plan", "kv_history_cutover_readiness", "kv_history_dual_read_plan", "kv_history_dual_write_plan", "kv_history_cutover_plan_ready", "kv_history_cutover_readiness_ready", "cutover_readiness_check_ready", "dual_read_plan_ready", "dual_read_parity_check_ready", "dual_read_parity_ready", "parity_passed", "dual_write_plan_ready", "cutover_ready", "auditLinks", "auditLinksPreview", "auditLinksWritebackPlan", "auditLinksWritebackStore", "auditLinksWritebackExecutorPlan", "auditVerify", "kv_audit_link_writeback_plan", "kv_audit_link_writeback_actions", "kv_audit_link_writeback_store", "kv_audit_link_writeback_records", "kv_audit_link_writeback_plan_ready", "kv_audit_link_writeback_store_ready", "kv_audit_link_writeback_ready", "writes_audit_link_writeback_store", "backfills_audit_seq", "backfills_audit_hash", "consumes_audit_link_preview", "kv_audit_link_writeback_executor_plan", "kv_audit_link_writeback_executor_plan_ready", "executor_input_contract_ready", "audit_proof_link_executor_ready", "consumes_audit_link_writeback_store", "audit_append_plan_ready", "writes_audit_chain"]) {
  if (!memoryTimeTravelSdk.includes(token)) fail(`Memory Time Travel TypeScript SDK missing token: ${token}`);
}
for (const token of ["/v1/memory-time-travel/status", "/v1/memory-time-travel/diff", "/v1/memory-time-travel/rollback/approved-plan", "/v1/memory-time-travel/retention/plan?namespace=memory_snapshot", "/v1/memory-time-travel/retention/prune-plan", "/v1/memory-time-travel/kv-history/native-plan?namespace=memory_snapshot", "/v1/memory-time-travel/kv-history/migration-preview?namespace=memory_snapshot&limit=50", "/v1/memory-time-travel/kv-history/dual-read/parity", "/v1/memory-time-travel/kv-history/cutover/plan", "/v1/memory-time-travel/kv-history/cutover/readiness", "/v1/memory-time-travel/audit/links/preview", "/v1/memory-time-travel/audit/links/writeback-plan", "/v1/memory-time-travel/audit/links/writeback/store", "/v1/memory-time-travel/audit/links/writeback/executor/plan", "/v1/memory-time-travel/audit/links?namespace=memory_snapshot", "/v1/memory-time-travel/audit/verify?limit=3", "/v1/memory-time-travel/evidence/baseline"]) {
  if (!memoryTimeTravelClientTest.includes(token)) fail(`Memory Time Travel frontend client test missing token: ${token}`);
}
for (const token of ["json-memory-time-travel-evidence", "approved-rollback-plan.json", "approved_rollback_plan", "rollback-writeback-plan.json", "rollback_writeback_plan", "approval-request-plan.json", "approval_request_plan", "approved_rollback_plan_ready", "approval_request_plan_ready", "approval_manager_bridge_plan_ready", "global_approval_enqueue_ready", "rollback_writeback_plan_ready", "writes_ledger_kv", "writes_temporal_kv", "memory.rollback.approved_plan", "memory.rollback.writeback.plan", "retention-plan.json", "retention_plan", "retention-prune-plan.json", "retention_prune_plan", "retention_plan_ready", "retention_prune_plan_ready", "retention_prune_ready", "memory.retention.plan", "memory.retention.prune_plan", "memory.kv_history.native_plan", "memory.kv_history.migration_preview", "memory.kv_history.dual_read.parity", "memory.kv_history.cutover.plan", "memory.kv_history.cutover.readiness", "native-kv-history-plan.json", "native_kv_history_plan", "kv-history-migration-plan.json", "kv_history_migration_plan", "kv-history-index-plan.json", "kv_history_index_plan", "kv-history-migration-preview.json", "kv_history_migration_preview", "kv-history-dual-read-parity.json", "kv_history_dual_read_parity", "kv-history-cutover-plan.json", "kv-history-cutover-readiness.json", "kv-history-dual-read-plan.json", "kv-history-dual-write-plan.json", "kv_history_cutover_plan", "kv_history_cutover_readiness", "kv_history_dual_read_plan", "kv_history_dual_write_plan", "kv_history_cutover_plan_ready", "kv_history_cutover_readiness_ready", "cutover_readiness_check_ready", "dual_read_plan_ready", "dual_read_parity_check_ready", "dual_read_parity_ready", "parity_passed", "dual_write_plan_ready", "dual_read_ready", "dual_write_ready", "cutover_ready", "switches_temporal_adapter", "native_kv_history_plan_ready", "kv_history_migration_plan_ready", "kv_history_index_plan_ready", "native_kv_history_preview_ready", "native_kv_history_ready", "writes_native_kv_history", "migrates_kv_history", "max_snapshots_per_namespace", "audit-links.json", "audit-link-preview.json", "audit-link-writeback-plan.json", "audit-link-writeback-store.json", "audit-link-writeback-record.json", "audit-link-writeback-executor-plan.json", "audit-link-executor-handoff-plan.json", "audit-link-executor-audit-plan.json", "kv_audit_link_schema", "kv_audit_link_preview", "kv_audit_link_writeback_plan", "kv_audit_link_writeback_actions", "kv_audit_links", "kv_audit_link_schema_ready", "kv_audit_link_preview_ready", "kv_audit_link_writeback_plan_ready", "kv_audit_link_writeback_store_ready", "kv_audit_link_writeback_executor_plan_ready", "executor_input_contract_ready", "audit_proof_link_executor_ready", "consumes_audit_link_writeback_store", "audit_append_plan_ready", "writes_audit_chain", "writes_audit_link_writeback_store", "kv_audit_link_writeback_ready", "kv_audit_linkage_ready", "backfills_audit_seq", "backfills_audit_hash", "consumes_audit_link_preview", "memory.audit.links.schema", "memory.audit.links.preview", "memory.audit.links.writeback_plan", "memory.audit.links.writeback_store", "memory.audit.links.writeback_executor_plan", "audit-verification.json", "audit_verification", "snapshot_store_ready", "temporal_query_ready", "ledger_history_ready", "memory_persister_writeback_ready", "TemporalKVReader", "SnapshotRawAt", "NativeKVHistoryPreviewer", "PreviewNativeKVHistoryRows", "KVHistoryDualReadParity", "ledger-kv-history", "merkle_verification_ready", "memory.audit.verify", "MerkleVerifier", "VerifyMerkleAuditChain", "rollback_writeback_ready", "kv_audit_link_writeback_executor_plan", "audit_link_executor_handoff_plan", "audit_link_executor_audit_plan"]) {
  if (!memoryTimeTravelSource.includes(token)) fail(`Memory Time Travel handler missing memory governance shell token: ${token}`);
}
for (const token of ["NewTemporalKVStore", "PutRawVersionedAt", "GetRawAt", "ListVersions", "SnapshotRawAt", "PreviewNativeKVHistoryRows", "NativeKVHistoryMigrationPreview", "__kv_history__", "TestTemporalKVStorePutVersionedAndGetRawAt", "TestTemporalKVStoreListVersionsAndSnapshotRawAt", "TestTemporalKVStorePreviewNativeKVHistoryRowsIsReadOnly"]) {
  if (!temporalKVSource.includes(token)) fail(`Temporal KV history slice missing token: ${token}`);
}
for (const token of ["WithLedgerPersisterTemporalKV", "TemporalWritebackReady", "temporalMemoryRecord", "writeTemporalMemory", "memory_snapshot", "TestLedgerPersisterFlushMirrorsMemoryIntoTemporalKV"]) {
  if (!ledgerPersisterSource.includes(token)) fail(`Ledger Persister temporal write-back slice missing token: ${token}`);
}
if (!gatewaySource.includes("memoryTimeTravelTemporalKV") || !gatewaySource.includes("NewTemporalKVStore(app.Ledger)")) {
  fail("Memory Time Travel pack must wire the Ledger temporal KV history reader in cmd/agent/init_tasks.go");
}
if (!gatewaySource.includes("MemoryPersisterWriteback") || !gatewaySource.includes("memoryPersisterTemporalWritebackReady")) {
  fail("Memory Time Travel pack status must receive Memory Persister temporal write-back readiness from cmd/agent/init_tasks.go");
}
if (!gatewaySource.includes("memoryTimeTravelMerkleVerifier") || !gatewaySource.includes("VerifyMerkleAuditChain")) {
  fail("Memory Time Travel pack must wire read-only Merkle audit-chain verification from cmd/agent/init_tasks.go");
}
for (const token of ["memoryTimeTravelStatus:", "memoryTimeTravelDiff:", "memoryTimeTravelEvidence:"]) {
  if (apiSource.includes(token)) fail(`monolithic api.ts still exposes Memory Time Travel method: ${token}`);
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
for (const token of ["createSkillAnomalyPackClient", "/v1/skill-anomaly/status", "/v1/skill-anomaly/detect", "/v1/skill-anomaly/audit-hook/plan", "/v1/skill-anomaly/approval-queue/writeback", "/v1/skill-anomaly/approval-queue/bridge/plan", "/v1/skill-anomaly/evidence/", "approval_queue_store_ready", "approval_manager_bridge_plan_ready", "global_approval_enqueue_ready", "approval_queue_store", "approval_queue_record", "approval_manager_bridge_plan", "approval-queue-store.json", "approval-queue-record.json", "approval-manager-bridge-plan.json", "SkillAnomalyApprovalQueueWriteback", "SkillAnomalyApprovalManagerBridgePlan", "approvalManagerBridgePlan", 'method: "POST"']) {
  if (!skillAnomalyClient.includes(token)) fail(`skill-anomaly-pack-client missing token: ${token}`);
}
if (!gatewaySource.includes('cfg.DataPath("skill-anomaly")')) {
  fail("Skill Anomaly runtime store must be wired through the configured data directory");
}
for (const token of ["TestSkillAnomaly", "StatusNotFound", "StatusMethodNotAllowed", "/v1/skill-anomaly/detect", "/v1/skill-anomaly/approval-queue/writeback", "/v1/skill-anomaly/approval-queue/bridge/plan", "approval_queue_store_ready", "approval_manager_bridge_plan_ready", "global_approval_enqueue_ready", "skill.approval_queue.writeback", "skill.approval_manager.bridge.plan", "writes_approval_queue_file", "execution_blocked", "action_allowed"]) {
  if (!skillAnomalyGateTest.includes(token)) fail(`Skill Anomaly gateway gate test missing token: ${token}`);
}
for (const token of ["createSkillAnomalyClient", "SkillAnomalyClientError", "/v1/skill-anomaly/status", "/v1/skill-anomaly/audit-hook/plan", "/v1/skill-anomaly/approval-queue/writeback", "/v1/skill-anomaly/approval-queue/bridge/plan", "/v1/skill-anomaly/evidence/", "SkillAnomalyApprovalQueueWriteback", "SkillAnomalyApprovalManagerBridgePlan", "approvalQueueWriteback", "approvalManagerBridgePlan", "approval_queue_store", "approval_queue_record", "approval_manager_bridge_plan"]) {
  if (!skillAnomalySdk.includes(token)) fail(`Skill Anomaly TypeScript SDK missing token: ${token}`);
}
for (const token of ["/v1/skill-anomaly/status", "/v1/skill-anomaly/detect", "/v1/skill-anomaly/approval-queue/writeback", "/v1/skill-anomaly/approval-queue/bridge/plan", "/v1/skill-anomaly/evidence/text_processing", "approval-queue-store.json", "approval-queue-record.json", "approval-manager-bridge-plan.json"]) {
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
for (const token of ["createWASMPluginPackClient", "/v1/wasm-plugin/status", "/v1/wasm-plugin/execute", "/v1/wasm-plugin/remote-install/plan", "/v1/wasm-plugin/remote-install/approval/plan", "/v1/wasm-plugin/remote-install/approval/decision/plan", "/v1/wasm-plugin/remote-install/approval/writeback/plan", "/v1/wasm-plugin/remote-install/approval/queue/writeback", "/v1/wasm-plugin/evidence/", "remoteInstallPlan", "remoteInstallApprovalPlan", "remoteInstallApprovalDecisionPlan", "remoteInstallApprovalWritebackPlan", "remoteInstallApprovalQueueWriteback", "WASMPluginRemoteInstallPlan", "WASMPluginRemoteInstallApprovalPlan", "WASMPluginRemoteInstallApprovalDecisionPlan", "WASMPluginRemoteInstallApprovalWritebackPlan", "WASMPluginRemoteInstallApprovalQueueWriteback", "WASMPluginApprovalQueueRecord", "WASMPluginApprovalQueueStoreSummary", "WASMPluginApprovalDecisionPlan", "WASMPluginApprovalWritebackPlan", "WASMPluginSignatureVerificationPlan", "remote_install_plan", "signature_verification", "approval_gate_plan", "approval_decision_plan", "approval_writeback_plan", 'method: "POST"']) {
  if (!wasmPluginClient.includes(token)) fail(`wasm-plugin-pack-client missing token: ${token}`);
}
for (const token of ["abi_plan_ready", "host_abi_plan", "WASMPluginHostABIPlan", "host_abi_gate", "WASMPluginHostABIExecutionGate", "host_abi_execution_gate_ready", "host_abi_enforcement_ready", "module_integrity_gate_ready", "module_integrity_gate", "WASMPluginModuleIntegrityGate", "integrity_gate_ready", "blocked_module_sha256_mismatch", "execution_gate_ready", "allows_execution", "blocked_until_host_abi_enforcement", "enforcement_ready", "writes_files"]) {
  if (!wasmPluginClient.includes(token)) fail(`wasm-plugin-pack-client missing Host ABI plan token: ${token}`);
}
for (const token of ["remote_install_plan_ready", "remote_install_ready", "remote-install-plan.json", "signature-verification.json", "signature_verification_plan_ready", "signature_verification", "verification_gate_ready", "blocked_until_signature_verifier", "allows_install", "download_ready", "signature_verify_ready", "downloads", "writes_files"]) {
  if (!wasmPluginClient.includes(token) || !wasmPluginPage.includes(token)) fail(`wasm-plugin frontend remote install plan missing token: ${token}`);
}
for (const token of ["approval_gate_plan_ready", "approval_gate_ready", "approval_queue_plan_ready", "approval_queue_entry", "approval-gate-plan.json", "approval-queue-entry.json", "blocked_until_approval_queue", "requires_approval", "writes_approval_queue", "remoteInstallApprovalPlan"]) {
  if (!wasmPluginClient.includes(token) || !wasmPluginPage.includes(token)) fail(`wasm-plugin frontend approval gate plan missing token: ${token}`);
}
for (const token of ["approval_decision_plan_ready", "approval_decision_ready", "applies_approval_decision", "approval_decision_plan", "approval-decision-plan.json", "would_allow_installer_continue", "blocks_installer", "decision_key", "remoteInstallApprovalDecisionPlan"]) {
  if (!wasmPluginClient.includes(token) || !wasmPluginPage.includes(token)) fail(`wasm-plugin frontend approval decision plan missing token: ${token}`);
}
for (const token of ["approval_writeback_plan_ready", "approval_writeback_ready", "approval_writeback_plan", "approval_queue_store", "approval_queue_record", "approval-queue-store.json", "approval-queue-record.json", "approval-writeback-plan.json", "installer_blocked_until_writeback", "installer_blocked_until_installer_wiring", "approval_queue_store_ready", "writes_approval_queue_store", "writeback_store", "remoteInstallApprovalWritebackPlan"]) {
  if (!wasmPluginClient.includes(token) || !wasmPluginPage.includes(token)) fail(`wasm-plugin frontend approval writeback plan missing token: ${token}`);
}
if (!gatewaySource.includes('cfg.DataPath("wasm-plugin")')) {
  fail("WASM Plugin runtime store must be wired through the configured data directory");
}
for (const token of ["TestWASMPlugin", "StatusNotFound", "StatusMethodNotAllowed", "/v1/wasm-plugin/execute", "/v1/wasm-plugin/remote-install/plan", "/v1/wasm-plugin/remote-install/approval/plan", "/v1/wasm-plugin/remote-install/approval/decision/plan", "/v1/wasm-plugin/remote-install/approval/writeback/plan"]) {
  if (!wasmPluginGateTest.includes(token)) fail(`WASM Plugin gateway gate test missing token: ${token}`);
}
for (const token of ["createWASMPluginClient", "WASMPluginClientError", "/v1/wasm-plugin/status", "/v1/wasm-plugin/remote-install/plan", "/v1/wasm-plugin/remote-install/approval/plan", "/v1/wasm-plugin/remote-install/approval/decision/plan", "/v1/wasm-plugin/remote-install/approval/writeback/plan", "/v1/wasm-plugin/remote-install/approval/queue/writeback", "/v1/wasm-plugin/evidence/", "remoteInstallPlan", "remoteInstallApprovalPlan", "remoteInstallApprovalDecisionPlan", "remoteInstallApprovalWritebackPlan", "remoteInstallApprovalQueueWriteback", "WASMPluginRemoteInstallPlan", "WASMPluginRemoteInstallApprovalPlan", "WASMPluginRemoteInstallApprovalDecisionPlan", "WASMPluginRemoteInstallApprovalWritebackPlan", "WASMPluginSignatureVerificationPlan", "WASMPluginApprovalQueueEntryPlan", "WASMPluginApprovalDecisionPlan", "WASMPluginApprovalWritebackPlan", "remote_install_plan", "signature_verification", "approval_queue_entry", "approval_gate_plan", "approval_decision_plan", "approval_writeback_plan"]) {
  if (!wasmPluginSdk.includes(token)) fail(`WASM Plugin TypeScript SDK missing token: ${token}`);
}
for (const token of ["WASMPluginHostABIPlan", "WASMPluginHostABIExecutionGate", "WASMPluginModuleIntegrityGate", "host_abi_plan", "host_abi_gate", "module_integrity_gate", "host_abi_execution_gate_ready", "host_abi_enforcement_ready", "module_integrity_gate_ready", "integrity_gate_ready", "blocked_module_sha256_mismatch", "execution_gate_ready", "allows_execution", "enforcement_ready", "writes_files"]) {
  if (!wasmPluginSdk.includes(token)) fail(`WASM Plugin TypeScript SDK missing Host ABI plan token: ${token}`);
}
for (const token of ["/v1/wasm-plugin/status", "/v1/wasm-plugin/execute", "/v1/wasm-plugin/remote-install/plan", "/v1/wasm-plugin/remote-install/approval/plan", "/v1/wasm-plugin/remote-install/approval/decision/plan", "/v1/wasm-plugin/remote-install/approval/writeback/plan", "/v1/wasm-plugin/remote-install/approval/queue/writeback", "/v1/wasm-plugin/evidence/calculator", "host-abi-plan.json", "module-integrity-gate.json", "remote-install-plan.json", "signature-verification.json", "approval-gate-plan.json", "approval-queue-entry.json", "approval-decision-plan.json", "approval-writeback-plan.json", "approval-queue-store.json", "approval-queue-record.json"]) {
  if (!wasmPluginClientTest.includes(token)) fail(`WASM Plugin frontend client test missing token: ${token}`);
}
for (const token of ["normalizeModulePath", "validateModulePath", "module_path must not contain traversal segments", "abi_plan_ready", "wasm.host_abi.plan", "wasm.host_abi.execution_gate", "wasm.module.integrity_gate", "host_abi_plan", "host_abi_gate", "module_integrity_gate", "module_integrity_gate_ready", "host_abi_execution_gate_ready", "host_abi_enforcement_ready", "blocked_until_host_abi_enforcement", "blocked_module_sha256_mismatch", "module-integrity-gate.json", "host-abi-plan.json", "enforcement_ready", "writes_files", "remote_install_plan_ready", "wasm.remote_install.plan", "wasm.remote_install.signature_verification_plan", "remote_install_plan", "signature_verification", "SignatureVerificationPlan", "remote-install-plan.json", "signature-verification.json", "signature_verification_plan_ready", "verification_gate_ready", "blocked_until_signature_verifier", "allows_install", "downloads", "approval_gate_plan_ready", "approval_queue_plan_ready", "approval_queue_entry", "ApprovalQueueEntryPlan", "blocked_until_approval_queue", "approval-queue-entry.json", "wasm.remote_install.approval_plan", "approval_gate_plan", "approval-gate-plan.json", "requires_approval", "writes_approval_queue", "approval_decision_plan_ready", "approval_decision_ready", "applies_approval_decision", "approval_decision_plan", "ApprovalDecisionPlan", "RemoteInstallApprovalDecisionPlanReport", "wasm.remote_install.approval_decision_plan", "approval-decision-plan.json", "would_allow_installer_continue", "blocks_installer", "approval_writeback_plan_ready", "approval_writeback_ready", "approval_writeback_plan", "ApprovalWritebackPlan", "RemoteInstallApprovalWritebackPlanReport", "wasm.remote_install.approval_writeback_plan", "approval-writeback-plan.json", "installer_blocked_until_writeback", "installer_blocked_until_installer_wiring", "approval_queue_store_ready", "writes_approval_queue_store", "approval_queue_store", "approval_queue_record", "approval-queue-store.json", "approval-queue-record.json", "RemoteInstallApprovalQueueWritebackReport", "ApprovalQueueRecord", "ApprovalQueueStoreSummary", "wasm.remote_install.approval_queue_writeback"]) {
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
