import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/plugin-memory-sdk.json"), "utf8"));
const failures = [];
function fail(message) { failures.push(message); }
function readRepoFile(path) { const fullPath = resolve(repoRoot, path); if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; } return readFileSync(fullPath, "utf8"); }
const requiredCapabilities = ["get", "set", "delete", "list", "search"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
for (const actual of capabilityNames) if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
const gatewayRoutes = readRepoFile("internal/controlplane/gateway/routes.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_plugins.go");
for (const route of ['"/v1/plugin-api/memory/get"', 'handleMemoryGet', '"/v1/plugin-api/memory/set"', 'handleMemorySet', '"/v1/plugin-api/memory/delete"', 'handleMemoryDelete', '"/v1/plugin-api/memory/list"', 'handleMemoryList', '"/v1/plugin-api/memory/search"', 'handleMemorySearch']) if (!gatewayRoutes.includes(route)) fail(`plugin memory route/handler not found: ${route}`);
function symbolAlternatives(symbol) { const raw = symbol.split("#").pop().replace(/\(\).*$/, ""); const tail = raw.replace(/^.*\./, "").replace(/^.*::/, ""); const snake = tail.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`).replace(/^_/, ""); return [raw, tail, tail.replace(/^[A-Z]/, (c) => c.toLowerCase()), tail.replace(/^[a-z]/, (c) => c.toUpperCase()), snake].filter(Boolean); }
for (const [language, config] of Object.entries(manifest.languages ?? {})) {
  const combinedSource = (config.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const capability of requiredCapabilities) if (!config.entrypoints?.[capability]) fail(`${language} entrypoints missing required PluginMemory capability: ${capability}`);
  for (const [capability, symbol] of Object.entries(config.entrypoints ?? {})) {
    if (!capabilityNames.has(capability)) fail(`${language} entrypoint references unknown PluginMemory capability: ${capability}`);
    if (!symbolAlternatives(symbol).some((candidate) => combinedSource.includes(candidate))) fail(`${language} implementation missing entrypoint for ${capability}: ${symbol}`);
  }
  for (const doc of config.docs ?? []) {
    const text = readRepoFile(doc);
    if (!/PluginMemory SDK|plugin-private memory|\/v1\/plugin-api\/memory\/get/i.test(text)) fail(`${language} doc ${doc} does not mention PluginMemory SDK helpers`);
  }
}
for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/PluginMemory SDK|plugin-private memory|\/v1\/plugin-api\/memory\/get/i.test(text)) fail(`overview doc ${doc} does not describe PluginMemory SDK surface`);
}
if (failures.length) { console.error("PluginMemory SDK manifest check failed:"); for (const failure of failures) console.error(`- ${failure}`); process.exit(1); }
console.log(`PluginMemory SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
