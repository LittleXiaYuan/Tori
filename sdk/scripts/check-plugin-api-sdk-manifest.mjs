import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifestPath = resolve(repoRoot, "sdk/manifest/plugin-api-sdk.json");
const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
const failures = [];

function fail(message) { failures.push(message); }
function readRepoFile(path) {
  const fullPath = resolve(repoRoot, path);
  if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; }
  return readFileSync(fullPath, "utf8");
}

const requiredCapabilities = [
  "llm", "search", "send",
  "memoryGet", "memorySet", "memoryDelete", "memoryList", "memorySearch",
  "agentMemorySearch", "agentMemoryAdd",
  "knowledgeSearch", "knowledgeIngest",
  "cronAdd", "cronRemove", "cronList",
  "registerProvider", "registerChannel", "registerSearch", "registerGuardrail",
  "registerEmbedding", "registerSpeech", "extensions",
];

const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) {
  if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
}
for (const actual of capabilityNames) {
  if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
}

const gatewayRoutes = readRepoFile("internal/controlplane/gateway/handlers_plugins.go");
for (const route of manifest.routes ?? []) {
  const [, path] = route.split(" ");
  if (!gatewayRoutes.includes(path)) fail(`gateway route not found for manifest route: ${route}`);
}

function symbolAlternatives(symbol) {
  const raw = symbol.split("#").pop().replace(/\(\).*$/, "");
  return [
    raw,
    raw.replace(/^.*\./, ""),
    raw.replace(/^.*::/, ""),
    raw.replace(/^.*\./, "").replace(/^[A-Z]/, (c) => c.toLowerCase()),
  ].filter(Boolean);
}

for (const [language, config] of Object.entries(manifest.languages ?? {})) {
  const combinedSource = (config.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const [capability, symbol] of Object.entries(config.entrypoints ?? {})) {
    if (!capabilityNames.has(capability)) {
      fail(`${language} entrypoint references unknown capability: ${capability}`);
    }
    if (!symbolAlternatives(symbol).some((candidate) => combinedSource.includes(candidate))) {
      fail(`${language} implementation missing entrypoint for ${capability}: ${symbol}`);
    }
  }
  for (const doc of config.docs ?? []) {
    const text = readRepoFile(doc);
    if (!/plugin|Plugin|插件|SDK/.test(text)) {
      fail(`${language} doc ${doc} does not mention plugin SDK helpers`);
    }
  }
}

const rust = manifest.languages?.rust;
if (!rust) {
  fail("manifest missing rust SDK language section");
} else {
  const rustEntrypoints = rust.entrypoints ?? {};
  const rustSource = (rust.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const capability of requiredCapabilities) {
    const symbol = rustEntrypoints[capability];
    if (!symbol) {
      fail(`rust entrypoints missing required Plugin API capability: ${capability}`);
      continue;
    }
    const method = symbol.replace(/^.*::/, "");
    const methodPattern = new RegExp(`pub\\s+async\\s+fn\\s+${method}\\s*\\(`);
    if (!methodPattern.test(rustSource)) {
      fail(`rust PluginApiClient missing public async method for ${capability}: ${method}`);
    }
  }
  const rustDocs = (rust.docs ?? []).map(readRepoFile).join("\n");
  for (const token of [
    "agent_memory_search",
    "agent_memory_add",
    "register_provider",
    "register_channel",
    "register_search",
    "register_guardrail",
    "register_embedding",
    "register_speech",
    "extensions",
  ]) {
    if (!rustDocs.includes(token)) {
      fail(`rust docs missing PluginApiClient method mention: ${token}`);
    }
  }
}

for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/plugin|Plugin|插件/.test(text)) {
    fail(`overview doc ${doc} does not describe plugin SDK surface`);
  }
}

if (failures.length) {
  console.error("plugin API SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}
console.log(`plugin API SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
