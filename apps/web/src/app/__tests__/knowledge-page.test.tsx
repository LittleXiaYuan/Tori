import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import KnowledgePage from "../knowledge/page";

const apiMock = vi.hoisted(() => ({
  kbSources: vi.fn(),
  kbStats: vi.fn(),
  kbSearch: vi.fn(),
  kbUpload: vi.fn(),
  kbImportURL: vi.fn(),
  kbImportRepo: vi.fn(),
  kbIngest: vi.fn(),
  kbUpdate: vi.fn(),
  kbDelete: vi.fn(),
}));

const toastMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", () => ({
  api: apiMock,
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: toastMock,
}));

const source = {
  id: "src-1",
  name: "项目文档",
  type: "url",
  chunk_count: 12,
  trigger: "当需要查文档时",
  added_at: "2026-06-01T00:00:00Z",
};

describe("KnowledgePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiMock.kbSources.mockResolvedValue({ sources: [source] });
    apiMock.kbStats.mockResolvedValue({ sources: 1, chunks: 12, total_chars: 34000 });
    apiMock.kbSearch.mockResolvedValue({ chunks: [] });
    apiMock.kbUpload.mockResolvedValue({ ok: true });
    apiMock.kbUpdate.mockResolvedValue({ ok: true });
    apiMock.kbDelete.mockResolvedValue({ ok: true });
  });

  it("renders the header, KPI stat titles and source card", async () => {
    render(<KnowledgePage />);

    expect(await screen.findByRole("heading", { name: "知识库" })).toBeInTheDocument();
    // The four Pro KPI cards. "知识源" also appears as the sources list heading,
    // so assert at least one match rather than a unique one.
    expect(screen.getAllByText("知识源").length).toBeGreaterThan(0);
    expect(screen.getByText("总片段")).toBeInTheDocument();
    expect(screen.getByText("总字数")).toBeInTheDocument();
    expect(screen.getByText("搜索结果")).toBeInTheDocument();
    // Source loaded from kbSources
    expect(screen.getByText("项目文档")).toBeInTheDocument();
  });

  it("runs a search through api.kbSearch", async () => {
    render(<KnowledgePage />);
    await screen.findByRole("heading", { name: "知识库" });

    const input = screen.getByPlaceholderText("搜索知识库内容…");
    fireEvent.change(input, { target: { value: "测试查询" } });
    fireEvent.click(screen.getByRole("button", { name: /搜索/ }));

    await waitFor(() => {
      expect(apiMock.kbSearch).toHaveBeenCalledWith("测试查询", 20, {});
    });
  });

  it("edits a source through api.kbUpdate", async () => {
    render(<KnowledgePage />);
    await screen.findByRole("heading", { name: "知识库" });

    fireEvent.click(screen.getByRole("button", { name: "编辑知识源 项目文档" }));
    // Modal opens with the current name; change and save
    const saveBtn = await screen.findByRole("button", { name: "保存" });
    fireEvent.click(saveBtn);

    await waitFor(() => {
      expect(apiMock.kbUpdate).toHaveBeenCalledWith("src-1", "项目文档", "当需要查文档时", "");
    });
  });

  it("deletes a source from the edit modal through api.kbDelete", async () => {
    render(<KnowledgePage />);
    await screen.findByRole("heading", { name: "知识库" });

    fireEvent.click(screen.getByRole("button", { name: "编辑知识源 项目文档" }));
    const deleteBtn = await screen.findByRole("button", { name: "删除" });
    fireEvent.click(deleteBtn);

    await waitFor(() => {
      expect(apiMock.kbDelete).toHaveBeenCalledWith("src-1");
    });
  });
});
