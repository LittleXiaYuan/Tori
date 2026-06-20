import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import RPAReplayPackPage from "../packs/rpa-replay/page";

const rpaReplayClientMock = vi.hoisted(() => ({
  status: vi.fn(),
  traces: vi.fn(),
  createTrace: vi.fn(),
  replay: vi.fn(),
  executorPlan: vi.fn(),
  evidence: vi.fn(),
}));

vi.mock("@/lib/rpa-replay-pack-client", () => ({
  createRPAReplayPackClient: () => rpaReplayClientMock,
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

describe("RPAReplayPackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    rpaReplayClientMock.status.mockResolvedValue({
      pack_id: "yunque.pack.rpa-replay",
      stage: "pack-shell",
      executor_plan_ready: true,
      executor_ready: false,
      action_tracer_plan_ready: true,
      action_tracer_ready: false,
      browser_intent_gate_plan_ready: true,
      browser_intent_ready: false,
      consumes_browser_intent: false,
      executes_browser_actions: false,
      writes_browser_state: false,
      writes_files: false,
      network_access: false,
      trace_count: 0,
      active_recordings: 0,
      capabilities: ["rpa.trace.store", "rpa.replay.plan"],
    });
    rpaReplayClientMock.traces.mockResolvedValue({ traces: [], count: 0 });
  });

  it("explains replay as plan-only trace preparation instead of browser automation", async () => {
    render(<RPAReplayPackPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("实验中")).toBeInTheDocument();
    expect(screen.getByText("只生成计划")).toBeInTheDocument();
    expect(screen.getByText("可导出证据")).toBeInTheDocument();
    expect(screen.getByText(/保存步骤、替换参数、生成 dry-run 回放计划和证据包/)).toBeInTheDocument();
    expect(screen.getByText("1. 保存流程轨迹")).toBeInTheDocument();
    expect(screen.getByText("2. 生成回放计划")).toBeInTheDocument();
    expect(screen.getByText("3. 导出交接证据")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会点击网页、输入表单或下载文件。")).toBeInTheDocument();
    expect(screen.getByText("不会消费 Browser Intent 会话或写浏览器状态。")).toBeInTheDocument();
    expect(screen.getByText("不会把 plan-only 轨迹当成已完成的自动化任务。")).toBeInTheDocument();
    expect(screen.getByText("技术状态")).toBeInTheDocument();
    expect(screen.getByText("从回放计划到可验证自动化")).toBeInTheDocument();
    expect(screen.getByText("2. 带回 Chat")).toBeInTheDocument();
    expect(screen.getByText("3. 看证据位置")).toBeInTheDocument();
    expect(screen.getByText("轨迹定义 JSON")).toBeInTheDocument();
    expect(screen.getByText("这里保存的是可审计轨迹，不会自动打开网页或执行点击；后续回放仍是 dry-run 计划。")).toBeInTheDocument();
    expect(screen.getByText("回放参数 JSON")).toBeInTheDocument();
    expect(screen.getByText("参数只用于生成计划和证据包；当前不会消费 Browser Intent、写浏览器状态或访问目标站点。")).toBeInTheDocument();
    expect(screen.getByLabelText("RPA trace slug")).toBeInTheDocument();
    expect(screen.getByLabelText("Trace JSON")).toBeInTheDocument();
    expect(screen.getByLabelText("Replay params JSON")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /带回 Chat/ })).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    expect(screen.getByRole("link", { name: /看任务/ })).toHaveAttribute("href", "/missions");
    expect(screen.getByRole("link", { name: "核对执行轨迹" })).toHaveAttribute("href", "/trace");
    expect(screen.getByRole("link", { name: "让小羽继续改" })).toHaveAttribute("href", "/packs/studio?packId=yunque.pack.rpa-replay");
  });
});
