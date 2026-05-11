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
          checkpoint: makeCheckpoint(),
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
    expect(screen.queryByText("下一步：inspect_dependencies")).not.toBeInTheDocument();
  });
});
