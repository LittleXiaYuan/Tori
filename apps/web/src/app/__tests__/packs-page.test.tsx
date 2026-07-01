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
  backendRouteAudit: vi.fn(),
}));
const toastMock = vi.hoisted(() => vi.fn());

const openFilterDrawer = () => {
  fireEvent.click(screen.getByRole("button", { name: /^找能力包/ }));
};

const openStoreSources = () => {
  const trigger = screen.queryByRole("button", { name: /^展开来源/ });
  if (trigger) fireEvent.click(trigger);
};

const cardFor = (title: string) => {
  // Installed packs render as compact .pack-row rows; installable packs and
  // other surfaces still render as .section-card. Match whichever wraps the title.
  const titleElement = screen.getAllByText(title).find((element) => element.closest(".section-card") || element.closest(".pack-row"));
  expect(titleElement).toBeTruthy();
  const card = titleElement?.closest(".pack-row") || titleElement?.closest(".section-card");
  expect(card).not.toBeNull();
  return card as HTMLElement;
};

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
  showToast: toastMock,
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
    description: `打磨队列分页测试能力包 ${index}`,
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
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
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
    packsClientMock.backendRouteAudit.mockResolvedValue({
      generated_at: "2026-06-19T00:00:00Z",
      packs: 0,
      enabled_packs: 0,
      mounted_modules: 0,
      declared_routes: 0,
      mounted_routes: 0,
      ok_routes: 0,
      missing_routes: 0,
      method_mismatches: 0,
      undeclared_routes: 0,
      entries: [],
    });
  });

  it("shows how infrastructure packs are used instead of only listing backend APIs", async () => {
    const { rerender } = render(<PacksPageOptimized />);

    expect(await screen.findByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.getByText("Files (产物文件)")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /添加能力包/ })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /能力包工坊/ })).not.toBeInTheDocument();
    expect(screen.queryByText("能力包不是都要单独打开")).not.toBeInTheDocument();
    expect(screen.queryByText("交付状态分布")).not.toBeInTheDocument();
    expect(screen.queryByText("能力包体检总览")).not.toBeInTheDocument();
    expect(screen.queryByText("打磨与验收队列")).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /小羽优化/ })).not.toBeInTheDocument();
    expect(screen.queryByText("说明完整")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /维护视图/ })).not.toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("开始生成文档")).toBeInTheDocument();
    });
    expect(screen.getByText("查看最近产物")).toBeInTheDocument();
    const docsCard = cardFor("Documents (文档生成)");
    expect(within(docsCard).queryByText("验源")).not.toBeInTheDocument();
    expect(within(docsCard).queryByText("已安装 · 已启用")).not.toBeInTheDocument();
    expect(within(docsCard).queryByText("可禁用/回滚")).not.toBeInTheDocument();
    expect(within(docsCard).queryByText("测试版")).not.toBeInTheDocument();
    // 普通态（零基础视图）：信任摘要（运行/信任/交付）折叠进高级态，
    // 卡片只保留名字+描述+状态chip+开关，降低对普通用户的复杂度。
    expect(within(docsCard).queryByText("运行")).not.toBeInTheDocument();
    expect(within(docsCard).queryByText("信任")).not.toBeInTheDocument();
    expect(within(docsCard).queryByText("需留意")).not.toBeInTheDocument();
    expect(within(docsCard).queryByText("交付")).not.toBeInTheDocument();
    expect(within(docsCard).queryByText(/入口 \/chat\?q=/)).not.toBeInTheDocument();
    // 风险评级（低风险/需留意）随信任摘要一起折叠进高级态，普通态不显示。
    expect(within(cardFor("Files (产物文件)")).queryByText("低风险")).not.toBeInTheDocument();

    navigationMock.query = "maintenance=1";
    rerender(<PacksPageOptimized />);
    expect(screen.getByRole("link", { name: /能力包工坊/ })).toHaveAttribute("href", "/packs/studio");
    expect(screen.getByText("能力包不是都要单独打开")).toBeInTheDocument();
    expect(screen.getByText("按入口、底座、实验、待打磨分流。")).toBeInTheDocument();
    expect(screen.queryByText("入口、底座、实验态；按能否验证价值处理。")).not.toBeInTheDocument();
    expect(screen.queryByText(/云雀会把能力包分成三类/)).not.toBeInTheDocument();
    expect(screen.getByText("交付状态分布")).toBeInTheDocument();
    expect(screen.getByText("能力包体检总览")).toBeInTheDocument();
    expect(screen.getAllByText("说明完整").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText("在 Chat、任务、记忆、知识或设置页生效。")).toBeInTheDocument();
    expect(screen.queryByText("通常不单独当应用打开，而是在 Chat、任务、记忆、知识或设置页里生效。")).not.toBeInTheDocument();
    const maintenanceDocsCard = cardFor("Documents (文档生成)");
    expect(within(maintenanceDocsCard).getByText(/入口 \/chat\?q=/)).toBeVisible();
    expect(within(maintenanceDocsCard).getByText("验源")).toBeVisible();
    // 信任摘要在维护/高级态回归可见
    expect(within(maintenanceDocsCard).getByText("运行")).toBeVisible();
    expect(within(maintenanceDocsCard).getByText("信任")).toBeVisible();
    expect(within(maintenanceDocsCard).getByText("交付")).toBeVisible();
    expect(within(maintenanceDocsCard).getByText("已安装 · 已启用")).toBeVisible();
    expect(within(maintenanceDocsCard).getByText("可禁用/回滚")).toBeVisible();
    expect(within(maintenanceDocsCard).getByText("测试版")).toBeVisible();
    const detailsButton = within(maintenanceDocsCard).getByRole("button", { name: "展开详情" });
    expect(detailsButton).toHaveAttribute("aria-expanded", "false");
    expect(within(maintenanceDocsCard).getByText("用户能感知到的位置：Chat 自动发起文档任务、任务产物区、文档生成技能与模板目录")).not.toBeVisible();
    fireEvent.click(detailsButton);
    expect(within(maintenanceDocsCard).getByRole("button", { name: "收起详情" })).toHaveAttribute("aria-expanded", "true");
    expect(within(maintenanceDocsCard).getByText("用户能感知到的位置：Chat 自动发起文档任务、任务产物区、文档生成技能与模板目录")).toBeVisible();
    expect(within(maintenanceDocsCard).getByText("主入口：开始生成文档 · 帮我生成一份可下载的文档")).toBeVisible();
    const studioLink = screen.getAllByRole("link", { name: /小羽优化/ })
      .find((link) => link.getAttribute("href")?.includes("/packs/studio?packId=yunque.pack.documents"));
    expect(studioLink).toBeTruthy();
    expect(decodeURIComponent(studioLink?.getAttribute("href") || "").replace(/\+/g, " ")).toContain("让 Documents (文档生成) 更像一个用户能直接理解和使用的能力包");
    expect(decodeURIComponent(studioLink?.getAttribute("href") || "").replace(/\+/g, " ")).toContain("继续打磨更具体的用户场景和入口反馈");
  });

  it("filters installed packs by search and resets the store filters", async () => {
    const { rerender } = render(<PacksPageOptimized />);

    expect(await screen.findByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.getByText("Files (产物文件)")).toBeInTheDocument();
    expect(screen.getByText("未启用筛选")).toBeInTheDocument();

    openFilterDrawer();
    fireEvent.change(screen.getByLabelText("找能力包"), { target: { value: "文档" } });

    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.queryByText("Files (产物文件)")).not.toBeInTheDocument();
    expect(screen.getByText(/匹配 1 个/)).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "筛选能力包" })).toBeInTheDocument();
    expect(screen.getByText("搜索：文档")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("找能力包"), { target: { value: "不存在" } });
    expect(screen.getByText("没有符合筛选条件的已安装能力包")).toBeInTheDocument();
    expect(screen.getByText("可以清空搜索，或切换类型、状态、风险和来源信任。")).toBeInTheDocument();
    expect(screen.queryByText("可以清空搜索，或切换类型、状态、风险和来源筛选。")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("找能力包"), { target: { value: "文档" } });

    expect(screen.getByRole("dialog", { name: "筛选能力包" })).toBeInTheDocument();
    expect(screen.getByText("来源构成")).not.toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "更多筛选" }));
    expect(screen.getByText("来源构成")).toBeInTheDocument();
    expect(screen.getByText("已安装 1 · 官方 0 · 私有 0")).toBeInTheDocument();
    expect(screen.getByText("交付构成")).toBeInTheDocument();
    expect(screen.getByText("可交付 0 · 后台 1 · 实验 0 · 待打磨 0")).toBeInTheDocument();
    expect(screen.getByText("当前视图")).toBeInTheDocument();
    expect(screen.getByText("共 1 个 · 可用 0 · 基础 1 · 实验 0")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "完成" }));

    fireEvent.click(screen.getByRole("button", { name: "清除搜索" }));

    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.getByText("Files (产物文件)")).toBeInTheDocument();
    expect(screen.getByText("未启用筛选")).toBeInTheDocument();

    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.getByText("Files (产物文件)")).toBeInTheDocument();
  });

  it("focuses a pack from the URL query when returning from Studio", async () => {
    navigationMock.query = "q=yunque.pack.files&from=studio";

    const { rerender } = render(<PacksPageOptimized />);

    expect(await screen.findByText("Files (产物文件)")).toBeInTheDocument();
    openFilterDrawer();
    expect((screen.getByLabelText("找能力包") as HTMLInputElement).value).toBe("yunque.pack.files");
    fireEvent.click(screen.getByRole("button", { name: "完成" }));
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText("工坊返回验收")).toBeInTheDocument();
    expect(screen.getByText("已聚焦 Files (产物文件)。查权限，复验入口；不符就回工坊或禁用。")).toBeInTheDocument();
    expect(screen.queryByText("已聚焦 Files (产物文件)。先看权限和状态，再打开入口复验；不符预期就回工坊或禁用。")).not.toBeInTheDocument();
    expect(screen.queryByText(/先确认来源、权限和交付状态/)).not.toBeInTheDocument();
    expect(screen.getByText("搜索已聚焦")).toBeInTheDocument();
    const returnPanel = screen.getByText("工坊返回验收").closest("div")?.parentElement?.parentElement;
    expect(returnPanel).not.toBeNull();
    expect(within(returnPanel as HTMLElement).getByText("先触发一次")).toBeInTheDocument();
    expect(within(returnPanel as HTMLElement).getByText(/从「查看最近产物」进入/)).toBeInTheDocument();
    expect(within(returnPanel as HTMLElement).getByText("看结果在哪")).toBeInTheDocument();
    expect(within(returnPanel as HTMLElement).getByText(/到 Chat 产物区、任务结果页、文件预览与下载入口/)).toBeInTheDocument();
    expect(within(returnPanel as HTMLElement).getByText("复验失败怎么退")).toBeInTheDocument();
    expect(within(returnPanel as HTMLElement).getByText("先禁用；有上一版再回滚。仍不符就交给小羽改。")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /权限与详情/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.files");
    expect(screen.getByRole("link", { name: /打开入口复验/ })).toHaveAttribute("href", "/chat?q=%E5%88%97%E5%87%BA%E6%88%91%E6%9C%80%E8%BF%91%E7%94%9F%E6%88%90%E7%9A%84%E6%96%87%E4%BB%B6");
    expect(screen.getByRole("link", { name: /继续让小羽改/ })).toHaveAttribute("href", expect.stringContaining("/packs/studio?packId=yunque.pack.files"));
  });

  it("filters source and install state without hiding official release cards", async () => {
    // Actionable usability so the card shows in the default catalog (infra
    // kind is hidden by default now); this test is about source/install
    // filtering, not the default-hide rule.
    const remoteDocsManifest = {
      ...documentsManifest,
      id: "yunque.pack.remote-docs",
      name: "Remote Docs Pack",
      metadata: {
        ...documentsManifest.metadata,
        usability: "actionable",
      },
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

    const { rerender } = render(<PacksPageOptimized />);

    expect(await screen.findByText("Remote Docs Pack")).toBeInTheDocument();

    openFilterDrawer();
    fireEvent.click(screen.getByRole("radio", { name: "可安装" }));
    fireEvent.click(screen.getByRole("radio", { name: "官方源" }));
    expect(screen.getByText("已安装 0 · 官方 1 · 私有 0")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "完成" }));

    expect(screen.getByText("Remote Docs Pack")).toBeInTheDocument();
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText(/当前匹配 1 个/)).toBeInTheDocument();
    expect(screen.getByText("状态：可安装")).toBeInTheDocument();
    expect(screen.getByText("包含可安装来源；展开看官方、私有或本地。")).toBeInTheDocument();
    expect(screen.queryByText(/当前筛选包含可安装来源/)).not.toBeInTheDocument();
    const remoteDocsCard = cardFor("Remote Docs Pack");
    expect(within(remoteDocsCard).getByText("验源")).toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("官方源 · example.com")).toBeInTheDocument();
    expect(within(remoteDocsCard).queryByText("安装前只读检查")).not.toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("有后端能力")).toBeInTheDocument();
    expect(within(remoteDocsCard).queryByText("后端能力")).not.toBeInTheDocument();
    expect(within(remoteDocsCard).getAllByText("安装").length).toBeGreaterThan(0);
    expect(within(remoteDocsCard).getByText("信任")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /展开来源/ })).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(screen.getByRole("button", { name: /展开来源/ }));
    expect(screen.getByRole("button", { name: /收起来源/ })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("button", { name: "来源与安装诊断" })).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(screen.getByRole("button", { name: "来源与安装诊断" }));
    expect(screen.getByText("安装失败怎么处理")).toBeInTheDocument();
    expect(screen.getByText("下载失败")).toBeInTheDocument();
    expect(screen.getAllByText("SHA 不匹配").length).toBeGreaterThan(0);
    expect(screen.getAllByText("签名失败").length).toBeGreaterThan(0);
    expect(screen.getByText("来源：官方源 · example.com")).not.toBeVisible();
    expect(screen.queryByRole("link", { name: /小羽优化/ })).not.toBeInTheDocument();

    navigationMock.query = "maintenance=1";
    rerender(<PacksPageOptimized />);
    fireEvent.click(within(remoteDocsCard).getByRole("button", { name: "展开详情" }));
    expect(within(remoteDocsCard).getByText("安装前只读检查")).toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("来源：官方源 · example.com")).toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("https://example.com/docs.yqpack")).toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("SHA256 abc")).toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("安装前看这几点")).toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("确认来源")).toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("理解权限")).toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("能力边界")).toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("回滚路径")).toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("权限：读取、写入、沙箱；启用前建议确认")).toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("来源：官方源 · example.com。安装前可先在工坊只读检查包内容、SHA 与能力声明。")).toBeInTheDocument();
    expect(within(remoteDocsCard).getByText("边界：不会自动泄露 API Key，不会绕过权限声明，也不能调用未声明的后端路由。")).toBeInTheDocument();
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
    packsClientMock.installed.mockResolvedValueOnce({
      packs: [
        { manifest: documentsManifest, status: "enabled", updatedAt: "2026-06-19T00:00:00Z" },
        { manifest: filesManifest, status: "enabled", updatedAt: "2026-06-19T00:00:00Z" },
        { manifest: remoteDocsManifest, status: "enabled", updatedAt: "2026-06-19T00:02:00Z" },
      ],
      count: 3,
    });
    packsClientMock.catalog.mockResolvedValueOnce({
      generated_at: "2026-06-19T00:02:00Z",
      sources: [],
      source_reports: [],
      count: 0,
      installed: 3,
      enabled: 3,
      downloadable: 0,
      capabilities: 0,
      entries: [],
    });
    packsClientMock.releaseCatalog.mockResolvedValueOnce({
      generated_at: "2026-06-19T00:02:00Z",
      releases: ["https://example.com/releases/tag/pack%2Fdocs%2Fv0.1.0"],
      count: 0,
      entries: [],
    });

    fireEvent.click(screen.getAllByRole("button", { name: "安装" })[0]);

    await waitFor(() => {
      expect(packsClientMock.install).toHaveBeenCalledWith({
        packageUrl: "https://example.com/docs.yqpack",
        sha256: "abc",
        source: "https://example.com/releases/tag/pack%2Fdocs%2Fv0.1.0",
        download: true,
      });
    });
    expect(await screen.findByText("能力包已安装")).toBeInTheDocument();
    expect(screen.getByText("下一步先查看详情确认权限和入口，再启用；也可以继续筛选、固定或交给小羽打磨。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /立即启用/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /查看详情并启用/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.remote-docs");
    expect(screen.getByRole("link", { name: /交给小羽打磨/ })).toHaveAttribute("href", expect.stringContaining("/packs/studio?packId=yunque.pack.remote-docs"));
    fireEvent.click(screen.getByRole("button", { name: /立即启用/ }));
    await waitFor(() => {
      expect(packsClientMock.enable).toHaveBeenCalledWith("yunque.pack.remote-docs");
    });
    expect(await screen.findByText("能力包已启用")).toBeInTheDocument();
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
          // Actionable so the card shows in the default catalog (infra kind is
          // hidden by default now); this test is about the private source
          // origin badge, not the default-hide rule.
          metadata: {
            ...filesManifest.metadata,
            usability: "actionable",
          },
        },
      }],
    });

    const { rerender } = render(<PacksPageOptimized />);

    expect(await screen.findByText("Private Files Pack")).toBeInTheDocument();
    expect(screen.getByText("来源：私有源 · oss.example.com")).not.toBeVisible();
    expect(screen.getByRole("button", { name: /^展开来源/ })).toHaveAttribute("aria-expanded", "false");
    openStoreSources();
    expect(screen.getByRole("button", { name: "私有源诊断" })).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(screen.getByRole("button", { name: "私有源诊断" }));
    expect(screen.getByText("源可用")).toBeInTheDocument();
    expect(screen.getByText("私有源安装前确认")).toBeInTheDocument();
    expect(screen.getByText("声明不合法")).toBeInTheDocument();
    expect(screen.getByText("平台不支持")).toBeInTheDocument();

    navigationMock.query = "maintenance=1";
    rerender(<PacksPageOptimized />);
    const privateFilesCard = cardFor("Private Files Pack");
    expect(within(privateFilesCard).getByText("验源")).toBeInTheDocument();
    expect(within(privateFilesCard).getByText("先验 SHA/权限")).toBeInTheDocument();
    fireEvent.click(within(privateFilesCard).getByRole("button", { name: "展开详情" }));
    expect(within(privateFilesCard).getByText("来源：私有源 · oss.example.com")).toBeInTheDocument();
    expect(within(privateFilesCard).getByText("https://oss.example.com/yunque/private/private-pack.yqpack")).toBeInTheDocument();
    expect(within(privateFilesCard).getByText("SHA256 def")).toBeInTheDocument();
    expect(within(privateFilesCard).getByText("安装前看这几点")).toBeInTheDocument();
    expect(within(privateFilesCard).getByText("来源：私有源 · oss.example.com。安装前可先在工坊只读检查包内容、SHA 与能力声明。")).toBeInTheDocument();
    expect(within(privateFilesCard).getByText("回滚：声明支持版本回滚；也可以随时禁用能力包。")).toBeInTheDocument();
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

    const { rerender } = render(<PacksPageOptimized />);

    expect((await screen.findAllByText("Needs Context Pack")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("Needs Entry Pack").length).toBeGreaterThan(0);
    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
    expect(screen.queryByText("打磨与验收队列")).not.toBeInTheDocument();
    expect(screen.queryByText("能力包体检总览")).not.toBeInTheDocument();
    expect(screen.queryByText(/可用性体检：还缺/)).not.toBeInTheDocument();

    navigationMock.query = "maintenance=1";
    rerender(<PacksPageOptimized />);
    expect(screen.getByText("打磨与验收队列")).toBeInTheDocument();
    expect(screen.getByText("打磨与验收队列").closest("#readiness-queue")).not.toBeNull();
    expect(screen.getByText("能力包体检总览")).toBeInTheDocument();
    expect(screen.getByText("已体检 3 个；先看 P0。")).toBeInTheDocument();
    expect(screen.queryByText("已体检 3 个；P0 阻塞验收，P1/P2 补用途、入口或边界。")).not.toBeInTheDocument();
    expect(screen.queryByText(/按用途说明、用户能感知的位置、入口和后端能力声明/)).not.toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /复制体检报告 JSON/ }).length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: /优先打磨2/ })).toBeInTheDocument();
    expect(screen.getByText("交付状态分布")).toBeInTheDocument();
    expect(screen.getByText("能否直接验价值。")).toBeInTheDocument();
    expect(screen.queryByText(/安装后能否直接验证价值/)).not.toBeInTheDocument();
    expect(screen.getAllByText("后台支撑").length).toBeGreaterThan(0);
    expect(screen.getAllByText("需打磨").length).toBeGreaterThan(0);
    expect(screen.getByText("不当稳定主路径。")).toBeInTheDocument();
    expect(screen.getByText("缺入口或验收路径。")).toBeInTheDocument();
    expect(screen.queryByText(/不包装成稳定主路径/)).not.toBeInTheDocument();
    expect(screen.queryByText(/不等于不可用/)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /说明完整1/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /需补说明1/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /需补入口1/ })).toBeInTheDocument();
    expect(screen.getByText("清楚，可展示。")).toBeInTheDocument();
    expect(screen.queryByText(/用户能看懂用途、入口、示例和能力边界/)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /查看打磨队列/ })).toBeInTheDocument();
    expect(screen.getByText("拦 404、未挂载路由和权限缺口。")).toBeInTheDocument();
    expect(screen.queryByText(/维护视图会合并运行态 route audit/)).not.toBeInTheDocument();
    expect(screen.getByText("第 1 / 1 批 · 2 / 2 个。")).toBeInTheDocument();
    expect(screen.queryByText("按 P0/P1/P2 排队。第 1 / 1 批，展示 2 / 2 个。")).not.toBeInTheDocument();
    expect(screen.queryByText(/P1\/P2 多数是可用但需要讲清楚边界和结果/)).not.toBeInTheDocument();
    expect(screen.getByText("本批焦点")).toBeInTheDocument();
    expect(screen.getByText("P0 1 · P1 1 · P2 0；缺入口 1 · 补说明 1")).toBeInTheDocument();
    expect(screen.getByText("处理顺序")).toBeInTheDocument();
    expect(screen.getByText("P0 进工坊；P1/P2 先复验入口再重包。")).toBeInTheDocument();
    expect(screen.getByText("验收")).toBeInTheDocument();
    expect(screen.getByText("1 个有入口；其余走 Chat、任务、记忆或知识。")).toBeInTheDocument();
    expect(screen.queryByText("验收出口")).not.toBeInTheDocument();
    expect(screen.getByText("边界提醒")).toBeInTheDocument();
    expect(screen.getByText("高风险 0 · 计划态 0 · 审计阻塞 0。")).toBeInTheDocument();
    expect(screen.queryByText(/不能把计划能力包装成稳定执行/)).not.toBeInTheDocument();
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
    expect(screen.getAllByText((_, node) => node?.textContent === "来源：已安装").length).toBeGreaterThan(0);
    expect(screen.getAllByText((_, node) => node?.textContent === "权限：权限：未声明额外权限；低风险").length).toBeGreaterThan(0);
    expect(screen.getAllByText((_, node) => node?.textContent === "先做：先看详情确认是否缺入口或能力声明，再进工坊只读检查。").length).toBeGreaterThan(0);
    const queueDetailLink = screen.getAllByRole("link", { name: /权限与详情/ })
      .find((link) => link.getAttribute("href") === "/packs/detail?id=yunque.pack.needs-entry");
    expect(queueDetailLink).toBeTruthy();
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
    expect(batchPrompt).toContain("\"handoff_links\"");
    expect(batchPrompt).toContain("\"center\": \"/packs?q=yunque.pack.needs-entry&from=studio\"");
    expect(batchPrompt).toContain("\"detail\": \"/packs/detail?id=yunque.pack.needs-entry\"");
    expect(batchPrompt).toContain("\"open\": null");
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
    expect(batchStudioPrompt).toContain("handoff_links");
    expect(batchStudioPrompt).toContain("/packs?q=yunque.pack.needs-entry&from=studio");
    expect(batchStudioPrompt).toContain("/packs/detail?id=yunque.pack.needs-entry");
    expect(batchStudioPrompt).toContain("permission_summary");
    expect(batchStudioPrompt).toContain("polish_guidance");
    expect(batchStudioPrompt).toContain("\"priority\"");
    fireEvent.click(screen.getAllByRole("button", { name: /复制体检报告 JSON/ })[0]);
    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalled();
    });
    const reportText = vi.mocked(navigator.clipboard.writeText).mock.calls.at(-1)?.[0] || "";
    const report = JSON.parse(String(reportText));
    expect(report.kind).toBe("yunque.pack_usability_report.v1");
    expect(report.source).toBe("pack-center");
    expect(report.summary.total).toBe(3);
    expect(report.summary.queue.total).toBe(2);
    expect(report.queue.map((item: { id: string }) => item.id)).toContain("yunque.pack.needs-entry");
    expect(report.queue[0].handoff_links).toEqual(expect.objectContaining({
      center: "/packs?q=yunque.pack.needs-entry&from=studio",
      detail: "/packs/detail?id=yunque.pack.needs-entry",
      open: null,
    }));
    expect(report.queue[0].handoff_links.studio).toContain("/packs/studio?");
    expect(report.queue[0].next_step).toContain("后端能力");
    expect(toastMock).toHaveBeenCalledWith("已复制能力包体检报告", "success");
    const queueStudioLink = screen.getAllByRole("link", { name: /交给小羽打磨/ })
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

    fireEvent.click(screen.getByRole("button", { name: /需补说明1/ }));

    expect(screen.getAllByText("Needs Context Pack").length).toBeGreaterThan(0);
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText("体检：需补说明")).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "展开详情" })[0]);
    expect(screen.getByText("可用性体检：还缺 用户感知位置。可以交给小羽打磨用途、入口或使用说明。")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除体检" }));

    expect(screen.getAllByText("Needs Context Pack").length).toBeGreaterThan(0);
    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();
  });

  it("promotes runtime route audit issues into the Xiaoyu polishing queue", async () => {
    packsClientMock.backendRouteAudit.mockResolvedValueOnce({
      generated_at: "2026-06-19T00:00:00Z",
      packs: 1,
      enabled_packs: 1,
      mounted_modules: 0,
      declared_routes: 1,
      mounted_routes: 0,
      ok_routes: 0,
      missing_routes: 1,
      method_mismatches: 0,
      undeclared_routes: 0,
      entries: [{
        pack_id: "yunque.pack.documents",
        pack_name: "Documents (文档生成)",
        pack_status: "enabled",
        enabled: true,
        status: "missing",
        declared: true,
        mounted: false,
        method: "GET",
        path: "/v1/documents/status",
      }],
    });
    navigationMock.query = "maintenance=1";

    render(<PacksPageOptimized />);

    expect((await screen.findAllByText("Documents (文档生成)")).length).toBeGreaterThan(0);
    await waitFor(() => {
      expect(screen.getByText("运行态 1 个问题")).toBeInTheDocument();
    });
    expect(screen.getByText("打磨与验收队列")).toBeInTheDocument();
    expect(screen.getByText("运行路由未挂载")).toBeInTheDocument();
    expect(screen.getByText("P0 先修审计阻塞：")).toBeInTheDocument();
    expect(screen.getByText("高风险 0 · 计划态 0 · 审计阻塞 1。")).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: /复制体检报告 JSON/ })[0]);
    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalled();
    });
    const reportText = vi.mocked(navigator.clipboard.writeText).mock.calls.at(-1)?.[0] || "";
    const report = JSON.parse(String(reportText));
    expect(report.summary.manifest_audit.blocked).toBe(1);
    expect(report.queue[0].manifest_audit.issues[0].key).toBe("runtime-route-missing");
  });

  it("paginates the readiness queue so Xiaoyu batch work stays scoped", async () => {
    packsClientMock.installed.mockResolvedValueOnce({
      packs: Array.from({ length: 8 }, (_, index) => makeNeedsEntryPack(index + 1)),
      count: 8,
    });

    const { rerender } = render(<PacksPageOptimized />);

    expect(await screen.findByText("Needs Entry Pack 1")).toBeInTheDocument();
    expect(screen.queryByText("打磨与验收队列")).not.toBeInTheDocument();

    navigationMock.query = "maintenance=1";
    rerender(<PacksPageOptimized />);
    expect(screen.getByText("打磨与验收队列")).toBeInTheDocument();
    const queue = screen.getByText("打磨与验收队列").closest("#readiness-queue");
    expect(queue).not.toBeNull();
    expect(screen.getByText("第 1 / 2 批 · 6 / 8 个。")).toBeInTheDocument();
    expect(screen.getByText("P0 6 · P1 0 · P2 0；缺入口 6 · 补说明 0")).toBeInTheDocument();
    expect(screen.getByText("0 个有入口；其余走 Chat、任务、记忆或知识。")).toBeInTheDocument();
    expect(screen.queryByText("0 个有入口可打开复验；其余从 Chat、任务、记忆或知识流程观察结果。")).not.toBeInTheDocument();
    expect(screen.getByText("打磨队列 · 第 1 / 2 页 · 共 8 个")).toBeInTheDocument();
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

    expect(screen.getByText("第 2 / 2 批 · 2 / 8 个。")).toBeInTheDocument();
    expect(screen.getByText("P0 2 · P1 0 · P2 0；缺入口 2 · 补说明 0")).toBeInTheDocument();
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

    const { rerender } = render(<PacksPageOptimized />);

    expect((await screen.findAllByText("Plan Only Pack")).length).toBeGreaterThan(0);
    expect(screen.queryByText("打磨与验收队列")).not.toBeInTheDocument();

    navigationMock.query = "maintenance=1";
    rerender(<PacksPageOptimized />);
    expect(screen.getByText("打磨与验收队列")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /优先打磨1/ })).toBeInTheDocument();
    expect(screen.getByText("交付状态：实验/计划。先保留限制说明；如果要变成主路径，下一轮补真实执行、结果查看和回滚证据。")).toBeInTheDocument();
    const planOnlyCard = cardFor("Plan Only Pack");
    expect(within(planOnlyCard).getByText("交付状态：实验/计划")).not.toBeVisible();
    expect(within(planOnlyCard).getByText("下一步：先保留限制说明；如果要变成主路径，下一轮补真实执行、结果查看和回滚证据。")).not.toBeVisible();
    fireEvent.click(within(planOnlyCard).getByRole("button", { name: "展开详情" }));
    expect(within(planOnlyCard).getByText("交付状态：实验/计划")).toBeVisible();
    expect(within(planOnlyCard).getByText("下一步：先保留限制说明；如果要变成主路径，下一轮补真实执行、结果查看和回滚证据。")).toBeVisible();
    expect(screen.getByText("P0 1 · P1 0 · P2 0；缺入口 0 · 补说明 0")).toBeInTheDocument();
    expect(screen.getByText("高风险 0 · 计划态 1 · 审计阻塞 1。")).toBeInTheDocument();

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

    expect(await screen.findByRole("link", { name: /Alpha Pack/ })).toBeInTheDocument();
    expect(screen.getByText("Documents (文档生成)")).toBeInTheDocument();

    openFilterDrawer();
    fireEvent.click(screen.getByRole("radio", { name: "实验" }));
    fireEvent.click(screen.getByRole("button", { name: "完成" }));

    expect(screen.getByRole("link", { name: /Alpha Pack/ })).toBeInTheDocument();
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText("类型：实验中")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除类型" }));

    openFilterDrawer();
    fireEvent.click(screen.getByRole("button", { name: "更多筛选" }));
    fireEvent.click(screen.getByRole("radio", { name: "开发中" }));
    fireEvent.click(screen.getByRole("button", { name: "完成" }));

    expect(screen.getByRole("link", { name: /Alpha Pack/ })).toBeInTheDocument();
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText("稳定性：开发中")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清除稳定性" }));

    expect(screen.getByRole("link", { name: /Alpha Pack/ })).toBeInTheDocument();
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
    expect(screen.getByText(/已安装 · 第 1 \/ 2 页 · 共 \d+ 个/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));

    expect(screen.getByText("Generated Pack 9")).toBeInTheDocument();
    expect(screen.queryByText("Documents (文档生成)")).not.toBeInTheDocument();
    expect(screen.getByText(/已安装 · 第 2 \/ 2 页 · 共 \d+ 个/)).toBeInTheDocument();
  });
});
