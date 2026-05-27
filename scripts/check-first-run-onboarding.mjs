import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "..");

const checks = [
  {
    label: "Windows Inno installer script",
    path: "installer/yunque.iss",
    kind: "file",
    includes: [
      "..\\dist\\{#MyAppExeName}",
      "..\\apps/web\\out\\*",
      "[Run]",
      "postinstall",
      "autostart",
    ],
  },
  {
    label: "Web setup wizard page",
    path: "apps/web/src/app/setup/page.tsx",
    kind: "file",
    includes: [
      "STEP_CHOOSE",
      "STEP_DETECT",
      "STEP_MODEL",
      "STEP_TEMPLATE",
      "STEP_DONE",
      "api.setupDetect",
      "api.setupTemplates",
      "api.setupTestProvider",
      "api.setupApply",
      "setup.detect.firstRun",
    ],
  },
  {
    label: "Setup backend handlers",
    path: "internal/controlplane/gateway/handlers_setup.go",
    kind: "file",
    includes: [
      "handleSetupDetect",
      "handleSetupTemplates",
      "handleSetupTestProvider",
      "handleSetupApply",
      "template_id",
      "base_url",
      "model",
    ],
  },
  {
    label: "Setup HTTP routes",
    path: "internal/controlplane/gateway/routes.go",
    kind: "file",
    includes: [
      "/v1/setup/detect",
      "/v1/setup/templates",
      "/v1/setup/test-provider",
      "/v1/setup/apply",
      "/v1/setup/install-component",
    ],
  },
  {
    label: "Agent first-run bootstrap contract",
    path: "cmd/agent/bootstrap.go",
    kind: "file",
    includes: ["web UI (/setup)", "starts even when .env is missing", "/v1/setup/*"],
  },
  {
    label: "Installer onboarding audit doc",
    path: "docs/INSTALLER-ONBOARDING-AUDIT.md",
    kind: "file",
    includes: ["5 分钟到第一次对话", "workload feedback", "卡点", "node scripts/check-first-run-onboarding.mjs"],
  },
];

function exists(abs, kind) {
  if (!fs.existsSync(abs)) return false;
  const stat = fs.statSync(abs);
  return kind === "dir" ? stat.isDirectory() : stat.isFile();
}

let failed = false;
const passed = [];
for (const check of checks) {
  const abs = path.join(ROOT, check.path);
  if (!exists(abs, check.kind)) {
    console.error(`first-run onboarding check failed: missing ${check.label}: ${check.path}`);
    failed = true;
    continue;
  }
  if (check.kind === "file") {
    const text = fs.readFileSync(abs, "utf8");
    for (const token of check.includes ?? []) {
      if (!text.includes(token)) {
        console.error(`first-run onboarding check failed: ${check.path} missing token ${JSON.stringify(token)}`);
        failed = true;
      }
    }
  }
  passed.push(check.label);
}

const firstConversationSteps = [
  "Run the Windows installer or start the packaged desktop app.",
  "Launch Yunque Agent; the gateway must not block on a missing .env.",
  "Open the local Web UI and enter /setup.",
  "Choose Tori bind for the short path, or choose API Key for manual setup.",
  "Run environment detection and confirm first-run hints are visible.",
  "Enter provider Base URL, API Key if needed, and model name.",
  "Test provider connectivity from the backend.",
  "Pick a scenario template.",
  "Apply configuration; restart the agent if runtime env reload is not available.",
  "Open /chat and send the first prompt.",
];

const feedback = {
  findability_30s: "partial",
  estimated_minutes_to_first_chat: 5,
  smoothest_part: "The modern /setup flow is discoverable from the running gateway and covers provider test plus templates.",
  least_smooth_part: "The legacy Inno installer and the newer Tauri desktop release path are both present, so release docs must state which package users should install.",
  next_step_to_remove: "Avoid requiring a manual restart after /v1/setup/apply by adding safe runtime config reload or an explicit one-click restart action.",
};

if (failed) process.exit(1);

console.log("first-run onboarding check ok");
console.log(`passed checks: ${passed.join(", ")}`);
console.log("5-minute first conversation path:");
firstConversationSteps.forEach((step, index) => console.log(`${index + 1}. ${step}`));
console.log("workload feedback self-review:");
console.log(JSON.stringify(feedback, null, 2));
