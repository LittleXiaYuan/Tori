import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ExperiencePackPage from "../packs/experience/page";

const experienceClientMock = vi.hoisted(() => ({
  recommendations: vi.fn(),
  preferences: vi.fn(),
  evaluations: vi.fn(),
  items: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("@/lib/experience-pack-client", () => ({
  createExperiencePackClient: () => experienceClientMock,
}));

describe("ExperiencePackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    experienceClientMock.recommendations.mockResolvedValue({ recommendations: [], context: "" });
    experienceClientMock.preferences.mockResolvedValue({
      preferred_categories: [],
      preferred_tags: [],
      avoid_categories: [],
      interaction_count: 0,
    });
    experienceClientMock.evaluations.mockResolvedValue({ recent: [] });
  });

  it("explains experience as recommendations, preferences and task review actions", async () => {
    render(<ExperiencePackPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("可直接使用")).toBeInTheDocument();
    expect(screen.getByText("推荐下一步")).toBeInTheDocument();
    expect(screen.getByText("可回到 Chat")).toBeInTheDocument();
    expect(screen.getByText(/反馈、偏好和自评分数变成可查看的经验面板/)).toBeInTheDocument();
    expect(screen.getByText("1. 看推荐能力")).toBeInTheDocument();
    expect(screen.getByText("2. 检查偏好画像")).toBeInTheDocument();
    expect(screen.getByText("3. 复盘任务自评")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会替你自动修改偏好画像。")).toBeInTheDocument();
    expect(screen.getByText("不会把低置信度推荐当成必须执行的决定。")).toBeInTheDocument();
  });
});
