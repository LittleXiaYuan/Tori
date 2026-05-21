import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const webRoot = resolve(import.meta.dirname, "..");
const failures = [];

function fail(message) {
  failures.push(message);
}

function read(path) {
  const fullPath = resolve(webRoot, path);
  if (!existsSync(fullPath)) {
    fail(`missing file: ${path}`);
    return "";
  }
  return readFileSync(fullPath, "utf8");
}

const sdkBackedAdapters = [
  ["src/lib/pack-types.ts", "yunque-client/packs"],
  ["src/lib/packs-client.ts", "yunque-client/packs"],
  ["src/lib/wasm-plugin-pack-client.ts", "yunque-client/wasm-plugin"],
  ["src/lib/memory-time-travel-pack-client.ts", "yunque-client/memory-time-travel"],
  ["src/lib/sbom-drift-pack-client.ts", "yunque-client/sbom-drift"],
];

const directSdkConsumers = [
  ["src/lib/pack-sync.tsx", "yunque-client/packs"],
  ["src/app/packs/page.tsx", "yunque-client/packs"],
  ["src/components/cherry/settings-modal.tsx", "yunque-client/packs"],
  ["src/app/packs/wasm-plugin/page.tsx", "yunque-client/wasm-plugin"],
  ["src/app/packs/memory-time-travel/page.tsx", "yunque-client/memory-time-travel"],
  ["src/app/packs/sbom-drift/page.tsx", "yunque-client/sbom-drift"],
];

for (const [path, sdkSubpath] of sdkBackedAdapters) {
  const text = read(path);
  if (!text.includes(`"${sdkSubpath}"`) && !text.includes(`'${sdkSubpath}'`)) {
    fail(`${path} must consume ${sdkSubpath} as its SDK source of truth`);
  }
  if (/import\s+\{[^}]*\bfetcher\b[^}]*\}\s+from\s+["']\.\/api-core["']/.test(text) || /\bfetcher\s*</.test(text)) {
    fail(`${path} must not re-implement HTTP transport with api-core.fetcher`);
  }
}

for (const [path, sdkSubpath] of directSdkConsumers) {
  const text = read(path);
  if (!text.includes(`"${sdkSubpath}"`) && !text.includes(`'${sdkSubpath}'`)) {
    fail(`${path} must import ${sdkSubpath} directly`);
  }
}

const packTypes = read("src/lib/pack-types.ts");
if (/from\s+["']\.\/api-types/.test(packTypes) || /interface\s+Pack[A-Z]/.test(packTypes)) {
  fail("src/lib/pack-types.ts must stay a re-export-only SDK adapter");
}

const apiFacade = read("src/lib/api.ts");
if (/["'`]\/v1\/packs(?:\/|["'`?])/.test(apiFacade)) {
  fail("src/lib/api.ts facade is frozen for pack-domain methods; use yunque-client/packs instead");
}

if (failures.length > 0) {
  console.error("SDK boundary check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`SDK boundary check ok: ${sdkBackedAdapters.length} SDK-backed adapters`);
