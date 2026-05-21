import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";
import { spawnSync } from "child_process";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "..", "..", "..");
const WEB_DIR = path.join(ROOT, "apps", "web");
const OUT_DIR = path.join(WEB_DIR, "out");
const OUT_NEXT_DIR = path.join(OUT_DIR, "_next");
const FRONTEND_DIR = path.resolve(__dirname, "..", "src-tauri", "frontend");

const args = new Set(process.argv.slice(2));
const shouldBuild = args.has("--build");
const dryRun = args.has("--dry-run");
const help = args.has("--help") || args.has("-h");

function usage() {
  console.log(`Sync the static Next.js export used by the Tauri desktop bundle.

Usage:
  node scripts/sync-frontend.mjs             # copy apps/web/out -> src-tauri/frontend
  node scripts/sync-frontend.mjs --build     # run npm build in apps/web first
  node scripts/sync-frontend.mjs --dry-run   # validate and print planned paths

Why this exists:
  Tauri packages ./src-tauri/frontend, while the canonical UI source lives in
  ../web. Running this script prevents desktop builds from silently
  embedding stale tracked static assets.`);
}

function fail(message) {
  console.error(`sync-frontend: ${message}`);
  process.exit(1);
}

function runBuild() {
  console.log("Building apps/web static export...");
  const result = spawnSync("npm", ["run", "build"], {
    cwd: WEB_DIR,
    stdio: "inherit",
    shell: process.platform === "win32",
    env: { ...process.env, NODE_ENV: "production" },
  });
  if (result.status !== 0) {
    fail(`apps/web build failed with exit code ${result.status ?? "unknown"}`);
  }
}

function ensureExportReady() {
  if (!fs.existsSync(OUT_NEXT_DIR)) {
    fail(`static export not found at ${OUT_NEXT_DIR}. Run \`npm run build --prefix apps/web\` or pass --build.`);
  }
  const indexPath = path.join(OUT_DIR, "index.html");
  if (!fs.existsSync(indexPath)) {
    fail(`static export is incomplete: missing ${indexPath}`);
  }
}

function removeDirContents(dir) {
  fs.mkdirSync(dir, { recursive: true });
  for (const entry of fs.readdirSync(dir)) {
    fs.rmSync(path.join(dir, entry), { recursive: true, force: true });
  }
}

function copyRecursive(src, dest) {
  const stat = fs.statSync(src);
  if (stat.isDirectory()) {
    fs.mkdirSync(dest, { recursive: true });
    let count = 0;
    let bytes = 0;
    for (const entry of fs.readdirSync(src)) {
      const child = copyRecursive(path.join(src, entry), path.join(dest, entry));
      count += child.count;
      bytes += child.bytes;
    }
    return { count, bytes };
  }
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  fs.copyFileSync(src, dest);
  return { count: 1, bytes: stat.size };
}

function formatBytes(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

if (help) {
  usage();
  process.exit(0);
}

if (shouldBuild) {
  runBuild();
}

ensureExportReady();

console.log("Tauri frontend sync");
console.log(`  source: ${OUT_DIR}`);
console.log(`  target: ${FRONTEND_DIR}`);

if (dryRun) {
  console.log("  mode:   dry-run (no files copied)");
  process.exit(0);
}

removeDirContents(FRONTEND_DIR);
const { count, bytes } = copyRecursive(OUT_DIR, FRONTEND_DIR);

console.log(`Done: copied ${count} files (${formatBytes(bytes)}).`);
console.log("Next: run `npm run build --prefix apps/desktop` to package the refreshed desktop UI.");
