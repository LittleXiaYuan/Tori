import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/cognis-sdk.json"), "utf8"));
const failures = [];
function fail(message) { failures.push(message); }
function readRepoFile(path) { const fullPath = resolve(repoRoot, path); if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; } return readFileSync(fullPath, "utf8"); }

const requiredCapabilities = ["list", "create", "get", "remove", "enable", "disable", "reload", "traces", "trace", "stats", "health", "verify", "alerts", "scanAlerts", "generate", "exportBundle", "importBundle", "workflows", "runWorkflow", "experience", "recordExperience", "confirmExperiencePattern", "evolve", "evolution", "federation", "federationPeers", "discoverFederation", "expose", "unexpose", "economics"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
for (const actual of capabilityNames) if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);

const gatewayRoutes = readRepoFile("internal/controlplane/gateway/handlers_cogni.go") + "\n" + readRepoFile("internal/controlplane/gateway/routes_system.go");
for (const route of manifest.routes ?? []) {
  const [, rawPath] = route.split(" ");
  if (rawPath.includes("/experience/record") && gatewayRoutes.includes("cogniExperienceRecord")) continue;
  if (rawPath.includes("/experience/patterns/") && gatewayRoutes.includes("cogniExperiencePatternRoute")) continue;
  const normalized = rawPath.replaceAll("{id}", "").replaceAll("{name}", "").replaceAll("{patternId}", "").replaceAll("//", "/");
  const candidates = [rawPath, normalized, rawPath.replace("/v1/cognis/{id}", ""), rawPath.replace("/v1/cognis/{id}/", ""), rawPath.replace("/{name}", ""), rawPath.replace("/{patternId}", ""), rawPath.replace("/patterns/{patternId}/confirm", "/patterns/")].filter(Boolean);
  if (!candidates.some((path) => gatewayRoutes.includes(path) || gatewayRoutes.includes(path.replace("/v1/cognis/", "")))) fail(`gateway route not found for manifest route: ${route}`);
}

function symbolAlternatives(symbol) {
  const raw = symbol.split("#").pop().replace(/\(\).*$/, "");
  const tail = raw.replace(/^.*\./, "").replace(/^.*::/, "");
  const snake = tail.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`).replace(/^_/, "");
  return [raw, tail, tail.replace(/^[A-Z]/, (c) => c.toLowerCase()), tail.replace(/^[a-z]/, (c) => c.toUpperCase()), snake].filter(Boolean);
}

for (const [language, config] of Object.entries(manifest.languages ?? {})) {
  const combinedSource = (config.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const capability of requiredCapabilities) if (!config.entrypoints?.[capability]) fail(`${language} entrypoints missing required cognis capability: ${capability}`);
  for (const [capability, symbol] of Object.entries(config.entrypoints ?? {})) {
    if (!capabilityNames.has(capability)) fail(`${language} entrypoint references unknown cognis capability: ${capability}`);
    if (!symbolAlternatives(symbol).some((candidate) => combinedSource.includes(candidate))) fail(`${language} implementation missing entrypoint for ${capability}: ${symbol}`);
  }
  for (const doc of config.docs ?? []) {
    const text = readRepoFile(doc);
    if (!/Cogni|Cognis|cogni|cognis|Cognitive|federation|evolution/.test(text)) fail(`${language} doc ${doc} does not mention Cognis helpers`);
  }
}
for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/Cognis SDK|Cogni|cognis|evolution|federation/.test(text)) fail(`overview doc ${doc} does not describe Cognis SDK surface`);
}
if (failures.length) { console.error("Cognis SDK manifest check failed:"); for (const failure of failures) console.error(`- ${failure}`); process.exit(1); }
console.log(`Cognis SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
