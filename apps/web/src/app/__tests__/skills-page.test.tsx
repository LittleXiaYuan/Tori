import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SkillsPage from "../skills/page";

const apiMock = vi.hoisted(() => ({
  skills: vi.fn(),
  skillHubInstalled: vi.fn(),
  getDynamicSkills: vi.fn(),
  skillHubTrending: vi.fn(),
  skillHubSearch: vi.fn(),
  skillHubInstall: vi.fn(),
  skillHubUninstall: vi.fn(),
  approveDynamicSkill: vi.fn(),
  scanSkills: vi.fn(),
}));

const toastMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", () => ({
  api: apiMock,
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: toastMock,
}));

const localSkill = {
  name: "document_writer",
  description: "生成结构化文档",
  category: "writing",
  usage_total: 12,
  success_rate: 0.92,
};

const marketSkill = {
  name: "github-issue-triage",
  description: "整理 GitHub Issue",
  source: "clawhub",
  rating: 4.8,
  installed: false,
};

describe("SkillsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiMock.skills.mockResolvedValue({
      skills: [localSkill],
      categories: [{ id: "writing", name: "写作", description: "写作技能" }],
    });
    apiMock.skillHubInstalled.mockResolvedValue({ skills: [] });
    apiMock.getDynamicSkills.mockResolvedValue([]);
    apiMock.skillHubTrending.mockResolvedValue({ skills: [marketSkill] });
    apiMock.skillHubSearch.mockResolvedValue({ results: [marketSkill] });
    apiMock.skillHubInstall.mockResolvedValue({ ok: true });
    apiMock.skillHubUninstall.mockResolvedValue({ ok: true });
    apiMock.approveDynamicSkill.mockResolvedValue({ ok: true });
    apiMock.scanSkills.mockResolvedValue({ skills_loaded: 2 });
  });

  it("presents installed skills as a compact recovery surface", async () => {
    render(<SkillsPage />);

    expect(await screen.findByRole("heading", { name: "技能" })).toBeInTheDocument();
    expect(screen.getByText("修复缺失技能，扫描目录或安装社区技能。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "扫描本地技能目录" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "刷新技能" })).toBeInTheDocument();
    const recovery = screen.getByLabelText("技能恢复入口");
    expect(recovery).toHaveTextContent("缺技能？");
    expect(recovery).toHaveTextContent("扫描本地，找不到再装或审核。");
    fireEvent.click(screen.getByRole("button", { name: "扫描本地" }));
    await waitFor(() => {
      expect(apiMock.scanSkills).toHaveBeenCalled();
    });
    fireEvent.click(screen.getByRole("button", { name: "去市场" }));
    expect(await screen.findByText("github-issue-triage")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "待审核 0" }));
    expect(await screen.findByText("暂无动态技能")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: /已安装/ }));

    expect(screen.getByLabelText("找技能")).toHaveAttribute("placeholder", "名称或描述");
    expect(screen.queryByPlaceholderText("搜索已安装技能...")).not.toBeInTheDocument();
    expect(screen.getByText("document_writer")).toBeInTheDocument();

    expect(screen.getByRole("button", { name: "名称" })).toHaveAttribute("aria-current", "true");
    fireEvent.click(screen.getByRole("button", { name: "使用量" }));
    expect(screen.getByRole("button", { name: "使用量" })).toHaveAttribute("aria-current", "true");

    expect(screen.getByRole("button", { name: "全部" })).toHaveAttribute("aria-current", "true");
    fireEvent.click(screen.getByRole("button", { name: "写作" }));
    expect(screen.getByRole("button", { name: "写作" })).toHaveAttribute("aria-current", "true");
  });

  it("keeps market install controls real and labeled", async () => {
    render(<SkillsPage />);

    fireEvent.click(await screen.findByRole("tab", { name: "技能市场" }));

    expect(await screen.findByText("github-issue-triage")).toBeInTheDocument();
    expect(screen.getByLabelText("搜索市场")).toHaveAttribute("placeholder", "关键词");
    expect(screen.queryByPlaceholderText("搜索技能..")).not.toBeInTheDocument();

    expect(screen.getByRole("button", { name: "全部" })).toHaveAttribute("aria-current", "true");
    fireEvent.click(screen.getByRole("button", { name: "ClawHub" }));
    expect(screen.getByRole("button", { name: "ClawHub" })).toHaveAttribute("aria-current", "true");

    const githubInput = screen.getByLabelText("GitHub");
    expect(githubInput).toHaveAttribute("placeholder", "owner/repo");
    fireEvent.change(githubInput, { target: { value: "openai/example-skill" } });
    fireEvent.keyDown(githubInput, { key: "Enter", code: "Enter" });

    await waitFor(() => {
      expect(apiMock.skillHubInstall).toHaveBeenCalledWith("openai/example-skill");
    });
  });
});
