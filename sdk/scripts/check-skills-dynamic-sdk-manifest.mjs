import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/skills-dynamic-sdk.json"), "utf8"));
const failures = [];
function fail(message) { failures.push(message); }
function readRepoFile(path) { const fullPath = resolve(repoRoot, path); if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; } return readFileSync(fullPath, "utf8"); }
const requiredCapabilities = ["list", "approve", "reject"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
for (const actual of capabilityNames) if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
const gatewayRoutes = readRepoFile("internal/controlplane/gateway/routes.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_admin_skills.go");
for (const route of ['"/v1/skills/dynamic"', '"/v1/skills/approve"', '"/v1/skills/reject"', "handleSkillsDynamicGet", "handleSkillsDynamicApprove", "handleSkillsDynamicReject"]) if (!gatewayRoutes.includes(route)) fail(`skills dynamic route/handler not found: ${route}`);
function symbolAlternatives(symbol) { const raw = symbol.split("#").pop().replace(/\(\).*$/, ""); const tail = raw.replace(/^.*\./, "").replace(/^.*::/, ""); const snake = tail.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`).replace(/^_/, ""); return [raw, tail, tail.replace(/^[A-Z]/, (c) => c.toLowerCase()), tail.replace(/^[a-z]/, (c) => c.toUpperCase()), snake].filter(Boolean); }
for (const [language, config] of Object.entries(manifest.languages ?? {})) {
  const combinedSource = (config.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const capability of requiredCapabilities) if (!config.entrypoints?.[capability]) fail(`${language} entrypoints missing required SkillsDynamic capability: ${capability}`);
  for (const [capability, symbol] of Object.entries(config.entrypoints ?? {})) {
    if (!capabilityNames.has(capability)) fail(`${language} entrypoint references unknown SkillsDynamic capability: ${capability}`);
    if (!symbolAlternatives(symbol).some((candidate) => combinedSource.includes(candidate))) fail(`${language} implementation missing entrypoint for ${capability}: ${symbol}`);
  }
  for (const doc of config.docs ?? []) {
    const text = readRepoFile(doc);
    if (!/SkillsDynamic SDK|dynamic skill review|\/v1\/skills\/dynamic|\/v1\/skills\/approve|\/v1\/skills\/reject/i.test(text)) fail(`${language} doc ${doc} does not mention SkillsDynamic SDK helpers`);
  }
}
for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/SkillsDynamic SDK|dynamic skill review|\/v1\/skills\/dynamic|\/v1\/skills\/approve|\/v1\/skills\/reject/i.test(text)) fail(`overview doc ${doc} does not describe SkillsDynamic SDK surface`);
}
if (failures.length) { console.error("SkillsDynamic SDK manifest check failed:"); for (const failure of failures) console.error(`- ${failure}`); process.exit(1); }
console.log(`SkillsDynamic SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
