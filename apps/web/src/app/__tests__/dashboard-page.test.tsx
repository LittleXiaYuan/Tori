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
    RefreshCw: Icon,
    Rocket: Icon,
    Search: Icon,
    Server: Icon,
    TrendingDown: Icon,
    TrendingUp: Icon,
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

  it("opens as an action-first workspace instead of an architecture console", async () => {
    render(<DashboardPage />);

    expect(await screen.findByRole("heading", { name: "工作台" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "从一句话开始，交付一个可验收结果" })).toBeInTheDocument();
    expect(screen.getByRole("list", { name: "云雀工作闭环" })).toBeInTheDocument();
    expect(screen.getByText("开口")).toBeInTheDocument();
    expect(screen.getByText("执行")).toBeInTheDocument();
    expect(screen.getByText("验收")).toBeInTheDocument();
    expect(screen.getByText("沉淀")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /开始对话/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /查看任务中心/ })).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /整理知识/ }).length).toBeGreaterThan(0);
    expect(screen.getByRole("heading", { name: "常用开始方式" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "运行概况" })).toBeInTheDocument();

    const text = document.body.textContent || "";
    expect(text).not.toMatch(/\bPack\b|Cogni|微内核|WASM|DLC/);
  });

  it("keeps setup and scenario actions routed to the concrete user paths", async () => {
    apiMock.checkSetup.mockResolvedValueOnce({ setup_needed: true });

    render(<DashboardPage />);

    expect(await screen.findByRole("button", { name: /去配置模型/ })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /去配置模型/ }));
    expect(routerMock.push).toHaveBeenCalledWith("/setup");

    fireEvent.click(screen.getByRole("button", { name: /开始对话/ }));
    expect(routerMock.push).toHaveBeenCalledWith("/chat");

    fireEvent.click(screen.getByRole("button", { name: /查看任务中心/ }));
    expect(routerMock.push).toHaveBeenCalledWith("/missions");

    fireEvent.click(screen.getByRole("button", { name: /写周报/ }));
    await waitFor(() => {
      expect(routerMock.push).toHaveBeenCalledWith(expect.stringContaining("/chat?q="));
    });
  });
});
