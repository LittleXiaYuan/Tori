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
  "src/task-context.ts",
  "src/task-context.test.ts",
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
  "src/trust.ts",
  "src/trust.test.ts",
  "src/audit.ts",
  "src/audit.test.ts",
  "src/heartbeat.ts",
  "src/heartbeat.test.ts",
  "src/reverie.ts",
  "src/reverie.test.ts",
  "src/federation.ts",
  "src/federation.test.ts",
  "src/system.ts",
  "src/system.test.ts",
  "src/settings.ts",
  "src/settings.test.ts",
  "src/tori.ts",
  "src/tori.test.ts",
  "src/speech.ts",
  "src/speech.test.ts",
  "src/admin.ts",
  "src/admin.test.ts",
  "src/files.ts",
  "src/files.test.ts",
  "src/cron.ts",
  "src/cron.test.ts",
  "src/skillhub.ts",
  "src/skillhub.test.ts",
  "src/plugins.ts",
  "src/plugins.test.ts",
  "src/graph.ts",
  "src/graph.test.ts",
  "src/plugin-api.ts",
  "src/plugin-api.test.ts",
  "src/state.ts",
  "src/state.test.ts",
  "src/triggers.ts",
  "src/triggers.test.ts",
  "src/missions.ts",
  "src/missions.test.ts",
  "src/tools.ts",
  "src/tools.test.ts",
  "src/sandbox.ts",
  "src/sandbox.test.ts",
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
  "task-context.test",
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
  "trust.test",
  "audit.test",
  "heartbeat.test",
  "reverie.test",
  "federation.test",
  "system.test",
  "settings.test",
  "tori.test",
  "speech.test",
  "admin.test",
  "files.test",
  "cron.test",
  "skillhub.test",
  "plugins.test",
  "graph.test",
  "plugin-api.test",
  "state.test",
  "triggers.test",
  "missions.test",
  "tools.test",
  "sandbox.test",
]) {
  const compiledTestPath = join(outDir, `${testName}.js`);
  let compiledTest = readFileSync(compiledTestPath, "utf8");
  compiledTest = compiledTest
    .replace('from "./planner-recovery"', 'from "./planner-recovery.js"')
    .replace('from "./chat"', 'from "./chat.js"')
    .replace('from "./memory"', 'from "./memory.js"')
    .replace('from "./tasks"', 'from "./tasks.js"')
    .replace('from "./task-context"', 'from "./task-context.js"')
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
    .replace('from "./iterate"', 'from "./iterate.js"')
    .replace('from "./trust"', 'from "./trust.js"')
    .replace('from "./audit"', 'from "./audit.js"')
    .replace('from "./heartbeat"', 'from "./heartbeat.js"')
    .replace('from "./reverie"', 'from "./reverie.js"')
    .replace('from "./federation"', 'from "./federation.js"')
    .replace('from "./system"', 'from "./system.js"')
    .replace('from "./settings"', 'from "./settings.js"')
    .replace('from "./tori"', 'from "./tori.js"')
    .replace('from "./speech"', 'from "./speech.js"')
    .replace('from "./admin"', 'from "./admin.js"')
    .replace('from "./files"', 'from "./files.js"')
    .replace('from "./cron"', 'from "./cron.js"')
    .replace('from "./skillhub"', 'from "./skillhub.js"')
    .replace('from "./plugins"', 'from "./plugins.js"')
    .replace('from "./graph"', 'from "./graph.js"')
    .replace('from "./plugin-api"', 'from "./plugin-api.js"')
    .replace('from "./state"', 'from "./state.js"')
    .replace('from "./triggers"', 'from "./triggers.js"')
    .replace('from "./missions"', 'from "./missions.js"')
    .replace('from "./tools"', 'from "./tools.js"')
    .replace('from "./sandbox"', 'from "./sandbox.js"');
  writeFileSync(compiledTestPath, compiledTest);

  const run = spawnSync(process.execPath, [compiledTestPath], { stdio: "inherit" });
  if (run.error || run.status !== 0) {
    rmSync(outDir, { recursive: true, force: true });
    if (run.error) console.error(run.error);
    process.exit(run.status ?? 1);
  }
}

rmSync(outDir, { recursive: true, force: true });
