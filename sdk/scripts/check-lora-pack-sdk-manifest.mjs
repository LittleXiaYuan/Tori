import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/lora-pack-sdk.json"), "utf8"));
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

if (pack.id !== "yunque.pack.lora") fail(`unexpected LoRA pack id: ${pack.id}`);
if (pack.sdk?.typescript !== manifest.sdkImport) fail("LoRA pack sdk.typescript must match lora-pack-sdk.json sdkImport");
if (pack.frontend?.menus?.[0]?.path !== manifest.frontend.menuPath) fail("LoRA pack frontend menu path must remain /packs/lora");
if (pack.frontend?.routes?.[0]?.component !== manifest.frontend.component) fail("LoRA pack frontend route component drifted");
if (pack.update?.rollback !== true) fail("LoRA pack must be rollbackable");

const routeSpecs = new Set((pack.backend?.routeSpecs ?? []).map((route) => `${route.method} ${route.path}`));
for (const route of manifest.routes ?? []) {
  if (!routeSpecs.has(route)) fail(`LoRA pack manifest missing routeSpec: ${route}`);
}

const client = readRepoFile(manifest.frontend.client);
for (const token of ["createLoRAPackClient", "/v1/lora/status", "/v1/lora/trigger", "/v1/lora/config", "method: \"PUT\""]) {
  if (!client.includes(token)) fail(`lora-pack-client missing token: ${token}`);
}

const page = readRepoFile("apps/web/src/app/packs/lora/page.tsx");
if (!page.includes("createLoRAPackClient") || !page.includes("pack route is not enabled") || page.includes('from "@/lib/api"') || page.includes("api.getLoRA")) {
  fail("LoRA pack page must use lora-pack-client instead of monolithic api.ts");
}

const legacy = readRepoFile("apps/web/src/app/lora/page.tsx");
if (!legacy.includes('redirect("/packs/lora")')) fail("legacy /lora page must redirect to /packs/lora");

const shell = readRepoFile("apps/web/src/components/sidebar.tsx") + "\n" + readRepoFile("apps/web/src/lib/nav-items.tsx");
if (shell.includes('href: "/lora"') || shell.includes('nav-lora')) fail("LoRA must not remain a hard-coded main-shell nav item");

const backend = readRepoFile("internal/packs/lora/handler.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_packs.go") + "\n" + readRepoFile("cmd/agent/init_tasks.go");
for (const token of ["const PackID = \"yunque.pack.lora\"", "Methods: []string", "http.MethodPatch", "BackendRouteInfo{Method", "Methods: methods", "lorapack.NewHandler", "ensureBuiltinPacks", "loadBuiltinPackManifest"]) {
  if (!backend.includes(token)) fail(`LoRA backend pack or multi-method gate missing token: ${token}`);
}

if (failures.length) {
  console.error("LoRA Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`LoRA Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
