import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "..");

const checks = [
  {
    label: "Tauri source directory",
    path: "apps/desktop/src-tauri",
    kind: "dir",
  },
  {
    label: "Tauri config",
    path: "apps/desktop/src-tauri/tauri.conf.json",
    kind: "file",
    includes: ["externalBin", "binaries/yunque-agent"],
  },
  {
    label: "Desktop package script",
    path: "apps/desktop/package.json",
    kind: "file",
    includes: ["copy-sidecar:all", "sync-frontend:build", "tauri build"],
  },
  {
    label: "Sidecar copy script",
    path: "apps/desktop/scripts/copy-sidecar.mjs",
    kind: "file",
    includes: ["windows-amd64", "darwin-amd64", "linux-amd64", "x86_64-pc-windows-msvc"],
  },
  {
    label: "Desktop release workflow",
    path: ".github/workflows/desktop-release.yml",
    kind: "file",
    includes: ["windows-latest", "macos-13", "ubuntu-latest", "actions/upload-artifact@v4", "npm run build"],
  },
];

let failed = false;
for (const check of checks) {
  const abs = path.join(ROOT, check.path);
  if (!fs.existsSync(abs)) {
    console.error(`desktop release check failed: missing ${check.label}: ${check.path}`);
    failed = true;
    continue;
  }
  const stat = fs.statSync(abs);
  if (check.kind === "dir" && !stat.isDirectory()) {
    console.error(`desktop release check failed: ${check.path} is not a directory`);
    failed = true;
  }
  if (check.kind === "file") {
    if (!stat.isFile()) {
      console.error(`desktop release check failed: ${check.path} is not a file`);
      failed = true;
      continue;
    }
    const text = fs.readFileSync(abs, "utf8");
    for (const token of check.includes ?? []) {
      if (!text.includes(token)) {
        console.error(`desktop release check failed: ${check.path} missing token ${JSON.stringify(token)}`);
        failed = true;
      }
    }
  }
}

const binDir = path.join(ROOT, "apps/desktop/src-tauri/binaries");
const binaries = fs.existsSync(binDir) ? fs.readdirSync(binDir) : [];
if (!binaries.some((name) => name.endsWith(".exe") && name.startsWith("yunque-agent-"))) {
  console.error("desktop release check failed: apps/desktop/src-tauri/binaries must contain a yunque-agent *.exe sidecar for Windows packaging");
  failed = true;
}

if (failed) {
  process.exit(1);
}

console.log("desktop release packaging check ok: src-tauri, sidecar binaries, Tauri config, scripts, and multi-OS workflow are present");
