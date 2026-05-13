import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/cogni-kernel-pack-sdk.json"), "utf8"));
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

if (pack.id !== "yunque.pack.cogni-kernel") fail(`unexpected Cogni Kernel pack id: ${pack.id}`);
if (pack.sdk?.typescript !== manifest.sdkImport) fail("Cogni Kernel pack sdk.typescript must match cogni-kernel-pack-sdk.json sdkImport");
if (pack.frontend?.menus?.[0]?.path !== manifest.frontend.menuPath) fail("Cogni Kernel pack frontend menu path must remain /packs/cognis");
if (pack.frontend?.routes?.[0]?.component !== manifest.frontend.component) fail("Cogni Kernel pack frontend route component drifted");
if (pack.update?.rollback !== true) fail("Cogni Kernel pack must be rollbackable");
if (pack.defaultState !== "enabled") fail("Cogni Kernel bridge pack should stay default enabled until runtime-loop gating lands");

const routeSpecs = new Set((pack.backend?.routeSpecs ?? []).map((route) => `${route.method} ${route.path}`));
for (const route of manifest.routes ?? []) {
  if (!routeSpecs.has(route)) fail(`Cogni Kernel pack manifest missing routeSpec: ${route}`);
}

const client = readRepoFile(manifest.frontend.client);
for (const token of [
  "createCogniKernelPackClient",
  "/v1/cognis",
  "/v1/cognis/reload",
  "/v1/cognis/alerts",
  "/v1/cognis/export",
  "/v1/cognis/import",
  "/v1/cognis/${enc(id)}/experience",
  "method: \"DELETE\"",
]) {
  if (!client.includes(token)) fail(`cogni-kernel-pack-client missing token: ${token}`);
}

const page = readRepoFile("heroui-web/src/app/packs/cognis/page.tsx");
if (!page.includes("createCogniKernelPackClient") || page.includes('from "@/lib/api"') || page.includes("api.listCognis") || page.includes("api.reloadCognis")) {
  fail("Cogni Kernel pack page must use cogni-kernel-pack-client instead of monolithic api.ts");
}

const legacy = readRepoFile(manifest.frontend.legacyPath);
if (!legacy.includes('redirect("/packs/cognis")')) fail("legacy /cognis page must redirect to /packs/cognis");

const shell = readRepoFile("heroui-web/src/components/sidebar.tsx") + "\n" + readRepoFile("heroui-web/src/lib/nav-items.tsx") + "\n" + readRepoFile("heroui-web/src/components/layout/account-rail.tsx");
if (shell.includes('href: "/cognis"') || shell.includes("nav-cognis")) fail("Cogni Kernel must not remain a hard-coded main-shell nav item");
if (!shell.includes("fetchEnabledPacks") || !shell.includes("buildPackNavItems")) fail("front shell should sync Cogni Kernel entry through enabled packs");

const backend = readRepoFile("internal/packs/cognikernel/handler.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_cogni.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_packs.go")
  + "\n" + readRepoFile("cmd/agent/module_cogni.go")
  + "\n" + readRepoFile("cmd/agent/init_tasks.go");
for (const token of [
  "const PackID = \"yunque.pack.cogni-kernel\"",
  "type CogniGateway interface",
  "HandleCogniKernelPack",
  "Methods: []string",
  "http.MethodDelete",
  "BackendRouteInfo{Method",
  "Methods: methods",
  "cognikernelpack.NewHandler",
  "packs/examples/cogni-kernel-pack/pack.json",
  "ensureBuiltinPacks",
  "loadBuiltinPackManifest",
]) {
  if (!backend.includes(token)) fail(`Cogni Kernel backend pack or gate missing token: ${token}`);
}

const monolithicApi = readRepoFile("heroui-web/src/lib/api.ts");
for (const token of ["listCognis:", "reloadCognis:", "getCogniHealth:", "triggerCogniEvolution:", "getCogniFederation:"]) {
  if (monolithicApi.includes(token)) fail(`monolithic api.ts still exposes Cogni method: ${token}`);
}

if (failures.length) {
  console.error("Cogni Kernel Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Cogni Kernel Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
