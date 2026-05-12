import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/speech-sdk.json"), "utf8"));
const failures = [];
function fail(message) { failures.push(message); }
function readRepoFile(path) { const fullPath = resolve(repoRoot, path); if (!existsSync(fullPath)) { fail(`missing file: ${path}`); return ""; } return readFileSync(fullPath, "utf8"); }
const requiredCapabilities = ["tts", "stt", "voices", "sttStreamUrl", "upload"];
const capabilityNames = new Set((manifest.capabilities ?? []).map((cap) => cap.name));
for (const required of requiredCapabilities) if (!capabilityNames.has(required)) fail(`manifest missing capability: ${required}`);
for (const actual of capabilityNames) if (!requiredCapabilities.includes(actual)) fail(`manifest has unexpected capability: ${actual}`);
const routes = readRepoFile("internal/controlplane/gateway/routes_system.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_speech.go") + "\n" + readRepoFile("internal/controlplane/gateway/handlers_admin.go");
for (const route of ['"/v1/speech/tts"', '"/v1/speech/stt"', '"/v1/speech/stt/stream"', '"/v1/speech/voices"', '"/v1/upload"']) if (!routes.includes(route)) fail(`gateway route not found: ${route}`);
for (const handler of ["handleTTS", "handleSTT", "handleSTTStream", "handleVoices", "handleFileUpload"]) if (!routes.includes(handler)) fail(`gateway handler not found: ${handler}`);
function symbolAlternatives(symbol) { const raw = symbol.split("#").pop().replace(/\(\).*$/, ""); const tail = raw.replace(/^.*\./, "").replace(/^.*::/, ""); const snake = tail.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`).replace(/^_/, ""); return [raw, tail, tail.replace(/^[A-Z]/, (c) => c.toLowerCase()), tail.replace(/^[a-z]/, (c) => c.toUpperCase()), snake].filter(Boolean); }
for (const [language, config] of Object.entries(manifest.languages ?? {})) {
  const combinedSource = (config.implementationFiles ?? []).map(readRepoFile).join("\n");
  for (const capability of requiredCapabilities) if (!config.entrypoints?.[capability]) fail(`${language} entrypoints missing required Speech capability: ${capability}`);
  for (const [capability, symbol] of Object.entries(config.entrypoints ?? {})) {
    if (!capabilityNames.has(capability)) fail(`${language} entrypoint references unknown backup capability: ${capability}`);
    if (!symbolAlternatives(symbol).some((candidate) => combinedSource.includes(candidate))) fail(`${language} implementation missing entrypoint for ${capability}: ${symbol}`);
  }
  for (const doc of config.docs ?? []) {
    const text = readRepoFile(doc);
    if (!/Speech SDK|speech TTS|speech STT|voice|v1\/speech|v1\/upload/i.test(text)) fail(`${language} doc ${doc} does not mention Backup SDK helpers`);
  }
}
for (const doc of manifest.overviewDocs ?? []) {
  const text = readRepoFile(doc);
  if (!/Speech SDK|speech TTS|speech STT|voice|v1\/speech|v1\/upload/i.test(text)) fail(`overview doc ${doc} does not describe Backup SDK surface`);
}
if (failures.length) { console.error("Speech SDK manifest check failed:"); for (const failure of failures) console.error(`- ${failure}`); process.exit(1); }
console.log(`Speech SDK manifest ok: ${Object.keys(manifest.languages ?? {}).length} languages, ${capabilityNames.size} capabilities`);
