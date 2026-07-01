import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PlannerCheckpointDetail } from "../planner-checkpoint-detail";
import type { PlannerCheckpointSummary } from "@/lib/api-types";

function makeCheckpoint(): PlannerCheckpointSummary {
  return {
    plan_id: "plan-123",
    task_id: "task-123",
    goal: "验证 planner 依赖图",
    status: "running",
    current_step: 1,
    completed: 1,
    total: 3,
    steps_used: 3,
    revisions: 0,
    recoverable: true,
    resume_hint: "可继续",
    plan_snapshot: [
      { id: 0, action: "读取技术蓝图", status: "done" },
      { id: 1, action: "拆解路线图", depends_on: [0], status: "pending" },
      { id: 2, action: "补测试与文档", depends_on: [1], status: "pending" },
    ],
  };
}

const dummyResumeJob = {
  id: "job-123",
  status: "completed",
  action: "continue",
  plan_id: "plan-123",
  task_id: "task-123",
  started_at: "2026-05-11T00:00:00.000Z",
  finished_at: "2026-05-11T00:01:00.000Z",
  result: { plan: [] },
  events: [],
} as any;

describe("PlannerCheckpointDetail dependency view", () => {
  it("highlights ready, blocked and done steps clearly", async () => {
    render(
      <PlannerCheckpointDetail
        planId="plan-123"
        initialCheckpoint={makeCheckpoint()}
        fetchExecutionState={async () => null}
        fetchCheckpoint={async () => makeCheckpoint()}
        resumeCheckpoint={async () => ({ task_id: "task-123", status: "accepted", recovery_plan: { action: "continue", checkpoint: makeCheckpoint(), steps: [] }, run: true, checkpoint: makeCheckpoint() } as any)}
        resumePlan={async () => ({ status: "accepted", action: "continue", plan_id: "plan-123", recovery_plan: { action: "continue", checkpoint: makeCheckpoint(), steps: [] } } as any)}
        getResumePlanJob={async () => ({ job: dummyResumeJob })}
        subscribeResumePlanEvents={() => () => {}}
      />,
    );

    expect(screen.getByText("依赖视图")).toBeInTheDocument();
    expect(screen.getByText("可执行 1")).toBeInTheDocument();
    expect(screen.getByText("被阻塞 1")).toBeInTheDocument();
    expect(screen.getByText("已完成 1")).toBeInTheDocument();
    expect(screen.getByText("前置已完成：#0")).toBeInTheDocument();
    expect(screen.getByText("阻塞依赖：#1")).toBeInTheDocument();
    expect(screen.getByText("前置步骤已满足，可以继续推进。")).toBeInTheDocument();
  });

  it("shows unblocked pending steps as ready when prerequisites are done", () => {
    const checkpoint = makeCheckpoint();
    checkpoint.plan_snapshot = [
      { id: 0, action: "读取技术蓝图", status: "done" },
      { id: 1, action: "拆解路线图", depends_on: [0], status: "pending" },
    ];

    render(
      <PlannerCheckpointDetail
        planId="plan-123"
        initialCheckpoint={checkpoint}
        fetchExecutionState={async () => null}
        fetchCheckpoint={async () => checkpoint}
        resumeCheckpoint={async () => ({ task_id: "task-123", status: "accepted", recovery_plan: { action: "continue", checkpoint, steps: [] }, run: true, checkpoint } as any)}
        resumePlan={async () => ({ status: "accepted", action: "continue", plan_id: "plan-123", recovery_plan: { action: "continue", checkpoint, steps: [] } } as any)}
        getResumePlanJob={async () => ({ job: dummyResumeJob })}
        subscribeResumePlanEvents={() => () => {}}
      />,
    );

    expect(screen.getByText("可执行 1")).toBeInTheDocument();
    expect(screen.queryByText("被阻塞 1")).toBeNull();
    expect(screen.getByText("前置已完成：#0")).toBeInTheDocument();
  });

  it("renders execution-state next action as a user-facing recovery label", async () => {
    render(
      <PlannerCheckpointDetail
        planId="plan-123"
        initialCheckpoint={makeCheckpoint()}
        fetchExecutionState={async () => ({
          plan_id: "plan-123",
          status: "failed",
          action: "continue",
          next_action: "inspect_dependencies",
          checkpoint: { ...makeCheckpoint(), session_id: "planner-session-abcdef5678" },
          failure_summary: {
            failed_count: 1,
            completed_count: 1,
            ruled_out: ["#2: 这一步没有顺利完成，现场已保留。"],
            failed_steps: [{
              id: 2,
              action: "补测试与文档",
              skill: "file_edit",
              status: "failed",
              error: "这一步没有顺利完成，现场已保留。",
              recommendation: "改用当前可用工具，或先请求开放/替换工具后再继续。",
              recovery_target: {
                category: "tool",
                label: "检查工具",
                href: "/tools",
                action: "repair_tool",
              },
            }],
          },
          events: [],
        } as any)}
        fetchCheckpoint={async () => makeCheckpoint()}
        resumeCheckpoint={async () => ({ task_id: "task-123", status: "accepted", recovery_plan: { action: "continue", checkpoint: makeCheckpoint(), steps: [] }, run: true, checkpoint: makeCheckpoint() } as any)}
        resumePlan={async () => ({ status: "accepted", action: "continue", plan_id: "plan-123", recovery_plan: { action: "continue", checkpoint: makeCheckpoint(), steps: [] } } as any)}
        getResumePlanJob={async () => ({ job: dummyResumeJob })}
        subscribeResumePlanEvents={() => () => {}}
      />,
    );

    expect(await screen.findByText("统一执行现场")).toBeInTheDocument();
    expect(screen.getByText("下一步：先查看依赖关系")).toBeInTheDocument();
    expect(screen.getByText("会话 cdef5678")).toBeInTheDocument();
    expect(screen.queryByText(/planner-session-abcdef5678/)).not.toBeInTheDocument();
    expect(screen.queryByText("下一步：inspect_dependencies")).not.toBeInTheDocument();
    expect(screen.getByText("失败步骤")).toBeInTheDocument();
    expect(screen.getAllByText("#2").length).toBeGreaterThan(0);
    expect(screen.getAllByText("补测试与文档").length).toBeGreaterThan(0);
    expect(screen.getAllByText("file_edit").length).toBeGreaterThan(0);
    expect(screen.getByRole("link", { name: /检查工具/ })).toHaveAttribute("href", "/tools");
    expect(screen.queryByText(/已暂时排除/)).not.toBeInTheDocument();
    expect(screen.queryByText("这一步没有顺利完成，现场已保留。")).not.toBeInTheDocument();
    expect(screen.queryByText("改用当前可用工具，或先请求开放/替换工具后再继续。")).not.toBeInTheDocument();
  });

  it("promotes execution-state primary targets into a direct recovery link", async () => {
    render(
      <PlannerCheckpointDetail
        planId="plan-123"
        initialCheckpoint={makeCheckpoint()}
        fetchExecutionState={async () => ({
          plan_id: "plan-123",
          status: "failed",
          action: "retry_failed",
          next_action: "retry_failed",
          checkpoint: makeCheckpoint(),
          failure_summary: {
            failed_count: 1,
            completed_count: 1,
            primary_target: {
              category: "provider",
              label: "检查模型供应商",
              href: "/settings/providers?tab=providers",
              action: "open_provider_settings",
            },
            failed_steps: [{
              id: 2,
              action: "调用模型生成报告",
              skill: "llm",
              status: "failed",
              error: "provider returned 401",
              recommendation: "检查模型供应商后重试失败步骤。",
            }],
          },
          events: [],
        } as any)}
        fetchCheckpoint={async () => makeCheckpoint()}
        resumeCheckpoint={async () => ({ task_id: "task-123", status: "accepted", recovery_plan: { action: "continue", checkpoint: makeCheckpoint(), steps: [] }, run: true, checkpoint: makeCheckpoint() } as any)}
        resumePlan={async () => ({ status: "accepted", action: "continue", plan_id: "plan-123", recovery_plan: { action: "continue", checkpoint: makeCheckpoint(), steps: [] } } as any)}
        getResumePlanJob={async () => ({ job: dummyResumeJob })}
        subscribeResumePlanEvents={() => () => {}}
      />,
    );

    expect(await screen.findByLabelText("Planner 主恢复入口")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /检查模型供应商/ })).toHaveAttribute(
      "href",
      "/settings/providers?tab=providers",
    );
  });

  it("anchors dependency recovery targets without href to the dependency view", async () => {
    render(
      <PlannerCheckpointDetail
        planId="plan-123"
        initialCheckpoint={makeCheckpoint()}
        fetchExecutionState={async () => ({
          plan_id: "plan-123",
          status: "failed",
          action: "retry_failed",
          next_action: "inspect_dependencies",
          checkpoint: makeCheckpoint(),
          failure_summary: {
            failed_count: 1,
            completed_count: 1,
            primary_target: {
              category: "dependency",
              label: "查看依赖关系",
              action: "inspect_dependencies",
            },
            failed_steps: [{
              id: 2,
              action: "等待前置步骤",
              status: "failed",
              error: "dependency step 1 尚未完成",
              recommendation: "先补齐依赖，再执行后续步骤。",
              recovery_target: {
                category: "dependency",
                label: "查看依赖关系",
                action: "inspect_dependencies",
              },
            }],
          },
          events: [],
        } as any)}
        fetchCheckpoint={async () => makeCheckpoint()}
        resumeCheckpoint={async () => ({ task_id: "task-123", status: "accepted", recovery_plan: { action: "continue", checkpoint: makeCheckpoint(), steps: [] }, run: true, checkpoint: makeCheckpoint() } as any)}
        resumePlan={async () => ({ status: "accepted", action: "continue", plan_id: "plan-123", recovery_plan: { action: "continue", checkpoint: makeCheckpoint(), steps: [] } } as any)}
        getResumePlanJob={async () => ({ job: dummyResumeJob })}
        subscribeResumePlanEvents={() => () => {}}
      />,
    );

    expect(await screen.findByLabelText("Planner 主恢复入口")).toBeInTheDocument();
    expect(screen.getByText("下一步：先查看依赖关系")).toBeInTheDocument();
    const dependencyLinks = screen.getAllByRole("link", { name: /查看依赖关系/ });
    expect(dependencyLinks).toHaveLength(2);
    expect(dependencyLinks.map((link) => link.getAttribute("href"))).toEqual([
      "/planner-checkpoint?plan_id=plan-123#dependency-view",
      "/planner-checkpoint?plan_id=plan-123#dependency-view",
    ]);
    expect(screen.queryByText("dependency step 1 尚未完成")).not.toBeInTheDocument();
    expect(screen.queryByText("先补齐依赖，再执行后续步骤。")).not.toBeInTheDocument();
  });

  it("anchors approval recovery targets without href to the approvals page", async () => {
    render(
      <PlannerCheckpointDetail
        planId="plan-approval"
        initialCheckpoint={makeCheckpoint()}
        fetchExecutionState={async () => ({
          plan_id: "plan-approval",
          status: "failed",
          action: "retry_failed",
          checkpoint: makeCheckpoint(),
          failure_summary: {
            failed_count: 1,
            completed_count: 1,
            primary_target: {
              category: "approval",
              label: "处理审批",
              action: "handle_approval",
            },
          },
          events: [],
        } as any)}
        fetchCheckpoint={async () => makeCheckpoint()}
        resumeCheckpoint={async () => ({ task_id: "task-123", status: "accepted", recovery_plan: { action: "continue", checkpoint: makeCheckpoint(), steps: [] }, run: true, checkpoint: makeCheckpoint() } as any)}
        resumePlan={async () => ({ status: "accepted", action: "continue", plan_id: "plan-123", recovery_plan: { action: "continue", checkpoint: makeCheckpoint(), steps: [] } } as any)}
        getResumePlanJob={async () => ({ job: dummyResumeJob })}
        subscribeResumePlanEvents={() => () => {}}
      />,
    );

    expect(await screen.findByLabelText("Planner 主恢复入口")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /处理审批/ })).toHaveAttribute("href", "/approvals");
  });
});
