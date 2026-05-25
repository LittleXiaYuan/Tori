#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

const root = process.cwd();
const failures = [];

const promotedPackages = ["trait", "recommend", "react", "taskdistill", "eval", "causal", "curiosity", "world", "microagent"];

const allowedMainlineExperimentalImports = new Map([
  [
    "distill",
    {
      reason: "distillation service is still exposed through gateway/bootstrap wiring",
      files: [
        "cmd/agent/init_extensions.go",
        "cmd/agent/init_soul.go",
        "internal/controlplane/gateway/gateway.go",
        "internal/controlplane/gateway/gateway_setters.go",
      ],
    },
  ],
  [
    "docparse",
    {
      reason: "task skill surface; should move with extension/skill ownership cleanup",
      files: ["cmd/agent/init_tasks.go"],
    },
  ],
  [
    "filegen",
    {
      reason: "task skill surface; should move with extension/skill ownership cleanup",
      files: ["cmd/agent/init_tasks.go"],
    },
  ],
  [
    "heartbeat",
    {
      reason: "session/heartbeat runtime module still exposed through bootstrap/gateway",
      files: [
        "cmd/agent/init_session_auth.go",
        "cmd/agent/module_heartbeat.go",
        "internal/controlplane/gateway/gateway.go",
        "internal/controlplane/gateway/gateway_setters.go",
      ],
    },
  ],
  [
    "imagegen",
    {
      reason: "task skill surface; should move with extension/skill ownership cleanup",
      files: ["cmd/agent/init_tasks.go"],
    },
  ],
  [
    "iterate",
    {
      reason: "iteration service still exposed through gateway/bootstrap wiring",
      files: [
        "cmd/agent/init_extensions.go",
        "cmd/agent/init_soul.go",
        "internal/controlplane/gateway/gateway.go",
        "internal/controlplane/gateway/gateway_setters.go",
      ],
    },
  ],
  [
    "metacog",
    {
      reason: "metacognition bridge still uses this package until LearningSidecar/cognikernel ownership is completed",
      files: ["cmd/agent/init_soul.go", "internal/ledger/metacog_bridge.go", "internal/ledger/metacog_bridge_test.go"],
    },
  ],
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
  [
    "research",
    {
      reason: "task skill surface; should move with extension/skill ownership cleanup",
      files: ["cmd/agent/init_tasks.go"],
    },
  ],
  [
    "rlsched",
    {
      reason: "task scheduling policy; should move after task runtime owner is decided",
      files: [
        "cmd/agent/init_intelligence.go",
        "cmd/agent/init_task_engine.go",
        "cmd/agent/init_tasks.go",
      ],
    },
  ],
  [
    "skillgrow",
    {
      reason: "detect/generate adapter for canonical internal/agentcore/skillgrowth pipeline",
      files: [
        "cmd/agent/init_extensions.go",
        "cmd/agent/init_market.go",
        "cmd/agent/init_soul.go",
        "internal/controlplane/gateway/gateway.go",
        "internal/controlplane/gateway/gateway_setters.go",
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

const cognicoreDoc = read("internal/cognicore/doc.go");
for (const pkg of promotedPackages) {
  if (!cognicoreDoc.includes(pkg)) {
    failures.push(`internal/cognicore/doc.go does not mention promoted package ${pkg}`);
  }
}

const conceptMap = read("doc/AGENTCORE-CONCEPT-MAP.md");
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
  "internal/agentcore/runtime/circuit",
  "deterministic Bayesian success-rate",
  "post-task learning/evaluation",
  "soul-layer cognition modules",
  "scoped prompt-enhancement registry",
  "LLM/runtime resilience infrastructure",
  "remaining `internal/experimental/*` packages",
]) {
  if (!conceptMap.includes(needle)) {
    failures.push(`doc/AGENTCORE-CONCEPT-MAP.md missing ${JSON.stringify(needle)}`);
  }
}

const taskLedger = read("doc/LONG-TERM-TASKS.md");
for (const needle of [
  "scripts/check-experimental-boundaries.mjs",
  "internal/cognicore",
  "trait` / `recommend` / `react",
  "taskdistill` / `eval",
  "causal` / `curiosity` / `world",
  "microagent",
  "internal/agentcore/runtime/circuit",
  "TestRecommendCandidatesDeterministicVisibleRanking",
  "Bayesian success-rate score",
]) {
  if (!taskLedger.includes(needle)) {
    failures.push(`doc/LONG-TERM-TASKS.md missing ${JSON.stringify(needle)}`);
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
console.log("Promoted packages are under internal/cognicore: trait, recommend, react, taskdistill, eval, causal, curiosity, world, microagent.");
console.log(`Remaining mainline experimental imports are explicitly allow-listed: ${actualMainlineImports.length}.`);
