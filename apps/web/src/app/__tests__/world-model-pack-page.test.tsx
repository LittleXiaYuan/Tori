import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import WorldModelPackPage from "../packs/world-model/page";

const worldClientMock = vi.hoisted(() => ({
  state: vi.fn(),
  stale: vi.fn(),
  failurePatterns: vi.fn(),
  rootCause: vi.fn(),
  timeline: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("@/lib/world-model-pack-client", () => ({
  createWorldModelPackClient: () => worldClientMock,
}));

describe("WorldModelPackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    worldClientMock.state.mockResolvedValue({
      entries: [
        {
          key: "api.health",
          kind: "api",
          value: "healthy",
          confidence: 0.9,
          last_verified: "2026-06-19T00:00:00Z",
          updated_by: "test",
        },
      ],
      total: 1,
    });
    worldClientMock.stale.mockResolvedValue({ keys: [], max_age: 86400 });
    worldClientMock.failurePatterns.mockResolvedValue({
      patterns: [
        {
          cause_kind: "config",
          effect_kind: "task_failed",
          mechanism: "missing provider key",
          occurrences: 2,
          task_ids: ["task-1"],
        },
      ],
    });
  });

  it("explains world model as inspectable facts and root-cause actions", async () => {
    render(<WorldModelPackPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("可直接使用")).toBeInTheDocument();
    expect(screen.getByText("看事实")).toBeInTheDocument();
    expect(screen.getByText("查根因")).toBeInTheDocument();
    expect(screen.getByText(/可查看、可质疑、可追溯的事实和因果线索/)).toBeInTheDocument();
    expect(screen.getByText("1. 查看云雀相信的事实")).toBeInTheDocument();
    expect(screen.getByText("2. 发现反复失败模式")).toBeInTheDocument();
    expect(screen.getByText("3. 追溯单个任务根因")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会自动改文件、数据库、API 或配置。")).toBeInTheDocument();
    expect(screen.getByText("不会把低置信度状态当成事实直接执行。")).toBeInTheDocument();
    expect(screen.getByText("不会自动重跑失败任务。")).toBeInTheDocument();
  });
});
