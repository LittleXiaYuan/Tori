import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import InnerLifePackPage from "../packs/inner-life/page";

const innerLifeClient = vi.hoisted(() => ({
  curiosity: vi.fn(),
  reflection: vi.fn(),
  dreaming: vi.fn(),
}));

vi.mock("@/lib/inner-life-pack-client", () => ({
  createInnerLifePackClient: vi.fn(() => innerLifeClient),
}));

describe("InnerLifePackPage", () => {
  beforeEach(() => {
    innerLifeClient.curiosity.mockReset().mockResolvedValue({
      pending: [{
        question: "为什么用户打开能力包后不知道下一步做什么？",
        category: "product",
        priority: 0.86,
        context: "能力包需要从展示变成行动入口。",
        related_to: ["Pack", "UX"],
      }],
      recent: [],
    });
    innerLifeClient.reflection.mockReset().mockResolvedValue({
      recent: [{
        id: "reflection-1",
        kind: "reflection",
        actor: "agent",
        created_at: "2026-06-19T00:00:00Z",
        payload: { quality: 8, satisfied: true, user_intent: "补齐 Pack 体验" },
      }],
    });
    innerLifeClient.dreaming.mockReset().mockResolvedValue({
      recent: [{
        id: "dream-1",
        kind: "dreaming",
        actor: "agent",
        created_at: "2026-06-19T01:00:00Z",
        payload: {
          explorations_run: 2,
          facts_discovered: 3,
          thoughts_generated: 4,
          skills_suggested: 1,
        },
      }],
    });
  });

  it("frames inner life as actionable task leads instead of passive internals", async () => {
    render(<InnerLifePackPage />);

    expect(await screen.findByText("这个能力包有什么用")).toBeInTheDocument();
    expect(screen.getByText("把好奇心变成任务")).toBeInTheDocument();
    expect(screen.getByText("把反思变成改进")).toBeInTheDocument();
    expect(screen.getByText("把夜游变成缺口清单")).toBeInTheDocument();
    expect(screen.getByText("待探索问题")).toBeInTheDocument();
    expect(screen.getByText("反思记录")).toBeInTheDocument();
    expect(screen.getByText("夜游记录")).toBeInTheDocument();
  });

  it("keeps curiosity items connected to a concrete chat action", async () => {
    render(<InnerLifePackPage />);

    expect(await screen.findByText("为什么用户打开能力包后不知道下一步做什么？")).toBeInTheDocument();
    const action = screen.getByRole("link", { name: /继续探索/ });
    expect(action).toHaveAttribute("href", expect.stringContaining("/chat?q="));
  });

  it("offers concrete actions when inner life has no data yet", async () => {
    innerLifeClient.curiosity.mockResolvedValue({ pending: [], recent: [] });
    innerLifeClient.reflection.mockResolvedValue({ recent: [] });
    innerLifeClient.dreaming.mockResolvedValue({ recent: [] });

    render(<InnerLifePackPage />);

    expect(await screen.findByText("生成探索问题")).toBeInTheDocument();
    expect(screen.getByText("生成一次复盘")).toBeInTheDocument();
    expect(screen.getByText("查看能力缺口")).toBeInTheDocument();
  });
});
