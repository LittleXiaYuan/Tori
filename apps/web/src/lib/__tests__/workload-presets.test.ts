import { describe, expect, it } from "vitest";
import {
  buildWorkloadFeedbackPrompt,
  buildWorkloadCatalogHref,
  createWorkloadFeedbackEntry,
  emptyWorkloadFeedbackDraft,
  formatWorkloadFeedbackExport,
  hasWorkloadFeedbackContent,
  getWorkloadPresetById,
  parseWorkloadFeedbackEntries,
  serializeWorkloadFeedbackEntries,
  workloadFeedbackEntryToExperience,
  WORKLOAD_PRESETS,
} from "../workload-presets";

describe("workload presets", () => {
  it("builds user-facing feedback prompts with capability context", () => {
    const [preset] = WORKLOAD_PRESETS;
    expect(preset).toBeTruthy();

    const prompt = buildWorkloadFeedbackPrompt(preset!);

    expect(prompt).toContain(`我刚试了【${preset!.title}】工作负载`);
    expect(prompt).toContain(preset!.capabilities[0]);
    expect(prompt).toContain("30 秒");
    expect(buildWorkloadCatalogHref(preset!)).toBe(`/packs?preset=${preset!.id}`);
    expect(getWorkloadPresetById(preset!.id)).toEqual(preset);
    expect(getWorkloadPresetById("missing")).toBeUndefined();
  });

  it("creates and exports persisted workload feedback entries", () => {
    const preset = WORKLOAD_PRESETS[1]!;
    const entry = createWorkloadFeedbackEntry(preset, {
      ...emptyWorkloadFeedbackDraft(),
      triedScenario: "复盘昨天的任务变更",
      friction: "入口要从 Pack 页绕一下",
      foundIn30Seconds: "partial",
    }, "2026-05-22T00:00:00.000Z");

    expect(entry).toMatchObject({
      id: `${preset.id}:2026-05-22T00:00:00.000Z`,
      workloadId: preset.id,
      workloadTitle: preset.title,
      triedScenario: "复盘昨天的任务变更",
      foundIn30Seconds: "partial",
    });
    expect(entry.capabilities).toEqual(preset.capabilities);

    const exported = formatWorkloadFeedbackExport([entry]);
    expect(exported).toContain("# 云雀工作负载体验反馈");
    expect(exported).toContain("大致能");
    expect(exported).toContain("复盘昨天的任务变更");
  });

  it("parses stored feedback defensively", () => {
    const preset = WORKLOAD_PRESETS[0]!;
    const valid = createWorkloadFeedbackEntry(preset, {
      ...emptyWorkloadFeedbackDraft(),
      mostUseful: "一键套用 capability",
      foundIn30Seconds: "yes",
    }, "2026-05-22T01:00:00.000Z");

    const serialized = serializeWorkloadFeedbackEntries([valid]);
    expect(parseWorkloadFeedbackEntries(serialized)).toEqual([valid]);
    expect(parseWorkloadFeedbackEntries("{bad json")).toEqual([]);
    expect(parseWorkloadFeedbackEntries(JSON.stringify([{ id: "bad" }]))).toEqual([]);
  });

  it("detects whether a draft has enough user signal to save", () => {
    expect(hasWorkloadFeedbackContent(emptyWorkloadFeedbackDraft())).toBe(false);
    expect(hasWorkloadFeedbackContent({ ...emptyWorkloadFeedbackDraft(), foundIn30Seconds: "no" })).toBe(true);
    expect(hasWorkloadFeedbackContent({ ...emptyWorkloadFeedbackDraft(), friction: "入口藏太深" })).toBe(true);
  });
});

it("maps workload feedback into reflection experience payloads", () => {
  const entry = createWorkloadFeedbackEntry(WORKLOAD_PRESETS[0], {
    triedScenario: "复盘一次浏览器回放",
    mostUseful: "意图计划入口清楚",
    friction: "反馈保存后换设备看不到",
    nextStepToRemove: "不再手动复制",
    foundIn30Seconds: "partial",
  }, "2026-05-23T00:00:00.000Z");
  const exp = workloadFeedbackEntryToExperience(entry);

  expect(exp.source).toBe("workload_feedback");
  expect(exp.category).toBe("workload_feedback");
  expect(exp.outcome).toBe("partial");
  expect(exp.source_id).toBe("browser-rpa");
  expect(String(exp.lesson)).toContain("反馈保存后换设备看不到");
  expect(exp.tags).toContain("workload:browser-rpa");
  expect(exp.tags).toContain("capability:browser.intent.plan");
});
