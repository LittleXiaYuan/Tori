import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SBOMDriftPackPage from "../packs/sbom-drift/page";

const sbomClientMock = vi.hoisted(() => ({
  status: vi.fn(),
  snapshots: vi.fn(),
  createSnapshot: vi.fn(),
  diff: vi.fn(),
  cycloneDX: vi.fn(),
  ciGatePlan: vi.fn(),
  baselineArtifactSourcePlan: vi.fn(),
  ciBaselineWriteback: vi.fn(),
  ciWorkflowWritebackPlan: vi.fn(),
  evidence: vi.fn(),
}));

vi.mock("@/lib/sdk-client", () => ({
  createYunqueSDKClientOptions: () => ({
    baseUrl: "http://localhost",
    fetch: vi.fn(),
  }),
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("yunque-client/sbom-drift", () => ({
  createSBOMDriftClient: () => sbomClientMock,
}));

describe("SBOMDriftPackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sbomClientMock.status.mockResolvedValue({
      pack_id: "yunque.pack.sbom-drift",
      stage: "pack-shell",
      scanner_ready: true,
      cyclonedx_ready: true,
      ci_gate_plan_ready: true,
      govulncheck_plan_ready: true,
      govulncheck_ready: false,
      artifact_source_plan_ready: true,
      ci_baseline_writeback_ready: true,
      ci_workflow_writeback_plan_ready: true,
      snapshot_count: 0,
      ci_baseline_store: { record_count: 0 },
    });
    sbomClientMock.snapshots.mockResolvedValue({ snapshots: [], count: 0 });
  });

  it("explains dependency drift as baseline and CI handoff planning", async () => {
    render(<SBOMDriftPackPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("实验中")).toBeInTheDocument();
    expect(screen.getByText("可保存基线")).toBeInTheDocument();
    expect(screen.getByText("CI 只生成计划")).toBeInTheDocument();
    expect(screen.getByText(/保存快照、生成漂移报告、导出 CycloneDX 和证据包/)).toBeInTheDocument();
    expect(screen.getByText("1. 建一个依赖基线")).toBeInTheDocument();
    expect(screen.getByText("2. 看依赖漂移")).toBeInTheDocument();
    expect(screen.getByText("3. 生成 CI 交接计划")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会修改 GitHub Actions 或 CI 配置。")).toBeInTheDocument();
    expect(screen.getByText("不会联网拉取漏洞库或执行 govulncheck。")).toBeInTheDocument();
    expect(screen.getByText("不会把计划结果写成真实发布阻断。")).toBeInTheDocument();
    expect(screen.getByText("技术状态")).toBeInTheDocument();
    expect(screen.getByText("从依赖漂移到发布判断")).toBeInTheDocument();
    expect(screen.getByText("2. 带回 Chat")).toBeInTheDocument();
    expect(screen.getByText("3. 看发布依据")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /带回 Chat/ })).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    expect(screen.getByRole("link", { name: /看任务/ })).toHaveAttribute("href", "/missions");
    expect(screen.getByRole("link", { name: "核对执行轨迹" })).toHaveAttribute("href", "/trace");
    expect(screen.getByRole("link", { name: "让小羽继续改" })).toHaveAttribute("href", "/packs/studio?packId=yunque.pack.sbom-drift");
  });
});
