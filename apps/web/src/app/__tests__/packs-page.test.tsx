import { render, screen, waitFor } from "@testing-library/react";
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

    await waitFor(() => {
      expect(screen.getAllByText("怎么用它").length).toBeGreaterThanOrEqual(2);
    });
    expect(screen.getByText("用户能感知到的位置：Chat 自动发起文档任务、任务产物区、文档生成技能与模板目录")).toBeInTheDocument();
    expect(screen.getByText("用户能感知到的位置：Chat 产物区、任务结果页、文件预览与下载入口")).toBeInTheDocument();
    expect(screen.getByText("开始生成文档")).toBeInTheDocument();
    expect(screen.getByText("查看最近产物")).toBeInTheDocument();
  });
});
