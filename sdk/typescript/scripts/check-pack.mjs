import { spawnSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";

const baseUnpackedSize = 1_203_000;
const baseExportedFiles = 137;
const baseManifestCapabilities = 12;
const basePackSdkHelperExports = 0;
const maxUnpackedGrowthPerExport = 2_500;
const maxUnpackedGrowthPerManifestCapability = 1_000;
const maxUnpackedGrowthPerMemoryTimeTravelCapability = 2_000;
const maxUnpackedGrowthPerPackSdkHelperExport = 700;
const maxUnpackedGrowthForPackPrepareSummaryHelperExport = 2_800;
const maxNonEntryFiles = 16;
const pkg = JSON.parse(readFileSync("package.json", "utf8"));
const packManifestPath = "../manifest/packs-sdk.json";
const packManifest = existsSync(packManifestPath)
  ? JSON.parse(readFileSync(packManifestPath, "utf8"))
  : { capabilities: [] };
const memoryTimeTravelManifestPath = "../manifest/memory-time-travel-pack-sdk.json";
const memoryTimeTravelManifest = existsSync(memoryTimeTravelManifestPath)
  ? JSON.parse(readFileSync(memoryTimeTravelManifestPath, "utf8"))
  : { capabilities: [] };
const packsSource = existsSync("src/packs.ts") ? readFileSync("src/packs.ts", "utf8") : "";

if (pkg.sideEffects !== false) {
  console.error("package.json must declare sideEffects=false so bundlers can tree-shake unused SDK slices");
  process.exit(1);
}

const result = process.platform === "win32"
  ? spawnSync("npm pack --dry-run --json", {
      encoding: "utf8",
      shell: true,
    })
  : spawnSync("npm", ["pack", "--dry-run", "--json"], {
      encoding: "utf8",
      shell: false,
    });

if (result.status !== 0) {
  process.stderr.write(result.stderr || result.stdout || result.error?.message || "npm pack failed");
  process.exit(result.status ?? 1);
}

const packs = JSON.parse(result.stdout);
const pack = packs[0];
const files = Array.isArray(pack?.files) ? pack.files : [];
const paths = files.map((file) => String(file.path || ""));
const testFiles = paths.filter((file) => /\.test\.ts$/.test(file));
const exportedFiles = new Set();
for (const entry of Object.values(pkg.exports ?? {})) {
  if (!entry || typeof entry !== "object") continue;
  for (const value of Object.values(entry)) {
    if (typeof value === "string" && value.startsWith("./src/")) {
      exportedFiles.add(value.slice(2));
    }
  }
}
const requiredFiles = [
  "src/sdk.gen.ts",
  "src/types.gen.ts",
  "README.md",
  "package.json",
  ...[...exportedFiles].sort(),
];
// The SDK intentionally grows by one published source file for each incremental
// subpath. Keep the pack gate strict for unexpected extras, but scale the file
// budget with declared exports so adding a legitimate slice does not require
// repeatedly bumping a hard-coded total.
const maxEntryCount = requiredFiles.length + maxNonEntryFiles;
const forbiddenPatterns = [
  /^scripts\//,
  /^node_modules\//,
  /^\.tmp\//,
  /^openapi-ts-error-.*\.log$/,
  /^src\/.*\.test\.ts$/,
];

if (testFiles.length > 0) {
  console.error(`pack contains test files:\n${testFiles.join("\n")}`);
  process.exit(1);
}

const missingRequiredFiles = requiredFiles.filter((file) => !paths.includes(file));

if (missingRequiredFiles.length > 0) {
  console.error(`pack is missing required SDK entry files:\n${missingRequiredFiles.join("\n")}`);
  process.exit(1);
}

const forbiddenFiles = paths.filter((file) => forbiddenPatterns.some((pattern) => pattern.test(file)));

if (forbiddenFiles.length > 0) {
  console.error(`pack contains forbidden development artifacts:\n${forbiddenFiles.join("\n")}`);
  process.exit(1);
}

// A few SDK surfaces intentionally grow an existing subpath instead of adding a
// new export. Tie that budget to the checked manifest so real feature growth is
// explicit and reviewable instead of hiding behind a broad global size bump.
const manifestCapabilityCount = Array.isArray(packManifest.capabilities)
  ? packManifest.capabilities.length
  : 0;
const memoryTimeTravelCapabilityCount = Array.isArray(memoryTimeTravelManifest.capabilities)
  ? memoryTimeTravelManifest.capabilities.length
  : 0;
const packSdkHelperExports = (packsSource.match(/export function (summarizeCatalogSourceReports|hasCatalogSourceIssues)\b/g) ?? []).length;
const packSdkPrepareSummaryHelperExports = (packsSource.match(/export function summarizeCapabilityPrepare\b/g) ?? []).length;
const maxUnpackedSize = baseUnpackedSize
  + Math.max(0, exportedFiles.size - baseExportedFiles) * maxUnpackedGrowthPerExport
  + Math.max(0, manifestCapabilityCount - baseManifestCapabilities) * maxUnpackedGrowthPerManifestCapability
  + Math.max(0, memoryTimeTravelCapabilityCount - 20) * maxUnpackedGrowthPerMemoryTimeTravelCapability
  + Math.max(0, packSdkHelperExports - basePackSdkHelperExports) * maxUnpackedGrowthPerPackSdkHelperExport
  + packSdkPrepareSummaryHelperExports * maxUnpackedGrowthForPackPrepareSummaryHelperExport;
if (pack.unpackedSize > maxUnpackedSize) {
  console.error(`pack unpacked size ${pack.unpackedSize} exceeds dynamic budget ${maxUnpackedSize}`);
  process.exit(1);
}

if (pack.entryCount > maxEntryCount) {
  console.error(`pack entry count ${pack.entryCount} exceeds dynamic budget ${maxEntryCount} (${requiredFiles.length} required files + ${maxNonEntryFiles} package overhead files)`);
  process.exit(1);
}

console.log(`pack check ok: ${pack.entryCount}/${maxEntryCount} files, ${pack.unpackedSize}/${maxUnpackedSize} bytes unpacked, ${pack.size} bytes tarball, ${exportedFiles.size} exported entry files verified`);
