#!/usr/bin/env node
import { existsSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const failures = [];
const evidence = [];

function fail(item, message) {
  failures.push(`${item}: ${message}`);
}

function ok(item, message) {
  evidence.push({ item, message });
}

function read(path) {
  const full = resolve(repoRoot, path);
  if (!existsSync(full)) {
    fail(path, "missing file");
    return "";
  }
  return readFileSync(full, "utf8");
}

function requireTokens(item, text, tokens) {
  const missing = tokens.filter((token) => !text.includes(token));
  if (missing.length > 0) fail(item, `missing tokens: ${missing.join(", ")}`);
  else ok(item, `${tokens.length} tokens present`);
}

function runCheck(item, args) {
  const result = spawnSync(process.execPath, args, { cwd: repoRoot, encoding: "utf8" });
  if (result.status !== 0) fail(item, `${process.execPath} ${args.join(" ")} exited ${result.status}: ${result.stderr || result.stdout}`);
  else ok(item, `${args.join(" ")} passed`);
}

const manifest = read("pkg/packruntime/manifest.go");
const registry = read("pkg/packruntime/registry.go");
const backend = read("pkg/packruntime/backend.go");
const gateway = [
  "internal/controlplane/gateway/handlers_packs.go",
  "internal/controlplane/gateway/gateway.go",
  "internal/controlplane/gateway/gateway_setters.go",
  "cmd/agent/init_tasks.go",
].map(read).join("\n");
const packCenter = read("apps/web/src/app/packs/page.tsx");
const packDetail = read("apps/web/src/app/packs/detail/client-page.tsx");
const packStudio = read("apps/web/src/app/packs/studio/page.tsx");
const presentation = read("apps/web/src/lib/pack-presentation.ts");
const releaseSources = read("apps/web/src/lib/pack-release-sources.ts");
const capabilityRegistry = read("packs/capability-registry.json");

requireTokens("Pack Runtime manifest contract", manifest, [
  "type Manifest",
  "BackendManifest",
  "FrontendManifest",
  "SDKManifest",
  "DistributionManifest",
  "UpdateManifest",
  "BackendRouteSpec",
]);

requireTokens("Installed Pack registry lifecycle", registry, [
  "type Registry",
  "InstallWithArtifacts",
  "Enable(id string)",
  "Disable(id string)",
  "Rollback(id string)",
  "PreviousVersion",
  "PruneArtifacts",
]);

requireTokens("Backend Pack module and gateway route gates", backend + gateway, [
  "type BackendModule interface",
  "RegisterBackendPack",
  "BackendPacks",
  "/v1/packs/install",
  "/v1/packs/enable",
  "/v1/packs/disable",
  "/v1/packs/rollback",
  "/v1/packs/catalog",
  "/v1/packs/capabilities",
  "/v1/packs/backend-route-audit",
  "http.StatusMethodNotAllowed",
]);

requireTokens("能力包中心和 Studio 闭环", packCenter + packDetail + packStudio + presentation + releaseSources, [
  "能力包中心",
  "官方源",
  "私有源",
  "本地高级安装",
  "来源与安装包",
  "先在 Studio 只读检查",
  "Pack Studio",
  "只读检查",
  "diff 预览",
  "运行内置审计",
  "重新打包",
  "回滚",
  "查看权限与来源",
  "releaseCatalog",
  "groupPackPermissions",
  "riskProfileForPack",
  "packReadiness",
]);

requireTokens("Capability registry fact source", capabilityRegistry, [
  "\"version\": 1",
  "\"capabilities\"",
  "\"ownerPack\"",
  "\"lifecycle\"",
  "yunque.pack.computer-use",
  "computer.intent.plan",
  "yunque.pack.cogni-kernel",
]);

runCheck("Pack contract", ["scripts/check-pack-contract.mjs"]);

if (failures.length > 0) {
  console.error("Pack Runtime completion audit failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log("Pack Runtime completion audit ok:");
for (const item of evidence) console.log(`- ${item.item}: ${item.message}`);
