import { spawnSync } from "node:child_process";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const checks = [
  ["Auth SDK manifest", "sdk/scripts/check-auth-sdk-manifest.mjs"],
  ["System SDK manifest", "sdk/scripts/check-system-sdk-manifest.mjs"],
  ["Settings SDK manifest", "sdk/scripts/check-settings-sdk-manifest.mjs"],
  ["Backup SDK manifest", "sdk/scripts/check-backup-sdk-manifest.mjs"],
  ["Tori SDK manifest", "sdk/scripts/check-tori-sdk-manifest.mjs"],
  ["Speech SDK manifest", "sdk/scripts/check-speech-sdk-manifest.mjs"],
  ["Setup SDK manifest", "sdk/scripts/check-setup-sdk-manifest.mjs"],
  ["Admin SDK manifest", "sdk/scripts/check-admin-sdk-manifest.mjs"],
  ["Federation SDK manifest", "sdk/scripts/check-federation-sdk-manifest.mjs"],
  ["Planner SDK manifest", "sdk/scripts/check-planner-sdk-manifest.mjs"],
  ["IDE SDK manifest", "sdk/scripts/check-ide-sdk-manifest.mjs"],
  ["Tasks SDK manifest", "sdk/scripts/check-tasks-sdk-manifest.mjs"],
  ["Permissions SDK manifest", "sdk/scripts/check-permissions-sdk-manifest.mjs"],
  ["Reactions SDK manifest", "sdk/scripts/check-reactions-sdk-manifest.mjs"],
  ["Instructions SDK manifest", "sdk/scripts/check-instructions-sdk-manifest.mjs"],
  ["Emotion SDK manifest", "sdk/scripts/check-emotion-sdk-manifest.mjs"],
  ["Audit SDK manifest", "sdk/scripts/check-audit-sdk-manifest.mjs"],
  ["Trust SDK manifest", "sdk/scripts/check-trust-sdk-manifest.mjs"],
  ["Iterate SDK manifest", "sdk/scripts/check-iterate-sdk-manifest.mjs"],
  ["Persona SDK manifest", "sdk/scripts/check-persona-sdk-manifest.mjs"],
  ["State SDK manifest", "sdk/scripts/check-state-sdk-manifest.mjs"],
  ["Reflect SDK manifest", "sdk/scripts/check-reflect-sdk-manifest.mjs"],
  ["Mission Parse SDK manifest", "sdk/scripts/check-mission-parse-sdk-manifest.mjs"],
  ["Scheduler SDK manifest", "sdk/scripts/check-scheduler-sdk-manifest.mjs"],
  ["Cron SDK manifest", "sdk/scripts/check-cron-sdk-manifest.mjs"],
  ["Triggers SDK manifest", "sdk/scripts/check-triggers-sdk-manifest.mjs"],
  ["Memory SDK manifest", "sdk/scripts/check-memory-sdk-manifest.mjs"],
  ["Graph SDK manifest", "sdk/scripts/check-graph-sdk-manifest.mjs"],
  ["Knowledge SDK manifest", "sdk/scripts/check-knowledge-sdk-manifest.mjs"],
  ["LoRA SDK manifest", "sdk/scripts/check-lora-sdk-manifest.mjs"],
  ["Workflow SDK manifest", "sdk/scripts/check-workflow-sdk-manifest.mjs"],
  ["Connectors SDK manifest", "sdk/scripts/check-connectors-sdk-manifest.mjs"],
  ["Notify SDK manifest", "sdk/scripts/check-notify-sdk-manifest.mjs"],
  ["Projects SDK manifest", "sdk/scripts/check-projects-sdk-manifest.mjs"],
  ["Skill Market SDK manifest", "sdk/scripts/check-market-sdk-manifest.mjs"],
  ["Dispatch SDK manifest", "sdk/scripts/check-dispatch-sdk-manifest.mjs"],
  ["Orchestrator SDK manifest", "sdk/scripts/check-orchestrator-sdk-manifest.mjs"],
  ["Fork SDK manifest", "sdk/scripts/check-fork-sdk-manifest.mjs"],
  ["Cost SDK manifest", "sdk/scripts/check-cost-sdk-manifest.mjs"],
  ["Providers SDK manifest", "sdk/scripts/check-providers-sdk-manifest.mjs"],
  ["Cognis SDK manifest", "sdk/scripts/check-cognis-sdk-manifest.mjs"],
  ["Trace SDK manifest", "sdk/scripts/check-trace-sdk-manifest.mjs"],
  ["Heartbeat SDK manifest", "sdk/scripts/check-heartbeat-sdk-manifest.mjs"],
  ["Events SDK manifest", "sdk/scripts/check-events-sdk-manifest.mjs"],
  ["Runtime SDK manifest", "sdk/scripts/check-runtime-sdk-manifest.mjs"],
  ["Subagents SDK manifest", "sdk/scripts/check-subagents-sdk-manifest.mjs"],
  ["Tools SDK manifest", "sdk/scripts/check-tools-sdk-manifest.mjs"],
  ["Reverie SDK manifest", "sdk/scripts/check-reverie-sdk-manifest.mjs"],
  ["Realtime SDK manifest", "sdk/scripts/check-realtime-sdk-manifest.mjs"],
  ["Chat SDK manifest", "sdk/scripts/check-chat-sdk-manifest.mjs"],
  ["Conversations SDK manifest", "sdk/scripts/check-conversations-sdk-manifest.mjs"],
  ["Approvals SDK manifest", "sdk/scripts/check-approvals-sdk-manifest.mjs"],
  ["RBAC SDK manifest", "sdk/scripts/check-rbac-sdk-manifest.mjs"],
  ["Files SDK manifest", "sdk/scripts/check-files-sdk-manifest.mjs"],
  ["Browser SDK manifest", "sdk/scripts/check-browser-sdk-manifest.mjs"],
  ["Plugin API SDK manifest", "sdk/scripts/check-plugin-api-sdk-manifest.mjs"],
  ["Agent Kit SDK manifest", "sdk/scripts/check-agent-kit-sdk-manifest.mjs"],
];

for (const [name, script] of checks) {
  console.log(`\n=== ${name} ===`);
  const result = spawnSync(process.execPath, [resolve(repoRoot, script)], {
    cwd: repoRoot,
    stdio: "inherit",
  });
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

console.log(`\nSDK manifest suite ok: ${checks.length} domains`);
