import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AccountRailFlyout from "../account-rail-flyout";
import { CONTROL_PLANE_PACK_ID, SKILLS_PACK_ID } from "@/lib/nav-items";

const routerMock = vi.hoisted(() => ({
  push: vi.fn(),
}));

const pathnameMock = vi.hoisted(() => ({
  value: "/chat",
}));

vi.mock("next/navigation", () => ({
  usePathname: () => pathnameMock.value,
  useRouter: () => routerMock,
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    locale: "zh",
    t: (key: string) => ({
      "nav.item.chat": "对话",
      "nav.item.missions": "任务中心",
      "nav.item.knowledge": "知识库",
      "nav.item.memory": "记忆",
      "nav.item.packs": "能力包",
      "nav.item.skills": "技能",
      "nav.item.plugins": "插件宿主",
      "nav.item.tools": "终端",
      "nav.item.settings": "设置",
    } as Record<string, string>)[key] || key,
  }),
}));

vi.mock("lucide-react", () => {
  const Icon = () => <svg aria-hidden="true" />;
  return {
    BarChart3: Icon,
    BookOpen: Icon,
    Blocks: Icon,
    Bot: Icon,
    Boxes: Icon,
    Brain: Icon,
    BrainCircuit: Icon,
    Cpu: Icon,
    FolderGit2: Icon,
    Globe: Icon,
    HeartPulse: Icon,
    Languages: Icon,
    LayoutDashboard: Icon,
    LayoutGrid: Icon,
    Lightbulb: Icon,
    LogOut: Icon,
    MailWarning: Icon,
    MessageCircle: Icon,
    Moon: Icon,
    Package: Icon,
    Puzzle: Icon,
    ScanFace: Icon,
    Search: Icon,
    Settings: Icon,
    Share2: Icon,
    Shield: Icon,
    ShieldCheck: Icon,
    SmilePlus: Icon,
    Sun: Icon,
    Terminal: Icon,
    Users: Icon,
    Workflow: Icon,
    Wrench: Icon,
    Zap: Icon,
  };
});

describe("AccountRailFlyout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    pathnameMock.value = "/chat";
  });

  it("keeps advanced and control-plane entries folded out of the default desktop flyout", () => {
    render(
      <AccountRailFlyout
        open
        enabledPackIds={new Set([CONTROL_PLANE_PACK_ID, SKILLS_PACK_ID])}
      />,
    );

    const flyout = screen.getByText("全部功能").closest("aside");
    expect(flyout).not.toBeNull();
    expect(within(flyout as HTMLElement).getByRole("button", { name: "对话" })).toBeInTheDocument();
    expect(within(flyout as HTMLElement).getByRole("button", { name: "任务中心" })).toBeInTheDocument();
    expect(within(flyout as HTMLElement).getByRole("button", { name: "能力包" })).toBeInTheDocument();

    expect(within(flyout as HTMLElement).queryByRole("button", { name: "技能" })).not.toBeInTheDocument();
    expect(within(flyout as HTMLElement).queryByRole("button", { name: "插件宿主" })).not.toBeInTheDocument();
    expect(within(flyout as HTMLElement).queryByRole("button", { name: "终端" })).not.toBeInTheDocument();

    const advanced = within(flyout as HTMLElement).getByRole("button", { name: /高级入口/ });
    expect(advanced).toHaveAttribute("aria-expanded", "false");
  });

  it("reveals advanced entries on demand and still navigates", () => {
    render(
      <AccountRailFlyout
        open
        enabledPackIds={new Set([CONTROL_PLANE_PACK_ID, SKILLS_PACK_ID])}
      />,
    );

    const flyout = screen.getByText("全部功能").closest("aside") as HTMLElement;
    fireEvent.click(within(flyout).getByRole("button", { name: /高级入口/ }));

    expect(within(flyout).getByRole("button", { name: /高级入口/ })).toHaveAttribute("aria-expanded", "true");
    const tools = within(flyout).getByRole("button", { name: "终端" });
    expect(tools).toBeInTheDocument();

    fireEvent.click(tools);
    expect(routerMock.push).toHaveBeenCalledWith("/tools");
  });
});
