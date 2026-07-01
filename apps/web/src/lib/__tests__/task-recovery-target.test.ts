import { describe, expect, it } from "vitest";
import type { TaskInfo } from "@/lib/api-types";
import { taskRecoveryTarget } from "../task-recovery-target";

function task(overrides: Partial<TaskInfo>): TaskInfo {
  return {
    id: "task-recovery",
    title: "恢复任务",
    description: "等待恢复",
    status: "failed",
    tenant_id: "tenant-a",
    created_at: "2026-05-11T00:00:00Z",
    updated_at: "2026-05-11T00:00:00Z",
    steps: [],
    ...overrides,
  };
}

describe("taskRecoveryTarget", () => {
  it("focuses known connector recovery hints when backend omits href", () => {
    const target = taskRecoveryTarget(task({
      recovery_hint: {
        category: "connector",
        severity: "warning",
        summary: "GitHub connector needs attention",
        detail: "connector github token expired",
        source: "runner:step",
        primary_action: {
          id: "repair_connector",
          label: "修复连接器",
        },
      },
    }));

    expect(target).toEqual({
      href: "/settings/connectors?focus=github",
      label: "修复连接器",
    });
  });

  it("keeps generic connector hints on the connector settings page", () => {
    const target = taskRecoveryTarget(task({
      recovery_hint: {
        category: "connector",
        severity: "warning",
        summary: "连接器需要重新授权",
        source: "runner:step",
        primary_action: {
          id: "repair_connector",
          label: "修复连接器",
        },
      },
    }));

    expect(target).toEqual({
      href: "/settings/connectors",
      label: "修复连接器",
    });
  });

  it("focuses provider recovery hints when backend omits href but keeps provider id evidence", () => {
    const target = taskRecoveryTarget(task({
      recovery_hint: {
        category: "provider",
        severity: "danger",
        summary: "模型供应商认证失败",
        detail: "provider_id=qwen-backup returned 403 forbidden",
        source: "runner:step",
        primary_action: {
          id: "open_providers",
          label: "检查模型供应商",
        },
      },
    }));

    expect(target).toEqual({
      href: "/settings/providers?focus=qwen-backup",
      label: "检查模型供应商",
    });
  });

  it("keeps generic provider recovery hints on the provider settings tab", () => {
    const target = taskRecoveryTarget(task({
      recovery_hint: {
        category: "provider",
        severity: "warning",
        summary: "OpenAI provider returned 403 forbidden",
        source: "runner:step",
        primary_action: {
          id: "open_providers",
          label: "检查模型供应商",
        },
      },
    }));

    expect(target).toEqual({
      href: "/settings/providers?tab=providers",
      label: "检查模型供应商",
    });
  });
});
