import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifestPath = resolve(repoRoot, "sdk/manifest/triggers-sdk.json");
const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
const failures = [];

function fail(message) { failures.push(message); }
function readRepoFile(path) {
  const fullPath = resolve(repoRoot, path);
  if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; }
  return readFileSync(fullPath, "utf8");
}

const requiredCapabilities = ["list", "get", "create", "update", "delete", "emit", "runs", "events"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) {
  if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
}
for (const actual of capabilityNames) {
  if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
}

const gatewayRoutes = readRepoFile("internal/controlplane/gateway/handlers_automation.go") + "\n" + readRepoFile("internal/controlplane/gateway/routes.go");
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
  for (const capability of requiredCapabilities) {
    if (!config.entrypoints?.[capability]) {
      fail(`${language} entrypoints missing required trigger capability: ${capability}`);
    }
  }
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
    if (!/trigger|Trigger|触发器|触发/.test(text)) {
      fail(`${language} doc ${doc} does not mention trigger helpers`);
    }
  }
}

for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/Triggers|triggers|Trigger|触发器|触发/.test(text)) {
    fail(`overview doc ${doc} does not describe triggers SDK surface`);
  }
}

if (failures.length) {
  console.error("triggers SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}
console.log(`triggers SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
