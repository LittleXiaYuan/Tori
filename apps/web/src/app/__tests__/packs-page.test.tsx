import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PacksPageOptimized from "../packs/page";

const packsClientMock = vi.hoisted(() => ({
  installed: vi.fn(),
  catalog: vi.fn(),
  releaseCatalog: vi.fn(),
  install: vi.fn(),
  enable: vi.fn(),
  disable: vi.fn(),
  rollback: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("yunque-client/packs", () => ({
  createPacksClient: () => packsClientMock,
}));

vi.mock("@/lib/sdk-client", () => ({
  createYunqueSDKClientOptions: () => ({
    baseUrl: "http://localhost",
    fetch: vi.fn(),
  }),
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

vi.mock("@/hooks/use-user-preferences", () => ({
  useNavigationPreferences: () => ({
    pinnedItems: [],
    pinItem: vi.fn(),
    unpinItem: vi.fn(),
  }),
}));

const documentsManifest = {
  id: "yunque.pack.documents",
  name: "Documents (文档生成)",
  version: "0.1.0",
  description: "文档生成能力：通过技能生成 docx/xlsx/html/pptx，并读取文档模板目录。",
  status: "beta",
  backend: {
    capabilities: ["documents.generate", "documents.templates.read"],
    permissions: ["documents:read", "documents:write", "skills:execute", "filesystem:write"],
  },
  frontend: {
    menus: [],
    assets: { type: "builtin" },
  },
  update: { channel: "stable", rollback: true },
  metadata: {
    usability: "infrastructure",
    primaryActionLabel: "开始生成文档",
    primaryActionPath: "/chat?q=%E5%B8%AE%E6%88%91%E7%94%9F%E6%88%90%E4%B8%80%E4%BB%BD%E5%8F%AF%E4%B8%8B%E8%BD%BD%E7%9A%84%E6%96%87%E6%A1%A3",
    usageSurface: "Chat 自动发起文档任务、任务产物区、文档生成技能与模板目录",
    example1: "在对话里要求云雀生成 docx、xlsx、html 或 pptx。",
    example2: "任务完成后在产物区预览或下载生成文件。",
    example3: "读取模板目录，让文档输出沿用固定格式。",
  },
};

const filesManifest = {
  id: "yunque.pack.files",
  name: "Files (产物文件)",
  version: "0.1.0",
  description: "产物文件能力：列出、预览和下载云雀生成的输出文件。",
  status: "beta",
  backend: {
    capabilities: ["files.list", "files.preview", "files.download"],
    permissions: ["files:read", "filesystem:read"],
  },
  frontend: {
    menus: [],
    assets: { type: "builtin" },
  },
  update: { channel: "stable", rollback: true },
  metadata: {
    usability: "infrastructure",
    primaryActionLabel: "查看最近产物",
    primaryActionPath: "/chat?q=%E5%88%97%E5%87%BA%E6%88%91%E6%9C%80%E8%BF%91%E7%94%9F%E6%88%90%E7%9A%84%E6%96%87%E4%BB%B6",
    usageSurface: "Chat 产物区、任务结果页、文件预览与下载入口",
    example1: "在 Chat 里列出云雀生成的文件。",
    example2: "预览或下载任务输出的报告、表格和页面。",
    example3: "把产物继续交给云雀处理、分享或沉淀。",
  },
};

const makePack = (index: number) => ({
  manifest: {
    ...filesManifest,
    id: `yunque.pack.generated-${index}`,
    name: `Generated Pack ${index}`,
    description: `分页测试能力包 ${index}`,
    metadata: {
      ...filesManifest.metadata,
      primaryActionLabel: `打开分页能力 ${index}`,
      usageSurface: `分页测试入口 ${index}`,
      example1: `分页测试示例 ${index}`,
    },
  },
  status: "disabled",
  updatedAt: "2026-06-19T00:00:00Z",
});

const needsContextManifest = {
  ...documentsManifest,
  id: "yunque.pack.needs-context",
  name: "Needs Context Pack",
  metadata: {
    ...documentsManifest.metadata,
    usageSurface: "",
  },
};

const needsEntryManifest = {
  ...filesManifest,
  id: "yunque.pack.needs-entry",
  name: "Needs Entry Pack",
  backend: {
    capabilities: [],
    permissions: [],
  },
  frontend: {
    menus: [],
    routes: [],
    assets: { type: "builtin" },
  },
  metadata: {},
};

const alphaManifest = {
  ...filesManifest,
  id: "yunque.pack.alpha",
  name: "Alpha Pack",
  status: "alpha",
};

describe("PacksPageOptimized", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    packsClientMock.installed.mockResolvedValue({
      packs: [
        { manifest: documentsManifest, status: "enabled", updatedAt: "2026-06-19T00:00:00Z" },
        { manifest: filesManifest, status: "enabled", updatedAt: "2026-06-19T00:00:00Z" },
      ],
      count: 2,
    });
    packsClientMock.catalog.mockResolvedValue({
      generated_at: "2026-06-19T00:00:00Z",
      sources: [],
      source_reports: [],
      count: 0,
      installed: 2,
      enabled: 2,
      downloadable: 0,
      capabilities: 0,
      entries: [],
    });
    packsClientMock.releaseCatalog.mockResolvedValue({
      generated_at: "2026-06-19T00:00:00Z",
      releases: [],
      count: 0,
      entries: [],
    });
  });

  it("shows how infrastructure packs are used instead of only listing backend APIs", async () => {
    render(<PacksPageOptimized />);

    expect(await screen.findByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.getByText("Files (产物文件)")).toBeInTheDocument();
    expect(screen.getByText("能力包不是都要单独打开")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Pack Studio/ })).toHaveAttribute("href", "/packs/studio");
    expect(screen.getByText("可直接使用")).toBeInTheDocument();
    expect(screen.getByText("基础能力")).toBeInTheDocument();
    expect(screen.getByText("实验中")).toBeInTheDocument();
    expect(screen.getAllByText("说明完整").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText("通常不单独当应用打开，而是在 Chat、任务、记忆、知识或设置页里生效。")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getAllByText("怎么用它").length).toBeGreaterThanOrEqual(2);
    });
    expect(screen.getByText("用户能感知到的位置：Chat 自动发起文档任务、任务产物区、文档生成技能与模板目录")).toBeInTheDocument();
    expect(screen.getByText("用户能感知到的位置：Chat 产物区、任务结果页、文件预览与下载入口")).toBeInTheDocument();
    expect(screen.getByText("开始生成文档")).toBeInTheDocument();
    expect(screen.getByText("查看最近产物")).toBeInTheDocument();
    const studioLinks = screen.getAllByRole("link", { name: /小羽优化/ });
    expect(studioLinks[0]).toHaveAttribute("href", expect.stringContaining("/packs/studio?packId=yunque.pack.documents"));
    expect(decodeURIComponent(studioLinks[0].getAttribute("href") || "").replace(/\+/g, " ")).toContain("让 Documents (文档生成) 更像一个用户能直接理解和使用的能力包");
    expect(decodeURIComponent(studioLinks[0].getAttribute("href") || "").replace(/\+/g, " ")).toContain("继续打磨更具体的用户场景和入口反馈");
  });

  it("filters installed packs by search and resets the store filters", async () => {
    render(<PacksPageOptimized />);

    expect(await screen.findByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.getByText("Files (产物文件)")).toBeInTheDocument();
    expect(screen.getByText("未启用筛选")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("搜索能力包"), { target: { value: "文档" } });

    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.queryByText("Files (产物文件)")).not.toBeInTheDocument();
    expect(screen.getByText(/匹配 1 个/)).toBeInTheDocument();
    expect(screen.getByText("搜索：文档")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除搜索" }));

    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.getByText("Files (产物文件)")).toBeInTheDocument();
    expect(screen.getByText("未启用筛选")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重置" }));

    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.getByText("Files (产物文件)")).toBeInTheDocument();
  });

  it("filters source and install state without hiding official release cards", async () => {
    packsClientMock.releaseCatalog.mockResolvedValueOnce({
      generated_at: "2026-06-19T00:00:00Z",
      releases: ["https://example.com/releases/tag/pack%2Fdocs%2Fv0.1.0"],
      count: 1,
      entries: [{
        release_url: "https://example.com/releases/tag/pack%2Fdocs%2Fv0.1.0",
        release_tag: "pack/docs/v0.1.0",
        package_url: "https://example.com/docs.yqpack",
        package_name: "docs.yqpack",
        size_bytes: 2048,
        sha256: "abc",
        manifest: {
          ...documentsManifest,
          id: "yunque.pack.remote-docs",
          name: "Remote Docs Pack",
        },
      }],
    });

    render(<PacksPageOptimized />);

    expect(await screen.findByText("Remote Docs Pack")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "可安装" }));
    fireEvent.click(screen.getByRole("button", { name: "官方" }));

    expect(screen.getByText("Remote Docs Pack")).toBeInTheDocument();
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText(/官方源 1/)).toBeInTheDocument();
    expect(screen.getByText("状态：可安装")).toBeInTheDocument();
    expect(screen.getByText("来源：官方源")).toBeInTheDocument();
    expect(screen.getByText("来源：官方源 · example.com")).toBeInTheDocument();
    expect(screen.getByText("https://example.com/docs.yqpack")).toBeInTheDocument();
    const remoteStudioLink = screen.getAllByRole("link", { name: /小羽优化/ })
      .find((link) => link.getAttribute("href")?.includes("yunque.pack.remote-docs"));
    expect(remoteStudioLink).toHaveAttribute("href", expect.stringContaining("packageUrl=https%3A%2F%2Fexample.com%2Fdocs.yqpack"));
    expect(remoteStudioLink).toHaveAttribute("href", expect.stringContaining("sha256=abc"));
  });

  it("shows private catalog source origin on installable cards", async () => {
    packsClientMock.catalog.mockResolvedValueOnce({
      generated_at: "2026-06-19T00:00:00Z",
      sources: ["https://oss.example.com/yunque/private/catalog.json"],
      source_reports: [{
        source: "https://oss.example.com/yunque/private/catalog.json",
        ok: true,
        manifest_count: 1,
        matched_entries: 1,
      }],
      count: 1,
      installed: 0,
      enabled: 0,
      downloadable: 1,
      capabilities: 1,
      entries: [{
        source: "https://oss.example.com/yunque/private/catalog.json",
        manifest_url: "https://oss.example.com/yunque/private/private-pack.json",
        package_url: "https://oss.example.com/yunque/private/private-pack.yqpack",
        sha256: "def",
        downloadable: true,
        installed: false,
        enabled: false,
        manifest: {
          ...filesManifest,
          id: "yunque.pack.private-files",
          name: "Private Files Pack",
        },
      }],
    });

    render(<PacksPageOptimized />);

    expect(await screen.findByText("Private Files Pack")).toBeInTheDocument();
    expect(screen.getByText("源可用")).toBeInTheDocument();
    expect(screen.getByText("来源：私有源 · oss.example.com")).toBeInTheDocument();
    expect(screen.getByText("https://oss.example.com/yunque/private/private-pack.yqpack")).toBeInTheDocument();
  });

  it("filters packs by readiness so unclear packs can be sent to Xiaoyu first", async () => {
    packsClientMock.installed.mockResolvedValueOnce({
      packs: [
        { manifest: documentsManifest, status: "enabled", updatedAt: "2026-06-19T00:00:00Z" },
        { manifest: needsContextManifest, status: "disabled", updatedAt: "2026-06-19T00:00:00Z" },
        { manifest: needsEntryManifest, status: "disabled", updatedAt: "2026-06-19T00:00:00Z" },
      ],
      count: 3,
    });

    render(<PacksPageOptimized />);

    expect((await screen.findAllByText("Needs Context Pack")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("Needs Entry Pack").length).toBeGreaterThan(0);
    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.getByText("补肉优先队列")).toBeInTheDocument();
    expect(screen.getByText("补肉优先队列").closest("#readiness-queue")).not.toBeNull();
    expect(screen.getByText("能力包体检总览")).toBeInTheDocument();
    expect(screen.getByText("已体检 3 个能力包，按用途说明、用户能感知的位置、入口和后端能力声明判断是否需要补肉。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /说明完整1/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /需补说明1/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /需补入口1/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /查看补肉队列/ })).toBeInTheDocument();
    expect(screen.getByText("按体检缺口自动挑出最需要小羽补用途、入口、示例或能力边界的能力包。当前展示 2 个，共 2 个待补肉。")).toBeInTheDocument();
    expect(screen.getByText("还缺：使用示例、用户感知位置、打开/使用入口、后端能力声明")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "复制批量补肉任务" })).toBeInTheDocument();
    const batchChatLink = screen.getByRole("link", { name: /交给 Chat 批量补肉/ });
    expect(batchChatLink).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    const batchPrompt = new URL(batchChatLink.getAttribute("href")!, "http://localhost").searchParams.get("q") || "";
    expect(batchPrompt).toContain("yunque.pack_studio.batch_draft_request.v1");
    expect(batchPrompt).toContain("yunque.pack.needs-entry");
    expect(batchPrompt).toContain("yunque.pack.needs-context");
    expect(batchPrompt).toContain("不要自动应用改动");
    expect(batchPrompt).toContain("预览 diff");
    expect(batchPrompt).toContain("studio_url");
    const queueStudioLink = screen.getAllByRole("link", { name: /交给小羽补齐/ })
      .find((link) => link.getAttribute("href")?.includes("yunque.pack.needs-entry"));
    expect(queueStudioLink).toHaveAttribute("href", expect.stringContaining("/packs/studio?"));

    fireEvent.click(screen.getByRole("button", { name: "只看需补入口" }));

    expect(screen.getAllByText("Needs Entry Pack").length).toBeGreaterThan(0);
    expect(screen.getByText("体检：需补入口")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除体检" }));

    fireEvent.click(screen.getByRole("button", { name: /需补入口1/ }));

    expect(screen.getAllByText("Needs Entry Pack").length).toBeGreaterThan(0);
    expect(screen.getByText("体检：需补入口")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除体检" }));

    fireEvent.click(screen.getByRole("button", { name: "补说明" }));

    expect(screen.getAllByText("Needs Context Pack").length).toBeGreaterThan(0);
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText("体检：需补说明")).toBeInTheDocument();
    expect(screen.getByText("可用性体检：还缺 用户感知位置。可以交给小羽优化补齐用途、入口或使用说明。")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除体检" }));

    expect(screen.getAllByText("Needs Context Pack").length).toBeGreaterThan(0);
    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
  });

  it("filters packs by stability so users can avoid experimental packs", async () => {
    packsClientMock.installed.mockResolvedValueOnce({
      packs: [
        { manifest: documentsManifest, status: "enabled", updatedAt: "2026-06-19T00:00:00Z" },
        { manifest: alphaManifest, status: "disabled", updatedAt: "2026-06-19T00:00:00Z" },
      ],
      count: 2,
    });

    render(<PacksPageOptimized />);

    expect(await screen.findByText("Alpha Pack")).toBeInTheDocument();
    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "开发中" }));

    expect(screen.getByText("Alpha Pack")).toBeInTheDocument();
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText("稳定性：开发中")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除稳定性" }));

    expect(screen.getByText("Alpha Pack")).toBeInTheDocument();
    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
  });

  it("paginates installed packs so a large pack set stays scannable", async () => {
    packsClientMock.installed.mockResolvedValueOnce({
      packs: [
        { manifest: documentsManifest, status: "enabled", updatedAt: "2026-06-19T00:00:00Z" },
        ...Array.from({ length: 13 }, (_, index) => makePack(index + 1)),
      ],
      count: 14,
    });

    render(<PacksPageOptimized />);

    expect(await screen.findByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.queryByText("Generated Pack 9")).not.toBeInTheDocument();
    expect(screen.getByText("已安装 · 第 1 / 2 页 · 共 14 个")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));

    expect(screen.getByText("Generated Pack 9")).toBeInTheDocument();
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText("已安装 · 第 2 / 2 页 · 共 14 个")).toBeInTheDocument();
  });
});
