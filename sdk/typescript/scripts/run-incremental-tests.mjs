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
  "src/memory.ts",
  "src/memory.test.ts",
  "src/tasks.ts",
  "src/tasks.test.ts",
  "src/knowledge.ts",
  "src/knowledge.test.ts",
  "src/providers.ts",
  "src/providers.test.ts",
  "src/setup.ts",
  "src/setup.test.ts",
  "src/documents.ts",
  "src/documents.test.ts",
  "src/approvals.ts",
  "src/approvals.test.ts",
  "src/trace.ts",
  "src/trace.test.ts",
  "src/browser.ts",
  "src/browser.test.ts",
  "src/runtime.ts",
  "src/runtime.test.ts",
  "src/modes.ts",
  "src/modes.test.ts",
  "src/ide.ts",
  "src/ide.test.ts",
  "src/persona.ts",
  "src/persona.test.ts",
  "src/workflow.ts",
  "src/workflow.test.ts",
  "src/cost.ts",
  "src/cost.test.ts",
  "src/lora.ts",
  "src/lora.test.ts",
  "src/iterate.ts",
  "src/iterate.test.ts",
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

for (const testName of [
  "planner-recovery.test",
  "chat.test",
  "memory.test",
  "tasks.test",
  "knowledge.test",
  "providers.test",
  "setup.test",
  "documents.test",
  "approvals.test",
  "trace.test",
  "browser.test",
  "runtime.test",
  "modes.test",
  "ide.test",
  "persona.test",
  "workflow.test",
  "cost.test",
  "lora.test",
  "iterate.test",
]) {
  const compiledTestPath = join(outDir, `${testName}.js`);
  let compiledTest = readFileSync(compiledTestPath, "utf8");
  compiledTest = compiledTest
    .replace('from "./planner-recovery"', 'from "./planner-recovery.js"')
    .replace('from "./chat"', 'from "./chat.js"')
    .replace('from "./memory"', 'from "./memory.js"')
    .replace('from "./tasks"', 'from "./tasks.js"')
    .replace('from "./knowledge"', 'from "./knowledge.js"')
    .replace('from "./providers"', 'from "./providers.js"')
    .replace('from "./setup"', 'from "./setup.js"')
    .replace('from "./documents"', 'from "./documents.js"')
    .replace('from "./approvals"', 'from "./approvals.js"')
    .replace('from "./trace"', 'from "./trace.js"')
    .replace('from "./browser"', 'from "./browser.js"')
    .replace('from "./runtime"', 'from "./runtime.js"')
    .replace('from "./modes"', 'from "./modes.js"')
    .replace('from "./ide"', 'from "./ide.js"')
    .replace('from "./persona"', 'from "./persona.js"')
    .replace('from "./workflow"', 'from "./workflow.js"')
    .replace('from "./cost"', 'from "./cost.js"')
    .replace('from "./lora"', 'from "./lora.js"')
    .replace('from "./iterate"', 'from "./iterate.js"');
  writeFileSync(compiledTestPath, compiledTest);

  const run = spawnSync(process.execPath, [compiledTestPath], { stdio: "inherit" });
  if (run.error || run.status !== 0) {
    rmSync(outDir, { recursive: true, force: true });
    if (run.error) console.error(run.error);
    process.exit(run.status ?? 1);
  }
}

rmSync(outDir, { recursive: true, force: true });
