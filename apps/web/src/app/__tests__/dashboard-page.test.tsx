import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DashboardPage from "../dashboard/page";

const routerMock = vi.hoisted(() => ({
  push: vi.fn(),
}));

const apiMock = vi.hoisted(() => ({
  healthz: vi.fn(),
  metrics: vi.fn(),
  version: vi.fn(),
  skills: vi.fn(),
  costSummary: vi.fn(),
  systemInfo: vi.fn(),
  checkSetup: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => routerMock,
}));

vi.mock("@/lib/api", () => ({
  api: apiMock,
}));

vi.mock("@/lib/use-polling", () => ({
  usePolling: vi.fn(),
}));

vi.mock("@/components/skeleton-loader", () => ({
  DashboardSkeleton: () => <div data-testid="dashboard-skeleton" />,
}));

vi.mock("lucide-react", () => {
  const Icon = () => <svg aria-hidden="true" />;
  return {
    Activity: Icon,
    AlertTriangle: Icon,
    ArrowRight: Icon,
    BarChart3: Icon,
    BookOpen: Icon,
    Brain: Icon,
    CheckCircle2: Icon,
    ClipboardCheck: Icon,
    ClipboardList: Icon,
    Clock: Icon,
    Cpu: Icon,
    DollarSign: Icon,
    FileText: Icon,
    MessageCircle: Icon,
    Monitor: Icon,
    Package: Icon,
    Plus: Icon,
    RefreshCw: Icon,
    Rocket: Icon,
    Search: Icon,
    Server: Icon,
    Settings: Icon,
    Sparkles: Icon,
    TrendingDown: Icon,
    TrendingUp: Icon,
    XCircle: Icon,
    Zap: Icon,
  };
});

describe("DashboardPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiMock.healthz.mockResolvedValue({ uptime_sec: 180, version: "0.1.0-beta.1" });
    apiMock.metrics.mockResolvedValue({
      requests_total: 0,
      requests_success: 0,
      tokens_total: 0,
      tokens_in: 0,
      tokens_out: 0,
      request_latency: { avg_ms: 0, p99_ms: 0 },
      uptime: 180,
      skills: [],
      recent_errors: [],
    });
    apiMock.version.mockResolvedValue({
      version: "0.1.0-beta.1",
      git_commit: "",
      build_date: "",
      go_version: "",
      os: "windows",
      arch: "amd64",
    });
    apiMock.skills.mockResolvedValue({ skills: [] });
    apiMock.costSummary.mockResolvedValue(null);
    apiMock.systemInfo.mockResolvedValue({ memory_mb: 512, goroutines: 12, cpu_count: 8 });
    apiMock.checkSetup.mockResolvedValue({ setup_needed: false });
  });

  it("opens as a greeting workspace that falls back to starter scenarios", async () => {
    // The cogni-kernel pack client is not mocked here, so cogniPack.list()
    // rejects → the dashboard degrades to the scenarios fallback (the common
    // case when the cogni-kernel pack is disabled).
    render(<DashboardPage />);

    expect(await screen.findByRole("heading", { name: "你好" })).toBeInTheDocument();
    expect(screen.getByText("说一句话，云雀帮你规划和执行。")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "常用场景" })).toBeInTheDocument();

    // Starter scenarios from DASHBOARD_SCENARIOS render as action cards.
    expect(screen.getByRole("button", { name: /写周报/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /整理知识/ })).toBeInTheDocument();

    // Primary action + quick links route to concrete user surfaces.
    expect(screen.getByRole("button", { name: /开始对话/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /任务中心/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /知识库/ })).toBeInTheDocument();
  });

  it("keeps setup and scenario actions routed to the concrete user paths", async () => {
    apiMock.checkSetup.mockResolvedValueOnce({ setup_needed: true });

    render(<DashboardPage />);

    expect(await screen.findByRole("button", { name: /去配置/ })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /去配置/ }));
    expect(routerMock.push).toHaveBeenCalledWith("/setup");

    fireEvent.click(screen.getByRole("button", { name: /开始对话/ }));
    expect(routerMock.push).toHaveBeenCalledWith("/chat");

    fireEvent.click(screen.getByRole("button", { name: /任务中心/ }));
    expect(routerMock.push).toHaveBeenCalledWith("/missions");

    fireEvent.click(screen.getByRole("button", { name: /写周报/ }));
    await waitFor(() => {
      expect(routerMock.push).toHaveBeenCalledWith(expect.stringContaining("/chat?q="));
    });
  });
});
