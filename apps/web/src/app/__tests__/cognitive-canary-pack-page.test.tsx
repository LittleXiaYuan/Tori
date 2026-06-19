import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CognitiveCanaryPackPage from "../packs/cognitive-canary/page";

const canaryClientMock = vi.hoisted(() => ({
  status: vi.fn(),
  scenarios: vi.fn(),
  reports: vi.fn(),
  report: vi.fn(),
  saveScenarios: vi.fn(),
  evaluate: vi.fn(),
  shadowPlan: vi.fn(),
  responseCollectorWriteback: vi.fn(),
  responseCollectorPipelinePlan: vi.fn(),
  evidence: vi.fn(),
}));

vi.mock("@/lib/cognitive-canary-pack-client", () => ({
  createCognitiveCanaryPackClient: () => canaryClientMock,
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

describe("CognitiveCanaryPackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    canaryClientMock.status.mockResolvedValue({
      pack_id: "yunque.pack.cognitive-canary",
      stage: "pack-shell-before-shadow-traffic",
      shadow_traffic_ready: false,
      shadow_plan_ready: true,
      response_collector_store: { record_count: 0 },
      scenario_count: 0,
      report_count: 0,
      capabilities: [],
    });
    canaryClientMock.scenarios.mockResolvedValue({ scenarios: [], count: 0 });
    canaryClientMock.reports.mockResolvedValue({ reports: [], count: 0 });
  });

  it("explains cognitive canaries as regression checks and plan-only rollout handoff", async () => {
    render(<CognitiveCanaryPackPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("实验中")).toBeInTheDocument();
    expect(screen.getByText("可运行回归")).toBeInTheDocument();
    expect(screen.getByText("上线只生成计划")).toBeInTheDocument();
    expect(screen.getByText(/模型、提示词或 Cogni 策略变更前做认知回归检查/)).toBeInTheDocument();
    expect(screen.getByText("1. 准备回归题集")).toBeInTheDocument();
    expect(screen.getByText("2. 对比候选表现")).toBeInTheDocument();
    expect(screen.getByText("3. 生成上线计划")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会自动切换模型版本。")).toBeInTheDocument();
    expect(screen.getByText("不会 mirror 真实流量或采集用户回答。")).toBeInTheDocument();
    expect(screen.getByText("不会发布指标或执行自动回滚。")).toBeInTheDocument();
    expect(screen.getByText("技术状态")).toBeInTheDocument();
  });
});
