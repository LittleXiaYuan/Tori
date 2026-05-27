import { existsSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "../../..");
const sdkRoot = join(repoRoot, "packages/yunque-client");
const workloadPresetPath = join(sdkRoot, "src/workloads.ts");
const pkg = JSON.parse(readFileSync(join(sdkRoot, "package.json"), "utf8"));
const readme = readFileSync(join(sdkRoot, "README.md"), "utf8");
const workloadPresetSource = readFileSync(workloadPresetPath, "utf8");

const boundaries = {
  "browser.intent.plan": {
    manifest: "sdk/manifest/browser-intent-pack-sdk.json",
    sdkImport: "yunque-client/browser",
    slice: "browser",
    sourceTokens: ["browserActPlan", "/v1/browser/intent/plan"],
    manifestRoutes: ["POST /v1/browser/intent/plan"],
  },
  "rpa.replay.dry_run": {
    manifest: "sdk/manifest/rpa-replay-pack-sdk.json",
    sdkImport: "yunque-client/rpa-replay",
    slice: "rpa-replay",
    sourceTokens: ["replay(", "/v1/rpa-replay/replay"],
    manifestRoutes: ["POST /v1/rpa-replay/replay"],
  },
  "rpa.executor.plan": {
    manifest: "sdk/manifest/rpa-replay-pack-sdk.json",
    sdkImport: "yunque-client/rpa-replay",
    slice: "rpa-replay",
    sourceTokens: ["executorPlan", "/v1/rpa-replay/executor/plan"],
    manifestRoutes: ["POST /v1/rpa-replay/executor/plan"],
  },
  "memory_time_travel.snapshot_at": {
    manifest: "sdk/manifest/memory-time-travel-pack-sdk.json",
    sdkImport: "yunque-client/memory-time-travel",
    slice: "memory-time-travel",
    sourceTokens: ["snapshotAt", "/v1/memory-time-travel/snapshot-at"],
    manifestRoutes: ["POST /v1/memory-time-travel/snapshot-at"],
  },
  "memory_time_travel.diff": {
    manifest: "sdk/manifest/memory-time-travel-pack-sdk.json",
    sdkImport: "yunque-client/memory-time-travel",
    slice: "memory-time-travel",
    sourceTokens: ["diff(", "/v1/memory-time-travel/diff"],
    manifestRoutes: ["POST /v1/memory-time-travel/diff"],
  },
  "memory_time_travel.rollback_plan": {
    manifest: "sdk/manifest/memory-time-travel-pack-sdk.json",
    sdkImport: "yunque-client/memory-time-travel",
    slice: "memory-time-travel",
    sourceTokens: ["rollbackPlan", "/v1/memory-time-travel/rollback-plan"],
    manifestRoutes: ["POST /v1/memory-time-travel/rollback-plan"],
  },
  "memory_time_travel.audit.verify": {
    manifest: "sdk/manifest/memory-time-travel-pack-sdk.json",
    sdkImport: "yunque-client/memory-time-travel",
    slice: "memory-time-travel",
    sourceTokens: ["auditVerify", "/v1/memory-time-travel/audit/verify"],
    manifestRoutes: ["GET /v1/memory-time-travel/audit/verify"],
  },
  "wasm.host_abi.plan": {
    manifest: "sdk/manifest/wasm-plugin-pack-sdk.json",
    sdkImport: "yunque-client/wasm-plugin",
    slice: "wasm-plugin",
    sourceTokens: ["host_abi_plan", "/v1/wasm-plugin/status"],
    manifestRoutes: ["GET /v1/wasm-plugin/status"],
  },
  "wasm.remote_install.plan": {
    manifest: "sdk/manifest/wasm-plugin-pack-sdk.json",
    sdkImport: "yunque-client/wasm-plugin",
    slice: "wasm-plugin",
    sourceTokens: ["remoteInstallPlan", "/v1/wasm-plugin/remote-install/plan"],
    manifestRoutes: ["POST /v1/wasm-plugin/remote-install/plan"],
  },
  "wasm.remote_install.approval_plan": {
    manifest: "sdk/manifest/wasm-plugin-pack-sdk.json",
    sdkImport: "yunque-client/wasm-plugin",
    slice: "wasm-plugin",
    sourceTokens: ["remoteInstallApprovalPlan", "/v1/wasm-plugin/remote-install/approval/plan"],
    manifestRoutes: ["POST /v1/wasm-plugin/remote-install/approval/plan"],
  },
  "wasm.plugin.execute": {
    manifest: "sdk/manifest/wasm-plugin-pack-sdk.json",
    sdkImport: "yunque-client/wasm-plugin",
    slice: "wasm-plugin",
    sourceTokens: ["execute(", "/v1/wasm-plugin/execute"],
    manifestRoutes: ["POST /v1/wasm-plugin/execute"],
  },
  "sbom.ci_gate.plan": {
    manifest: "sdk/manifest/sbom-drift-pack-sdk.json",
    sdkImport: "yunque-client/sbom-drift",
    slice: "sbom-drift",
    sourceTokens: ["ciGatePlan", "/v1/sbom-drift/ci-gate/plan"],
    manifestRoutes: ["POST /v1/sbom-drift/ci-gate/plan"],
  },
  "guardrail_fuzzer.ci_gate.plan": {
    manifest: "sdk/manifest/guardrail-fuzzer-pack-sdk.json",
    sdkImport: "yunque-client/guardrail-fuzzer",
    slice: "guardrail-fuzzer",
    sourceTokens: ["ciGatePlan", "/v1/guardrail-fuzzer/ci-gate/plan"],
    manifestRoutes: ["POST /v1/guardrail-fuzzer/ci-gate/plan"],
  },
  "chaos_probe.scheduler.plan": {
    manifest: "sdk/manifest/chaos-probe-pack-sdk.json",
    sdkImport: "yunque-client/chaos-probe",
    slice: "chaos-probe",
    sourceTokens: ["schedulerPlan", "/v1/chaos-probe/scheduler/plan"],
    manifestRoutes: ["POST /v1/chaos-probe/scheduler/plan"],
  },
  "cognis.generate": {
    manifest: "sdk/manifest/cognis-sdk.json",
    sdkImport: "yunque-client/cognis",
    slice: "cognis",
    sourceTokens: ["generate(", "/v1/cognis/generate"],
    manifestRoutes: ["POST /v1/cognis/generate"],
  },
  "cognis.workflows": {
    manifest: "sdk/manifest/cognis-sdk.json",
    sdkImport: "yunque-client/cognis-workflows",
    slice: "cognis-workflows",
    sourceTokens: ["createCognisWorkflowsClient", "workflows"],
    manifestRoutes: ["GET /v1/cognis/{id}/workflows", "POST /v1/cognis/{id}/workflow/{name}"],
  },
  "cognis.experience": {
    manifest: "sdk/manifest/cognis-sdk.json",
    sdkImport: "yunque-client/cognis-experience",
    slice: "cognis-experience",
    sourceTokens: ["createCognisExperienceClient", "recordExperience"],
    manifestRoutes: ["GET /v1/cognis/{id}/experience", "POST /v1/cognis/{id}/experience/record"],
  },
  "cognis.evolution": {
    manifest: "sdk/manifest/cognis-sdk.json",
    sdkImport: "yunque-client/cognis-evolution",
    slice: "cognis-evolution",
    sourceTokens: ["createCognisEvolutionClient", "evolution"],
    manifestRoutes: ["GET /v1/cognis/evolution", "POST /v1/cognis/{id}/evolve"],
  },
};

const failures = [];
function fail(message) { failures.push(message); }

function extractWorkloadCapabilities(source) {
  const out = [];
  const capabilitiesBlocks = source.matchAll(/capabilities:\s*\[([\s\S]*?)\]/g);
  for (const block of capabilitiesBlocks) {
    for (const item of block[1].matchAll(/["']([^"']+)["']/g)) {
      out.push(item[1]);
    }
  }
  return [...new Set(out)].sort();
}

const workloadCapabilities = extractWorkloadCapabilities(workloadPresetSource);
for (const capability of workloadCapabilities) {
  const boundary = boundaries[capability];
  if (!boundary) {
    fail(`workload capability has no SDK boundary mapping: ${capability}`);
    continue;
  }

  const manifestPath = join(repoRoot, boundary.manifest);
  if (!existsSync(manifestPath)) {
    fail(`${capability}: missing manifest ${boundary.manifest}`);
    continue;
  }
  const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
  const manifestText = JSON.stringify(manifest);
  const manifestSdkImports = [manifest.sdkImport, ...Object.values(manifest.languages?.typescript?.entrypoints ?? {})];
  const manifestImplementationFiles = manifest.languages?.typescript?.implementationFiles ?? [];
  const manifestImportCovered = manifestSdkImports.includes(boundary.sdkImport) || manifestImplementationFiles.includes(`packages/yunque-client/src/${boundary.slice}.ts`);
  if (!manifestImportCovered) {
    fail(`${capability}: ${boundary.manifest} does not bind TypeScript SDK boundary ${boundary.sdkImport}`);
  }
  for (const route of boundary.manifestRoutes ?? []) {
    if (!manifestText.includes(route)) fail(`${capability}: ${boundary.manifest} missing route ${route}`);
  }

  const exportKey = `./${boundary.slice}`;
  const exportInfo = pkg.exports?.[exportKey];
  if (!exportInfo) fail(`${capability}: package.json missing export ${exportKey}`);
  if (exportInfo?.import !== `./src/${boundary.slice}.ts`) fail(`${capability}: package export ${exportKey} does not point to ./src/${boundary.slice}.ts`);

  const sourcePath = join(sdkRoot, "src", `${boundary.slice}.ts`);
  const testPath = join(sdkRoot, "src", `${boundary.slice}.test.ts`);
  if (!existsSync(sourcePath)) fail(`${capability}: missing SDK slice source src/${boundary.slice}.ts`);
  if (!existsSync(testPath)) fail(`${capability}: missing SDK slice test src/${boundary.slice}.test.ts`);
  const source = existsSync(sourcePath) ? readFileSync(sourcePath, "utf8") : "";
  const test = existsSync(testPath) ? readFileSync(testPath, "utf8") : "";
  for (const token of boundary.sourceTokens ?? []) {
    if (!source.includes(token)) fail(`${capability}: src/${boundary.slice}.ts missing token ${token}`);
  }
  for (const token of boundary.sourceTokens ?? []) {
    if (token.startsWith("/v1/") && !test.includes(token)) fail(`${capability}: src/${boundary.slice}.test.ts missing route assertion ${token}`);
  }
  if (!readme.includes(boundary.sdkImport)) fail(`${capability}: README.md missing ${boundary.sdkImport}`);
}

for (const capability of Object.keys(boundaries).sort()) {
  if (!workloadCapabilities.includes(capability)) fail(`SDK boundary mapping is stale; capability not used by workload presets: ${capability}`);
}

if (failures.length > 0) {
  console.error(`workload metadata SDK boundary check failed (${failures.length} issues):`);
  for (const item of failures) console.error(`- ${item}`);
  process.exit(1);
}

const slices = [...new Set(workloadCapabilities.map((capability) => boundaries[capability].slice))].sort();
console.log(`workload metadata SDK boundaries ok: ${workloadCapabilities.length} capabilities protected by ${slices.length} yunque-client slices (${slices.join(", ")})`);
