import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/system-sdk.json"), "utf8"));
const failures = [];
function fail(message) { failures.push(message); }
function readRepoFile(path) { const fullPath = resolve(repoRoot, path); if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; } return readFileSync(fullPath, "utf8"); }
const requiredCapabilities = ["health", "livez", "readyz", "cognitiveHealth", "version", "info", "stats", "metrics", "metricsPrometheus", "cacheStats", "modules", "sbom"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
for (const actual of capabilityNames) if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
const routes = readRepoFile("internal/controlplane/gateway/routes_system.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_health.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_admin.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_services.go");
for (const route of ['"/healthz"', '"/livez"', '"/readyz"', '"/healthz/cognitive"', '"/v1/version"', '"/v1/system/info"', '"/v1/system/stats"', '"/v1/metrics"', '"/v1/metrics/prometheus"', '"/v1/cache/stats"', '"/v1/modules"', '"/sbom"']) if (!routes.includes(route)) fail(`gateway route not found: ${route}`);
for (const handler of ["handleLivez", "handleReadyz", "handleCognitiveHealth", "handleSystemInfo", "handleSystemStats", "handleMetrics", "handleMetricsPrometheus", "handleCacheStats", "handleModules"]) if (!routes.includes(handler)) fail(`gateway handler not found: ${handler}`);
function symbolAlternatives(symbol) { const raw = symbol.split("#").pop().replace(/\(\).*$/, ""); const tail = raw.replace(/^.*\./, "").replace(/^.*::/, ""); const snake = tail.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`).replace(/^_/, ""); return [raw, tail, tail.replace(/^[A-Z]/, (c) => c.toLowerCase()), tail.replace(/^[a-z]/, (c) => c.toUpperCase()), snake].filter(Boolean); }
for (const [language, config] of Object.entries(manifest.languages ?? {})) {
  const combinedSource = (config.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const capability of requiredCapabilities) if (!config.entrypoints?.[capability]) fail(`${language} entrypoints missing required system capability: ${capability}`);
  for (const [capability, symbol] of Object.entries(config.entrypoints ?? {})) {
    if (!capabilityNames.has(capability)) fail(`${language} entrypoint references unknown system capability: ${capability}`);
    if (!symbolAlternatives(symbol).some((candidate) => combinedSource.includes(candidate))) fail(`${language} implementation missing entrypoint for ${capability}: ${symbol}`);
  }
  for (const doc of config.docs ?? []) {
    const text = readRepoFile(doc);
    if (!/System SDK|health\/readiness|version\/SBOM|metrics|cache stats|module observability|v1\/system|healthz/i.test(text)) fail(`${language} doc ${doc} does not mention System SDK helpers`);
  }
}
for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/System SDK|health\/readiness|version\/SBOM|metrics|cache stats|module observability|v1\/system|healthz/i.test(text)) fail(`overview doc ${doc} does not describe System SDK surface`);
}
if (failures.length) { console.error("System SDK manifest check failed:"); for (const failure of failures) console.error(`- ${failure}`); process.exit(1); }
console.log(`System SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
