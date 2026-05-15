import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/wasm-plugin-pack-sdk.json"), "utf8"));
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

if (pack.id !== "yunque.pack.wasm-plugin") fail(`unexpected WASM Plugin pack id: ${pack.id}`);
if (pack.sdk?.typescript !== manifest.sdkImport) fail("WASM Plugin pack sdk.typescript must match wasm-plugin-pack-sdk.json sdkImport");
if (pack.frontend?.menus?.[0]?.path !== manifest.frontend.menuPath) fail("WASM Plugin pack frontend menu path must remain /packs/wasm-plugin");
if (pack.frontend?.routes?.[0]?.component !== manifest.frontend.component) fail("WASM Plugin pack frontend route component drifted");
if (pack.update?.rollback !== true) fail("WASM Plugin pack must be rollbackable");
if (pack.defaultState !== "disabled") fail("WASM Plugin pack should stay default disabled because it is a high-risk execution surface");
if (pack.metadata?.stage !== "pack-shell-before-runtime-hosts") fail("WASM Plugin pack should declare pack-shell-before-runtime-hosts stage");
if (pack.metadata?.risk !== "high") fail("WASM Plugin pack should keep high-risk metadata");

const routeSpecs = new Set((pack.backend?.routeSpecs ?? []).map((route) => `${route.method} ${route.path}`));
for (const route of manifest.routes ?? []) {
  if (!routeSpecs.has(route)) fail(`WASM Plugin pack manifest missing routeSpec: ${route}`);
}

const client = readRepoFile(manifest.frontend.client);
for (const token of [
  "createWASMPluginPackClient",
  "/v1/wasm-plugin/status",
  "/v1/wasm-plugin/plugins",
  "/v1/wasm-plugin/plugins/load",
  "/v1/wasm-plugin/plugins/unload",
  "/v1/wasm-plugin/execute",
  "/v1/wasm-plugin/evidence/",
  "method: \"POST\"",
]) {
  if (!client.includes(token)) fail(`wasm-plugin-pack-client missing token: ${token}`);
}

const page = readRepoFile(manifest.frontend.page);
if (!page.includes("createWASMPluginPackClient") || page.includes('from "@/lib/api"') || page.includes("api.wasm")) {
  fail("WASM Plugin pack page must use wasm-plugin-pack-client instead of monolithic api.ts");
}
for (const token of ["WASM 插件引擎", "校验 / 注册插件", "Dry-run", "导出证据包", "pack-shell"]) {
  if (!page.includes(token)) fail(`WASM Plugin pack page missing product token: ${token}`);
}

const frontendTest = readRepoFile("heroui-web/src/lib/__tests__/wasm-plugin-pack-client.test.ts");
for (const token of ["/v1/wasm-plugin/status", "/v1/wasm-plugin/execute", "/v1/wasm-plugin/evidence/calculator"]) {
  if (!frontendTest.includes(token)) fail(`WASM Plugin frontend client test missing token: ${token}`);
}

const backend = readRepoFile("internal/packs/wasmplugin/handler.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_wasm_plugin_pack_test.go")
  + "\n" + readRepoFile("cmd/agent/init_tasks.go")
  + "\n" + readRepoFile("cmd/agent/packruntime_bootstrap_test.go");
for (const token of [
  "const PackID = \"yunque.pack.wasm-plugin\"",
  "runtime_ready",
  "abi_ready",
  "json-wasm-plugin-evidence",
  "cfg.DataPath(\"wasm-plugin\")",
  "wasmpluginpack.New",
  "packs/examples/wasm-plugin-pack/pack.json",
  "ensureBuiltinPacks",
  "TestWASMPluginPackGateReturnsNotFoundWhenDisabled",
  "StatusMethodNotAllowed",
]) {
  if (!backend.includes(token)) fail(`WASM Plugin backend pack or gate missing token: ${token}`);
}

const sdk = readRepoFile("sdk/typescript/src/wasm-plugin.ts") + "\n" + readRepoFile("sdk/typescript/src/wasm-plugin.test.ts");
for (const token of [
  "createWASMPluginClient",
  "WASMPluginClientError",
  "/v1/wasm-plugin/status",
  "/v1/wasm-plugin/execute",
  "/v1/wasm-plugin/evidence/",
  "WASM Plugin request failed",
]) {
  if (!sdk.includes(token)) fail(`TypeScript WASM Plugin SDK slice missing token: ${token}`);
}

const pkg = JSON.parse(readRepoFile("sdk/typescript/package.json") || "{}");
if (pkg.exports?.["./wasm-plugin"]?.import !== "./src/wasm-plugin.ts") fail("yunque-client/wasm-plugin subpath export is missing or drifted");

const monolithicApi = readRepoFile("heroui-web/src/lib/api.ts");
for (const token of ["wasmPluginStatus:", "createWASMPlugin:", "wasmPluginExecute:", "wasmPluginEvidence:"]) {
  if (monolithicApi.includes(token)) fail(`monolithic api.ts should not expose WASM Plugin method: ${token}`);
}

if (failures.length) {
  console.error("WASM Plugin Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`WASM Plugin Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
