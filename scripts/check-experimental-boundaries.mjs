#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

const root = process.cwd();
const failures = [];

const promotedPackages = ["trait", "recommend", "react", "taskdistill", "eval", "causal", "curiosity", "world", "microagent", "metacog"];

const allowedMainlineExperimentalImports = new Map([
  [
    "reflect",
    {
      reason: "legacy compatibility evaluator feeding the canonical cognikernel ReflectiveLoop",
      files: [
        "cmd/agent/init_gateway_handler.go",
        "cmd/agent/init_gateway.go",
        "cmd/agent/init_memory.go",
        "cmd/agent/init_planner.go",
        "cmd/agent/init_task_cognition.go",
        "cmd/agent/init_task_engine.go",
        "cmd/agent/init_tasks.go",
        "internal/controlplane/gateway/accessors.go",
        "internal/controlplane/gateway/gateway.go",
        "internal/controlplane/gateway/gateway_setters.go",
        "internal/controlplane/gateway/gateway_wire.go",
        "internal/controlplane/gateway/handlers_reasoning.go",
        "internal/controlplane/gateway/handlers_reflect_experience_test.go",
        "internal/export/userdata/export.go",
        "internal/export/userdata/export_test.go",
      ],
    },
  ],
]);

function rel(abs) {
  return path.relative(root, abs).replaceAll(path.sep, "/");
}

function exists(relPath) {
  return fs.existsSync(path.join(root, relPath));
}

function read(relPath) {
  const abs = path.join(root, relPath);
  if (!fs.existsSync(abs)) {
    failures.push(`missing file: ${relPath}`);
    return "";
  }
  return fs.readFileSync(abs, "utf8");
}

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const abs = path.join(dir, entry.name);
    const r = rel(abs);
    if (entry.isDirectory()) {
      if (
        entry.name === ".git" ||
        entry.name === "node_modules" ||
        entry.name === ".next" ||
        entry.name === "out" ||
        entry.name === "dist" ||
        r.startsWith("apps/web/out/") ||
        r.startsWith("apps/web/.next/")
      ) {
        continue;
      }
      walk(abs, out);
    } else if (entry.isFile() && entry.name.endsWith(".go")) {
      out.push(abs);
    }
  }
  return out;
}

for (const pkg of promotedPackages) {
  if (exists(`internal/experimental/${pkg}`)) {
    failures.push(`promoted package still exists under internal/experimental: ${pkg}`);
  }
  if (!exists(`internal/cognicore/${pkg}`)) {
    failures.push(`promoted package missing from internal/cognicore: ${pkg}`);
  }
}

if (exists("internal/experimental/circuit")) {
  failures.push("runtime circuit breaker still exists under internal/experimental/circuit");
}
if (!exists("internal/agentcore/runtime/circuit")) {
  failures.push("runtime circuit breaker missing from internal/agentcore/runtime/circuit");
}
if (exists("internal/experimental/rlsched")) {
  failures.push("task scheduler policy still exists under internal/experimental/rlsched");
}
if (!exists("internal/agentcore/tasksched/rlsched")) {
  failures.push("task scheduler policy missing from internal/agentcore/tasksched/rlsched");
}
if (exists("internal/experimental/heartbeat")) {
  failures.push("heartbeat runtime module still exists under internal/experimental/heartbeat");
}
if (!exists("internal/agentcore/runtime/heartbeat")) {
  failures.push("heartbeat runtime module missing from internal/agentcore/runtime/heartbeat");
}
if (exists("internal/experimental/distill")) {
  failures.push("distillation service still exists under internal/experimental/distill");
}
if (!exists("internal/agentcore/llm/distill")) {
  failures.push("distillation service missing from internal/agentcore/llm/distill");
}
if (exists("internal/experimental/iterate")) {
  failures.push("iterate service still exists under internal/experimental/iterate");
}
if (!exists("internal/agentcore/selfheal/iterate")) {
  failures.push("iterate service missing from internal/agentcore/selfheal/iterate");
}
if (exists("internal/experimental/skillgrow")) {
  failures.push("skillgrow adapter still exists under internal/experimental/skillgrow");
}
if (!exists("internal/agentcore/skillgrowth/adapter")) {
  failures.push("skillgrow adapter missing from internal/agentcore/skillgrowth/adapter");
}
for (const pkg of ["docparse", "filegen", "imagegen", "research"]) {
  if (exists(`internal/experimental/${pkg}`)) {
    failures.push(`task skill surface still exists under internal/experimental/${pkg}`);
  }
  if (!exists(`internal/agentcore/taskskills/${pkg}`)) {
    failures.push(`task skill surface missing from internal/agentcore/taskskills/${pkg}`);
  }
}

const cognicoreDoc = read("internal/cognicore/doc.go");
for (const pkg of promotedPackages) {
  if (!cognicoreDoc.includes(pkg)) {
    failures.push(`internal/cognicore/doc.go does not mention promoted package ${pkg}`);
  }
}

const conceptMap = read("docs/design/agentcore-concept-map.md");
for (const needle of [
  "T8 first promotion slice",
  "internal/cognicore/trait",
  "internal/cognicore/recommend",
  "internal/cognicore/react",
  "internal/cognicore/taskdistill",
  "internal/cognicore/eval",
  "internal/cognicore/causal",
  "internal/cognicore/curiosity",
  "internal/cognicore/world",
  "internal/cognicore/microagent",
  "internal/cognicore/metacog",
  "internal/agentcore/runtime/circuit",
  "internal/agentcore/tasksched/rlsched",
  "internal/agentcore/taskskills",
  "deterministic Bayesian success-rate",
  "post-task learning/evaluation",
  "soul-layer cognition modules",
  "scoped prompt-enhancement registry",
  "real-time reasoning anomaly monitor",
  "LLM/runtime resilience infrastructure",
  "task scheduling policy",
  "task skill surface",
  "remaining `internal/experimental/*` packages",
]) {
  if (!conceptMap.includes(needle)) {
    failures.push(`docs/design/agentcore-concept-map.md missing ${JSON.stringify(needle)}`);
  }
}

const taskLedgerPath = "doc/LONG-TERM-TASKS.md";
const taskLedger = fs.existsSync(path.join(root, taskLedgerPath)) ? fs.readFileSync(path.join(root, taskLedgerPath), "utf8") : null;
if (taskLedger !== null) {
  for (const needle of [
    "scripts/check-experimental-boundaries.mjs",
    "internal/cognicore",
    "trait` / `recommend` / `react",
    "taskdistill` / `eval",
    "causal` / `curiosity` / `world",
    "microagent",
    "metacog",
    "internal/agentcore/runtime/circuit",
    "internal/agentcore/tasksched/rlsched",
    "internal/agentcore/taskskills",
    "task skill surface",
    "TestRecommendCandidatesDeterministicVisibleRanking",
    "Bayesian success-rate score",
  ]) {
    if (!taskLedger.includes(needle)) {
      failures.push(`doc/LONG-TERM-TASKS.md missing ${JSON.stringify(needle)}`);
    }
  }
}

const goFiles = walk(root);
const importRe = /"yunque-agent\/internal\/experimental\/([^"\/]+)(?:\/[^"]*)?"/g;
const actualMainlineImports = [];

for (const abs of goFiles) {
  const file = rel(abs);
  const text = fs.readFileSync(abs, "utf8");
  let match;
  while ((match = importRe.exec(text)) !== null) {
    const pkg = match[1];
    if (promotedPackages.includes(pkg)) {
      failures.push(`${file} imports promoted package from internal/experimental/${pkg}`);
    }
    if (file.startsWith("internal/experimental/")) {
      continue;
    }
    actualMainlineImports.push({ file, pkg });
  }
}

const allowedActual = new Set();
for (const { file, pkg } of actualMainlineImports) {
  const allowed = allowedMainlineExperimentalImports.get(pkg);
  if (!allowed) {
    failures.push(`${file} imports internal/experimental/${pkg}, but ${pkg} is not in the allowed boundary map`);
    continue;
  }
  if (!allowed.files.includes(file)) {
    failures.push(`${file} imports internal/experimental/${pkg}, but this file is not in the allowed boundary map`);
    continue;
  }
  allowedActual.add(`${pkg}:${file}`);
}

for (const [pkg, meta] of allowedMainlineExperimentalImports) {
  for (const file of meta.files) {
    if (!exists(file)) {
      failures.push(`allowed boundary map references missing file ${file} for ${pkg}`);
      continue;
    }
    if (!allowedActual.has(`${pkg}:${file}`)) {
      failures.push(`allowed boundary map is stale: ${file} no longer imports internal/experimental/${pkg}`);
    }
  }
  if (!meta.reason || meta.reason.length < 12) {
    failures.push(`allowed boundary map for ${pkg} must include a meaningful reason`);
  }
}

if (failures.length > 0) {
  console.error("Experimental boundary check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log("Experimental boundary check passed.");
console.log("Promoted packages are under internal/cognicore: trait, recommend, react, taskdistill, eval, causal, curiosity, world, microagent, metacog.");
console.log("Task skill surfaces are under internal/agentcore/taskskills: docparse, filegen, imagegen, research.");
console.log(`Remaining mainline experimental imports are explicitly allow-listed: ${actualMainlineImports.length}.`);
