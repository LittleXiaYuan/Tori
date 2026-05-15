#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const npmCli = resolve(dirname(process.execPath), "node_modules/npm/bin/npm-cli.js");
const windowsToolPaths = new Map([
  ["go", "C:/Program Files/Go/bin/go.exe"],
]);

const checks = [
  {
    name: "Pack Runtime completion audit",
    command: process.execPath,
    args: ["scripts/check-pack-runtime-completion.mjs"],
  },
  {
    name: "Pack manifest/backend/frontend contract",
    command: process.execPath,
    args: ["scripts/check-pack-contract.mjs"],
  },
  {
    name: "Pack scaffold contract",
    command: process.execPath,
    args: ["scripts/check-pack-scaffold.mjs"],
  },
  {
    name: "Packs SDK manifest",
    command: process.execPath,
    args: ["sdk/scripts/check-packs-sdk-manifest.mjs"],
  },
  {
    name: "All SDK manifests",
    command: process.execPath,
    args: ["sdk/scripts/check-sdk-manifests.mjs"],
  },
  {
    name: "Pack Runtime Go tests",
    command: "go",
    args: [
      "test",
      "./pkg/packruntime",
      "./internal/ledger",
      "./internal/packs/backup",
      "./internal/packs/lora",
      "./internal/packs/cognikernel",
      "./internal/packs/browserintent",
      "./internal/packs/chaosprobe",
      "./internal/packs/cognitivecanary",
      "./internal/packs/guardrailfuzzer",
      "./internal/packs/memorytimetravel",
      "./internal/packs/rpareplay",
      "./internal/packs/sbomdrift",
      "./internal/packs/skillanomaly",
      "./internal/packs/wasmplugin",
      "./internal/controlplane/gateway",
      "./cmd/agent",
      "-run",
      "Test(PackRoutes|BackupRoutes|LoRAPackRoutes|LoRAPack|CogniKernel|CogniExperience|BrowserIntent|ChaosProbe|CognitiveCanary|GuardrailFuzzer|MemoryTimeTravel|TemporalKV|RPAReplay|SBOMDrift|SkillAnomaly|WASMPlugin|BackendPack|RegisterBackendPack|Manifest|Registry|EnsureBuiltinPacks)|^$",
      "-count=1",
    ],
  },
  {
    name: "Frontend typecheck",
    command: process.execPath,
    args: [npmCli, "run", "typecheck", "--prefix", "heroui-web"],
  },
  {
    name: "Frontend Pack sync tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/pack-sync.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend packs client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/packs-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend backup pack client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/backup-pack-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend LoRA pack client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/lora-pack-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend Cogni Kernel pack client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/cogni-kernel-pack-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend Browser Intent pack client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/browser-intent-pack-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend Chaos Probe pack client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/chaos-probe-pack-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend Cognitive Canary pack client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/cognitive-canary-pack-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend Guardrail Fuzzer pack client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/guardrail-fuzzer-pack-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend Memory Time Travel pack client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/memory-time-travel-pack-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend RPA Replay pack client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/rpa-replay-pack-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend SBOM Drift pack client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/sbom-drift-pack-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend Skill Anomaly pack client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/skill-anomaly-pack-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend WASM Plugin pack client tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/lib/__tests__/wasm-plugin-pack-client.test.ts"],
    cwd: "heroui-web",
  },
  {
    name: "Frontend shell pack entry tests",
    command: process.execPath,
    args: [npmCli, "run", "test", "--", "src/components/cherry/__tests__/settings-modal-pack-entry.test.tsx"],
    cwd: "heroui-web",
  },
  {
    name: "TypeScript SDK typecheck",
    command: process.execPath,
    args: [npmCli, "run", "typecheck"],
    cwd: "sdk/typescript",
  },
  {
    name: "TypeScript SDK test typecheck",
    command: process.execPath,
    args: [npmCli, "run", "typecheck:test"],
    cwd: "sdk/typescript",
  },
  {
    name: "TypeScript packs SDK contract",
    command: process.execPath,
    args: [npmCli, "run", "check:pack"],
    cwd: "sdk/typescript",
  },
  {
    name: "TypeScript packs SDK incremental tests",
    command: process.execPath,
    args: ["scripts/run-incremental-tests.mjs", "packs", "chaos-probe", "cognitive-canary", "guardrail-fuzzer", "memory-time-travel", "rpa-replay", "sbom-drift", "skill-anomaly", "wasm-plugin"],
    cwd: "sdk/typescript",
  },
  {
    name: "Pack Runtime docs build",
    command: process.execPath,
    args: [npmCli, "run", "build"],
    cwd: "docs",
  },
];


function resolveCommand(command) {
  if (process.platform !== "win32") return command;
  return windowsToolPaths.get(command) ?? command;
}

function runCheck(check) {
  const cwd = resolve(repoRoot, check.cwd ?? ".");
  console.log(`\n=== ${check.name} ===`);
  console.log(`cwd: ${cwd}`);
  const command = resolveCommand(check.command);
  console.log(`$ ${command} ${check.args.join(" ")}`);

  if (!existsSync(cwd)) {
    console.error(`missing cwd: ${cwd}`);
    return 1;
  }

  const result = spawnSync(command, check.args, {
    cwd,
    env: { ...process.env, GOWORK: "off" },
    stdio: "inherit",
  });

  if (result.error) {
    console.error(result.error);
    return 1;
  }

  return result.status ?? 1;
}

const startedAt = Date.now();
const failed = [];

for (const check of checks) {
  const status = runCheck(check);
  if (status !== 0) {
    failed.push({ name: check.name, status });
    break;
  }
}

const elapsed = ((Date.now() - startedAt) / 1000).toFixed(1);

if (failed.length > 0) {
  console.error("\nPack Runtime full verification failed:");
  for (const item of failed) console.error(`- ${item.name}: exit ${item.status}`);
  console.error(`Elapsed: ${elapsed}s`);
  process.exit(1);
}

console.log(`\nPack Runtime full verification passed in ${elapsed}s.`);
