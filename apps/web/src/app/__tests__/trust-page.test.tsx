import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import TrustPage from "../trust/page";

const apiMock = vi.hoisted(() => ({
  trustScores: vi.fn(),
  trustGrant: vi.fn(),
  trustReset: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: apiMock,
}));

describe("TrustPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiMock.trustScores.mockResolvedValue({
      scores: {
        "browser-intent": { score: 62 },
        "computer-use": { score: 18 },
      },
    });
  });

  it("frames control-plane as governance for high-permission abilities", async () => {
    render(<TrustPage />);

    expect(await screen.findByText("这个能力包用来管住高权限能力")).toBeInTheDocument();
    expect(screen.getByText("Control Plane")).toBeInTheDocument();
    expect(screen.getByText("默认启用")).toBeInTheDocument();
    expect(screen.getByText("可回滚")).toBeInTheDocument();
    expect(screen.getByText(/提供信任分数、审批、审计、指标和运行状态入口/)).toBeInTheDocument();
    expect(screen.getByText("它不会替你自动放权")).toBeInTheDocument();
    expect(screen.getByText("不会绕过审批直接允许高风险动作。")).toBeInTheDocument();
    expect(screen.getByText("不会把实验能力默认变成生产级权限。")).toBeInTheDocument();
    expect(screen.getByText("处理待审批")).toBeInTheDocument();
    expect(screen.getAllByText("查看审计").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("观察健康")).toBeInTheDocument();
    expect(screen.getByText("管理模型")).toBeInTheDocument();
    expect(screen.getByText("查看工具执行")).toBeInTheDocument();
    expect(screen.getByText("这里承接的能力包")).toBeInTheDocument();
    expect(screen.getByText("控制面")).toBeInTheDocument();
    expect(screen.getByText("权限")).toBeInTheDocument();
    expect(screen.getByText(/控制工具、联网、写入、远程包和模型配置的授权边界/)).toBeInTheDocument();
    expect(await screen.findByText("browser-intent")).toBeInTheDocument();
  });
});
