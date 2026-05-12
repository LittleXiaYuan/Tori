import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifestPath = resolve(repoRoot, "sdk/manifest/agent-kit-sdk.json");
const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
const failures = [];

function fail(message) { failures.push(message); }
function readRepoFile(path) {
  const fullPath = resolve(repoRoot, path);
  if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; }
  return readFileSync(fullPath, "utf8");
}

const requiredCapabilities = ["state", "reflect", "missions", "scheduler", "cron", "triggers", "memory", "graph", "plugin"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) {
  if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
}
for (const actual of capabilityNames) {
  if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
}

const requiredLanguages = ["typescript", "go", "python", "rust"];
const languages = manifest.languages ?? {};
for (const language of requiredLanguages) {
  if (!languages[language]) fail(`manifest missing language: ${language}`);
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

for (const [language, config] of Object.entries(languages)) {
  const combinedSource = (config.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const required of ["create", ...requiredCapabilities]) {
    if (!config.entrypoints?.[required]) {
      fail(`${language} entrypoints missing required Agent Kit entrypoint: ${required}`);
    }
  }
  for (const [capability, symbol] of Object.entries(config.entrypoints ?? {})) {
    if (!["create", ...requiredCapabilities].includes(capability)) {
      fail(`${language} entrypoint references unknown Agent Kit capability: ${capability}`);
    }
    if (!symbolAlternatives(symbol).some((candidate) => combinedSource.includes(candidate))) {
      fail(`${language} implementation missing Agent Kit entrypoint for ${capability}: ${symbol}`);
    }
  }
  const docs = (config.docs ?? []).map(readRepoFile).join("\n");
  for (const token of ["Agent Kit", "State", "Reflect", "Mission", "Scheduler", "Cron", "Trigger", "Memory", "Graph", "Plugin"]) {
    if (!docs.includes(token)) fail(`${language} docs missing Agent Kit token: ${token}`);
  }
}

for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/Agent Kit|agent-kit|createAgentKit|NewAgentKit|create_agent_kit/.test(text)) {
    fail(`overview doc ${doc} does not describe Agent Kit SDK surface`);
  }
}

if (failures.length) {
  console.error("agent kit SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}
console.log(`agent kit SDK manifest ok: ${Object.keys(languages).length} languages, ${capabilityNames.size} capabilities`);
