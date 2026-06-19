import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PackDetailClientPage from "../packs/detail/client-page";

const packsClientMock = vi.hoisted(() => ({
  installed: vi.fn(),
  catalog: vi.fn(),
  releaseCatalog: vi.fn(),
  install: vi.fn(),
  enable: vi.fn(),
  disable: vi.fn(),
  rollback: vi.fn(),
}));

const routerPushMock = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams("id=yunque.pack.needs-context"),
  useRouter: () => ({ push: routerPushMock }),
}));

vi.mock("@/lib/sdk-client", () => ({
  createYunqueSDKClientOptions: () => ({
    baseUrl: "http://localhost",
    fetch: vi.fn(),
  }),
}));

vi.mock("yunque-client/packs", () => ({
  createPacksClient: () => packsClientMock,
}));

const manifest = {
  id: "yunque.pack.needs-context",
  name: "Needs Context Pack",
  version: "0.1.0",
  status: "beta",
  description: "用于演示能力包详情页如何承接用户下一步。",
  backend: {
    capabilities: ["needs.context.run"],
    permissions: ["read:context"],
    routeSpecs: [{ method: "POST", path: "/v1/needs-context/run", description: "运行演示能力" }],
  },
  frontend: {
    menus: [{ key: "needs-context", label: "Needs Context", path: "/packs/needs-context" }],
    routes: [{ path: "/packs/needs-context", component: "PackRuntimeRouteClientPage", title: "Needs Context" }],
  },
  metadata: {
    usageSurface: "/packs/needs-context",
    limitation: "当前仍是演示能力，不要包装成稳定承诺。",
  },
};

describe("PackDetailClientPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    packsClientMock.installed.mockResolvedValue({
      packs: [{ manifest, status: "enabled", installedAt: "2026-06-19T00:00:00Z", updatedAt: "2026-06-19T00:00:00Z" }],
      count: 1,
    });
    packsClientMock.catalog.mockResolvedValue({
      entries: [],
      count: 0,
      installed: 1,
      enabled: 1,
      downloadable: 0,
      capabilities: 1,
      generated_at: "2026-06-19T00:00:00Z",
    });
    packsClientMock.releaseCatalog.mockResolvedValue({
      generated_at: "2026-06-19T00:00:00Z",
      releases: [],
      count: 0,
      entries: [],
    });
    packsClientMock.install.mockResolvedValue({ ok: true });
  });

  it("links unclear packs directly to Chat guidance and Xiaoyu Studio improvement", async () => {
    render(<PackDetailClientPage />);

    expect(await screen.findByText("Needs Context Pack")).toBeInTheDocument();
    expect(screen.getByText("用户能拿它做什么")).toBeInTheDocument();
    expect(screen.getByText("能力包体检")).toBeInTheDocument();
    expect(screen.getByText(/还缺：使用示例/)).toBeInTheDocument();
    expect(screen.getByText("确认来源")).toBeInTheDocument();
    expect(screen.getByText(/来源：本机已安装记录/)).toBeInTheDocument();
    expect(screen.getByText("能力边界")).toBeInTheDocument();
    expect(screen.getAllByText(/不会自动泄露 API Key/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/此包未声明版本回滚/).length).toBeGreaterThan(0);

    const chatLink = screen.getByRole("link", { name: /问云雀怎么用/ });
    expect(chatLink).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    expect(decodeURIComponent(chatLink.getAttribute("href") || "")).toContain("Needs Context Pack");
    expect(decodeURIComponent(chatLink.getAttribute("href") || "")).toContain("不要把实验能力说成稳定能力");

    const studioLinks = screen.getAllByRole("link", { name: /交给小羽补齐/ });
    expect(studioLinks.length).toBeGreaterThan(0);
    expect(studioLinks[0]).toHaveAttribute("href", expect.stringContaining("/packs/studio?packId=yunque.pack.needs-context"));
    expect(studioLinks[0]).toHaveAttribute("href", expect.stringContaining("goal="));
  });

  it("loads release-only packs from official sources and installs the yqpack asset", async () => {
    packsClientMock.installed.mockResolvedValueOnce({
      packs: [],
      count: 0,
    });
    packsClientMock.catalog.mockResolvedValueOnce({
      entries: [],
      count: 0,
      installed: 0,
      enabled: 0,
      downloadable: 0,
      capabilities: 0,
      generated_at: "2026-06-19T00:00:00Z",
    });
    packsClientMock.releaseCatalog.mockResolvedValueOnce({
      generated_at: "2026-06-19T00:00:00Z",
      releases: ["https://example.com/releases/tag/pack%2Fneeds-context%2Fv0.1.0"],
      count: 1,
      entries: [{
        release_url: "https://example.com/releases/tag/pack%2Fneeds-context%2Fv0.1.0",
        release_tag: "pack/needs-context/v0.1.0",
        package_url: "https://example.com/needs-context.yqpack",
        package_name: "needs-context.yqpack",
        sha256: "abc123",
        size_bytes: 4096,
        installed: false,
        enabled: false,
        status: "disabled",
        update_action: "install",
        downloadable: true,
        manifest,
      }],
    });

    render(<PackDetailClientPage />);

    expect(await screen.findByText("Needs Context Pack")).toBeInTheDocument();
    expect(screen.getByText("来源与安装包")).toBeInTheDocument();
    expect(screen.getByText("官方发布源 · example.com")).toBeInTheDocument();
    expect(screen.getByText("https://example.com/needs-context.yqpack")).toBeInTheDocument();
    expect(screen.getByText("SHA256 abc123")).toBeInTheDocument();
    expect(screen.getByText("4 KB")).toBeInTheDocument();
    expect(screen.getByText(/来源：官方发布源 · example.com/)).toBeInTheDocument();
    expect(screen.getByText(/安装前可先在 Studio 只读检查包内容、SHA 与 manifest/)).toBeInTheDocument();

    const sourceStudioLink = screen.getByRole("link", { name: /先在 Studio 只读检查/ });
    expect(sourceStudioLink).toHaveAttribute("href", expect.stringContaining("/packs/studio?"));
    expect(sourceStudioLink).toHaveAttribute("href", expect.stringContaining("packId=yunque.pack.needs-context"));
    expect(sourceStudioLink).toHaveAttribute("href", expect.stringContaining("packageUrl=https%3A%2F%2Fexample.com%2Fneeds-context.yqpack"));
    expect(sourceStudioLink).toHaveAttribute("href", expect.stringContaining("sha256=abc123"));
    expect(screen.getByRole("link", { name: /回能力包中心/ })).toHaveAttribute("href", "/packs");

    const studioLinks = screen.getAllByRole("link", { name: /交给小羽补齐/ });
    expect(studioLinks[0]).toHaveAttribute("href", expect.stringContaining("packageUrl=https%3A%2F%2Fexample.com%2Fneeds-context.yqpack"));
    expect(studioLinks[0]).toHaveAttribute("href", expect.stringContaining("sha256=abc123"));

    fireEvent.click(screen.getByRole("button", { name: "安装" }));

    await waitFor(() => {
      expect(packsClientMock.install).toHaveBeenCalledWith({
        packageUrl: "https://example.com/needs-context.yqpack",
        sha256: "abc123",
        source: "https://example.com/releases/tag/pack%2Fneeds-context%2Fv0.1.0",
        download: true,
      });
    });
  });
});
