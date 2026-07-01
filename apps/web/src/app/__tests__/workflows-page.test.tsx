import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import WorkflowsPage from "../workflows/page";

const routerMock = vi.hoisted(() => ({
  push: vi.fn(),
}));

const apiMock = vi.hoisted(() => ({
  workflowList: vi.fn(),
  workflowInstances: vi.fn(),
  workflowGenerate: vi.fn(),
  workflowRun: vi.fn(),
  workflowDelete: vi.fn(),
  workflowCancel: vi.fn(),
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
    CheckCircle2: Icon,
    GitBranch: Icon,
    Layers: Icon,
    Pencil: Icon,
    Play: Icon,
    RefreshCw: Icon,
    Sparkles: Icon,
    Square: Icon,
    Trash2: Icon,
    Wand2: Icon,
  };
});

const workflow = {
  id: "wf-1",
  name: "日报流程",
  description: "每天整理任务进展",
  version: 1,
  tenant_id: "t1",
  nodes: [
    { id: "start", name: "开始", type: "start", position: { x: 0, y: 0 }, config: {} },
    { id: "step_1", name: "汇总任务", type: "llm", position: { x: 1, y: 0 }, config: {} },
    { id: "end", name: "结束", type: "end", position: { x: 2, y: 0 }, config: {} },
  ],
  edges: [],
  created_at: "2026-06-21T00:00:00Z",
  updated_at: "2026-06-21T00:00:00Z",
};

describe("WorkflowsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiMock.workflowList.mockResolvedValue({ workflows: [workflow], total: 1 });
    apiMock.workflowInstances.mockResolvedValue({
      instances: [
        {
          id: "inst-1",
          definition_id: "wf-1",
          status: "completed",
          created_at: "2026-06-21T00:02:00Z",
          started_at: "2026-06-21T00:03:00Z",
        },
      ],
      total: 1,
    });
    apiMock.workflowRun.mockResolvedValue({ status: "accepted", instance_id: "inst-2" });
  });

  it("presents workflows as a desktop task list with clear design and run actions", async () => {
    render(<WorkflowsPage />);

    expect(await screen.findByRole("heading", { name: "工作流" })).toBeInTheDocument();
    expect(screen.getByText("把重复动作保存成可运行流程。先用自然语言生成，再按步骤微调和试运行。")).toBeInTheDocument();
    expect(screen.getByText("日报流程")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /设计工作流/ }));
    expect(routerMock.push).toHaveBeenCalledWith("/workflow-editor");

    fireEvent.click(screen.getByRole("button", { name: /^设计$/ }));
    expect(routerMock.push).toHaveBeenCalledWith("/workflow-editor?id=wf-1");

    fireEvent.click(screen.getByRole("button", { name: /^运行$/ }));
    await waitFor(() => expect(apiMock.workflowRun).toHaveBeenCalledWith("wf-1"));
  });
});
