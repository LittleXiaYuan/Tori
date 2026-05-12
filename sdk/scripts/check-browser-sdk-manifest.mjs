import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/browser-sdk.json"), "utf8"));
const failures = [];
function fail(message) { failures.push(message); }
function readRepoFile(path) { const fullPath = resolve(repoRoot, path); if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; } return readFileSync(fullPath, "utf8"); }
const requiredCapabilities = ["status", "config", "navigate", "screenshot", "latestScreenshot", "ocr", "oppPending", "oppDecide", "extensionStatus", "extensionSession", "extensionAction", "scenarios", "runScenario"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
for (const actual of capabilityNames) if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
const gatewayRoutes = readRepoFile("internal/controlplane/gateway/routes.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_browser_opp.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_browser_ext.go");
for (const route of ['"/v1/browser/status"','"/v1/browser/config"','"/v1/browser/navigate"','"/v1/browser/screenshot"','"/v1/browser/ocr"','"/v1/browser/screenshot/latest"','"/v1/browser/opp/pending"','"/v1/browser/opp/decide"','"/api/browser/ext/status"','"/api/browser/ext/session"','"/api/browser/ext/action"','"/api/browser/ext/scenarios"','"/api/browser/ext/scenarios/run"']) if (!gatewayRoutes.includes(route)) fail(`gateway route not found: ${route}`);
function symbolAlternatives(symbol) { const raw = symbol.split("#").pop().replace(/\(\).*$/, ""); const tail = raw.replace(/^.*\./, "").replace(/^.*::/, ""); const snake = tail.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`).replace(/^_/, ""); return [raw, tail, tail.replace(/^[A-Z]/, (c) => c.toLowerCase()), tail.replace(/^[a-z]/, (c) => c.toUpperCase()), snake].filter(Boolean); }
for (const [language, config] of Object.entries(manifest.languages ?? {})) {
  const combinedSource = (config.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const capability of requiredCapabilities) if (!config.entrypoints?.[capability]) fail(`${language} entrypoints missing required Browser capability: ${capability}`);
  for (const [capability, symbol] of Object.entries(config.entrypoints ?? {})) {
    if (!capabilityNames.has(capability)) fail(`${language} entrypoint references unknown Browser capability: ${capability}`);
    if (!symbolAlternatives(symbol).some((candidate) => combinedSource.includes(candidate))) fail(`${language} implementation missing entrypoint for ${capability}: ${symbol}`);
  }
  for (const doc of config.docs ?? []) {
    const text = readRepoFile(doc);
    if (!/Browser SDK|browser|\/v1\/browser|\/api\/browser/.test(text)) fail(`${language} doc ${doc} does not mention Browser helpers`);
  }
}
for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/Browser SDK|browser|\/v1\/browser|\/api\/browser/.test(text)) fail(`overview doc ${doc} does not describe Browser SDK surface`);
}
if (failures.length) { console.error("Browser SDK manifest check failed:"); for (const failure of failures) console.error(`- ${failure}`); process.exit(1); }
console.log(`Browser SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
