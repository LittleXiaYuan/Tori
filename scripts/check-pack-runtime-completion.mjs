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

function runCheck(item, command, args, expected = 0) {
  const result = spawnSync(command, args, { cwd: repoRoot, encoding: "utf8" });
  if (result.status !== expected) {
    fail(item, `${command} ${args.join(" ")} exited ${result.status}: ${result.stderr || result.stdout}`);
  } else {
    ok(item, `${command} ${args.join(" ")} passed`);
  }
}

const manifest = read("pkg/packruntime/manifest.go");
const registry = read("pkg/packruntime/registry.go");
const backend = read("pkg/packruntime/backend.go");
const gateway = [
  "internal/controlplane/gateway/handlers_packs.go",
  "internal/controlplane/gateway/gateway.go",
  "internal/controlplane/gateway/gateway_setters.go",
  "internal/controlplane/gateway/handlers_packs_test.go",
].map(read).join("\n");
const backup = read("internal/packs/backup/handler.go");
const backupManifest = read("packs/examples/backup-pack/pack.json");
const scaffold = read("scripts/scaffold-pack.mjs") + "\n" + read("scripts/check-pack-scaffold.mjs");
const fullVerification = read("scripts/check-pack-runtime-all.mjs");
const frontend = [
  "heroui-web/src/lib/pack-sync.tsx",
  "heroui-web/src/lib/__tests__/pack-sync.test.ts",
  "heroui-web/src/app/packs/page.tsx",
  "heroui-web/src/app/packs/[...slug]/page.tsx",
  "heroui-web/src/lib/api.ts",
  "heroui-web/src/lib/api-types/skills.ts",
].map(read).join("\n");
const frontendShell = [
  "heroui-web/src/components/sidebar.tsx",
  "heroui-web/src/lib/nav-items.tsx",
  "heroui-web/src/components/command-palette.tsx",
].map(read).join("\n");
const sdk = [
  "sdk/typescript/src/packs.ts",
  "sdk/typescript/src/packs.test.ts",
  "sdk/manifest/packs-sdk.json",
  "sdk/scripts/check-packs-sdk-manifest.mjs",
].map(read).join("\n");
const docs = [
  "packs/AUTHORING.md",
  "docs/guide/pack-runtime.md",
  "docs/zh/guide/pack-runtime.md",
  "docs/guide/pack-runtime-state.md",
  "docs/zh/guide/pack-runtime-state.md",
].map(read).join("\n");

requireTokens("轻内核/manifest 协议", manifest + docs + backupManifest, [
  "type Manifest",
  "BackendManifest",
  "FrontendManifest",
  "SDKManifest",
  "DistributionManifest",
  "UpdateManifest",
  "frontend.menus",
  "frontend.routes",
  "sdk.typescript",
  "distribution.packageUrl",
  "update.rollback",
]);

requireTokens("本地 installed registry / install-enable-disable-rollback", registry + gateway, [
  "type Registry",
  "RegistryFileName",
  "InstallWithArtifacts",
  "Enable(id string)",
  "Disable(id string)",
  "Rollback(id string)",
  "PreviousVersion",
  "PreviousArtifacts",
  "PruneArtifacts",
  "/v1/packs/install",
  "/v1/packs/enable",
  "/v1/packs/disable",
  "/v1/packs/rollback",
  "/v1/packs/prune",
]);

requireTokens("后端 backend pack module registry / route gates", backend + gateway, [
  "type BackendModule interface",
  "PackID() string",
  "Routes() []BackendRoute",
  "RegisterBackendPack",
  "BackendPacks",
  "backendPackRoutes",
  "backendPackRouteInfos",
  "requirePackRoute",
  "packRouteEnabled",
  "http.StatusMethodNotAllowed",
  "route conflict",
  "handlePackBackendModules",
]);

requireTokens("backup-pack 示例包", backup + backupManifest, [
  "const PackID = \"yunque.pack.backup\"",
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/backup/info",
  "/v1/backup/export",
  "/v1/backup/import",
  "yunque-client/backup",
  "distribution",
  "rollback",
]);

requireTokens("前端同步菜单/路由/资源/控制台", frontend + fullVerification, [
  "fetchEnabledPacks",
  "buildPackNavItems",
  "buildPackRouteBindings",
  "findPackRouteBinding",
  "pack-sync frontend runtime",
  "Frontend Pack sync tests",
  "PackRuntimeRoutePage",
  "packsEnabled",
  "frontend?.menus",
  "frontend?.routes",
  "/v1/packs/enabled",
  "manifest.distribution",
  "packBackendModules",
  "packInstallFromURL",
  "downloadArtifact",
  "packPrune",
  "previousArtifacts",
  "SDK 调用入口",
]);


if (frontendShell.includes('href: "/backup"') || frontendShell.includes('href: "/backup"') || frontendShell.includes('nav-backup')) {
  fail("前端同步菜单/路由/资源/控制台", "backup pack must not be exposed as a hard-coded main-shell nav item; use /v1/packs/enabled pack sync");
} else {
  ok("前端轻内核导航", "backup entry is not hard-coded in sidebar/nav-items/command-palette");
}

requireTokens("TypeScript packs SDK", sdk, [
  "createPacksClient",
  "installed()",
  "enabled()",
  "backendModules()",
  "install(request",
  "enable(id",
  "disable(id",
  "rollback(id",
  "prune()",
  "frontendSync()",
  "routeBinding(path",
  "PackRouteBinding",
  "PackDistributionManifest",
  "PackPruneResponse",
  "download?: boolean",
  "distributions:",
  "routeBindings:",
]);

requireTokens("脚手架和可回滚工程化", scaffold + fullVerification + docs, [
  "check-pack-runtime-completion.mjs",
  "check-pack-runtime-all.mjs",
  "scaffold-pack.mjs",
  "--dry-run",
  "--json",
  "check-pack-scaffold.mjs",
  "distribution.packageUrl",
  "update.rollback=true",
  "GatewayConfig.BackendPacks",
  "RegisterBackendPack",
]);

runCheck("contract checker", process.execPath, ["scripts/check-pack-contract.mjs"]);
runCheck("scaffold checker", process.execPath, ["scripts/check-pack-scaffold.mjs"]);
runCheck("packs sdk checker", process.execPath, ["sdk/scripts/check-packs-sdk-manifest.mjs"]);

if (failures.length > 0) {
  console.error("Pack Runtime completion audit failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log("Pack Runtime completion audit ok:");
for (const item of evidence) console.log(`- ${item.item}: ${item.message}`);
