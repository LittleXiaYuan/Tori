import { describe, expect, it } from "vitest";
import { resolvePlannerRecoveryTarget, resolvePlannerRecoveryTargetFromDetail } from "../planner-recovery-target";

describe("planner recovery targets", () => {
  it("links dependency targets to the checkpoint dependency view when plan id is available", () => {
    expect(resolvePlannerRecoveryTarget({ category: "dependency", label: "查看依赖" }, "plan/a b")).toEqual({
      category: "dependency",
      group_key: "dependency|/planner-checkpoint?plan_id=plan%2Fa%20b#dependency-view",
      label: "查看依赖",
      href: "/planner-checkpoint?plan_id=plan%2Fa%20b#dependency-view",
    });
  });

  it("preserves backend group keys when resolving direct planner targets", () => {
    expect(resolvePlannerRecoveryTarget({
      category: "provider",
      label: "检查模型供应商",
      href: "/settings/providers?focus=qwen-backup",
      group_key: "provider|/settings/providers?focus=qwen-backup",
    }, "plan-1")?.group_key).toBe("provider|/settings/providers?focus=qwen-backup");
  });

  it("adds sandbox and approval fallbacks when backend omits href", () => {
    expect(resolvePlannerRecoveryTarget({ category: "sandbox", label: "检查沙箱" }, "plan-1")?.href).toBe("/packs/computer-use");
    expect(resolvePlannerRecoveryTarget({ category: "approval", label: "处理审批" }, "plan-1")?.href).toBe("/approvals");
  });

  it("focuses known connector targets when backend omits href", () => {
    expect(resolvePlannerRecoveryTarget({
      category: "connector",
      label: "修复 GitHub 连接器",
      action: "repair_connector",
    }, "plan-1")?.href).toBe("/settings/connectors?focus=github");
  });

  it("does not invent a link for unknown categories without href", () => {
    expect(resolvePlannerRecoveryTarget({ category: "custom", label: "自定义入口" }, "plan-1")).toEqual({
      category: "custom",
      group_key: undefined,
      label: "自定义入口",
    });
  });

  it("resolves trace detail targets with real hrefs only", () => {
    expect(resolvePlannerRecoveryTargetFromDetail({
      plan_id: "plan-2",
      primary_target: { category: "dependency" },
    })).toEqual({
      category: "dependency",
      label: "查看依赖关系",
      href: "/planner-checkpoint?plan_id=plan-2#dependency-view",
      action: undefined,
      groupKey: "dependency|/planner-checkpoint?plan_id=plan-2#dependency-view",
    });

    expect(resolvePlannerRecoveryTargetFromDetail({
      primary_target: { category: "custom", label: "自定义入口" },
    })).toBeNull();
  });

  it("can focus connector recovery targets from trace failure detail", () => {
    expect(resolvePlannerRecoveryTargetFromDetail({
      primary_target: { category: "connector" },
      failed_tools: ["github"],
    })).toEqual({
      category: "connector",
      label: "修复连接器",
      href: "/settings/connectors?focus=github",
      action: undefined,
      groupKey: "connector|/settings/connectors?focus=github",
    });
  });

  it("can focus provider recovery targets from trace failure detail", () => {
    expect(resolvePlannerRecoveryTargetFromDetail({
      primary_target: { category: "provider" },
      ruled_out: ["provider_id=qwen-backup returned 403 forbidden"],
    })).toEqual({
      category: "provider",
      label: "检查模型供应商",
      href: "/settings/providers?focus=qwen-backup",
      action: undefined,
      groupKey: "provider|/settings/providers?focus=qwen-backup",
    });
  });

  it("preserves backend group keys from trace details", () => {
    expect(resolvePlannerRecoveryTargetFromDetail({
      primary_target: {
        category: "browser",
        label: "打开浏览器包",
        href: "/packs/browser",
        group_key: "connector|/packs/browser",
      },
    })?.groupKey).toBe("connector|/packs/browser");
  });
});
