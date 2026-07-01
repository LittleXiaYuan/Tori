import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ExecutionPanel, OverviewPanel, TaskThreadMessage } from "../task-detail/page";
import type { TaskInfo } from "@/lib/api-types";

describe("Task detail execution panel", () => {
  it("shows preserved checkpoint evidence naturally without raw diagnostics", () => {
    const task: TaskInfo = {
      id: "task-resume-evidence",
      title: "恢复：根据申请表生成材料",
      description: "恢复 Planner 后台任务",
      status: "running",
      tenant_id: "tenant-a",
      created_at: "2026-05-11T00:00:00Z",
      updated_at: "2026-05-11T00:00:00Z",
      steps: [
        {
          id: 1,
          action: "根据附件生成申请材料",
          skill_name: "doc_writer",
          status: "pending",
          input: [
            "[读取附件]: [Parsed document: 申请表.docx]",
            "Workspace path: C:\\Code\\AI\\云雀\\yunque-agent\\tmp\\申请表.docx",
            "公司名称\t云鸢科技",
            '工具返回：handoff agent "file_exec" execution failed: context deadline exceeded EOF',
          ].join("\n"),
          result: '阶段结果：公司名称 云鸢科技\nhandoff agent "general_exec" execution failed: EOF',
          metadata: { planner_step_id: 1 },
        },
      ],
    };

    render(<ExecutionPanel task={task} />);

    expect(screen.getByText("接续输入（已保留证据）")).toBeInTheDocument();
    expect(screen.getByText(/附件内容：申请表\.docx/)).toBeInTheDocument();
    expect(screen.getByText(/附件名称：申请表\.docx/)).toBeInTheDocument();
    expect(screen.getAllByText(/公司名称\s+云鸢科技/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/任务暂时没有完成，已保留现场/).length).toBeGreaterThan(0);
    expect(screen.getByText("原规划步骤 #1")).toBeInTheDocument();
    expect(screen.queryByText(/\[Parsed document:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Workspace path:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/handoff agent/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/execution failed/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/\bEOF\b/)).not.toBeInTheDocument();
  });
});

describe("Task detail overview recovery", () => {
  it("links backend recovery hints to their real recovery surface", () => {
    const task: TaskInfo = {
      id: "task-provider-recovery",
      title: "生成报告",
      description: "调用模型生成报告",
      status: "failed",
      tenant_id: "tenant-a",
      created_at: "2026-05-11T00:00:00Z",
      updated_at: "2026-05-11T00:00:00Z",
      steps: [],
      error: "provider openai returned 401 unauthorized",
      recovery_hint: {
        category: "provider",
        severity: "danger",
        summary: "模型供应商认证失败，需要检查 API Key、Base URL 或账号权限",
        detail: 'handoff agent "file_exec" execution failed: context deadline exceeded EOF',
        source: "runner:step",
        primary_action: {
          id: "open_providers",
          label: "检查模型供应商",
          href: "/settings/providers?tab=providers",
        },
      },
    };

    render(<OverviewPanel task={task} wm={null} />);

    expect(screen.getByLabelText("任务恢复入口")).toBeInTheDocument();
    expect(screen.getByText("模型供应商认证失败，需要检查 API Key、Base URL 或账号权限")).toBeInTheDocument();
    expect(screen.getByText(/响应暂时超时，已保留现场/)).toBeInTheDocument();
    expect(screen.queryByText(/handoff agent/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/\bEOF\b/)).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /检查模型供应商/ })).toHaveAttribute(
      "href",
      "/settings/providers?tab=providers",
    );
  });

  it("does not render a fake recovery link when the backend only provides an endpoint action", () => {
    const task: TaskInfo = {
      id: "task-endpoint-recovery",
      title: "重试任务",
      description: "等待恢复",
      status: "failed",
      tenant_id: "tenant-a",
      created_at: "2026-05-11T00:00:00Z",
      updated_at: "2026-05-11T00:00:00Z",
      steps: [],
      recovery_hint: {
        category: "generic",
        severity: "warning",
        summary: "可以重试任务",
        source: "runner:step",
        primary_action: {
          id: "restart_task",
          label: "重试任务",
          method: "POST",
          endpoint: "/v1/tasks/restart",
        },
      },
    };

    render(<OverviewPanel task={task} wm={null} />);

    expect(screen.getByLabelText("任务恢复入口")).toBeInTheDocument();
    expect(screen.getByText("重试任务")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /重试任务/ })).not.toBeInTheDocument();
  });

  it("derives conservative recovery links when backend hints have no href", () => {
    const connectorTask: TaskInfo = {
      id: "task-connector-recovery",
      title: "同步连接器",
      description: "连接器凭据失效",
      status: "failed",
      tenant_id: "tenant-a",
      created_at: "2026-05-11T00:00:00Z",
      updated_at: "2026-05-11T00:00:00Z",
      steps: [],
      recovery_hint: {
        category: "connector",
        severity: "warning",
        summary: "GitHub 连接器需要重新授权",
        detail: "connector github token expired",
        source: "runner:step",
        primary_action: {
          id: "repair_connector",
          label: "修复连接器",
        },
      },
    };
    const dependencyTask: TaskInfo = {
      id: "task-dependency-fallback",
      title: "恢复依赖",
      description: "等待前置步骤",
      status: "interrupted",
      tenant_id: "tenant-a",
      created_at: "2026-05-11T00:00:00Z",
      updated_at: "2026-05-11T00:00:00Z",
      steps: [],
      recovery_hint: {
        category: "dependency",
        severity: "warning",
        summary: "任务依赖未满足，需要先查看执行链",
        source: "runner:dependency",
        primary_action: {
          id: "open_task_execution",
          label: "查看执行链",
        },
      },
    };

    const { rerender } = render(<OverviewPanel task={connectorTask} wm={null} />);

    expect(screen.getByRole("link", { name: /修复连接器/ })).toHaveAttribute(
      "href",
      "/settings/connectors?focus=github",
    );

    rerender(<OverviewPanel task={dependencyTask} wm={null} />);

    expect(screen.getByRole("link", { name: /查看执行链/ })).toHaveAttribute(
      "href",
      "/task-detail?id=task-dependency-fallback&tab=execution",
    );
  });

  it("links dependency recovery hints to the execution chain tab", () => {
    const task: TaskInfo = {
      id: "task-dependency-recovery",
      title: "恢复依赖阻塞",
      description: "等待前置步骤",
      status: "interrupted",
      tenant_id: "tenant-a",
      created_at: "2026-05-11T00:00:00Z",
      updated_at: "2026-05-11T00:00:00Z",
      steps: [
        { id: 1, action: "前置步骤", status: "pending" },
        { id: 2, action: "等待后续", status: "pending", depends_on: [1] },
      ],
      error: "步骤 2 等待依赖步骤完成：1",
      recovery_hint: {
        category: "dependency",
        severity: "warning",
        summary: "任务依赖未满足，需要先查看执行链",
        detail: "步骤 2 等待依赖步骤完成：1",
        source: "runner:dependency",
        primary_action: {
          id: "open_task_execution",
          label: "查看执行链",
          href: "/task-detail?id=task-dependency-recovery&tab=execution",
        },
      },
    };

    render(<OverviewPanel task={task} wm={null} />);

    expect(screen.getByLabelText("任务恢复入口")).toBeInTheDocument();
    expect(screen.getByText("任务依赖未满足，需要先查看执行链")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /查看执行链/ })).toHaveAttribute(
      "href",
      "/task-detail?id=task-dependency-recovery&tab=execution",
    );
  });
});

describe("Task detail thread message", () => {
  it("keeps thread evidence readable while hiding raw parser and runtime diagnostics", () => {
    render(<TaskThreadMessage msg={{
      role: "assistant",
      content: [
        "已接收附件：[Parsed document: 申请表.docx]",
        "Workspace path: C:\\Code\\AI\\云雀\\yunque-agent\\tmp\\申请表.docx",
        "公司名称\t云鸢科技",
        'handoff agent "file_exec" execution failed: context deadline exceeded EOF',
      ].join("\n"),
    }} />);

    expect(screen.getByText(/附件内容：申请表\.docx/)).toBeInTheDocument();
    expect(screen.getByText(/附件名称：申请表\.docx/)).toBeInTheDocument();
    expect(screen.getByText(/公司名称\s+云鸢科技/)).toBeInTheDocument();
    expect(screen.getByText(/响应暂时超时，已保留现场/)).toBeInTheDocument();
    expect(screen.queryByText(/\[Parsed document:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Workspace path:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/handoff agent/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/execution failed/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/\bEOF\b/)).not.toBeInTheDocument();
  });
});
