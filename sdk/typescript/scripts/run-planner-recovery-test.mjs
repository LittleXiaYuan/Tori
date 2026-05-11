import { readFileSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { spawnSync } from "node:child_process";

const outDir = ".tmp/planner-recovery-test";
rmSync(outDir, { recursive: true, force: true });

const tscBin = process.platform === "win32" ? "node_modules\\.bin\\tsc.cmd" : "node_modules/.bin/tsc";
const compile = spawnSync(
  tscBin,
  [
    "--module",
    "ES2022",
    "--moduleResolution",
    "Bundler",
    "--target",
    "ES2022",
    "--lib",
    "ES2022,DOM,DOM.Iterable",
    "--noEmit",
    "false",
    "--outDir",
    outDir,
    "src/planner-recovery.ts",
    "src/planner-recovery.test.ts",
  ],
  { stdio: "inherit", shell: process.platform === "win32" },
);

if (compile.error || compile.status !== 0) {
  if (compile.error) console.error(compile.error);
  process.exit(compile.status ?? 1);
}

const compiledTestPath = join(outDir, "planner-recovery.test.js");
const compiledTest = readFileSync(compiledTestPath, "utf8");
writeFileSync(compiledTestPath, compiledTest.replace('from "./planner-recovery"', 'from "./planner-recovery.js"'));

const run = spawnSync(process.execPath, [compiledTestPath], { stdio: "inherit" });
rmSync(outDir, { recursive: true, force: true });
if (run.error) {
  console.error(run.error);
  process.exit(1);
}
process.exit(run.status ?? 1);
