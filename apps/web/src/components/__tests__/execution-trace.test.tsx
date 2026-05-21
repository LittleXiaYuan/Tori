import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ExecutionTrace, type AgentEvent } from "../execution-trace";

function evt(id: string, summary: string, detail: unknown): AgentEvent {
  return {
    id,
    trace_id: "trace-1234567890",
    ts: "2026-05-11T00:00:00.000Z",
    domain: "planner",
    type: "plan",
    summary,
    detail,
    meta: {},
  };
}

function expandTrace() {
  fireEvent.click(screen.getByRole("button", { name: /执行过程/ }));
}

describe("ExecutionTrace detail cards", () => {
  it("renders Cogni detail as a readable card", () => {
    render(<ExecutionTrace events={[evt("evt-cogni", "Cogni 已激活：文档助手", {
      activated: ["文档助手"],
      context_bytes: 128,
      tool_before: 12,
      tool_after: 3,
      removed: ["browser_search", "shell_exec"],
    })]} />);

    expandTrace();
    fireEvent.click(screen.getByText("Cogni 已激活：文档助手"));

    expect(screen.getByText("激活 Cogni")).toBeInTheDocument();
    expect(screen.getByText("文档助手")).toBeInTheDocument();
    expect(screen.getByText("12 → 3")).toBeInTheDocument();
    expect(screen.getByText("browser_search、shell_exec")).toBeInTheDocument();
  });

  it("renders long-horizon checkpoint detail as a recovery card", () => {
    render(<ExecutionTrace events={[evt("evt-plan", "长程规划已保存失败现场，可继续恢复", {
      plan_id: "plan-1",
      task_id: "task-1",
      status: "failed",
      completed: 1,
      total: 2,
      steps_used: 2,
      error: "boom",
      recoverable: true,
      resume_hint: "可根据 plan_snapshot 继续、重试失败步骤，或先返回已完成部分。",
      plan_snapshot: [
        { id: 0, action: "读取文档", status: "done" },
        { id: 1, action: "执行失败步骤", status: "failed" },
      ],
    })]} />);

    expandTrace();
    fireEvent.click(screen.getByText("长程规划已保存失败现场，可继续恢复"));

    expect(screen.getByText("进度")).toBeInTheDocument();
    expect(screen.getByText("1/2")).toBeInTheDocument();
    expect(screen.getByText("失败原因：boom")).toBeInTheDocument();
    expect(screen.getByText("执行失败步骤")).toBeInTheDocument();
  });

  it("renders model fallback detail naturally", () => {
    render(<ExecutionTrace events={[evt("evt-fallback", "模型暂时没有回应，正在换用 backup-model 继续。", {
      model: "backup-model",
      attempt: 2,
      reason: "EOF",
    })]} />);

    expandTrace();
    fireEvent.click(screen.getByText("模型暂时没有回应，正在换用 backup-model 继续。"));

    expect(screen.getByText("切换到")).toBeInTheDocument();
    expect(screen.getByText("backup-model")).toBeInTheDocument();
    expect(screen.getByText("原因：任务暂时没有完成，已保留现场，可切换策略或稍后继续。")).toBeInTheDocument();
    expect(screen.queryByText(/EOF/)).toBeNull();
    expect(screen.queryByText(/调用栈|级联|唤醒/)).toBeNull();
  });

  it("renders handoff failure detail as a recoverable card", () => {
    render(<ExecutionTrace events={[evt("evt-handoff", "子代理 [file_exec] 响应超时，正在切换策略或返回已完成部分。", {
      agent: "file_exec",
      error: "handoff agent \"file_exec\" execution failed: context deadline exceeded",
      dur_ms: 90000,
      recoverable: true,
      next_step: "已保留失败原因；主规划器会缩小任务粒度、改用直接工具或返回已完成部分。",
    })]} />);

    expandTrace();
    fireEvent.click(screen.getByText("子代理 [file_exec] 响应超时，正在切换策略或返回已完成部分。"));

    expect(screen.getByText("子代理")).toBeInTheDocument();
    expect(screen.getByText("file_exec")).toBeInTheDocument();
    expect(screen.getByText("可恢复")).toBeInTheDocument();
    expect(screen.getByText("原因已保留：响应暂时超时，已保留现场，可稍后重试或继续。")).toBeInTheDocument();
    expect(screen.getByText(/主规划器会缩小任务粒度/)).toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded|handoff agent|execution failed/)).toBeNull();
  });


  it("offers recovery prompts for recoverable long-horizon checkpoints", () => {
    const onRecoveryPrompt = vi.fn();
    render(<ExecutionTrace onRecoveryPrompt={onRecoveryPrompt} events={[evt("evt-recover", "长程规划已保存失败现场，可继续恢复", {
      plan_id: "plan-1",
      task_id: "task-1",
      status: "failed",
      completed: 1,
      total: 2,
      error: "boom",
      recoverable: true,
      plan_snapshot: [
        { id: 0, action: "读取文档", skill: "file_open", status: "done" },
        { id: 1, action: "执行失败步骤", skill: "bad_skill", status: "failed", error: "boom" },
      ],
    })]} />);

    expandTrace();
    fireEvent.click(screen.getByText("长程规划已保存失败现场，可继续恢复"));
    fireEvent.click(screen.getByText("重试失败步骤"));

    expect(onRecoveryPrompt).toHaveBeenCalledTimes(1);
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("重试失败步骤");
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("Plan ID：plan-1");
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("skill=bad_skill");
  });


  it("offers strategy-switch prompts for recoverable handoff failures", () => {
    const onRecoveryPrompt = vi.fn();
    render(<ExecutionTrace onRecoveryPrompt={onRecoveryPrompt} events={[evt("evt-handoff-action", "子代理 [file_exec] 响应超时，正在切换策略或返回已完成部分。", {
      agent: "file_exec",
      input: "[读取附件]: [Parsed document: 申请表.docx]\nWorkspace path: C:\\Code\\AI\\云雀\\tmp\\申请表.docx\nParser: local\nStatus: parsed\n公司名称\t云鸢科技",
      reply: "已读取：公司名称 云鸢科技",
      error: "context deadline exceeded",
      recoverable: true,
      next_step: "已保留失败原因；主规划器会缩小任务粒度、改用直接工具或返回已完成部分。",
    })]} />);

    expandTrace();
    fireEvent.click(screen.getByText("子代理 [file_exec] 响应超时，正在切换策略或返回已完成部分。"));
    fireEvent.click(screen.getByText("改用直接工具"));

    expect(screen.getByText(/附件内容：申请表\.docx/)).toBeInTheDocument();
    expect(screen.getByText(/附件名称：申请表\.docx/)).toBeInTheDocument();
    expect(screen.getAllByText(/公司名称\s+云鸢科技/).length).toBeGreaterThan(0);
    expect(screen.queryByText(/\[Parsed document:/)).toBeNull();
    expect(screen.queryByText(/Workspace path:/)).toBeNull();
    expect(screen.getByText(/附件解析器：local/)).toBeInTheDocument();
    expect(screen.getByText(/附件状态：parsed/)).toBeInTheDocument();
    expect(onRecoveryPrompt).toHaveBeenCalledTimes(1);
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("不要再次委派同一个子代理");
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("附件内容：申请表.docx");
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("附件名称：申请表.docx");
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("附件解析器：local");
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("附件状态：parsed");
    expect(onRecoveryPrompt.mock.calls[0][0]).not.toContain("[Parsed document:");
    expect(onRecoveryPrompt.mock.calls[0][0]).not.toContain("Workspace path:");
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("响应暂时超时，已保留现场");
    expect(onRecoveryPrompt.mock.calls[0][0]).not.toContain("context deadline exceeded");
  });

  it("sanitizes unknown trace detail JSON before rendering", () => {
    render(<ExecutionTrace events={[evt("evt-raw-detail", "内部状态已记录", {
      stage: "planner",
      raw_error: "handoff agent execution failed: context deadline exceeded",
      nested: {
        reason: "all fallback LLM clients failed (FC): EOF",
      },
    })]} />);

    expandTrace();
    fireEvent.click(screen.getByText("内部状态已记录"));

    expect(screen.getByText(/响应暂时超时，已保留现场/)).toBeInTheDocument();
    expect(screen.getByText(/所有可用模型通道暂时失败，已保留现场/)).toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded|handoff agent|all fallback|EOF/)).toBeNull();
  });

  it("sanitizes raw event summaries before rendering", () => {
    render(<ExecutionTrace events={[evt("evt-raw-summary", "handoff agent \"file_exec\" execution failed: context deadline exceeded", {
      stage: "planner",
    })]} />);

    expandTrace();

    expect(screen.getByText("响应暂时超时，已保留现场，可稍后重试或继续。")).toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded|handoff agent|execution failed/)).toBeNull();
  });

  it("sanitizes raw context cancellation summaries before rendering", () => {
    render(<ExecutionTrace events={[evt("evt-cancel-summary", "context canceled", {
      reason: "context cancelled",
    })]} />);

    expandTrace();
    fireEvent.click(screen.getByText("连接暂时中断，已保留现场，可稍后继续或先查看阶段结果。"));

    expect(screen.getAllByText(/连接暂时中断，已保留现场/).length).toBeGreaterThan(0);
    expect(screen.queryByText(/context canceled|context cancelled/)).toBeNull();
  });

  it("renders repeated failure recovery detail as a strategy card", () => {
    const onRecoveryPrompt = vi.fn();
    render(<ExecutionTrace onRecoveryPrompt={onRecoveryPrompt} events={[evt("evt-recovery", "检测到连续失败，正在切换执行策略", {
      failed_count: 2,
      completed_count: 1,
      failed_tools: ["transfer_to_file_exec"],
      failure_pattern: "模型或子任务响应不稳定",
      recommendation: "先返回阶段结果或切为后台任务；继续时降低任务粒度，暂不重复使用 transfer_to_file_exec。",
      recoverable: true,
      tried: ["file_open: read README"],
      ruled_out: [
        "transfer_to_file_exec: context deadline exceeded",
        "transfer_to_file_exec: all fallback LLM clients failed (FC): EOF",
      ],
      next_step: "停止重复失败路径，换一个工具、降低任务粒度，或先汇总已获得证据再继续。",
    })]} />);

    expandTrace();
    fireEvent.click(screen.getByText("检测到连续失败，正在切换执行策略"));

    expect(screen.getByText("已完成")).toBeInTheDocument();
    expect(screen.getByText("未完成")).toBeInTheDocument();
    expect(screen.getByText("transfer_to_file_exec")).toBeInTheDocument();
    expect(screen.getByText("失败模式：模型或子任务响应不稳定")).toBeInTheDocument();
    expect(screen.getByText(/推荐策略：先返回阶段结果或切为后台任务/)).toBeInTheDocument();
    expect(screen.getByText(/响应暂时超时，已保留现场/)).toBeInTheDocument();
    expect(screen.getByText(/所有可用模型通道暂时失败，已保留现场/)).toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded|all fallback|EOF/)).toBeNull();

    fireEvent.click(screen.getByText("换策略继续"));
    expect(onRecoveryPrompt).toHaveBeenCalledTimes(1);
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("不要重复已经失败的同一路径");
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("失败模式：模型或子任务响应不稳定");
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("推荐策略：先返回阶段结果或切为后台任务");
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("响应暂时超时，已保留现场");
    expect(onRecoveryPrompt.mock.calls[0][0]).not.toContain("context deadline exceeded");
    expect(onRecoveryPrompt.mock.calls[0][0]).not.toContain("EOF");
  });

  it("renders partial planner result detail with recovery actions", () => {
    const onRecoveryPrompt = vi.fn();
    render(<ExecutionTrace onRecoveryPrompt={onRecoveryPrompt} events={[evt("evt-partial", "已返回阶段结果，现场已保留，可继续恢复", {
      recoverable: true,
      completed_count: 1,
      failed_count: 0,
      reason: "当前模型连接不稳定，现场已保留，可切换为后台任务或稍后重试。",
      next_step: "可以直接继续，我会基于已完成步骤往下恢复。",
      steps: [
        { id: 1, skill: "stage_tool", status: "done", result: "[Parsed document: 蓝图.docx]\n阶段资料：已经读取技术蓝图。\nhandoff agent execution failed: EOF" },
      ],
    })]} />);

    expandTrace();
    fireEvent.click(screen.getByText("已返回阶段结果，现场已保留，可继续恢复"));

    expect(screen.getByText("已完成")).toBeInTheDocument();
    expect(screen.getByText(/stage_tool/)).toBeInTheDocument();
    expect(screen.getByText(/附件内容：蓝图\.docx/)).toBeInTheDocument();
    expect(screen.getByText(/阶段资料：已经读取技术蓝图/)).toBeInTheDocument();
    expect(screen.getByText(/任务暂时没有完成，已保留现场/)).toBeInTheDocument();
    expect(screen.queryByText(/\[Parsed document:|all fallback|EOF|context deadline exceeded|handoff agent|execution failed/)).toBeNull();

    fireEvent.click(screen.getByText("基于阶段结果继续"));
    expect(onRecoveryPrompt).toHaveBeenCalledTimes(1);
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("不要从头重跑");
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("附件内容：蓝图.docx");
    expect(onRecoveryPrompt.mock.calls[0][0]).toContain("阶段资料：已经读取技术蓝图");
    expect(onRecoveryPrompt.mock.calls[0][0]).not.toContain("[Parsed document:");
    expect(onRecoveryPrompt.mock.calls[0][0]).not.toContain("handoff agent");
    expect(onRecoveryPrompt.mock.calls[0][0]).not.toContain("EOF");
  });

});

