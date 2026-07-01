import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AccountRail from "../account-rail";

const routerMock = vi.hoisted(() => ({
  push: vi.fn(),
  replace: vi.fn(),
}));

const pathnameMock = vi.hoisted(() => ({
  value: "/chat",
}));

const apiMock = vi.hoisted(() => ({
  healthz: vi.fn(),
  pluginUITabs: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => pathnameMock.value,
  useRouter: () => routerMock,
}));

vi.mock("@/lib/api", () => ({
  api: apiMock,
}));

vi.mock("@/lib/pack-sync", () => ({
  buildPackNavItems: vi.fn(() => []),
  fetchEnabledPacks: vi.fn(() => Promise.resolve([])),
}));

vi.mock("@/hooks/use-user-preferences", () => ({
  useNavigationPreferences: () => ({ pinnedItems: [] }),
}));

vi.mock("@/components/title-bar", () => ({
  WindowControls: () => <div data-testid="window-controls" />,
}));

vi.mock("@/components/layout/account-rail-flyout", () => ({
  default: () => <div data-testid="account-rail-flyout" />,
}));

vi.mock("@/lib/theme-engine", () => ({
  loadTheme: () => ({ presetTheme: "dark" }),
  patchAndApply: vi.fn(),
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    locale: "zh",
    setLocale: vi.fn(),
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
    Search: Icon,
    Settings: Icon,
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

describe("AccountRail", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    pathnameMock.value = "/chat";
    apiMock.healthz.mockResolvedValue({ status: "ok", version: "0.1.0" });
    apiMock.pluginUITabs.mockResolvedValue({ tabs: [] });
  });

  it("renders a readable desktop main path instead of icon-only navigation", async () => {
    const { container } = render(<AccountRail />);

    // Brand (云雀 / 桌面工作伙伴) now lives in the unified title bar, not the
    // rail. The rail keeps a slim labelled main path: only the high-frequency
    // 对话 entry is pinned; everything else moves into the 全功能 flyout / ⌘K.
    expect(screen.getByText("主路径")).toBeInTheDocument();
    const nav = screen.getByRole("navigation", { name: "主导航" });
    expect(within(nav).getByText("对话")).toBeInTheDocument();

    const chatButton = container.querySelector('button[aria-label="对话"]');
    expect(chatButton).toBeInstanceOf(HTMLButtonElement);
    fireEvent.click(chatButton as HTMLButtonElement);
    expect(routerMock.push).toHaveBeenCalledWith("/chat");

    await waitFor(() => {
      expect(apiMock.healthz).toHaveBeenCalled();
    });
  });
});
