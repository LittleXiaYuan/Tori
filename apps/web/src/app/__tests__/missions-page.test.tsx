import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import MissionsPage from "../missions/page";

const routerMock = vi.hoisted(() => ({
  push: vi.fn(),
}));

const apiMock = vi.hoisted(() => ({
  taskList: vi.fn(),
  cronList: vi.fn(),
  getTriggersV2: vi.fn(),
  getTemplates: vi.fn(),
  taskDelete: vi.fn(),
  taskRun: vi.fn(),
  taskPause: vi.fn(),
  taskResume: vi.fn(),
  taskCancel: vi.fn(),
  taskRestart: vi.fn(),
  missionParse: vi.fn(),
  cronAdd: vi.fn(),
  cronRemove: vi.fn(),
  cronRun: vi.fn(),
  createTriggerV2: vi.fn(),
  deleteTriggerV2: vi.fn(),
  instantiateTemplate: vi.fn(),
  deleteTemplate: vi.fn(),
}));

const toastMock = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  useRouter: () => routerMock,
}));

vi.mock("@/lib/api", () => ({
  api: apiMock,
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: toastMock,
}));

vi.mock("lucide-react", () => {
  const Icon = () => <svg aria-hidden="true" />;
  return {
    AlertTriangle: Icon,
    Calendar: Icon,
    CheckCircle2: Icon,
    ChevronDown: Icon,
    ChevronRight: Icon,
    Clock: Icon,
    Copy: Icon,
    Eye: Icon,
    FileText: Icon,
    GitBranch: Icon,
    ListTodo: Icon,
    MessageCircle: Icon,
    MoreHorizontal: Icon,
    Pause: Icon,
    Play: Icon,
    Plus: Icon,
    Power: Icon,
    PowerOff: Icon,
    Radio: Icon,
    RefreshCw: Icon,
    RotateCcw: Icon,
    Send: Icon,
    Sparkles: Icon,
    Timer: Icon,
    Trash2: Icon,
    X: Icon,
    Zap: Icon,
  };
});

const tasks = [
  {
    id: "task-1",
    title: "保留任务",
    description: "keep",
    status: "pending",
    steps: [],
    tenant_id: "t1",
    created_at: "2026-06-21T00:00:00Z",
    updated_at: "2026-06-21T00:00:00Z",
  },
  {
    id: "task-2",
    title: "删除任务",
    description: "remove",
    status: "completed",
    steps: [],
    tenant_id: "t1",
    created_at: "2026-06-21T00:01:00Z",
    updated_at: "2026-06-21T00:01:00Z",
  },
];

describe("MissionsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiMock.taskList
      .mockResolvedValueOnce(tasks)
      .mockResolvedValueOnce([tasks[0]]);
    apiMock.cronList.mockResolvedValue({ jobs: [] });
    apiMock.getTriggersV2.mockResolvedValue({ triggers: [] });
    apiMock.getTemplates.mockResolvedValue({ templates: [] });
    apiMock.taskDelete.mockResolvedValue({ deleted: "task-2" });
  });

  it("confirms task deletion and immediately reconciles the visible task count", async () => {
    render(<MissionsPage />);

    expect(await screen.findByText("删除任务")).toBeInTheDocument();
    expect(screen.getByText("总任务")).toBeInTheDocument();
    expect(screen.getAllByText("2").length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "更多任务入口" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "更多任务入口" }));
    fireEvent.click(await screen.findByRole("menuitem", { name: "工作流" }));
    expect(routerMock.push).toHaveBeenCalledWith("/workflows");
    fireEvent.click(screen.getByRole("button", { name: "更多任务入口" }));
    fireEvent.click(await screen.findByRole("menuitem", { name: "设计流程" }));
    expect(routerMock.push).toHaveBeenCalledWith("/workflow-editor");

    fireEvent.click(screen.getByRole("button", { name: /^删除任务 删除任务$/ }));

    expect(await screen.findByRole("alertdialog", { name: "删除这个任务？" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "删除任务" }));

    await waitFor(() => {
      expect(apiMock.taskDelete).toHaveBeenCalledWith("task-2");
    });
    await waitFor(() => {
      expect(screen.queryByText("删除任务")).not.toBeInTheDocument();
    });
    expect(screen.getByText("保留任务")).toBeInTheDocument();
    expect(screen.getAllByText("1").length).toBeGreaterThan(0);
    expect(toastMock).toHaveBeenCalledWith("任务已删除", "success");
  });

  it("opens smart create with a labeled task goal field instead of placeholder-only input", async () => {
    render(<MissionsPage />);

    expect(await screen.findByText("保留任务")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "智能创建" }));

    expect(screen.getByRole("textbox", { name: "任务目标" })).toBeInTheDocument();
    expect(screen.getByText("写清楚结果、时间或触发条件；云雀会先解析，再让你确认创建。")).toBeInTheDocument();
  });

  it("links failed task recovery hints only when there is a real recovery route", async () => {
    apiMock.taskList.mockReset();
    apiMock.taskList.mockResolvedValue([
      {
        id: "task-provider-recovery",
        title: "模型失败任务",
        description: "provider auth failed",
        status: "failed",
        steps: [],
        tenant_id: "t1",
        created_at: "2026-06-21T00:02:00Z",
        updated_at: "2026-06-21T00:02:00Z",
        recovery_hint: {
          category: "provider",
          severity: "danger",
          summary: "模型供应商认证失败",
          source: "runner:step",
          primary_action: {
            id: "open_providers",
            label: "检查模型供应商",
            href: "/settings/providers?tab=providers",
          },
        },
      },
      {
        id: "task-dependency-recovery",
        title: "依赖失败任务",
        description: "dependency blocked",
        status: "failed",
        steps: [],
        tenant_id: "t1",
        created_at: "2026-06-21T00:03:30Z",
        updated_at: "2026-06-21T00:03:30Z",
        recovery_hint: {
          category: "dependency",
          severity: "warning",
          summary: "任务依赖未满足，需要先查看执行链",
          source: "runner:dependency",
          primary_action: {
            id: "open_task_execution",
            label: "查看执行链",
          },
        },
      },
      {
        id: "task-connector-recovery",
        title: "连接器失败任务",
        description: "connector expired",
        status: "failed",
        steps: [],
        tenant_id: "t1",
        created_at: "2026-06-21T00:03:40Z",
        updated_at: "2026-06-21T00:03:40Z",
        recovery_hint: {
          category: "connector",
          severity: "warning",
          summary: "GitHub 连接器需要重新授权",
          detail: "connector github token expired",
          source: "runner:step",
          primary_action: {
            id: "repair_connector",
            label: "修复连接器",
          },
        },
      },
      {
        id: "task-browser-recovery",
        title: "浏览器失败任务",
        description: "browser pairing lost",
        status: "failed",
        steps: [],
        tenant_id: "t1",
        created_at: "2026-06-21T00:03:45Z",
        updated_at: "2026-06-21T00:03:45Z",
        recovery_hint: {
          category: "browser",
          severity: "warning",
          summary: "浏览器配对失效",
          source: "runner:step",
          primary_action: {
            id: "open_browser_pack",
            label: "打开浏览器包",
          },
        },
      },
      {
        id: "task-endpoint-recovery",
        title: "端点恢复任务",
        description: "endpoint recovery only",
        status: "failed",
        steps: [],
        tenant_id: "t1",
        created_at: "2026-06-21T00:03:00Z",
        updated_at: "2026-06-21T00:03:00Z",
        recovery_hint: {
          category: "generic",
          severity: "warning",
          summary: "可以重试任务",
          source: "runner:step",
          primary_action: {
            id: "restart_task",
            label: "重试任务",
            method: "POST",
            endpoint: "/v1/tasks/restart",
          },
        },
      },
    ]);

    render(<MissionsPage />);

    expect(await screen.findByText("模型失败任务")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /检查模型供应商/ })).toHaveAttribute(
      "href",
      "/settings/providers?tab=providers",
    );
    expect(screen.getByRole("link", { name: /查看执行链/ })).toHaveAttribute(
      "href",
      "/task-detail?id=task-dependency-recovery&tab=execution",
    );
    expect(screen.getByRole("link", { name: /修复连接器/ })).toHaveAttribute(
      "href",
      "/settings/connectors?focus=github",
    );
    expect(screen.getByRole("link", { name: /打开浏览器包/ })).toHaveAttribute("href", "/packs/browser");
    expect(screen.queryByRole("link", { name: /重试任务/ })).not.toBeInTheDocument();
  });

  it("uses labeled fields for cron and trigger creation", async () => {
    render(<MissionsPage />);

    expect(await screen.findByText("保留任务")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /定时/ }));
    fireEvent.click(screen.getByRole("button", { name: "新建定时任务" }));
    expect(screen.getByRole("textbox", { name: "任务名称" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "执行时间" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "执行内容" })).toBeInTheDocument();
    expect(screen.getByText("到时间后发给云雀的任务说明。")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /触发器/ }));
    fireEvent.click(screen.getByRole("button", { name: "新建触发器" }));
    expect(screen.getByRole("textbox", { name: "触发器名称" })).toBeInTheDocument();
    expect(screen.getByRole("radiogroup", { name: "触发方式" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /事件/ })).toBeChecked();
    expect(screen.getByRole("textbox", { name: "事件类型" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "触发后做什么" })).toBeInTheDocument();
  });
});
