export interface WorkloadPreset {
  id: string;
  title: string;
  subtitle: string;
  description: string;
  capabilities: string[];
}

export type WorkloadFeedbackFindability = "yes" | "partial" | "no" | "unknown";

export interface WorkloadFeedbackDraft {
  triedScenario: string;
  mostUseful: string;
  friction: string;
  nextStepToRemove: string;
  foundIn30Seconds: WorkloadFeedbackFindability;
}

export interface WorkloadFeedbackEntry extends WorkloadFeedbackDraft {
  id: string;
  workloadId: string;
  workloadTitle: string;
  capabilities: string[];
  createdAt: string;
}

export const WORKLOAD_FEEDBACK_STORAGE_KEY = "yunque_workload_feedback_v1";

export const WORKLOAD_PRESETS: WorkloadPreset[] = [
  {
    id: "browser-rpa",
    title: "浏览器 / RPA",
    subtitle: "像 Visual Studio workload 一样把常用能力打包",
    description: "把浏览器意图、回放和扩展场景合成一个用户可选的工作负载。",
    capabilities: ["browser.intent.plan", "rpa.replay.dry_run", "rpa.executor.plan"],
  },
  {
    id: "memory-review",
    title: "记忆 / 回溯",
    subtitle: "适合复盘、验收和长期事实沉淀",
    description: "把快照、差异、回滚计划和审计验证放到一条复盘路径里。",
    capabilities: [
      "memory_time_travel.snapshot_at",
      "memory_time_travel.diff",
      "memory_time_travel.rollback_plan",
      "memory_time_travel.audit.verify",
    ],
  },
  {
    id: "wasm-workload",
    title: "WASM / 插件",
    subtitle: "更像桌面开发的能力包，而不是开发者专属 API",
    description: "把 WASM 插件注册、加载、审批和远程安装当成一个可选工作负载。",
    capabilities: [
      "wasm.host_abi.plan",
      "wasm.remote_install.plan",
      "wasm.remote_install.approval_plan",
      "wasm.plugin.execute",
    ],
  },
  {
    id: "ops-guardrails",
    title: "守护 / 观测",
    subtitle: "给高风险能力加上安全围栏",
    description: "把依赖漂移、守护规则和混沌探针合成一个运维型工作负载。",
    capabilities: [
      "sbom.ci_gate.plan",
      "guardrail_fuzzer.ci_gate.plan",
      "chaos_probe.scheduler.plan",
    ],
  },
  {
    id: "cogni-lab",
    title: "Cogni / 实验",
    subtitle: "把声明式认知能力当成单独工作负载",
    description: "把 Cogni 声明、经验、进化和联邦能力收成一条实验路径。",
    capabilities: [
      "cognis.generate",
      "cognis.workflows",
      "cognis.experience",
      "cognis.evolution",
    ],
  },
];

export function formatWorkloadCapabilities(preset: WorkloadPreset): string {
  return preset.capabilities.join(", ");
}

export function buildWorkloadFeedbackPrompt(preset: WorkloadPreset): string {
  return [
    `我刚试了【${preset.title}】工作负载。`,
    `能力范围：${formatWorkloadCapabilities(preset)}`,
    "最顺手的地方：",
    "最不顺手的地方：",
    "我是否能在 30 秒内找到入口：",
    "如果下次再来，我最想少掉哪一步：",
  ].join("\n");
}

export function emptyWorkloadFeedbackDraft(): WorkloadFeedbackDraft {
  return {
    triedScenario: "",
    mostUseful: "",
    friction: "",
    nextStepToRemove: "",
    foundIn30Seconds: "unknown",
  };
}

export function hasWorkloadFeedbackContent(draft: WorkloadFeedbackDraft): boolean {
  return [
    draft.triedScenario,
    draft.mostUseful,
    draft.friction,
    draft.nextStepToRemove,
  ].some((value) => value.trim().length > 0) || draft.foundIn30Seconds !== "unknown";
}

export function createWorkloadFeedbackEntry(
  preset: WorkloadPreset,
  draft: WorkloadFeedbackDraft,
  createdAt = new Date().toISOString(),
): WorkloadFeedbackEntry {
  return {
    id: `${preset.id}:${createdAt}`,
    workloadId: preset.id,
    workloadTitle: preset.title,
    capabilities: [...preset.capabilities],
    createdAt,
    triedScenario: draft.triedScenario.trim(),
    mostUseful: draft.mostUseful.trim(),
    friction: draft.friction.trim(),
    nextStepToRemove: draft.nextStepToRemove.trim(),
    foundIn30Seconds: draft.foundIn30Seconds,
  };
}

export function parseWorkloadFeedbackEntries(raw: string | null): WorkloadFeedbackEntry[] {
  if (!raw) return [];

  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];

    return parsed.flatMap((item): WorkloadFeedbackEntry[] => {
      if (!item || typeof item !== "object") return [];
      const record = item as Partial<WorkloadFeedbackEntry>;
      if (typeof record.workloadId !== "string" || typeof record.workloadTitle !== "string") return [];
      const createdAt = typeof record.createdAt === "string" ? record.createdAt : "";
      const id = typeof record.id === "string" ? record.id : `${record.workloadId}:${createdAt}`;
      const capabilities = Array.isArray(record.capabilities)
        ? record.capabilities.filter((capability): capability is string => typeof capability === "string")
        : [];
      const foundIn30Seconds = isWorkloadFeedbackFindability(record.foundIn30Seconds)
        ? record.foundIn30Seconds
        : "unknown";

      return [{
        id,
        workloadId: record.workloadId,
        workloadTitle: record.workloadTitle,
        capabilities,
        createdAt,
        triedScenario: typeof record.triedScenario === "string" ? record.triedScenario : "",
        mostUseful: typeof record.mostUseful === "string" ? record.mostUseful : "",
        friction: typeof record.friction === "string" ? record.friction : "",
        nextStepToRemove: typeof record.nextStepToRemove === "string" ? record.nextStepToRemove : "",
        foundIn30Seconds,
      }];
    });
  } catch {
    return [];
  }
}

export function serializeWorkloadFeedbackEntries(entries: WorkloadFeedbackEntry[], limit = 30): string {
  return JSON.stringify(entries.slice(0, limit));
}

export function formatWorkloadFeedbackEntry(entry: WorkloadFeedbackEntry): string {
  return [
    `## ${entry.workloadTitle}`,
    `- 时间：${entry.createdAt || "-"}`,
    `- 能力范围：${entry.capabilities.join(", ") || "-"}`,
    `- 30 秒找到入口：${formatWorkloadFeedbackFindability(entry.foundIn30Seconds)}`,
    `- 真实场景：${entry.triedScenario || "-"}`,
    `- 最顺手：${entry.mostUseful || "-"}`,
    `- 最不顺手：${entry.friction || "-"}`,
    `- 下次希望少一步：${entry.nextStepToRemove || "-"}`,
  ].join("\n");
}

export function formatWorkloadFeedbackExport(entries: WorkloadFeedbackEntry[]): string {
  if (entries.length === 0) {
    return "还没有记录工作负载反馈。请先选一个工作负载，跑一次真实任务，再记录卡点。";
  }

  return [
    "# 云雀工作负载体验反馈",
    "",
    ...entries.map(formatWorkloadFeedbackEntry),
  ].join("\n\n");
}

export function formatWorkloadFeedbackFindability(value: WorkloadFeedbackFindability): string {
  switch (value) {
    case "yes":
      return "是";
    case "partial":
      return "大致能";
    case "no":
      return "不能";
    case "unknown":
    default:
      return "未记录";
  }
}

function isWorkloadFeedbackFindability(value: unknown): value is WorkloadFeedbackFindability {
  return value === "yes" || value === "partial" || value === "no" || value === "unknown";
}
