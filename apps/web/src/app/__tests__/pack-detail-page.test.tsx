import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PackDetailClientPage from "../packs/detail/client-page";

const packsClientMock = vi.hoisted(() => ({
  installed: vi.fn(),
  catalog: vi.fn(),
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
  });

  it("links unclear packs directly to Chat guidance and Xiaoyu Studio improvement", async () => {
    render(<PackDetailClientPage />);

    expect(await screen.findByText("Needs Context Pack")).toBeInTheDocument();
    expect(screen.getByText("用户能拿它做什么")).toBeInTheDocument();
    expect(screen.getByText("能力包体检")).toBeInTheDocument();
    expect(screen.getByText(/还缺：使用示例/)).toBeInTheDocument();

    const chatLink = screen.getByRole("link", { name: /问云雀怎么用/ });
    expect(chatLink).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    expect(decodeURIComponent(chatLink.getAttribute("href") || "")).toContain("Needs Context Pack");
    expect(decodeURIComponent(chatLink.getAttribute("href") || "")).toContain("不要把实验能力说成稳定能力");

    const studioLinks = screen.getAllByRole("link", { name: /交给小羽补齐/ });
    expect(studioLinks.length).toBeGreaterThan(0);
    expect(studioLinks[0]).toHaveAttribute("href", expect.stringContaining("/packs/studio?packId=yunque.pack.needs-context"));
    expect(studioLinks[0]).toHaveAttribute("href", expect.stringContaining("goal="));
  });
});
