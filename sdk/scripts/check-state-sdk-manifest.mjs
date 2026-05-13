import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifestPath = resolve(repoRoot, "sdk/manifest/state-sdk.json");
const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
const failures = [];

function fail(message) { failures.push(message); }
function readRepoFile(path) {
  const fullPath = resolve(repoRoot, path);
  if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; }
  return readFileSync(fullPath, "utf8");
}

const requiredCapabilities = ["snapshot", "actions", "capabilities", "goals", "saveGoal", "deleteGoal", "focus", "updateFocus", "resources", "trackResource", "releaseResource"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) {
  if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
}
for (const actual of capabilityNames) {
  if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
}

const gatewayRoutes = readRepoFile("internal/controlplane/gateway/routes_tasks.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_reasoning.go");
for (const route of manifest.routes ?? []) {
  const [, path] = route.split(" ");
  if (!gatewayRoutes.includes(path)) fail(`gateway route not found for manifest route: ${route}`);
}

for (const [language, config] of Object.entries(manifest.languages ?? {})) {
  const combinedSource = (config.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const capability of requiredCapabilities) {
    if (!config.entrypoints?.[capability]) {
      fail(`${language} entrypoints missing required state capability: ${capability}`);
    }
  }
  for (const [capability, symbol] of Object.entries(config.entrypoints ?? {})) {
    if (!capabilityNames.has(capability)) {
      fail(`${language} entrypoint references unknown capability: ${capability}`);
    }
    const raw = symbol.split("#").pop().replace(/\(\).*$/, "");
    const alternatives = [raw, raw.replace(/^.*\./, ""), raw.replace(/^.*::/, "")];
    if (!alternatives.some((candidate) => candidate && combinedSource.includes(candidate))) {
      fail(`${language} implementation missing entrypoint for ${capability}: ${symbol}`);
    }
  }
  for (const doc of config.docs ?? []) {
    const text = readRepoFile(doc);
    if (!/state|State|状态/.test(text)) fail(`${language} doc ${doc} does not mention state helpers`);
  }
}

for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/SDK|State Kernel|状态层/.test(text)) fail(`overview doc ${doc} does not describe SDK state surface`);
}

if (failures.length) {
  console.error("state SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}
console.log(`state SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
