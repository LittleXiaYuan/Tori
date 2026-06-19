import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SkillAnomalyPackPage from "../packs/skill-anomaly/page";

const skillAnomalyClientMock = vi.hoisted(() => ({
  status: vi.fn(),
  profiles: vi.fn(),
  observe: vi.fn(),
  detect: vi.fn(),
  auditHookPlan: vi.fn(),
  approvalQueueWriteback: vi.fn(),
  approvalManagerBridgePlan: vi.fn(),
  evidence: vi.fn(),
}));

vi.mock("@/lib/skill-anomaly-pack-client", () => ({
  createSkillAnomalyPackClient: () => skillAnomalyClientMock,
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

describe("SkillAnomalyPackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    skillAnomalyClientMock.status.mockResolvedValue({
      pack_id: "yunque.pack.skill-anomaly",
      stage: "pack-shell-before-audit-hook",
      audit_hook_ready: false,
      audit_hook_plan_ready: true,
      trust_mutation_plan_ready: true,
      approval_queue_store_ready: true,
      approval_manager_bridge_plan_ready: true,
      global_approval_enqueue_ready: false,
      profile_count: 0,
      active_profiles: 0,
      anomaly_count: 0,
      approval_queue_store: { record_count: 0 },
      capabilities: [],
    });
    skillAnomalyClientMock.profiles.mockResolvedValue({ profiles: [], count: 0 });
  });

  it("explains skill anomaly checks as approval planning instead of automatic trust mutation", async () => {
    render(<SkillAnomalyPackPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("实验中")).toBeInTheDocument();
    expect(screen.getByText("可检测异常")).toBeInTheDocument();
    expect(screen.getByText("审批只生成计划")).toBeInTheDocument();
    expect(screen.getByText(/观察 Skill 的正常行为，并在出现越权参数、异常动作或失败模式时生成 NeedsApproval 计划/)).toBeInTheDocument();
    expect(screen.getByText("1. 建立正常行为画像")).toBeInTheDocument();
    expect(screen.getByText("2. 检测可疑调用")).toBeInTheDocument();
    expect(screen.getByText("3. 交给治理流程")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会直接扣 Trust Score。")).toBeInTheDocument();
    expect(screen.getByText("不会自动批准或释放 runtime action。")).toBeInTheDocument();
    expect(screen.getByText("不会调用全局 Approval Manager。")).toBeInTheDocument();
    expect(screen.getByText("技术状态")).toBeInTheDocument();
  });
});
