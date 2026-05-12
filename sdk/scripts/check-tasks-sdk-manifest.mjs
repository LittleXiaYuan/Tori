import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/tasks-sdk.json"), "utf8"));
const failures = [];
function fail(message) { failures.push(message); }
function readRepoFile(path) { const fullPath = resolve(repoRoot, path); if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; } return readFileSync(fullPath, "utf8"); }
const requiredCapabilities = ["list", "get", "create", "run", "pause", "resume", "restart", "cancel", "delete", "templates", "template", "createTemplate", "deleteTemplate", "instantiateTemplate", "gaps", "gapStats", "resolveGap", "workingMemory", "threads", "thread", "postThreadMessage", "updateThreadState"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
for (const actual of capabilityNames) if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
const gatewayRoutes = readRepoFile("internal/controlplane/gateway/routes_tasks.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_tasks.go");
for (const route of ['"/v1/tasks"', '"/v1/tasks/run"', '"/v1/tasks/pause"', '"/v1/tasks/resume"', '"/v1/tasks/restart"', '"/v1/tasks/cancel"', '"/v1/tasks/templates"', '"/v1/tasks/templates/instantiate"', '"/v1/tasks/gaps"', '"/v1/tasks/gaps/resolve"', '"/v1/tasks/memory"', '"/v1/tasks/threads"']) if (!gatewayRoutes.includes(route)) fail(`gateway route not found: ${route}`);
for (const handler of ["handleTaskList", "handleTaskCreate", "handleTaskRun", "handleTaskPause", "handleTaskResume", "handleTaskRestart", "handleTaskCancel", "handleTaskDelete", "handleTemplates", "handleTemplateInstantiate", "handleGaps", "handleGapResolve", "handleTaskWorkingMemory", "handleTaskThread"]) if (!gatewayRoutes.includes(handler)) fail(`gateway handler not found: ${handler}`);
function symbolAlternatives(symbol) { const raw = symbol.split("#").pop().replace(/\(\).*$/, ""); const tail = raw.replace(/^.*\./, "").replace(/^.*::/, ""); const snake = tail.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`).replace(/^_/, ""); return [raw, tail, tail.replace(/^[A-Z]/, (c) => c.toLowerCase()), tail.replace(/^[a-z]/, (c) => c.toUpperCase()), snake].filter(Boolean); }
for (const [language, config] of Object.entries(manifest.languages ?? {})) {
  const combinedSource = (config.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const capability of requiredCapabilities) if (!config.entrypoints?.[capability]) fail(`${language} entrypoints missing required tasks capability: ${capability}`);
  for (const [capability, symbol] of Object.entries(config.entrypoints ?? {})) {
    if (!capabilityNames.has(capability)) fail(`${language} entrypoint references unknown tasks capability: ${capability}`);
    if (!symbolAlternatives(symbol).some((candidate) => combinedSource.includes(candidate))) fail(`${language} implementation missing entrypoint for ${capability}: ${symbol}`);
  }
  for (const doc of config.docs ?? []) {
    const text = readRepoFile(doc);
    if (!/Tasks SDK|task CRUD|task templates|task gaps|working memory|task threads|task pages|v1\/tasks/i.test(text)) fail(`${language} doc ${doc} does not mention Tasks helpers`);
  }
}
for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/Tasks SDK|task CRUD|task templates|task gaps|working memory|task threads|task pages|v1\/tasks/i.test(text)) fail(`overview doc ${doc} does not describe Tasks SDK surface`);
}
if (failures.length) { console.error("Tasks SDK manifest check failed:"); for (const failure of failures) console.error(`- ${failure}`); process.exit(1); }
console.log(`Tasks SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);