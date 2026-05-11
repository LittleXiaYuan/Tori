import { readFileSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { spawnSync } from "node:child_process";

const outDir = ".tmp/incremental-tests";
rmSync(outDir, { recursive: true, force: true });

const sources = [
  "src/planner-recovery.ts",
  "src/planner-recovery.test.ts",
  "src/chat.ts",
  "src/chat.test.ts",
];

const compile = spawnSync(
  process.execPath,
  [
    "node_modules/typescript/bin/tsc",
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
    ...sources,
  ],
  { stdio: "inherit" },
);

if (compile.error || compile.status !== 0) {
  if (compile.error) console.error(compile.error);
  process.exit(compile.status ?? 1);
}

for (const testName of ["planner-recovery.test", "chat.test"]) {
  const compiledTestPath = join(outDir, `${testName}.js`);
  let compiledTest = readFileSync(compiledTestPath, "utf8");
  compiledTest = compiledTest
    .replace('from "./planner-recovery"', 'from "./planner-recovery.js"')
    .replace('from "./chat"', 'from "./chat.js"');
  writeFileSync(compiledTestPath, compiledTest);

  const run = spawnSync(process.execPath, [compiledTestPath], { stdio: "inherit" });
  if (run.error || run.status !== 0) {
    rmSync(outDir, { recursive: true, force: true });
    if (run.error) console.error(run.error);
    process.exit(run.status ?? 1);
  }
}

rmSync(outDir, { recursive: true, force: true });
