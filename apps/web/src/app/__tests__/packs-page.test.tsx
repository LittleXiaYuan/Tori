import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PacksPageOptimized from "../packs/page";

const navigationMock = vi.hoisted(() => ({
  query: "",
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(navigationMock.query),
}));

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

const makeNeedsEntryPack = (index: number) => ({
  manifest: {
    ...filesManifest,
    id: `yunque.pack.needs-entry-${index}`,
    name: `Needs Entry Pack ${index}`,
    description: `补肉队列分页测试能力包 ${index}`,
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

const planOnlyManifest = {
  ...filesManifest,
  id: "yunque.pack.plan-only",
  name: "Plan Only Pack",
  status: "alpha",
  description: "实验能力：当前只生成计划，不执行真实控制。",
  metadata: {
    ...filesManifest.metadata,
    usability: "experimental",
    primaryActionLabel: "查看计划",
    primaryActionPath: "/packs/plan-only",
    usageSurface: "计划页、证据区和人工审批提示",
    example1: "生成一个需要人工确认的执行计划。",
    example2: "查看计划为什么还不能自动执行。",
    example3: "导出证据和后续转稳定待办。",
    limitation: "当前只生成计划，不执行真实控制。",
  },
};

describe("PacksPageOptimized", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    navigationMock.query = "";
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
    expect(screen.getByRole("link", { name: /能力包工坊/ })).toHaveAttribute("href", "/packs/studio");
    expect(screen.getByText("可直接使用")).toBeInTheDocument();
    expect(screen.getByText("基础能力")).toBeInTheDocument();
    expect(screen.getByText("实验中")).toBeInTheDocument();
    expect(screen.getByText("优先打磨")).toBeInTheDocument();
    expect(screen.getByText("从缺入口、实验计划和不易验证的包开始，交给小羽逐包补用途、结果、入口和回滚说明。")).toBeInTheDocument();
    expect(screen.getByText("交付状态分布")).toBeInTheDocument();
    expect(screen.getByText("可直接交付")).toBeInTheDocument();
    expect(screen.getAllByText("后台支撑").length).toBeGreaterThan(0);
    expect(screen.getByText("实验/计划")).toBeInTheDocument();
    expect(screen.getByText("有明确入口、示例和结果验证路径。")).toBeInTheDocument();
    expect(screen.getByText("在 Chat、任务、记忆、知识或设置里生效。")).toBeInTheDocument();
    expect(screen.getAllByText("说明完整").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText("通常不单独当应用打开，而是在 Chat、任务、记忆、知识或设置页里生效。")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getAllByText("怎么用它").length).toBeGreaterThanOrEqual(2);
    });
    expect(screen.getByText("用户能感知到的位置：Chat 自动发起文档任务、任务产物区、文档生成技能与模板目录")).toBeInTheDocument();
    expect(screen.getByText("用户能感知到的位置：Chat 产物区、任务结果页、文件预览与下载入口")).toBeInTheDocument();
    expect(screen.getByText("开始生成文档")).toBeInTheDocument();
    expect(screen.getByText("查看最近产物")).toBeInTheDocument();
    expect(screen.getAllByText("启用后去哪用").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("交付状态：后台支撑").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("它不一定单独打开，而是在 Chat、任务、记忆、知识或设置流程里被云雀调用。").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("下一步：从它声明的用户感知位置验证：能否在主路径里看到效果、结果或状态变化。").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText("权限：读取、写入、沙箱；启用前建议确认")).toBeInTheDocument();
    expect(screen.getByText("权限：读取；低风险")).toBeInTheDocument();
    expect(screen.getByText("主入口：开始生成文档 · 帮我生成一份可下载的文档")).toBeInTheDocument();
    expect(screen.getByText("主入口：查看最近产物 · 列出我最近生成的文件")).toBeInTheDocument();
    expect(screen.getAllByText("固定方式：没有独立侧栏入口，通常在 Chat、任务或其他能力里自动生效。").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("想继续补肉：进详情确认权限，或交给小羽补用途、入口和示例。").length).toBeGreaterThanOrEqual(2);
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
    expect(screen.getByText("当前视图：")).toBeInTheDocument();
    expect(screen.getByText("共 1 个，0 个可直接使用、1 个作为 Chat/任务/记忆/知识的底座能力、0 个仍建议先看边界再启用。")).toBeInTheDocument();
    expect(screen.getByText("来源构成")).toBeInTheDocument();
    expect(screen.getByText("已安装 1 · 官方 0 · 私有 0")).toBeInTheDocument();
    expect(screen.getByText("交付构成")).toBeInTheDocument();
    expect(screen.getByText("可交付 0 · 后台 1 · 实验 0 · 待补 0")).toBeInTheDocument();
    expect(screen.getByText("体检构成")).toBeInTheDocument();
    expect(screen.getByText("完整 1 · 补说明 0 · 补入口 0")).toBeInTheDocument();
    expect(screen.getByText("建议下一步")).toBeInTheDocument();
    expect(screen.getByText("建议从卡片里的入口或 Chat 主路径触发一次，确认结果、产物或状态变化可见。")).toBeInTheDocument();
    expect(screen.getByText("搜索：文档")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除搜索" }));

    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.getByText("Files (产物文件)")).toBeInTheDocument();
    expect(screen.getByText("未启用筛选")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重置" }));

    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.getByText("Files (产物文件)")).toBeInTheDocument();
  });

  it("focuses a pack from the URL query when returning from Studio", async () => {
    navigationMock.query = "q=yunque.pack.files&from=studio";

    render(<PacksPageOptimized />);

    expect(await screen.findByText("Files (产物文件)")).toBeInTheDocument();
    expect((screen.getByLabelText("搜索能力包") as HTMLInputElement).value).toBe("yunque.pack.files");
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText("工坊返回验收")).toBeInTheDocument();
    expect(screen.getByText("已聚焦 Files (产物文件)。先确认来源、权限和交付状态，再打开入口验证；如果结果不符合预期，可以继续回工坊补肉或禁用/回滚。")).toBeInTheDocument();
    expect(screen.getByText("搜索已聚焦")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /验收权限与来源/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.files");
    expect(screen.getByRole("link", { name: /打开入口复验/ })).toHaveAttribute("href", "/chat?q=%E5%88%97%E5%87%BA%E6%88%91%E6%9C%80%E8%BF%91%E7%94%9F%E6%88%90%E7%9A%84%E6%96%87%E4%BB%B6");
    expect(screen.getByRole("link", { name: /继续让小羽改/ })).toHaveAttribute("href", expect.stringContaining("/packs/studio?packId=yunque.pack.files"));
  });

  it("filters source and install state without hiding official release cards", async () => {
    const remoteDocsManifest = {
      ...documentsManifest,
      id: "yunque.pack.remote-docs",
      name: "Remote Docs Pack",
    };
    packsClientMock.install.mockResolvedValueOnce({ ok: true });
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
        manifest: remoteDocsManifest,
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
    expect(screen.getByText("已安装 0 · 官方 1 · 私有 0")).toBeInTheDocument();
    expect(screen.getByText("建议先打开详情或工坊只读检查，再安装、启用并回到中心验证入口。")).toBeInTheDocument();
    expect(screen.getByText("来源：官方源 · example.com")).toBeInTheDocument();
    expect(screen.getByText("https://example.com/docs.yqpack")).toBeInTheDocument();
    expect(screen.getByText("安装前看这几点")).toBeInTheDocument();
    expect(screen.getByText("确认来源")).toBeInTheDocument();
    expect(screen.getByText("理解权限")).toBeInTheDocument();
    expect(screen.getByText("能力边界")).toBeInTheDocument();
    expect(screen.getByText("回滚路径")).toBeInTheDocument();
    expect(screen.getByText("权限：读取、写入、沙箱；启用前建议确认")).toBeInTheDocument();
    expect(screen.getByText("来源：官方源 · example.com。安装前可先在工坊只读检查包内容、SHA 与能力声明。")).toBeInTheDocument();
    expect(screen.getByText("边界：不会自动泄露 API Key，不会绕过权限声明，也不能调用未声明的后端路由。")).toBeInTheDocument();
    const remoteStudioLink = screen.getAllByRole("link", { name: /小羽优化/ })
      .find((link) => link.getAttribute("href")?.includes("yunque.pack.remote-docs"));
    expect(remoteStudioLink).toHaveAttribute("href", expect.stringContaining("packageUrl=https%3A%2F%2Fexample.com%2Fdocs.yqpack"));
    expect(remoteStudioLink).toHaveAttribute("href", expect.stringContaining("sha256=abc"));

    packsClientMock.installed.mockResolvedValueOnce({
      packs: [
        { manifest: documentsManifest, status: "enabled", updatedAt: "2026-06-19T00:00:00Z" },
        { manifest: filesManifest, status: "enabled", updatedAt: "2026-06-19T00:00:00Z" },
        { manifest: remoteDocsManifest, status: "disabled", updatedAt: "2026-06-19T00:01:00Z" },
      ],
      count: 3,
    });
    packsClientMock.catalog.mockResolvedValueOnce({
      generated_at: "2026-06-19T00:01:00Z",
      sources: [],
      source_reports: [],
      count: 0,
      installed: 3,
      enabled: 2,
      downloadable: 0,
      capabilities: 0,
      entries: [],
    });
    packsClientMock.releaseCatalog.mockResolvedValueOnce({
      generated_at: "2026-06-19T00:01:00Z",
      releases: ["https://example.com/releases/tag/pack%2Fdocs%2Fv0.1.0"],
      count: 0,
      entries: [],
    });

    fireEvent.click(screen.getByRole("button", { name: "安装" }));

    await waitFor(() => {
      expect(packsClientMock.install).toHaveBeenCalledWith({
        packageUrl: "https://example.com/docs.yqpack",
        sha256: "abc",
        source: "https://example.com/releases/tag/pack%2Fdocs%2Fv0.1.0",
        download: true,
      });
    });
    expect(await screen.findByText("能力包已安装")).toBeInTheDocument();
    expect(screen.getByText("下一步先查看详情确认权限和入口，再启用；也可以继续筛选、固定或交给小羽补肉。")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /查看详情并启用/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.remote-docs");
    expect(screen.getByRole("link", { name: /交给小羽补齐/ })).toHaveAttribute("href", expect.stringContaining("/packs/studio?packId=yunque.pack.remote-docs"));
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
    expect(screen.getByText("安装前看这几点")).toBeInTheDocument();
    expect(screen.getByText("来源：私有源 · oss.example.com。安装前可先在工坊只读检查包内容、SHA 与能力声明。")).toBeInTheDocument();
    expect(screen.getByText("回滚：声明支持版本回滚；也可以随时禁用能力包。")).toBeInTheDocument();
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
    expect(screen.getByRole("button", { name: /优先打磨2/ })).toBeInTheDocument();
    expect(screen.getByText("可交付 0 · 后台 1 · 实验 0 · 待补 2")).toBeInTheDocument();
    expect(screen.getByText("完整 1 · 补说明 1 · 补入口 1")).toBeInTheDocument();
    expect(screen.getByText("建议先导入补肉队列或逐个交给小羽，补用途、入口、示例和能力边界。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /说明完整1/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /需补说明1/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /需补入口1/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /查看打磨队列/ })).toBeInTheDocument();
    expect(screen.getByText("按体检缺口和交付状态自动挑出最需要小羽补用途、入口、示例、真实结果或能力边界的能力包。当前第 1 / 1 批，展示 2 个，共 2 个待打磨。")).toBeInTheDocument();
    expect(screen.getByText("还缺：使用示例、用户感知位置、打开/使用入口、后端能力声明")).toBeInTheDocument();
    expect(screen.getByText("P0 先补可用路径：")).toBeInTheDocument();
    expect(screen.getByText("缺后端能力声明或打开入口，用户很难确认这个能力是否真的可用。")).toBeInTheDocument();
    expect(screen.getByText("P1 补用户理解：")).toBeInTheDocument();
    expect(screen.getByText("能力本体存在，但用户还缺少场景、示例或结果位置来判断价值。")).toBeInTheDocument();
    expect(screen.getAllByText("为什么进队列：").length).toBeGreaterThan(0);
    expect(screen.getByText("体检缺口：使用示例、用户感知位置、打开/使用入口、后端能力声明。")).toBeInTheDocument();
    expect(screen.getAllByText("优先修改：").length).toBeGreaterThan(0);
    expect(screen.getByText("先确认是否真有后端能力：有则补后端路由、权限和测试；没有就明确标为界面/说明型能力，不能伪造执行能力。")).toBeInTheDocument();
    expect(screen.getAllByText("验收路径：").length).toBeGreaterThan(0);
    expect(screen.getByText("改完回到能力包详情与 Chat/任务主路径验证：用户是否知道怎么触发、结果在哪里、出问题怎么禁用或回滚。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "复制批量打磨任务" })).toBeInTheDocument();
    const batchChatLink = screen.getByRole("link", { name: /交给 Chat 批量打磨/ });
    expect(batchChatLink).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    const batchPrompt = new URL(batchChatLink.getAttribute("href")!, "http://localhost").searchParams.get("q") || "";
    expect(batchPrompt).toContain("yunque.pack_studio.batch_draft_request.v1");
    expect(batchPrompt).toContain("yunque.pack.needs-entry");
    expect(batchPrompt).toContain("yunque.pack.needs-context");
    expect(batchPrompt).toContain("不要自动应用改动");
    expect(batchPrompt).toContain("预览差异");
    expect(batchPrompt).toContain("studio_url");
    expect(batchPrompt).toContain("\"delivery\"");
    expect(batchPrompt).toContain("\"permission_summary\"");
    expect(batchPrompt).toContain("\"risk\"");
    expect(batchPrompt).toContain("\"polish_guidance\"");
    expect(batchPrompt).toContain("\"priority\"");
    expect(batchPrompt).toContain("\"level\": \"P0\"");
    expect(batchPrompt).toContain("\"label\": \"P0 先补可用路径\"");
    expect(batchPrompt).toContain("\"first_edit\"");
    expect(batchPrompt).toContain("不能伪造执行能力");
    expect(batchPrompt).toContain("改完回到能力包详情与 Chat/任务主路径验证");
    const batchStudioLink = screen.getByRole("link", { name: /导入工坊逐包处理/ });
    expect(batchStudioLink).toHaveAttribute("href", expect.stringContaining("/packs/studio?batch="));
    const batchStudioPrompt = new URL(batchStudioLink.getAttribute("href")!, "http://localhost").searchParams.get("batch") || "";
    expect(batchStudioPrompt).toContain("yunque.pack_studio.batch_draft_request.v1");
    expect(batchStudioPrompt).toContain("yunque.pack.needs-entry");
    expect(batchStudioPrompt).toContain("studio_url");
    expect(batchStudioPrompt).toContain("permission_summary");
    expect(batchStudioPrompt).toContain("polish_guidance");
    expect(batchStudioPrompt).toContain("\"priority\"");
    const queueStudioLink = screen.getAllByRole("link", { name: /交给小羽补齐/ })
      .find((link) => link.getAttribute("href")?.includes("yunque.pack.needs-entry"));
    expect(queueStudioLink).toHaveAttribute("href", expect.stringContaining("/packs/studio?"));

    fireEvent.click(screen.getByRole("button", { name: "只看需补入口" }));

    expect(screen.getAllByText("Needs Entry Pack").length).toBeGreaterThan(0);
    expect(screen.getByText("体检：需补入口")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除体检" }));

    fireEvent.click(screen.getByRole("button", { name: /优先打磨2/ }));

    expect(screen.getAllByText("Needs Entry Pack").length).toBeGreaterThan(0);
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText("体检：需补入口")).toBeInTheDocument();
    expect(screen.getByText("排序：按体检")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除体检" }));
    fireEvent.click(screen.getByRole("button", { name: "恢复默认排序" }));

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

  it("paginates the readiness queue so Xiaoyu batch work stays scoped", async () => {
    packsClientMock.installed.mockResolvedValueOnce({
      packs: Array.from({ length: 8 }, (_, index) => makeNeedsEntryPack(index + 1)),
      count: 8,
    });

    render(<PacksPageOptimized />);

    expect(await screen.findByText("补肉优先队列")).toBeInTheDocument();
    const queue = screen.getByText("补肉优先队列").closest("#readiness-queue");
    expect(queue).not.toBeNull();
    expect(screen.getByText("按体检缺口和交付状态自动挑出最需要小羽补用途、入口、示例、真实结果或能力边界的能力包。当前第 1 / 2 批，展示 6 个，共 8 个待打磨。")).toBeInTheDocument();
    expect(screen.getByText("补肉队列 · 第 1 / 2 页 · 共 8 个")).toBeInTheDocument();
    expect(within(queue as HTMLElement).getByText("Needs Entry Pack 1")).toBeInTheDocument();
    expect(within(queue as HTMLElement).getByText("Needs Entry Pack 6")).toBeInTheDocument();
    expect(within(queue as HTMLElement).queryByText("Needs Entry Pack 7")).not.toBeInTheDocument();

    const firstBatchLink = screen.getByRole("link", { name: /导入工坊逐包处理/ });
    const firstBatch = new URL(firstBatchLink.getAttribute("href")!, "http://localhost").searchParams.get("batch") || "";
    expect(firstBatch).toContain("yunque.pack.needs-entry-1");
    expect(firstBatch).toContain("yunque.pack.needs-entry-6");
    expect(firstBatch).not.toContain("yunque.pack.needs-entry-7");
    expect(firstBatch).toContain("\"page\": 1");
    expect(firstBatch).toContain("\"page_count\": 2");
    expect(firstBatch).toContain("\"total\": 8");
    expect(firstBatch).toContain("\"page_size\": 6");

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));

    expect(screen.getByText("按体检缺口和交付状态自动挑出最需要小羽补用途、入口、示例、真实结果或能力边界的能力包。当前第 2 / 2 批，展示 2 个，共 8 个待打磨。")).toBeInTheDocument();
    expect(within(queue as HTMLElement).getByText("Needs Entry Pack 7")).toBeInTheDocument();
    expect(within(queue as HTMLElement).getByText("Needs Entry Pack 8")).toBeInTheDocument();
    expect(within(queue as HTMLElement).queryByText("Needs Entry Pack 1")).not.toBeInTheDocument();
    const secondBatchLink = screen.getByRole("link", { name: /导入工坊逐包处理/ });
    const secondBatch = new URL(secondBatchLink.getAttribute("href")!, "http://localhost").searchParams.get("batch") || "";
    expect(secondBatch).toContain("yunque.pack.needs-entry-7");
    expect(secondBatch).toContain("yunque.pack.needs-entry-8");
    expect(secondBatch).not.toContain("yunque.pack.needs-entry-1");
    expect(secondBatch).toContain("\"page\": 2");
    expect(secondBatch).toContain("\"page_count\": 2");
    expect(secondBatch).toContain("\"total\": 8");
  });

  it("puts complete experimental packs into the Xiaoyu polishing queue", async () => {
    packsClientMock.installed.mockResolvedValueOnce({
      packs: [
        { manifest: documentsManifest, status: "enabled", updatedAt: "2026-06-19T00:00:00Z" },
        { manifest: planOnlyManifest, status: "disabled", updatedAt: "2026-06-19T00:00:00Z" },
      ],
      count: 2,
    });

    render(<PacksPageOptimized />);

    expect((await screen.findAllByText("Plan Only Pack")).length).toBeGreaterThan(0);
    expect(screen.getByText("补肉优先队列")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /优先打磨1/ })).toBeInTheDocument();
    expect(screen.getByText("交付状态：实验/计划。先保留限制说明；如果要变成主路径，下一轮补真实执行、结果查看和回滚证据。")).toBeInTheDocument();

    const batchChatLink = screen.getByRole("link", { name: /交给 Chat 批量打磨/ });
    const batchPrompt = new URL(batchChatLink.getAttribute("href")!, "http://localhost").searchParams.get("q") || "";
    expect(batchPrompt).toContain("yunque.pack.plan-only");
    expect(batchPrompt).toContain("\"level\": \"plan_only\"");
    expect(batchPrompt).toContain("真实结果位置");
    expect(batchPrompt).toContain("不能包装成稳定承诺");
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

    expect((await screen.findAllByText("Alpha Pack")).length).toBeGreaterThan(0);
    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /实验中1/ }));

    expect(screen.getAllByText("Alpha Pack").length).toBeGreaterThan(0);
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText("类型：实验中")).toBeInTheDocument();
    expect(screen.getByText("稳定性：开发中")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除类型" }));
    fireEvent.click(screen.getByRole("button", { name: "清除稳定性" }));

    fireEvent.click(screen.getByRole("button", { name: "开发中" }));

    expect(screen.getAllByText("Alpha Pack").length).toBeGreaterThan(0);
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText("稳定性：开发中")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除稳定性" }));

    expect(screen.getAllByText("Alpha Pack").length).toBeGreaterThan(0);
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
