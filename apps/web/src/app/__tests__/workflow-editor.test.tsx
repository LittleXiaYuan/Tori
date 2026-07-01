import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import WorkflowEditorPage from "../workflow-editor/page";

const routerMock = vi.hoisted(() => ({
  push: vi.fn(),
  replace: vi.fn(),
}));

const navigationMock = vi.hoisted(() => ({
  query: "",
}));

const apiMock = vi.hoisted(() => ({
  workflowGet: vi.fn(),
  workflowSave: vi.fn(),
  workflowRun: vi.fn(),
}));

const toastMock = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  useRouter: () => routerMock,
  useSearchParams: () => new URLSearchParams(navigationMock.query),
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
    ArrowDown: Icon,
    ArrowLeft: Icon,
    ArrowUp: Icon,
    GitBranch: Icon,
    Play: Icon,
    Plus: Icon,
    Save: Icon,
    Trash2: Icon,
  };
});

describe("WorkflowEditorPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    navigationMock.query = "";
    apiMock.workflowSave.mockResolvedValue({
      id: "wf-new",
      name: "日报流程",
      description: "生成日报",
      version: 1,
      tenant_id: "t1",
      nodes: [],
      edges: [],
      created_at: "2026-06-21T00:00:00Z",
      updated_at: "2026-06-21T00:00:00Z",
    });
    apiMock.workflowRun.mockResolvedValue({ status: "accepted", instance_id: "inst-1" });
  });

  it("saves a structured linear workflow instead of asking users to edit raw JSON", async () => {
    render(<WorkflowEditorPage />);

    expect(screen.getByRole("heading", { name: /工作流设计/ })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "执行步骤" })).toBeInTheDocument();
    expect(screen.queryByText(/JSON/)).not.toBeInTheDocument();
    expect(screen.queryByText(/DAG|user_prompt|后端节点|节点配置|llm/)).not.toBeInTheDocument();
    expect(screen.getAllByText("模型处理").length).toBeGreaterThan(0);
    expect(screen.getAllByRole("button", { name: /步骤类型/ }).length).toBeGreaterThan(0);

    fireEvent.change(screen.getByLabelText("工作流名称"), { target: { value: "日报流程" } });
    fireEvent.change(screen.getByLabelText("说明"), { target: { value: "生成日报" } });
    fireEvent.change(screen.getAllByLabelText("执行说明")[0], { target: { value: "总结昨天完成的任务" } });
    fireEvent.change(screen.getAllByLabelText("执行说明")[1], { target: { value: "输出今天日报" } });

    fireEvent.click(screen.getByRole("button", { name: /保存/ }));

    await waitFor(() => {
      expect(apiMock.workflowSave).toHaveBeenCalled();
    });
    const payload = apiMock.workflowSave.mock.calls[0][0];
    expect(payload.name).toBe("日报流程");
    expect(payload.nodes.map((node: { type: string }) => node.type)).toEqual(["start", "llm", "llm", "end"]);
    expect(payload.edges).toHaveLength(3);
    expect(payload.nodes[1].config.user_prompt).toBe("总结昨天完成的任务");
    expect(routerMock.replace).toHaveBeenCalledWith("/workflow-editor?id=wf-new");
  });

  it("loads an existing workflow and can run it after editing", async () => {
    navigationMock.query = "id=wf-1";
    apiMock.workflowGet.mockResolvedValue({
      id: "wf-1",
      name: "已有流程",
      description: "已有说明",
      version: 1,
      tenant_id: "t1",
      created_at: "2026-06-21T00:00:00Z",
      updated_at: "2026-06-21T00:00:00Z",
      variables: [],
      nodes: [
        { id: "start", name: "开始", type: "start", position: { x: 80, y: 180 }, config: {} },
        { id: "collect", name: "检索资料", type: "knowledge", position: { x: 300, y: 180 }, config: { query: "项目进展", top_k: 5 } },
        { id: "end", name: "结束", type: "end", position: { x: 520, y: 180 }, config: {} },
      ],
      edges: [],
    });

    render(<WorkflowEditorPage />);

    expect(await screen.findByDisplayValue("已有流程")).toBeInTheDocument();
    expect(screen.getByDisplayValue("项目进展")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /运行/ }));

    await waitFor(() => {
      expect(apiMock.workflowRun).toHaveBeenCalledWith("wf-1");
    });
    expect(routerMock.push).toHaveBeenCalledWith("/workflows");
  });
});
