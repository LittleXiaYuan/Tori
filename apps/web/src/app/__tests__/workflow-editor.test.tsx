import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import WorkflowEditorPage from "../workflow-editor/page";
import { api } from "@/lib/api";

const push = vi.fn();
const replace = vi.fn();
let queryId = "";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace }),
  useSearchParams: () => new URLSearchParams(queryId ? `id=${queryId}` : ""),
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    workflowGet: vi.fn(),
    workflowSave: vi.fn(),
    workflowRun: vi.fn(),
  },
}));

describe("WorkflowEditorPage", () => {
  beforeEach(() => {
    queryId = "";
    push.mockReset();
    replace.mockReset();
    vi.mocked(api.workflowGet).mockReset();
    vi.mocked(api.workflowSave).mockReset();
    vi.mocked(api.workflowRun).mockReset();
  });

  it("saves a new workflow through the workflow API", async () => {
    vi.mocked(api.workflowSave).mockResolvedValue({
      id: "wf-daily",
      name: "每日项目日报",
      description: "每天生成日报",
      version: 1,
      nodes: [],
      edges: [],
      variables: [],
      tenant_id: "default",
      created_at: "now",
      updated_at: "now",
    });

    render(<WorkflowEditorPage />);

    fireEvent.change(screen.getByLabelText("名称"), { target: { value: "每日项目日报" } });
    fireEvent.change(screen.getByLabelText("描述"), { target: { value: "每天生成日报" } });
    fireEvent.click(screen.getByText("保存"));

    await waitFor(() => expect(api.workflowSave).toHaveBeenCalledOnce());
    expect(api.workflowSave).toHaveBeenCalledWith(expect.objectContaining({
      name: "每日项目日报",
      description: "每天生成日报",
      version: 1,
    }));
    expect(replace).toHaveBeenCalledWith("/workflow-editor?id=wf-daily");
  });

  it("loads an existing workflow and can start it", async () => {
    queryId = "wf-existing";
    vi.mocked(api.workflowGet).mockResolvedValue({
      id: "wf-existing",
      name: "客户反馈分诊",
      description: "整理反馈并创建跟进任务",
      version: 2,
      nodes: [{ id: "start", name: "收集反馈", type: "input", position: { x: 0, y: 0 } }],
      edges: [],
      variables: [],
      tenant_id: "default",
      created_at: "now",
      updated_at: "now",
    });
    vi.mocked(api.workflowRun).mockResolvedValue({
      status: "accepted",
      instance_id: "inst-1",
      instance: {
        id: "inst-1",
        definition_id: "wf-existing",
        version: 2,
        status: "pending",
        tenant_id: "default",
        created_at: "now",
        updated_at: "now",
      },
    });

    render(<WorkflowEditorPage />);

    expect(await screen.findByDisplayValue("客户反馈分诊")).toBeInTheDocument();
    fireEvent.click(screen.getByText("运行"));

    await waitFor(() => expect(api.workflowRun).toHaveBeenCalledWith("wf-existing"));
    expect(push).toHaveBeenCalledWith("/workflows");
  });
});
