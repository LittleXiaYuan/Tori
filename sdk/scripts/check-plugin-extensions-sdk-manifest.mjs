import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/plugin-extensions-sdk.json"), "utf8"));
const failures = [];
function fail(message) { failures.push(message); }
function readRepoFile(path) { const fullPath = resolve(repoRoot, path); if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; } return readFileSync(fullPath, "utf8"); }
const requiredCapabilities = ["registerProvider", "registerChannel", "registerSearch", "registerGuardrail", "registerEmbedding", "registerSpeech", "list"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
for (const actual of capabilityNames) if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
const gatewayRoutes = readRepoFile("internal/controlplane/gateway/routes.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_plugins.go");
for (const route of ['"/v1/plugin-api/register/provider"', 'handleRegisterProvider', '"/v1/plugin-api/register/channel"', 'handleRegisterChannel', '"/v1/plugin-api/register/search"', 'handleRegisterSearch', '"/v1/plugin-api/register/guardrail"', 'handleRegisterGuardrail', '"/v1/plugin-api/register/embedding"', 'handleRegisterEmbedding', '"/v1/plugin-api/register/speech"', 'handleRegisterSpeech', '"/v1/plugin-api/extensions"', 'handleListExtensions']) if (!gatewayRoutes.includes(route)) fail(`plugin extensions route/handler not found: ${route}`);
function symbolAlternatives(symbol) { const raw = symbol.split("#").pop().replace(/\(\).*$/, ""); const tail = raw.replace(/^.*\./, "").replace(/^.*::/, ""); const snake = tail.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`).replace(/^_/, ""); return [raw, tail, tail.replace(/^[A-Z]/, (c) => c.toLowerCase()), tail.replace(/^[a-z]/, (c) => c.toUpperCase()), snake].filter(Boolean); }
for (const [language, config] of Object.entries(manifest.languages ?? {})) {
  const combinedSource = (config.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const capability of requiredCapabilities) if (!config.entrypoints?.[capability]) fail(`${language} entrypoints missing required PluginExtensions capability: ${capability}`);
  for (const [capability, symbol] of Object.entries(config.entrypoints ?? {})) {
    if (!capabilityNames.has(capability)) fail(`${language} entrypoint references unknown PluginExtensions capability: ${capability}`);
    if (!symbolAlternatives(symbol).some((candidate) => combinedSource.includes(candidate))) fail(`${language} implementation missing entrypoint for ${capability}: ${symbol}`);
  }
  for (const doc of config.docs ?? []) {
    const text = readRepoFile(doc);
    if (!/PluginExtensions SDK|plugin-contributed system extensions|\/v1\/plugin-api\/register\/provider/i.test(text)) fail(`${language} doc ${doc} does not mention PluginExtensions SDK helpers`);
  }
}
for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/PluginExtensions SDK|plugin-contributed system extensions|\/v1\/plugin-api\/register\/provider/i.test(text)) fail(`overview doc ${doc} does not describe PluginExtensions SDK surface`);
}
if (failures.length) { console.error("PluginExtensions SDK manifest check failed:"); for (const failure of failures) console.error(`- ${failure}`); process.exit(1); }
console.log(`PluginExtensions SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
