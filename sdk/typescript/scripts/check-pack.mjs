import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";

const maxUnpackedSize = 1_200_000;
const maxEntryCount = 100;
const pkg = JSON.parse(readFileSync("package.json", "utf8"));

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

if (pack.unpackedSize > maxUnpackedSize) {
  console.error(`pack unpacked size ${pack.unpackedSize} exceeds ${maxUnpackedSize}`);
  process.exit(1);
}

if (pack.entryCount > maxEntryCount) {
  console.error(`pack entry count ${pack.entryCount} exceeds ${maxEntryCount}`);
  process.exit(1);
}

console.log(`pack check ok: ${pack.entryCount} files, ${pack.unpackedSize} bytes unpacked, ${pack.size} bytes tarball, ${exportedFiles.size} exported entry files verified`);
