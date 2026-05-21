import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const packageRoot = path.resolve(scriptDir, "..");
const srcDir = path.join(packageRoot, "src");
const backupRoot = fs.mkdtempSync(path.join(os.tmpdir(), "yunque-sdk-manual-"));

function walk(dir) {
  if (!fs.existsSync(dir)) return [];
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) return walk(full);
    return [full];
  });
}

function isManualSource(file) {
  const rel = path.relative(srcDir, file).replace(/\\/g, "/");
  if (rel.startsWith("client/") || rel.startsWith("core/")) return false;
  if (!rel.endsWith(".ts")) return false;
  if (rel === "index.ts") return false;
  if (rel.endsWith(".gen.ts")) return false;
  return true;
}

const manualFiles = walk(srcDir).filter(isManualSource);

try {
  for (const file of manualFiles) {
    const rel = path.relative(srcDir, file);
    const target = path.join(backupRoot, rel);
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.copyFileSync(file, target);
  }

  const cli = path.join(packageRoot, "node_modules", "@hey-api", "openapi-ts", "bin", "index.cjs");
  const result = spawnSync(process.execPath, [cli], {
    cwd: packageRoot,
    stdio: "inherit",
    shell: false,
  });
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }

  for (const file of manualFiles) {
    const rel = path.relative(srcDir, file);
    const backup = path.join(backupRoot, rel);
    const target = path.join(srcDir, rel);
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.copyFileSync(backup, target);
  }

  console.log(`preserved ${manualFiles.length} handcrafted incremental SDK source files`);
} finally {
  fs.rmSync(backupRoot, { recursive: true, force: true });
}
