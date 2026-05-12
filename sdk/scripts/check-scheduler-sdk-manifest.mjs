import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifestPath = resolve(repoRoot, "sdk/manifest/scheduler-sdk.json");
const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
const failures = [];

function fail(message) { failures.push(message); }
function readRepoFile(path) {
  const fullPath = resolve(repoRoot, path);
  if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; }
  return readFileSync(fullPath, "utf8");
}

const requiredCapabilities = ["jobs", "add", "remove"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) {
  if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
}
for (const actual of capabilityNames) {
  if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
}

const gatewayRoutes = readRepoFile("internal/controlplane/gateway/schedulerapi/handler.go");
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
      fail(`${language} entrypoints missing required scheduler capability: ${capability}`);
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
    if (!/scheduler|Scheduler|调度器|定时/.test(text)) {
      fail(`${language} doc ${doc} does not mention scheduler helpers`);
    }
  }
}

for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/Scheduler|scheduler|调度器|定时任务/.test(text)) {
    fail(`overview doc ${doc} does not describe scheduler SDK surface`);
  }
}

if (failures.length) {
  console.error("scheduler SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}
console.log(`scheduler SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
