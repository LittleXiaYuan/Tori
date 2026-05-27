import {
  buildWorkloadCatalogHref,
  formatWorkloadCapabilities,
  getWorkloadPresetById,
  listWorkloadPresets,
  WORKLOAD_PRESETS,
  type WorkloadPreset,
} from "yunque-client/workloads";

export {
  buildWorkloadCatalogHref,
  formatWorkloadCapabilities,
  getWorkloadPresetById,
  listWorkloadPresets,
  WORKLOAD_PRESETS,
  type WorkloadPreset,
};

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

export function workloadFeedbackEntryToExperience(entry: WorkloadFeedbackEntry): Record<string, unknown> {
  return {
    id: `workload-feedback:${entry.id}`,
    source: "workload_feedback",
    source_id: entry.workloadId,
    category: "workload_feedback",
    outcome: workloadFeedbackOutcome(entry.foundIn30Seconds),
    lesson: [
      `工作负载【${entry.workloadTitle}】体验反馈`,
      `30 秒找到入口：${formatWorkloadFeedbackFindability(entry.foundIn30Seconds)}`,
      `最顺手：${entry.mostUseful || "-"}`,
      `最不顺手：${entry.friction || "-"}`,
      `下次希望少一步：${entry.nextStepToRemove || "-"}`,
    ].join("\n"),
    context: [
      `真实场景：${entry.triedScenario || "-"}`,
      `能力范围：${entry.capabilities.join(", ") || "-"}`,
    ].join("\n"),
    tags: [
      `workload:${entry.workloadId}`,
      `findability:${entry.foundIn30Seconds}`,
      ...entry.capabilities.map((capability) => `capability:${capability}`),
    ],
    created_at: entry.createdAt,
  };
}

function workloadFeedbackOutcome(value: WorkloadFeedbackFindability): "success" | "failure" | "partial" {
  if (value === "yes") return "success";
  if (value === "no") return "failure";
  return "partial";
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
