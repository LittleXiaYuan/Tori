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
  "internal/controlplane/gateway/handlers_cogni.go",
  "internal/controlplane/gateway/handlers_browser_pack.go",
  "internal/controlplane/gateway/handlers_browser_pack_test.go",
  "internal/controlplane/gateway/handlers_rpa_replay_pack_test.go",
  "internal/controlplane/gateway/handlers_sbom_drift_pack_test.go",
  "internal/controlplane/gateway/handlers_cogni_experience_test.go",
  "cmd/agent/init_tasks.go",
].map(read).join("\n");
const backup = read("internal/packs/backup/handler.go");
const backupManifest = read("packs/examples/backup-pack/pack.json");
const loraPack = read("internal/packs/lora/handler.go");
const loraManifest = read("packs/examples/lora-pack/pack.json");
const cogniKernelPack = read("internal/packs/cognikernel/handler.go");
const cogniKernelManifest = read("packs/examples/cogni-kernel-pack/pack.json");
const browserIntentPack = read("internal/packs/browserintent/handler.go");
const browserIntentManifest = read("packs/examples/browser-intent-pack/pack.json");
const rpaReplayPack = read("internal/packs/rpareplay/handler.go");
const rpaReplayManifest = read("packs/examples/rpa-replay-pack/pack.json");
const sbomDriftPack = read("internal/packs/sbomdrift/handler.go");
const sbomDriftManifest = read("packs/examples/sbom-drift-pack/pack.json");
const scaffold = read("scripts/scaffold-pack.mjs") + "\n" + read("scripts/check-pack-scaffold.mjs");
const fullVerification = read("scripts/check-pack-runtime-all.mjs");
const frontend = [
  "heroui-web/src/lib/pack-sync.tsx",
  "heroui-web/src/lib/__tests__/pack-sync.test.ts",
  "heroui-web/src/lib/packs-client.ts",
  "heroui-web/src/lib/__tests__/packs-client.test.ts",
  "heroui-web/src/app/packs/page.tsx",
  "heroui-web/src/app/packs/[...slug]/page.tsx",
  "heroui-web/src/app/packs/backup/page.tsx",
  "heroui-web/src/app/packs/lora/page.tsx",
  "heroui-web/src/app/packs/cognis/page.tsx",
  "heroui-web/src/components/cherry/__tests__/settings-modal-pack-entry.test.tsx",
  "heroui-web/src/lib/backup-pack-client.ts",
  "heroui-web/src/lib/__tests__/backup-pack-client.test.ts",
  "heroui-web/src/lib/lora-pack-client.ts",
  "heroui-web/src/lib/__tests__/lora-pack-client.test.ts",
  "heroui-web/src/lib/cogni-kernel-pack-client.ts",
  "heroui-web/src/lib/__tests__/cogni-kernel-pack-client.test.ts",
  "heroui-web/src/app/packs/browser/page.tsx",
  "heroui-web/src/lib/browser-intent-pack-client.ts",
  "heroui-web/src/lib/__tests__/browser-intent-pack-client.test.ts",
  "heroui-web/src/app/packs/rpa-replay/page.tsx",
  "heroui-web/src/lib/rpa-replay-pack-client.ts",
  "heroui-web/src/lib/__tests__/rpa-replay-pack-client.test.ts",
  "heroui-web/src/app/packs/sbom-drift/page.tsx",
  "heroui-web/src/lib/sbom-drift-pack-client.ts",
  "heroui-web/src/lib/__tests__/sbom-drift-pack-client.test.ts",
  "heroui-web/src/lib/pack-types.ts",
  "heroui-web/src/lib/api.ts",
  "heroui-web/src/lib/api-types/skills.ts",
].map(read).join("\n");
const legacyBackupPage = read("heroui-web/src/app/backup/page.tsx");
const backupPackPage = read("heroui-web/src/app/packs/backup/page.tsx");
const legacyLoRAPage = read("heroui-web/src/app/lora/page.tsx");
const loraPackPage = read("heroui-web/src/app/packs/lora/page.tsx");
const legacyCogniPage = read("heroui-web/src/app/cognis/page.tsx");
const cogniPackPage = read("heroui-web/src/app/packs/cognis/page.tsx");
const legacyBrowserPage = read("heroui-web/src/app/browser/page.tsx");
const browserPackPage = read("heroui-web/src/app/packs/browser/page.tsx");
const rpaReplayPackPage = read("heroui-web/src/app/packs/rpa-replay/page.tsx");
const sbomDriftPackPage = read("heroui-web/src/app/packs/sbom-drift/page.tsx");
const frontendShell = [
  "heroui-web/src/components/sidebar.tsx",
  "heroui-web/src/lib/nav-items.tsx",
  "heroui-web/src/components/command-palette.tsx",
].map(read).join("\n");
const sdk = [
  "sdk/typescript/src/packs.ts",
  "sdk/typescript/src/packs.test.ts",
  "sdk/manifest/packs-sdk.json",
  "sdk/manifest/lora-pack-sdk.json",
  "sdk/manifest/cogni-kernel-pack-sdk.json",
  "sdk/manifest/browser-intent-pack-sdk.json",
  "sdk/manifest/rpa-replay-pack-sdk.json",
  "sdk/manifest/sbom-drift-pack-sdk.json",
  "sdk/typescript/src/rpa-replay.ts",
  "sdk/typescript/src/rpa-replay.test.ts",
  "sdk/typescript/src/sbom-drift.ts",
  "sdk/typescript/src/sbom-drift.test.ts",
  "sdk/scripts/check-packs-sdk-manifest.mjs",
  "sdk/scripts/check-lora-pack-sdk-manifest.mjs",
  "sdk/scripts/check-cogni-kernel-pack-sdk-manifest.mjs",
  "sdk/scripts/check-browser-intent-pack-sdk-manifest.mjs",
  "sdk/scripts/check-rpa-replay-pack-sdk-manifest.mjs",
  "sdk/scripts/check-sbom-drift-pack-sdk-manifest.mjs",
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
  "BackendRouteSpec",
  "AllowsRoute",
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
  "ensureBuiltinPacks",
  "loadBuiltinPackManifest",
  "packs/examples/lora-pack/pack.json",
  "packs/examples/cogni-kernel-pack/pack.json",
  "packs/examples/browser-intent-pack/pack.json",
  "packs/examples/rpa-replay-pack/pack.json",
  "packs/examples/sbom-drift-pack/pack.json",
]);

requireTokens("后端 backend pack module registry / route gates", backend + gateway, [
  "type BackendModule interface",
  "PackID() string",
  "Routes() []BackendRoute",
  "RegisterBackendPack",
  "BackendPacks",
  "backendPackRoutes",
  "backendPackRouteInfos",
  "backendPackAuth",
  "BackendRouteAuthPassthrough",
  "requirePackRoute",
  "packRouteEnabled",
  "normalizeBackendRouteMethods",
  "Methods: methods",
  "http.StatusMethodNotAllowed",
  "route conflict",
  "handlePackBackendModules",
  "backuppack.DefaultHandler()",
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

requireTokens("lora-pack 蓝图能力包", loraPack + loraManifest + frontend, [
  "const PackID = \"yunque.pack.lora\"",
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/lora/status",
  "/v1/lora/trigger",
  "/v1/lora/config",
  "http.MethodPatch",
  "yunque-client/lora",
  "LoRA / LAA Evolution Pack",
  "createLoRAPackClient",
  "pack route is not enabled",
  "lora-pack-client",
  "/packs/lora",
  "distribution",
  "rollback",
]);

requireTokens("cogni-kernel 蓝图能力包", cogniKernelPack + cogniKernelManifest + frontend + gateway, [
  'const PackID = "yunque.pack.cogni-kernel"',
  "type CogniGateway interface",
  "HandleCogniKernelPack",
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/cognis",
  "/v1/cognis/",
  "http.MethodDelete",
  "yunque-client/cognis",
  "Cogni Kernel Pack",
  "createCogniKernelPackClient",
  "cogni-kernel-pack-client",
  "TestCogniKernelPackGateReturnsNotFoundWhenDisabled",
  "/packs/cognis",
  "distribution",
  "rollback",
]);

requireTokens("browser-intent 蓝图能力包", browserIntentPack + browserIntentManifest + frontend + gateway, [
  'const PackID = "yunque.pack.browser-intent"',
  "type BrowserGateway interface",
  "HandleBrowserIntentPack",
  "HandleBrowserIntentSession",
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/browser/status",
  "/api/browser/ext/session",
  "/api/browser/ext/scenarios/run",
  "http.MethodPost",
  "yunque-client/browser",
  "Browser Intent Pack",
  "createBrowserIntentPackClient",
  "browser-intent-pack-client",
  "TestBrowserIntentPackGateReturnsNotFoundWhenDisabled",
  "/packs/browser",
  "distribution",
  "rollback",
]);

requireTokens("rpa-replay 蓝图能力包", rpaReplayPack + rpaReplayManifest + frontend + gateway + sdk, [
  'const PackID = "yunque.pack.rpa-replay"',
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/rpa-replay/status",
  "/v1/rpa-replay/traces",
  "/v1/rpa-replay/recordings/start",
  "/v1/rpa-replay/recordings/stop",
  "/v1/rpa-replay/replay",
  "/v1/rpa-replay/evidence/",
  "executor_ready",
  "cfg.DataPath(\"rpa-replay\")",
  "rpa.replay.dry_run",
  "json-evidence-pack",
  "http.MethodPost",
  "yunque-client/rpa-replay",
  "RPA Replay Pack",
  "createRPAReplayPackClient",
  "createRPAReplayClient",
  "rpa-replay-pack-client",
  "TestRPAReplayPackGateReturnsNotFoundWhenDisabled",
  "/packs/rpa-replay",
  "pack-shell-before-executor",
  "distribution",
  "rollback",
]);

requireTokens("sbom-drift 蓝图能力包", sbomDriftPack + sbomDriftManifest + frontend + gateway + sdk, [
  'const PackID = "yunque.pack.sbom-drift"',
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/sbom-drift/status",
  "/v1/sbom-drift/snapshots",
  "/v1/sbom-drift/diff",
  "/v1/sbom-drift/evidence/",
  "scanner_ready",
  "cfg.DataPath(\"sbom-drift\")",
  "json-sbom-drift-evidence",
  "http.MethodPost",
  "yunque-client/sbom-drift",
  "SBOM Drift Pack",
  "createSBOMDriftPackClient",
  "createSBOMDriftClient",
  "sbom-drift-pack-client",
  "TestSBOMDriftPackGateReturnsNotFoundWhenDisabled",
  "/packs/sbom-drift",
  "pack-shell-before-ci",
  "distribution",
  "rollback",
]);

requireTokens("前端同步菜单/路由/资源/控制台", frontend + fullVerification, [
  "fetchEnabledPacks",
  "buildPackNavItems",
  "buildPackRouteBindings",
  "findPackRouteBinding",
  "pack-sync frontend runtime",
  "createPacksClient",
  "packs-client",
  "pack-types",
  "createBackupPackClient",
  "backup-pack-client",
  "Frontend Pack sync tests",
  "Frontend packs client tests",
  "Frontend backup pack client tests",
  "Frontend LoRA pack client tests",
  "Frontend Cogni Kernel pack client tests",
  "Frontend Browser Intent pack client tests",
  "Frontend RPA Replay pack client tests",
  "Frontend SBOM Drift pack client tests",
  "Frontend shell pack entry tests",
  "PackRuntimeRoutePage",
  "enabled()",
  "frontend?.menus",
  "frontend?.routes",
  "routeSpecs",
  "buildPackBackendRouteBindings",
  "/v1/packs/enabled",
  "manifest.distribution",
  "backendModules()",
  "installFromURL",
  "downloadArtifact",
  "prune()",
  "previousArtifacts",
  "SDK 调用入口",
]);


if (frontendShell.includes('href: "/backup"') || frontendShell.includes('href: "/backup"') || frontendShell.includes('nav-backup')) {
  fail("前端同步菜单/路由/资源/控制台", "backup pack must not be exposed as a hard-coded main-shell nav item; use /v1/packs/enabled pack sync");
} else {
  ok("前端轻内核导航", "backup entry is not hard-coded in sidebar/nav-items/command-palette");
}

if (backupPackPage.includes("api.backupInfo") || backupPackPage.includes("api.backupExport") || backupPackPage.includes("api.backupImport") || backupPackPage.includes('from "@/lib/api"')) {
  fail("前端同步菜单/路由/资源/控制台", "backup pack page must use backup-pack-client instead of the monolithic api object");
} else {
  ok("前端 pack 客户端拆分", "backup page uses backup-pack-client instead of monolithic api backup methods");
}

if (!legacyLoRAPage.includes('redirect("/packs/lora")')) {
  fail("前端同步菜单/路由/资源/控制台", "legacy /lora page must redirect to the LoRA pack route");
} else {
  ok("前端 LoRA pack 路由兼容", "legacy /lora redirects to /packs/lora");
}

if (frontendShell.includes('href: "/lora"') || frontendShell.includes('nav-lora')) {
  fail("前端同步菜单/路由/资源/控制台", "LoRA must not be exposed as a hard-coded main-shell nav item; use enabled-pack sync");
} else {
  ok("前端轻内核导航", "LoRA entry is not hard-coded in sidebar/nav-items/command-palette");
}

if (loraPackPage.includes("api.getLoRA") || loraPackPage.includes("api.triggerLoRA") || loraPackPage.includes("api.rollbackLoRA") || loraPackPage.includes('from "@/lib/api"')) {
  fail("前端同步菜单/路由/资源/控制台", "LoRA pack page must use lora-pack-client instead of the monolithic api object");
} else {
  ok("前端 LoRA pack 客户端拆分", "LoRA page uses lora-pack-client instead of monolithic api LoRA methods");
}

if (!legacyCogniPage.includes('redirect("/packs/cognis")')) {
  fail("前端同步菜单/路由/资源/控制台", "legacy /cognis page must redirect to the Cogni Kernel pack route");
} else {
  ok("前端 Cogni Kernel pack 路由兼容", "legacy /cognis redirects to /packs/cognis");
}

if (frontendShell.includes('href: "/cognis"') || frontendShell.includes("nav-cognis")) {
  fail("前端同步菜单/路由/资源/控制台", "Cogni Kernel must not be exposed as a hard-coded main-shell nav item; use enabled-pack sync");
} else {
  ok("前端轻内核导航", "Cogni Kernel entry is not hard-coded in sidebar/nav-items/command-palette");
}

if (cogniPackPage.includes("api.listCognis") || cogniPackPage.includes("api.reloadCognis") || cogniPackPage.includes("api.triggerCogniEvolution") || cogniPackPage.includes('from "@/lib/api"')) {
  fail("前端同步菜单/路由/资源/控制台", "Cogni Kernel pack page must use cogni-kernel-pack-client instead of the monolithic api object");
} else {
  ok("前端 Cogni Kernel pack 客户端拆分", "Cogni Kernel page uses cogni-kernel-pack-client instead of monolithic api Cogni methods");
}

if (!legacyBrowserPage.includes('redirect("/packs/browser")')) {
  fail("前端同步菜单/路由/资源/控制台", "legacy /browser page must redirect to the Browser Intent pack route");
} else {
  ok("前端 Browser Intent pack 路由兼容", "legacy /browser redirects to /packs/browser");
}

if (frontendShell.includes('href: "/browser"') || frontendShell.includes("nav-browser")) {
  fail("前端同步菜单/路由/资源/控制台", "Browser Intent must not be exposed as a hard-coded main-shell nav item; use enabled-pack sync");
} else {
  ok("前端轻内核导航", "Browser Intent entry is not hard-coded in sidebar/nav-items/command-palette");
}

if (browserPackPage.includes("api.browser") || browserPackPage.includes('from "@/lib/api"') || !browserPackPage.includes("createBrowserIntentPackClient")) {
  fail("前端同步菜单/路由/资源/控制台", "Browser Intent pack page must use browser-intent-pack-client instead of the monolithic api object");
} else {
  ok("前端 Browser Intent pack 客户端拆分", "Browser Intent page uses browser-intent-pack-client instead of monolithic api browser methods");
}

if (rpaReplayPackPage.includes("api.rpa") || rpaReplayPackPage.includes('from "@/lib/api"') || !rpaReplayPackPage.includes("createRPAReplayPackClient")) {
  fail("前端同步菜单/路由/资源/控制台", "RPA Replay pack page must use rpa-replay-pack-client instead of the monolithic api object");
} else {
  ok("前端 RPA Replay pack 客户端拆分", "RPA Replay page uses rpa-replay-pack-client instead of monolithic api RPA methods");
}

if (sbomDriftPackPage.includes("api.sbom") || sbomDriftPackPage.includes('from "@/lib/api"') || !sbomDriftPackPage.includes("createSBOMDriftPackClient")) {
  fail("前端同步菜单/路由/资源/控制台", "SBOM Drift pack page must use sbom-drift-pack-client instead of the monolithic api object");
} else {
  ok("前端 SBOM Drift pack 客户端拆分", "SBOM Drift page uses sbom-drift-pack-client instead of monolithic api SBOM methods");
}

const packsConsolePage = read("heroui-web/src/app/packs/page.tsx");
if (packsConsolePage.includes("api.packsInstalled") || packsConsolePage.includes("api.packBackendModules") || packsConsolePage.includes("api.packInstall") || packsConsolePage.includes("api.packEnable") || packsConsolePage.includes("api.packDisable") || packsConsolePage.includes("api.packRollback") || packsConsolePage.includes("api.packPrune")) {
  fail("前端同步菜单/路由/资源/控制台", "Pack console must use packs-client instead of monolithic api pack methods");
} else {
  ok("前端 Pack Runtime 客户端拆分", "Pack console uses packs-client instead of monolithic api pack methods");
}

const monolithicApi = read("heroui-web/src/lib/api.ts");
const cherrySettings = read("heroui-web/src/components/cherry/settings-modal.tsx");
const forbiddenMonolithicPackMethods = [
  "backupInfo:",
  "backupExport:",
  "backupImport:",
  "packsInstalled:",
  "packsEnabled:",
  "packBackendModules:",
  "packInstall:",
  "packInstallFromURL:",
  "packEnable:",
  "packDisable:",
  "packRollback:",
  "packPrune:",
  "getLoRAStatus:",
  "getLoRAHistory:",
  "getLoRASummary:",
  "previewLoRATrainingData:",
  "triggerLoRATraining:",
  "rollbackLoRA:",
  "getEvolutionState:",
  "getLoRAConfig:",
  "updateLoRAConfig:",
  "listCognis:",
  "getCogni:",
  "addCogni:",
  "removeCogni:",
  "setCogniEnabled:",
  "reloadCognis:",
  "getCogniHealth:",
  "getCogniAlerts:",
  "scanCogniAlerts:",
  "verifyCognis:",
  "getCogniTraces:",
  "getCogniTracesByID:",
  "generateCogni:",
  "getCogniWorkflows:",
  "runCogniWorkflow:",
  "getCogniExperience:",
  "confirmCogniExperiencePattern:",
  "triggerCogniEvolution:",
  "getCogniEvolution:",
  "getCogniFederation:",
  "exposeCogni:",
  "browserNavigate:",
  "browserScreenshot:",
  "browserOcr:",
  "browserStatus:",
  "browserConfig:",
  "browserScreenshotLatest:",
  "browserOPPPending:",
  "browserOPPDecide:",
  "browserExtStatus:",
  "browserExtAction:",
  "browserExtScenarios:",
  "browserExtRunScenario:",
  "rpaReplayStatus:",
  "createRPAReplayTrace:",
  "rpaReplay:",
  "rpaReplayEvidence:",
  "sbomDriftStatus:",
  "createSBOMDriftSnapshot:",
  "sbomDriftDiff:",
  "sbomDriftEvidence:",
];
const leakedMonolithicMethods = forbiddenMonolithicPackMethods.filter((token) => monolithicApi.includes(token));
if (leakedMonolithicMethods.length > 0) {
  fail("前端轻内核 API 拆分", `monolithic api.ts still exposes pack methods: ${leakedMonolithicMethods.join(", ")}`);
} else {
  ok("前端轻内核 API 拆分", "backup/pack/browser/rpa/sbom methods live in lightweight clients instead of monolithic api.ts");
}

if (cherrySettings.includes("createBackupPackClient") || cherrySettings.includes("backupPack.export") || cherrySettings.includes("api.backup")) {
  fail("前端轻内核设置入口", "Cherry settings must not execute backup pack actions directly; link to Pack Runtime instead");
} else if (!cherrySettings.includes("BACKUP_PACK_ID") || !cherrySettings.includes('"/packs/backup"') || !cherrySettings.includes("packsClient") || !cherrySettings.includes(".installed()")) {
  fail("前端轻内核设置入口", "Cherry settings must expose backup as a pack-registry-aware entrypoint");
} else {
  ok("前端轻内核设置入口", "Cherry settings links to backup pack through pack registry state");
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
  "PackBackendRouteBinding",
  "PackDistributionManifest",
  "PackPruneResponse",
  "createRPAReplayClient",
  "RPAReplayClientError",
  "createSBOMDriftClient",
  "SBOMDriftClientError",
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
  "-pack-client.ts",
  "-pack-client.test.ts",
  "frontend.menus[].path",
  "frontend.routes[].path",
  "distribution.packageUrl",
  "update.rollback=true",
  "GatewayConfig.BackendPacks",
  "RegisterBackendPack",
]);

runCheck("contract checker", process.execPath, ["scripts/check-pack-contract.mjs"]);
runCheck("scaffold checker", process.execPath, ["scripts/check-pack-scaffold.mjs"]);
runCheck("packs sdk checker", process.execPath, ["sdk/scripts/check-packs-sdk-manifest.mjs"]);
runCheck("lora pack sdk checker", process.execPath, ["sdk/scripts/check-lora-pack-sdk-manifest.mjs"]);
runCheck("cogni kernel pack sdk checker", process.execPath, ["sdk/scripts/check-cogni-kernel-pack-sdk-manifest.mjs"]);
runCheck("browser intent pack sdk checker", process.execPath, ["sdk/scripts/check-browser-intent-pack-sdk-manifest.mjs"]);
runCheck("rpa replay pack sdk checker", process.execPath, ["sdk/scripts/check-rpa-replay-pack-sdk-manifest.mjs"]);
runCheck("sbom drift pack sdk checker", process.execPath, ["sdk/scripts/check-sbom-drift-pack-sdk-manifest.mjs"]);

if (failures.length > 0) {
  console.error("Pack Runtime completion audit failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log("Pack Runtime completion audit ok:");
for (const item of evidence) console.log(`- ${item.item}: ${item.message}`);
