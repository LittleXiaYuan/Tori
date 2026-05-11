import { spawnSync } from "node:child_process";

const maxUnpackedSize = 1_200_000;
const maxEntryCount = 100;

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
const testFiles = files.map((file) => String(file.path || "")).filter((file) => /\.test\.ts$/.test(file));

if (testFiles.length > 0) {
  console.error(`pack contains test files:\n${testFiles.join("\n")}`);
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

console.log(`pack check ok: ${pack.entryCount} files, ${pack.unpackedSize} bytes unpacked, ${pack.size} bytes tarball`);
