/**
 * Copies the yunque-agent binary to src-tauri/binaries/ with the
 * Tauri-required target-triple suffix.
 *
 * Usage:
 *   node scripts/copy-sidecar.mjs                 # auto-detect host triple
 *   node scripts/copy-sidecar.mjs --all           # copy all platform binaries
 */
import { execSync } from "child_process";
import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "..", "..", "..");
// Tauri v2 expects externalBin entries at <src-tauri>/<path>, e.g. the
// tauri.conf.json entry "binaries/yunque-agent" points at
// <src-tauri>/binaries/yunque-agent-<triple><ext>. Keeping the binaries in a
// sub-folder also keeps the src-tauri root clean and prevents the Tauri CLI
// from confusing sidecars with the build's own executable.
const BIN_DIR = path.resolve(__dirname, "..", "src-tauri", "binaries");

const PLATFORM_MAP = {
  "windows-amd64":  { triple: "x86_64-pc-windows-msvc",     ext: ".exe" },
  "windows-arm64":  { triple: "aarch64-pc-windows-msvc",    ext: ".exe" },
  "darwin-amd64":   { triple: "x86_64-apple-darwin",         ext: ""     },
  "darwin-arm64":   { triple: "aarch64-apple-darwin",        ext: ""     },
  "linux-amd64":    { triple: "x86_64-unknown-linux-gnu",    ext: ""     },
  "linux-arm64":    { triple: "aarch64-unknown-linux-gnu",   ext: ""     },
};

function copyBinary(platformKey) {
  const { triple, ext } = PLATFORM_MAP[platformKey];
  const srcName = `yunque-agent-${platformKey}${ext}`;
  const srcPath = path.join(ROOT, srcName);

  if (!fs.existsSync(srcPath)) {
    console.warn(`  skip: ${srcName} (not found)`);
    return false;
  }

  const dstName = `yunque-agent-${triple}${ext}`;
  const dstPath = path.join(BIN_DIR, dstName);
  fs.copyFileSync(srcPath, dstPath);
  const sizeMB = (fs.statSync(dstPath).size / 1024 / 1024).toFixed(1);
  console.log(`  ok:   ${srcName} -> ${dstName} (${sizeMB} MB)`);
  return true;
}

fs.mkdirSync(BIN_DIR, { recursive: true });

const copyAll = process.argv.includes("--all");

if (copyAll) {
  console.log("Copying all platform binaries...");
  let count = 0;
  for (const key of Object.keys(PLATFORM_MAP)) {
    if (copyBinary(key)) count++;
  }
  console.log(`Done: ${count} binaries copied.`);
} else {
  const hostTriple = execSync("rustc --print host-tuple").toString().trim();
  console.log(`Host triple: ${hostTriple}`);

  const entry = Object.entries(PLATFORM_MAP).find(([, v]) => v.triple === hostTriple);
  if (!entry) {
    console.error(`Unknown host triple: ${hostTriple}`);
    process.exit(1);
  }

  console.log(`Copying binary for ${entry[0]}...`);
  if (!copyBinary(entry[0])) {
    console.error("Binary not found. Run 'make release' first to build Go binaries.");
    process.exit(1);
  }
  console.log("Done.");
}
