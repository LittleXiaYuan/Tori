import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/guardrail-fuzzer-pack-sdk.json"), "utf8"));
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

if (pack.id !== "yunque.pack.guardrail-fuzzer") fail(`unexpected Guardrail Fuzzer pack id: ${pack.id}`);
if (pack.sdk?.typescript !== manifest.sdkImport) fail("Guardrail Fuzzer pack sdk.typescript must match guardrail-fuzzer-pack-sdk.json sdkImport");
if (pack.frontend?.menus?.[0]?.path !== manifest.frontend.menuPath) fail("Guardrail Fuzzer pack frontend menu path must remain /packs/guardrail-fuzzer");
if (pack.frontend?.routes?.[0]?.component !== manifest.frontend.component) fail("Guardrail Fuzzer pack frontend route component drifted");
if (pack.update?.rollback !== true) fail("Guardrail Fuzzer pack must be rollbackable");
if (pack.defaultState !== "disabled") fail("Guardrail Fuzzer pack should stay default disabled until CI fuzz gates are wired");
if (pack.metadata?.stage !== "pack-shell-before-ci-fuzz") fail("Guardrail Fuzzer pack should declare pack-shell-before-ci-fuzz stage");
if (pack.metadata?.blueprint !== "doc/GUARDRAIL-FUZZER.md") fail("Guardrail Fuzzer pack should point to doc/GUARDRAIL-FUZZER.md");

const routeSpecs = new Set((pack.backend?.routeSpecs ?? []).map((route) => `${route.method} ${route.path}`));
for (const route of manifest.routes ?? []) {
  if (!routeSpecs.has(route)) fail(`Guardrail Fuzzer pack manifest missing routeSpec: ${route}`);
}

const client = readRepoFile(manifest.frontend.client);
for (const token of [
  "createGuardrailFuzzerPackClient",
  "/v1/guardrail-fuzzer/status",
  "/v1/guardrail-fuzzer/corpus",
  "/v1/guardrail-fuzzer/run",
  "/v1/guardrail-fuzzer/reports",
  "/v1/guardrail-fuzzer/evidence/",
  "method: \"POST\"",
]) {
  if (!client.includes(token)) fail(`guardrail-fuzzer-pack-client missing token: ${token}`);
}

const page = readRepoFile(manifest.frontend.page);
if (!page.includes("createGuardrailFuzzerPackClient") || page.includes('from "@/lib/api"') || page.includes("api.guardrailFuzzer")) {
  fail("Guardrail Fuzzer pack page must use guardrail-fuzzer-pack-client instead of monolithic api.ts");
}
for (const token of ["Guardrail Fuzzer", "保存 Corpus", "运行 Fuzzer", "导出证据包", "Pack shell"]) {
  if (!page.includes(token)) fail(`Guardrail Fuzzer pack page missing product token: ${token}`);
}

const frontendTest = readRepoFile("heroui-web/src/lib/__tests__/guardrail-fuzzer-pack-client.test.ts");
for (const token of ["/v1/guardrail-fuzzer/status", "/v1/guardrail-fuzzer/run", "/v1/guardrail-fuzzer/evidence/fuzz-1"]) {
  if (!frontendTest.includes(token)) fail(`Guardrail Fuzzer frontend client test missing token: ${token}`);
}

const backend = readRepoFile("internal/packs/guardrailfuzzer/handler.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_guardrail_fuzzer_pack_test.go")
  + "\n" + readRepoFile("cmd/agent/init_tasks.go")
  + "\n" + readRepoFile("cmd/agent/packruntime_bootstrap_test.go");
for (const token of [
  "const PackID = \"yunque.pack.guardrail-fuzzer\"",
  "fuzzer_ready",
  "ci_gate_ready",
  "rule_writeback_ready",
  "json-guardrail-fuzzer-evidence",
  "cfg.DataPath(\"guardrail-fuzzer\")",
  "guardrailfuzzerpack.New",
  "packs/examples/guardrail-fuzzer-pack/pack.json",
  "ensureBuiltinPacks",
  "TestGuardrailFuzzerPackGateReturnsNotFoundWhenDisabled",
  "StatusMethodNotAllowed",
]) {
  if (!backend.includes(token)) fail(`Guardrail Fuzzer backend pack or gate missing token: ${token}`);
}

const sdk = readRepoFile("sdk/typescript/src/guardrail-fuzzer.ts") + "\n" + readRepoFile("sdk/typescript/src/guardrail-fuzzer.test.ts");
for (const token of [
  "createGuardrailFuzzerClient",
  "GuardrailFuzzerClientError",
  "/v1/guardrail-fuzzer/status",
  "/v1/guardrail-fuzzer/run",
  "/v1/guardrail-fuzzer/evidence/",
  "Guardrail Fuzzer request failed",
]) {
  if (!sdk.includes(token)) fail(`TypeScript Guardrail Fuzzer SDK slice missing token: ${token}`);
}

const pkg = JSON.parse(readRepoFile("sdk/typescript/package.json") || "{}");
if (pkg.exports?.["./guardrail-fuzzer"]?.import !== "./src/guardrail-fuzzer.ts") fail("yunque-client/guardrail-fuzzer subpath export is missing or drifted");

const monolithicApi = readRepoFile("heroui-web/src/lib/api.ts");
for (const token of ["guardrailFuzzerStatus:", "guardrailFuzzerRun:", "guardrailFuzzerEvidence:"]) {
  if (monolithicApi.includes(token)) fail(`monolithic api.ts should not expose Guardrail Fuzzer method: ${token}`);
}

if (failures.length) {
  console.error("Guardrail Fuzzer Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Guardrail Fuzzer Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
