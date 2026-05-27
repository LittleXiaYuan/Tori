#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

const root = process.cwd();
const failures = [];

function read(rel) {
  const abs = path.join(root, rel);
  if (!fs.existsSync(abs)) {
    failures.push(`missing file: ${rel}`);
    return "";
  }
  return fs.readFileSync(abs, "utf8");
}

function mustInclude(rel, needles) {
  const text = read(rel);
  for (const needle of needles) {
    if (!text.includes(needle)) {
      failures.push(`${rel} missing ${JSON.stringify(needle)}`);
    }
  }
}

function mustMatch(rel, patterns) {
  const text = read(rel);
  for (const pattern of patterns) {
    if (!pattern.test(text)) {
      failures.push(`${rel} does not match ${pattern}`);
    }
  }
}

const docPath = "doc/TORI-CURRENT-STATE.md";
mustInclude(docPath, [
  "integration boundary",
  "the full hosted enterprise control-plane implementation",
  "已确认实现的能力",
  "未实现或仍依赖外部 Tori",
  "源码地图",
  "验证命令",
  "node scripts/check-tori-current-state.mjs",
]);

mustInclude("README.md", ["Tori 做控制面", "团队", "企业", "Control Plane"]);
mustInclude("internal/controlplane/gateway/handlers_tori.go", [
  "validateToriURL", "TORI_URL_ALLOWLIST", "POST /v1/tori/bind", "GET /v1/tori/status", "POST /v1/tori/unbind", "GET /v1/tori/health", "GET /v1/tori/usage",
]);
mustInclude("internal/controlplane/gateway/routes_system.go", [
  '"/v1/tori/bind"', '"/v1/tori/status"', '"/v1/tori/unbind"', '"/v1/tori/health"', '"/v1/tori/usage"',
]);
mustMatch("internal/controlplane/gateway/routes_system.go", [
  /\/v1\/tori\/bind",\s*g\.requireAuth\(g\.handleToriBind\)/,
  /\/v1\/tori\/status",\s*g\.requireAuth\(g\.handleToriStatus\)/,
  /\/v1\/tori\/unbind",\s*g\.requireAuth\(g\.handleToriUnbind\)/,
  /\/v1\/tori\/health",\s*g\.requireAuth\(g\.handleToriHealth\)/,
  /\/v1\/tori\/usage",\s*g\.requireAuth\(g\.handleToriUsage\)/,
]);

mustInclude("internal/tori/config.go", ["DefaultToriURL", "ApplyLLMConfig", "RestoreLLMConfig"]);
mustInclude("internal/tori/oauth.go", ["PKCE", "OAuth"]);
mustInclude("internal/tori/token.go", ["TokenStore", "tori_token.json"]);
mustInclude("internal/tori/discover.go", ["/api/health", "/api/usage/summary", "/v1/models"]);
mustInclude("internal/tori/sync.go", ["/api/sync/push", "/api/sync/pull", "/api/sync/status"]);
mustInclude("internal/controlplane/gateway/handlers_providers.go", ["handleToriDiscover", "ProviderModeTori", "ProviderModeHybrid", "DiscoverModels"]);
mustInclude("internal/controlplane/gateway/handlers_auth.go", ["/v1/auth/oauth/tori", "handleOAuthToriStart", "handleOAuthToriCallback"]);
mustInclude("apps/web/src/app/setup/page.tsx", ["DEFAULT_TORI_URL", "toriBind", "toriStatus"]);
mustInclude("apps/web/src/app/login/page.tsx", ["Login with Tori", "/v1/auth/oauth/tori"]);
mustInclude("apps/web/src/app/settings/providers/page.tsx", ["toriBind", "toriHealth", "toriUsage", "toriDiscover"]);
mustInclude("sdk/manifest/tori-sdk.json", ["POST /v1/tori/bind", "GET /v1/tori/status", "POST /v1/tori/unbind", "GET /v1/tori/health", "GET /v1/tori/usage"]);
mustInclude("packages/yunque-client/src/tori.ts", ["ToriClient", "/v1/tori/bind", "/v1/tori/status"]);
mustInclude("packages/yunque-client/src/tori-observe.ts", ["ToriObserveClient", "health", "usage"]);
mustInclude("packages/yunque-client/src/tori-bind.ts", ["ToriBindClient", "bind", "unbind"]);
mustInclude("internal/agentcore/skillmarket/torihub.go", ["ToriHubProvider", "Search", "Fetch", "Trending"]);

if (failures.length > 0) {
  console.error("Tori current-state check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log("Tori current-state check passed.");
console.log("Verified doc/TORI-CURRENT-STATE.md against README, Gateway, internal/tori, frontend, SDK, and ToriHub evidence.");
