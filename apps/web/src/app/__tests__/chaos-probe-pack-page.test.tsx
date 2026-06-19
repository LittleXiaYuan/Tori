import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ChaosProbePackPage from "../packs/chaos-probe/page";

const chaosClientMock = vi.hoisted(() => ({
  status: vi.fn(),
  probes: vi.fn(),
  reports: vi.fn(),
  report: vi.fn(),
  saveProbes: vi.fn(),
  run: vi.fn(),
  schedulerPlan: vi.fn(),
  degradeStateWriteback: vi.fn(),
  degradeStateEnginePlan: vi.fn(),
  evidence: vi.fn(),
}));

vi.mock("@/lib/chaos-probe-pack-client", () => ({
  createChaosProbePackClient: () => chaosClientMock,
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

describe("ChaosProbePackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    chaosClientMock.status.mockResolvedValue({
      pack_id: "yunque.pack.chaos-probe",
      stage: "pack-shell-before-scheduler",
      scheduler_ready: false,
      scheduler_plan_ready: true,
      degrade_state_store_ready: true,
      runtime_degrade_state_ready: false,
      probe_count: 0,
      report_count: 0,
      capabilities: [],
    });
    chaosClientMock.probes.mockResolvedValue({ probes: [], count: 0 });
    chaosClientMock.reports.mockResolvedValue({ reports: [], count: 0 });
  });

  it("explains chaos probes as safe health checks instead of destructive fault injection", async () => {
    render(<ChaosProbePackPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("实验中")).toBeInTheDocument();
    expect(screen.getByText("只跑安全探针")).toBeInTheDocument();
    expect(screen.getByText("降级只生成计划")).toBeInTheDocument();
    expect(screen.getByText(/用安全探针检查云雀运行时、护栏和关键链路是否健康/)).toBeInTheDocument();
    expect(screen.getByText("1. 准备安全探针")).toBeInTheDocument();
    expect(screen.getByText("2. 运行一次演练")).toBeInTheDocument();
    expect(screen.getByText("3. 输出运行计划")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会破坏生产环境或注入真实故障。")).toBeInTheDocument();
    expect(screen.getByText("不会创建后台定时任务。")).toBeInTheDocument();
    expect(screen.getByText("不会写入真实 runtime degrade-state engine。")).toBeInTheDocument();
    expect(screen.getByText("技术状态")).toBeInTheDocument();
  });
});
