import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PlannerRecoveryShelf, fallbackPlannerCheckpointPrompt } from "../planner-recovery-shelf";
import type { PlannerCheckpointSummary } from "@/lib/api-types";
import { api } from "@/lib/api";

const failedCheckpoint: PlannerCheckpointSummary = {
  plan_id: "plan-restore-1",
  task_id: "task-1",
  status: "failed",
  current_step: 2,
  completed: 2,
  total: 4,
  steps_used: 3,
  revisions: 0,
  error: "工具暂时不可用",
  recoverable: true,
  updated_at: "2026-05-11T02:00:00Z",
};

afterEach(() => {
  vi.restoreAllMocks();
});

describe("PlannerRecoveryShelf", () => {
  it("reloads recoverable checkpoints when refreshSignal changes", async () => {
    const onSend = vi.fn();
    const list = vi.spyOn(api, "plannerCheckpoints")
      .mockResolvedValueOnce({ checkpoints: [], count: 0, limit: 5 })
      .mockResolvedValueOnce({ checkpoints: [failedCheckpoint], count: 1, limit: 5 });

    const { rerender } = render(<PlannerRecoveryShelf onSend={onSend} />);

    await waitFor(() => expect(list).toHaveBeenCalledTimes(1));
    expect(screen.queryByText("最近可恢复任务")).not.toBeInTheDocument();

    rerender(<PlannerRecoveryShelf onSend={onSend} refreshSignal={1} />);

    await waitFor(() => {
      expect(list).toHaveBeenCalledTimes(2);
      expect(screen.getByText("最近可恢复任务")).toBeInTheDocument();
      expect(screen.getByText("plan-restore-1")).toBeInTheDocument();
    });
  });

  it("renders recoverable checkpoints and sends a continue prompt", () => {
    const onSend = vi.fn();
    const recoverCheckpoint = vi.fn().mockResolvedValue({
      prompt: "后端恢复 prompt：plan-restore-1",
      recovery_plan: {
        mode: "continue",
        executable: true,
        plan_id: "plan-restore-1",
        steps: [
          { id: 0, action: "已完成", status: "done", selected: false },
          { id: 1, action: "继续处理", status: "pending", selected: true },
          { id: 2, action: "重试步骤", status: "failed", selected: true, depends_on: [1] },
        ],
        prompt: "后端恢复 prompt：plan-restore-1",
      },
    });
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} recoverCheckpoint={recoverCheckpoint} />);

    expect(screen.getByText("最近可恢复任务")).toBeInTheDocument();
    expect(screen.getByText("plan-restore-1")).toBeInTheDocument();
    expect(screen.getByText("工具暂时不可用")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /详情页/ })).toHaveAttribute("href", "/planner-checkpoint?plan_id=plan-restore-1");

    fireEvent.click(screen.getByRole("button", { name: "继续执行" }));

    return waitFor(() => {
      expect(recoverCheckpoint).toHaveBeenCalledWith("plan-restore-1", "continue");
      expect(onSend).toHaveBeenCalledWith("后端恢复 prompt：plan-restore-1");
      expect(screen.getByText("将继续 2 个步骤，按依赖顺序执行")).toBeInTheDocument();
    });
  });

  it("builds retry prompts for failed checkpoints", () => {
    const prompt = fallbackPlannerCheckpointPrompt(failedCheckpoint, "retry_failed");
    expect(prompt).toContain("请重试这个可恢复规划里的失败步骤");
    expect(prompt).toContain("失败原因：工具暂时不可用");
  });

  it("hides known raw checkpoint errors in shelf display and fallback prompts", () => {
    const onSend = vi.fn();
    const rawErrorCheckpoint: PlannerCheckpointSummary = {
      ...failedCheckpoint,
      error: `handoff agent "file_exec" execution failed: context deadline exceeded`,
    };

    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[rawErrorCheckpoint]} onSend={onSend} />);

    expect(screen.getByText(/响应暂时超时/)).toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded/)).not.toBeInTheDocument();
    expect(screen.queryByText(/handoff agent/)).not.toBeInTheDocument();

    const prompt = fallbackPlannerCheckpointPrompt(rawErrorCheckpoint, "continue");
    expect(prompt).toContain("响应暂时超时");
    expect(prompt).not.toContain("context deadline exceeded");
    expect(prompt).not.toContain("handoff agent");
  });

  it("can request partial results first", () => {
    const onSend = vi.fn();
    const recoverCheckpoint = vi.fn().mockResolvedValue({
      prompt: "请先返回阶段结果",
      recovery_plan: {
        mode: "partial",
        executable: false,
        reason: "当前操作只返回已完成部分，不会继续执行步骤。",
        plan_id: "plan-restore-1",
        steps: [],
        prompt: "请先返回阶段结果",
      },
    });
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} recoverCheckpoint={recoverCheckpoint} />);

    fireEvent.click(screen.getByRole("button", { name: "先返回阶段结果" }));

    return waitFor(() => {
      expect(recoverCheckpoint).toHaveBeenCalledWith("plan-restore-1", "partial");
      expect(onSend).toHaveBeenCalledWith("请先返回阶段结果");
      expect(screen.getByText("将先返回已完成部分")).toBeInTheDocument();
    });
  });

  it("can expand checkpoint step details", () => {
    const onSend = vi.fn();
    const getCheckpointDetails = vi.fn().mockResolvedValue({
      ...failedCheckpoint,
      plan_snapshot: [
        { id: 0, action: "读取文档蓝图", skill: "read_file", status: "done", result: "[Parsed document: 申请表.docx]\n公司名称\t云鸢科技\n联系电话\t13864841667" },
        { id: 1, action: "继续实现 Planner", skill: "edit", status: "pending", depends_on: [0] },
        { id: 2, action: "整理最终结果", status: "pending", depends_on: [1] },
      ],
    });
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} getCheckpointDetails={getCheckpointDetails} />);

    fireEvent.click(screen.getByRole("button", { name: "查看步骤" }));

    return waitFor(() => {
      expect(getCheckpointDetails).toHaveBeenCalledWith("plan-restore-1");
      expect(screen.getByText("读取文档蓝图")).toBeInTheDocument();
      expect(screen.getByText("继续实现 Planner")).toBeInTheDocument();
      expect(screen.getByText("整理最终结果")).toBeInTheDocument();
      expect(screen.getByText("可执行 1")).toBeInTheDocument();
      expect(screen.getByText("被阻塞 1")).toBeInTheDocument();
      expect(screen.getByText("已完成 1")).toBeInTheDocument();
      expect(screen.getByText("前置已完成：#0")).toBeInTheDocument();
      expect(screen.getByText("阻塞依赖：#1")).toBeInTheDocument();
      expect(screen.getByText("依赖：0")).toBeInTheDocument();
      expect(screen.getByText(/已保留证据/)).toBeInTheDocument();
      expect(screen.getByText(/公司名称\s+云鸢科技/)).toBeInTheDocument();
    });
  });

  it("hides raw diagnostic terms in completed step result previews", () => {
    const onSend = vi.fn();
    const getCheckpointDetails = vi.fn().mockResolvedValue({
      ...failedCheckpoint,
      plan_snapshot: [
        {
          id: 0,
          action: "解析申请表",
          skill: "file_exec",
          status: "done",
          result: `子代理返回：handoff agent "file_exec" execution failed: context deadline exceeded EOF，但现场已保留。`,
        },
      ],
    });
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} getCheckpointDetails={getCheckpointDetails} />);

    fireEvent.click(screen.getByRole("button", { name: "查看步骤" }));

    return waitFor(() => {
      expect(screen.getByText("解析申请表")).toBeInTheDocument();
      expect(screen.getByText(/已保留证据：响应暂时超时，已保留现场，可稍后重试或继续。/)).toBeInTheDocument();
      expect(screen.queryByText(/handoff agent/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/execution failed/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/context deadline exceeded/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/EOF/)).not.toBeInTheDocument();
    });
  });

  it("shows a friendly message when step detail loading fails", () => {
    const onSend = vi.fn();
    const getCheckpointDetails = vi.fn().mockRejectedValue(new Error("context deadline exceeded"));
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} getCheckpointDetails={getCheckpointDetails} />);

    fireEvent.click(screen.getByRole("button", { name: "查看步骤" }));

    return waitFor(() => {
      expect(screen.getByText("暂时不能读取步骤快照，可稍后再试。")).toBeInTheDocument();
      expect(screen.queryByText(/context deadline exceeded/)).not.toBeInTheDocument();
    });
  });

  it("can create a background resume task", () => {
    const onSend = vi.fn();
    const getTask = vi.fn()
      .mockResolvedValueOnce({
        id: "task-resume-1",
        title: "恢复：测试任务",
        status: "running",
      })
      .mockResolvedValueOnce({
        id: "task-resume-1",
        title: "恢复：测试任务",
        status: "completed",
      });
    const resumeCheckpoint = vi.fn().mockResolvedValue({
      task_id: "task-resume-1",
      status: "accepted",
      recovery_plan: {
        mode: "continue",
        executable: true,
        plan_id: "plan-restore-1",
        steps: [
          { id: 1, action: "继续处理", status: "pending", selected: true },
          { id: 2, action: "重试步骤", status: "failed", selected: true },
        ],
        prompt: "prompt",
      },
    });
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} resumeCheckpoint={resumeCheckpoint} getTask={getTask} />);

    fireEvent.click(screen.getByRole("button", { name: "后台续跑" }));

    return waitFor(() => {
      expect(resumeCheckpoint).toHaveBeenCalledWith("plan-restore-1", "continue", { run: true });
      expect(getTask).toHaveBeenCalledWith("task-resume-1");
      expect(onSend).not.toHaveBeenCalled();
      expect(screen.getByText("已创建后台恢复任务 task-resume-1：将继续 2 个步骤")).toBeInTheDocument();
      expect(screen.getByText(/后台任务 task-resume-1：执行中/)).toBeInTheDocument();
      expect(screen.getByRole("link", { name: /查看任务/ })).toHaveAttribute("href", "/task-detail?id=task-resume-1");
    }).then(async () => {
      fireEvent.click(screen.getByRole("button", { name: "刷新状态" }));
      await waitFor(() => {
        expect(getTask).toHaveBeenCalledTimes(2);
        expect(screen.getByText(/后台任务 task-resume-1：已完成/)).toBeInTheDocument();
      });
    });
  });

  it("can run a direct plan-level resume without sending a chat prompt", () => {
    const onSend = vi.fn();
    const resumePlan = vi.fn().mockResolvedValue({
      status: "completed",
      result: {
        plan: [
          { id: 0, action: "已完成", status: "done", result: "done output" },
          { id: 1, action: "继续处理", status: "done", result: "ok" },
        ],
      },
      recovery_plan: {
        mode: "continue",
        executable: true,
        plan_id: "plan-restore-1",
        steps: [
          { id: 0, action: "已完成", status: "done", selected: false },
          { id: 1, action: "继续处理", status: "pending", selected: true, depends_on: [0] },
        ],
        prompt: "prompt",
      },
    });
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} resumePlan={resumePlan} />);

    fireEvent.click(screen.getByRole("button", { name: "原规划续跑" }));

    return waitFor(() => {
      expect(resumePlan).toHaveBeenCalledWith("plan-restore-1", "continue", { async: true });
      expect(onSend).not.toHaveBeenCalled();
      expect(screen.getByText("已按原规划续跑完成，完成 2/2：将继续 1 个步骤，按依赖顺序执行")).toBeInTheDocument();
    });
  });

  it("shows a detail link when direct plan-level resume starts asynchronously", () => {
    const onSend = vi.fn();
    const resumePlan = vi.fn().mockResolvedValue({
      status: "accepted",
      job_id: "resume-plan-chat-1",
      recovery_plan: {
        mode: "continue",
        executable: true,
        plan_id: "plan-restore-1",
        steps: [],
        prompt: "prompt",
      },
    });
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} resumePlan={resumePlan} />);

    fireEvent.click(screen.getByRole("button", { name: "原规划续跑" }));

    return waitFor(() => {
      expect(resumePlan).toHaveBeenCalledWith("plan-restore-1", "continue", { async: true });
      expect(onSend).not.toHaveBeenCalled();
      expect(screen.getByText("已开始原规划续跑：resume-plan-chat-1")).toBeInTheDocument();
      expect(screen.getByRole("link", { name: /查看续跑/ })).toHaveAttribute("href", "/planner-checkpoint?plan_id=plan-restore-1&job_id=resume-plan-chat-1");
    });
  });

  it("can load the latest asynchronous resume job for a checkpoint", () => {
    const onSend = vi.fn();
    const getResumePlanJob = vi.fn().mockResolvedValue({
      job: {
        id: "resume-plan-latest-1",
        status: "running",
        action: "continue",
        plan_id: "plan-restore-1",
        events: [
          { id: "evt-latest-1", type: "step", summary: "正在续跑", timestamp: "2026-05-11T02:01:00Z" },
        ],
        started_at: "2026-05-11T02:00:00Z",
      },
    });
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} getResumePlanJob={getResumePlanJob} />);

    fireEvent.click(screen.getByRole("button", { name: "最近续跑" }));

    return waitFor(() => {
      expect(getResumePlanJob).toHaveBeenCalledWith({ planId: "plan-restore-1" });
      expect(screen.getByText("已读取最近续跑 resume-plan-latest-1：续跑中")).toBeInTheDocument();
      expect(screen.getByText("现场仍在更新，已记录 1 条事件。")).toBeInTheDocument();
      expect(screen.getByRole("link", { name: /查看续跑/ })).toHaveAttribute("href", "/planner-checkpoint?plan_id=plan-restore-1&job_id=resume-plan-latest-1");
      expect(onSend).not.toHaveBeenCalled();
    });
  });

  it("can refresh an asynchronous direct resume job and show completed progress", () => {
    const onSend = vi.fn();
    const resumePlan = vi.fn().mockResolvedValue({
      status: "accepted",
      job_id: "resume-plan-chat-1",
      recovery_plan: {
        mode: "continue",
        executable: true,
        plan_id: "plan-restore-1",
        steps: [],
        prompt: "prompt",
      },
    });
    const getResumePlanJob = vi.fn().mockResolvedValue({
      job: {
        id: "resume-plan-chat-1",
        status: "completed",
        action: "continue",
        plan_id: "plan-restore-1",
        result: {
          plan: [
            { id: 0, action: "已完成", status: "done" },
            { id: 1, action: "继续处理", status: "completed" },
            { id: 2, action: "已跳过", status: "skipped" },
          ],
        },
        events: [
          { id: "evt-1", type: "step", summary: "步骤已完成", timestamp: "2026-05-11T02:01:00Z" },
        ],
        started_at: "2026-05-11T02:00:00Z",
        finished_at: "2026-05-11T02:02:00Z",
      },
    });
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} resumePlan={resumePlan} getResumePlanJob={getResumePlanJob} />);

    fireEvent.click(screen.getByRole("button", { name: "原规划续跑" }));

    return waitFor(() => {
      expect(screen.getByRole("button", { name: "刷新续跑状态" })).toBeInTheDocument();
    }).then(async () => {
      fireEvent.click(screen.getByRole("button", { name: "刷新续跑状态" }));
      await waitFor(() => {
        expect(getResumePlanJob).toHaveBeenCalledWith("resume-plan-chat-1");
        expect(screen.getByText(/原规划续跑 resume-plan-chat-1：已完成/)).toBeInTheDocument();
        expect(screen.getAllByText(/原规划续跑已完成，完成 3\/3。/).length).toBeGreaterThan(0);
      });
    });
  });

  it("shows friendly asynchronous direct resume job failures without raw timeout text", () => {
    const onSend = vi.fn();
    const recoverCheckpoint = vi.fn().mockResolvedValue({
      prompt: "请按建议重试失败步骤",
      recovery_plan: {
        mode: "retry_failed",
        executable: true,
        plan_id: "plan-restore-1",
        steps: [
          { id: 2, action: "重试失败步骤", status: "failed", selected: true },
        ],
        prompt: "请按建议重试失败步骤",
      },
    });
    const resumePlan = vi.fn().mockResolvedValue({
      status: "accepted",
      job_id: "resume-plan-chat-raw",
      recovery_plan: {
        mode: "continue",
        executable: true,
        plan_id: "plan-restore-1",
        steps: [],
        prompt: "prompt",
      },
    });
    const getResumePlanJob = vi.fn().mockResolvedValue({
      job: {
        id: "resume-plan-chat-raw",
        status: "failed",
        action: "continue",
        plan_id: "plan-restore-1",
        error: "handoff agent execution failed: context deadline exceeded",
        friendly_error: "这次续跑等待时间过长，现场已经保留；建议重试失败步骤。",
        recoverable: true,
        next_action: "retry_failed",
        events: [],
        started_at: "2026-05-11T02:00:00Z",
        finished_at: "2026-05-11T02:02:00Z",
      },
    });
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} resumePlan={resumePlan} getResumePlanJob={getResumePlanJob} recoverCheckpoint={recoverCheckpoint} />);

    fireEvent.click(screen.getByRole("button", { name: "原规划续跑" }));

    return waitFor(() => {
      expect(screen.getByRole("button", { name: "刷新续跑状态" })).toBeInTheDocument();
    }).then(async () => {
      fireEvent.click(screen.getByRole("button", { name: "刷新续跑状态" }));
      await waitFor(() => {
        expect(screen.getAllByText(/这次续跑等待时间过长/).length).toBeGreaterThan(0);
        expect(screen.getByRole("button", { name: "按建议重试" })).toBeInTheDocument();
        expect(screen.queryByText(/context deadline exceeded/)).not.toBeInTheDocument();
        expect(screen.queryByText(/handoff agent/)).not.toBeInTheDocument();
      });
      fireEvent.click(screen.getByRole("button", { name: "按建议重试" }));
      await waitFor(() => {
        expect(recoverCheckpoint).toHaveBeenCalledWith("plan-restore-1", "retry_failed");
        expect(onSend).toHaveBeenCalledWith("请按建议重试失败步骤");
      });
    });
  });

  it("shows friendly direct resume failure advice without sending a chat prompt", () => {
    const onSend = vi.fn();
    const resumePlan = vi.fn().mockResolvedValue({
      status: "failed",
      friendly_error: "这次续跑等待时间过长，现场已经保留；建议重试失败步骤，或先返回阶段结果。",
      recoverable: true,
      next_action: "retry_failed",
      recovery_plan: {
        mode: "continue",
        executable: true,
        plan_id: "plan-restore-1",
        steps: [],
        prompt: "prompt",
      },
    });
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} resumePlan={resumePlan} />);

    fireEvent.click(screen.getByRole("button", { name: "原规划续跑" }));

    return waitFor(() => {
      expect(onSend).not.toHaveBeenCalled();
      expect(screen.getByText(/这次续跑等待时间过长/)).toBeInTheDocument();
      expect(screen.queryByText(/context deadline exceeded/)).not.toBeInTheDocument();
    });
  });

  it("shows a friendly dependency-blocked resume task hint without raw runner errors", () => {
    const onSend = vi.fn();
    const getTask = vi.fn().mockResolvedValue({
      id: "task-resume-1",
      title: "恢复：测试任务",
      status: "interrupted",
      error: "步骤 2 等待依赖步骤完成：1",
    });
    const resumeCheckpoint = vi.fn().mockResolvedValue({
      task_id: "task-resume-1",
      status: "accepted",
      recovery_plan: {
        mode: "continue",
        executable: true,
        plan_id: "plan-restore-1",
        steps: [
          { id: 1, action: "继续处理", status: "pending", selected: true },
        ],
        prompt: "prompt",
      },
    });
    render(<PlannerRecoveryShelf fetchOnMount={false} initialCheckpoints={[failedCheckpoint]} onSend={onSend} resumeCheckpoint={resumeCheckpoint} getTask={getTask} />);

    fireEvent.click(screen.getByRole("button", { name: "后台续跑" }));

    return waitFor(() => {
      expect(screen.getByText(/后台任务 task-resume-1：可恢复/)).toBeInTheDocument();
      expect(screen.getByText(/等待前置步骤完成，可进入任务详情确认依赖后继续。/)).toBeInTheDocument();
      expect(screen.queryByText(/步骤 2 等待依赖步骤完成/)).not.toBeInTheDocument();
    });
  });
});
