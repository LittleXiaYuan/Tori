import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import MicroAgentPackPage from "../packs/micro-agent/page";

const microClientMock = vi.hoisted(() => ({
  agents: vi.fn(),
  resolve: vi.fn(),
  trace: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("@/lib/micro-agent-pack-client", () => ({
  createMicroAgentPackClient: () => microClientMock,
}));

describe("MicroAgentPackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    microClientMock.agents.mockResolvedValue({ agents: [], total: 0 });
  });

  it("explains micro agents as previewable prompt injection and trace replay", async () => {
    render(<MicroAgentPackPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("可直接使用")).toBeInTheDocument();
    expect(screen.getByText("可试触发")).toBeInTheDocument();
    expect(screen.getByText("可回放轨迹")).toBeInTheDocument();
    expect(screen.getByText(/查看哪些轻量专家提示会参与任务/)).toBeInTheDocument();
    expect(screen.getByText("1. 查看已注册微代理")).toBeInTheDocument();
    expect(screen.getByText("2. 试一条触发消息")).toBeInTheDocument();
    expect(screen.getByText("3. 回放任务推理")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会自动执行微代理内容。")).toBeInTheDocument();
    expect(screen.getByText("不会修改 data/microagents 下的源文件。")).toBeInTheDocument();
  });
});
