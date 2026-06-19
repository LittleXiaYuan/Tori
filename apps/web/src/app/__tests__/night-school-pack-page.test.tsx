import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import NightSchoolPackPage from "../packs/night-school/page";

const nightSchoolClientMock = vi.hoisted(() => ({
  dreams: vi.fn(),
  distill: vi.fn(),
  traits: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("@/lib/night-school-pack-client", () => ({
  createNightSchoolPackClient: () => nightSchoolClientMock,
}));

describe("NightSchoolPackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    nightSchoolClientMock.dreams.mockResolvedValue({ recent: [] });
    nightSchoolClientMock.distill.mockResolvedValue({ rules: [], patterns: [], tool_insights: [] });
    nightSchoolClientMock.traits.mockResolvedValue({ traits: [] });
  });

  it("explains night school as review, distilled experience and preference learning", async () => {
    render(<NightSchoolPackPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("可直接使用")).toBeInTheDocument();
    expect(screen.getByText("复盘任务")).toBeInTheDocument();
    expect(screen.getByText("可带回 Chat")).toBeInTheDocument();
    expect(screen.getByText(/已完成任务里的经验、失败模式和用户偏好整理出来/)).toBeInTheDocument();
    expect(screen.getByText("1. 看夜间复盘")).toBeInTheDocument();
    expect(screen.getByText("2. 应用蒸馏经验")).toBeInTheDocument();
    expect(screen.getByText("3. 检查学到的画像")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会在夜间自动执行新任务。")).toBeInTheDocument();
    expect(screen.getByText("不会把低置信度画像当成硬规则。")).toBeInTheDocument();
  });
});
