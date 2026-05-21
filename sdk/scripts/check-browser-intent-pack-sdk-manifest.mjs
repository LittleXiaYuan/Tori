import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/browser-intent-pack-sdk.json"), "utf8"));
const pack = JSON.parse(readFileSync(resolve(repoRoot, manifest.packManifest), "utf8"));
const failures = [];

function fail(message) {
  failures.push(message);
}

function readRepoFile(path) {
  const fullPath = resolve(repoRoot, path);
  if (!existsSync(fullPath)) {
    fail(`missing file: ${path}`);
    return "";
  }
  return readFileSync(fullPath, "utf8");
}

if (pack.id !== "yunque.pack.browser-intent") fail(`unexpected Browser Intent pack id: ${pack.id}`);
if (pack.sdk?.typescript !== manifest.sdkImport) fail("Browser Intent pack sdk.typescript must match browser-intent-pack-sdk.json sdkImport");
if (pack.frontend?.menus?.[0]?.path !== manifest.frontend.menuPath) fail("Browser Intent pack frontend menu path must remain /packs/browser");
if (pack.frontend?.routes?.[0]?.component !== manifest.frontend.component) fail("Browser Intent pack frontend route component drifted");
if (pack.update?.rollback !== true) fail("Browser Intent pack must be rollbackable");
if (pack.defaultState !== "enabled") fail("Browser Intent bridge pack should stay default enabled while legacy browser UI migrates to pack sync");

const routeSpecs = new Set((pack.backend?.routeSpecs ?? []).map((route) => `${route.method} ${route.path}`));
for (const route of manifest.routes ?? []) {
  if (!routeSpecs.has(route)) fail(`Browser Intent pack manifest missing routeSpec: ${route}`);
}

const client = readRepoFile(manifest.frontend.client);
for (const token of [
  "createBrowserIntentPackClient",
  "/v1/browser/status",
  "/v1/browser/intent/plan",
  "browserActPlan",
  "browser_act_plan_ready",
  "permission_gate_ready",
  "runtime_skill_gate_ready",
  "opp_gate_ready",
  "/v1/browser/ocr",
  "/v1/browser/opp/decide",
  "/api/browser/ext/session",
  "/api/browser/ext/scenarios/run",
  "method: \"POST\"",
  "/v1/sandbox/desktop/status",
]) {
  if (!client.includes(token)) fail(`browser-intent-pack-client missing token: ${token}`);
}

const page = readRepoFile("apps/web/src/app/packs/browser/page.tsx");
if (!page.includes("createBrowserIntentPackClient") || page.includes('from "@/lib/api"') || page.includes("api.browser")) {
  fail("Browser Intent pack page must use browser-intent-pack-client instead of monolithic api.ts");
}

const monolithicApi = readRepoFile("apps/web/src/lib/api.ts");
for (const token of ["browserNavigate:", "browserScreenshot:", "browserOcr:", "browserStatus:", "browserConfig:", "browserExtStatus:", "browserExtAction:", "browserExtScenarios:", "browserExtRunScenario:"]) {
  if (monolithicApi.includes(token)) fail(`monolithic api.ts still exposes Browser Intent method: ${token}`);
}

const legacy = readRepoFile(manifest.frontend.legacyPath);
if (!legacy.includes('redirect("/packs/browser")')) fail("legacy /browser page must redirect to /packs/browser");

const shell = readRepoFile("apps/web/src/components/sidebar.tsx") + "\n" + readRepoFile("apps/web/src/lib/nav-items.tsx") + "\n" + readRepoFile("apps/web/src/components/command-palette.tsx");
if (shell.includes('href: "/browser"') || shell.includes("nav-browser")) fail("Browser Intent must not remain a hard-coded main-shell nav item");
if (!shell.includes("fetchEnabledPacks") || !shell.includes("buildPackNavItems")) fail("front shell should sync Browser Intent entry through enabled packs");

const backend = readRepoFile("internal/packs/browserintent/handler.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_browser_pack.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_browser_pack_test.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_packs.go")
  + "\n" + readRepoFile("cmd/agent/init_tasks.go");
for (const token of [
  "const PackID = \"yunque.pack.browser-intent\"",
  "type BrowserGateway interface",
  "BrowserActPlan",
  "browser-act-plan-before-runtime",
  "browser-act-plan.json",
  "HandleBrowserIntentPack",
  "HandleBrowserIntentSession",
  "BackendRouteAuthPassthrough",
  "Methods: methods",
  "BackendRouteInfo{Method",
  "browserintentpack.NewHandler",
  "packs/official/browser-intent-pack/pack.json",
  "ensureBuiltinPacks",
  "loadBuiltinPackManifest",
]) {
  if (!backend.includes(token)) fail(`Browser Intent backend pack or gate missing token: ${token}`);
}

if (failures.length) {
  console.error("Browser Intent Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Browser Intent Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
