import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/packs-sdk.json"), "utf8"));
const failures = [];
function fail(message) { failures.push(message); }
function readRepoFile(path) { const fullPath = resolve(repoRoot, path); if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; } return readFileSync(fullPath, "utf8"); }
const requiredCapabilities = ["list", "installed", "enabled", "backendModules", "install", "enable", "disable", "rollback", "frontendSync"];
const requiredRoutes = ["GET /v1/packs", "GET /v1/packs/installed", "GET /v1/packs/enabled", "GET /v1/packs/backend-modules", "POST /v1/packs/install", "POST /v1/packs/enable", "POST /v1/packs/disable", "POST /v1/packs/rollback"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
for (const actual of capabilityNames) if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
const manifestRoutes = new Set(manifest.routes ?? []);
for (const route of requiredRoutes) if (!manifestRoutes.has(route)) fail(`manifest missing route: ${route}`);
for (const route of manifestRoutes) if (!requiredRoutes.includes(route)) fail(`manifest has unexpected route: ${route}`);
const gateway = readRepoFile("internal/controlplane/gateway/handlers_packs.go") + "\n" + readRepoFile("internal/controlplane/gateway/gateway.go") + "\n" + readRepoFile("internal/controlplane/gateway/gateway_setters.go");
for (const route of ['"/v1/packs"', '"/v1/packs/installed"', '"/v1/packs/enabled"', '"/v1/packs/backend-modules"', '"/v1/packs/install"', '"/v1/packs/enable"', '"/v1/packs/disable"', '"/v1/packs/rollback"']) if (!gateway.includes(route)) fail(`gateway route not found: ${route}`);
for (const handler of ["handlePacksList", "handlePacksEnabled", "handlePackBackendModules", "handlePackInstall", "handlePackEnable", "handlePackDisable", "handlePackRollback", "RegisterBackendPack", "registerBackendPack", "requirePackRoute"]) if (!gateway.includes(handler)) fail(`pack runtime gateway contract not found: ${handler}`);
const packContract = readRepoFile("pkg/packruntime/manifest.go") + "\n" + readRepoFile("pkg/packruntime/registry.go") + "\n" + readRepoFile("pkg/packruntime/backend.go") + "\n" + readRepoFile("packs/AUTHORING.md");
for (const token of ["Manifest", "Registry", "BackendModule", "BackendRoute", "BackendModuleInfo", "BackendRouteInfo", "frontend", "menus", "routes", "sdk", "rollback"]) if (!packContract.includes(token)) fail(`pack runtime contract missing token: ${token}`);
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
    if (!/Pack Runtime SDK|Packs SDK|createPacksClient|frontendSync|backend-modules|v1\/packs/i.test(text)) fail(`${language} doc ${doc} does not mention Packs SDK helpers`);
  }
}
for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/Pack Runtime SDK|Packs SDK|frontendSync|backend-modules|v1\/packs/i.test(text)) fail(`overview doc ${doc} does not describe Packs SDK surface`);
}
if (!manifest.languages?.typescript) fail("manifest must expose a TypeScript Packs SDK implementation");
if (failures.length) { console.error("Packs SDK manifest check failed:"); for (const failure of failures) console.error(`- ${failure}`); process.exit(1); }
console.log(`Packs SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
