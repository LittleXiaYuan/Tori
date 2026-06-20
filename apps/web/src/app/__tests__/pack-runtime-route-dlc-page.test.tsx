import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PackRuntimeRouteClientPage from "../packs/[...slug]/client-page";

const packsClientMock = vi.hoisted(() => ({
  enabled: vi.fn(),
}));

const dlcHostMock = vi.hoisted(() => vi.fn());
const pathnameMock = vi.hoisted(() => ({
  value: "/packs/dlc-demo",
}));

vi.mock("next/navigation", () => ({
  usePathname: () => pathnameMock.value,
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

vi.mock("@/lib/pack-dlc-host", () => ({
  PackDlcHost: (props: unknown) => {
    dlcHostMock(props);
    return <div data-testid="pack-dlc-host">DLC host</div>;
  },
}));

const dlcDemoPack = {
  id: "yunque.pack.dlc-demo",
  status: "enabled",
  manifest: {
    id: "yunque.pack.dlc-demo",
    name: "DLC Demo Pack",
    version: "0.1.0",
    description: "前端 DLC（iframe 沙箱 + postMessage 桥）的参考增量包。",
    backend: {
      capabilities: ["dlc.demo.ping"],
      routes: ["/v1/dlc-demo/ping"],
      permissions: ["dlc:demo", "events:subscribe:/v1/events/stream"],
      routeSpecs: [{
        method: "POST",
        path: "/v1/dlc-demo/ping",
        description: "Echo a ping; demonstrates the bridge backend.call path.",
      }],
    },
    frontend: {
      menus: [{ key: "dlc-demo", label: "DLC 演示", path: "/packs/dlc-demo" }],
      routes: [{ path: "/packs/dlc-demo", component: "PackDlcHost", title: "DLC 演示" }],
      assets: { type: "iframe-bundle", entry: "index.html" },
    },
    metadata: {
      limitation: "这是 DLC/iframe/WASM 的参考包，用于验证热插拔界面和 bridge 权限，不是业务功能。",
    },
  },
};

describe("PackRuntimeRouteClientPage DLC route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    pathnameMock.value = "/packs/dlc-demo";
    packsClientMock.enabled.mockResolvedValue({ packs: [dlcDemoPack] });
  });

  it("explains iframe-bundle DLC boundaries before rendering the sandbox host", async () => {
    render(<PackRuntimeRouteClientPage />);

    expect(await screen.findByRole("link", { name: /回中心定位/ })).toHaveAttribute("href", "/packs?q=yunque.pack.dlc-demo");
    expect(await screen.findByText("这个能力包能帮你做什么")).toBeInTheDocument();
    expect(screen.getByText("这个能力包提供独立界面，已在沙箱中动态加载。")).toBeInTheDocument();
    expect(screen.getByText("可直接使用")).toBeInTheDocument();
    expect(screen.getByText("需补说明")).toBeInTheDocument();
    expect(screen.getByText("还没有写清使用示例，可以交给小羽打磨。")).toBeInTheDocument();
    expect(screen.getByText(/还缺：使用示例、用户感知位置/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /问云雀怎么用/ })).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    expect(screen.getByRole("link", { name: /权限与详情/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.dlc-demo");
    const studioLink = screen.getByRole("link", { name: /交给小羽打磨/ });
    expect(studioLink).toHaveAttribute("href", expect.stringContaining("/packs/studio?packId=yunque.pack.dlc-demo"));
    expect(decodeURIComponent(studioLink.getAttribute("href") || "")).toContain("优先补齐 使用示例、用户感知位置");
    expect(screen.getByText("从当前入口继续改包")).toBeInTheDocument();
    expect(screen.getByText(/你现在打开的是/)).toBeInTheDocument();
    expect(screen.getAllByText("/packs/dlc-demo").length).toBeGreaterThan(0);
    expect(screen.getByText("先触发一次")).toBeInTheDocument();
    expect(screen.getByText("看结果在哪")).toBeInTheDocument();
    expect(screen.getByText("决定留下还是改")).toBeInTheDocument();
    expect(screen.getByText("验收出口：")).toBeInTheDocument();
    expect(screen.getByText(/回中心确认状态，进详情复查权限，再打开入口复验。/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /看权限与来源/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.dlc-demo");
    const handoffLink = screen.getByRole("link", { name: /让小羽改这个包/ });
    expect(handoffLink).toHaveAttribute("href", expect.stringContaining("/packs/studio?packId=yunque.pack.dlc-demo"));
    expect(screen.getByRole("link", { name: /回中心筛选/ })).toHaveAttribute("href", "/packs?q=yunque.pack.dlc-demo");
    expect(await screen.findByText("这个能力界面来自能力包本身")).toBeInTheDocument();
    expect(screen.getAllByText("独立界面包").length).toBeGreaterThan(0);
    expect(screen.getByText("沙箱隔离")).toBeInTheDocument();
    expect(screen.getByText("按声明路由调用")).toBeInTheDocument();
    expect(screen.getByText(/随能力包一起下载的独立界面/)).toBeInTheDocument();
    expect(screen.getByText("沙箱边界")).toBeInTheDocument();
    expect(screen.getByText("独立界面拿不到云雀本地登录态或宿主 token。")).toBeInTheDocument();
    expect(screen.getByText("它只能调用自己声明过的后端路由。")).toBeInTheDocument();
    expect(screen.getByText("越权调用会被拒绝并留下审计线索。")).toBeInTheDocument();
    expect(screen.getByText("这是 DLC/iframe/WASM 的参考包，用于验证热插拔界面和 bridge 权限，不是业务功能。")).toBeInTheDocument();
    expect(screen.getByTestId("pack-dlc-host")).toBeInTheDocument();
    expect(screen.getByText("入口同步详情")).toBeInTheDocument();
    expect(screen.getByText("界面资源与安装包")).toBeInTheDocument();
    expect(screen.getByText("开发者 SDK 能力")).toBeInTheDocument();
    expect(screen.getByText(/技术说明：这个页面只读取已启用能力包返回的 manifest/)).toBeInTheDocument();
    expect(screen.queryByText("Pack 路由同步")).not.toBeInTheDocument();
    expect(screen.queryByText("Pack 路由未启用")).not.toBeInTheDocument();

    await waitFor(() => expect(dlcHostMock).toHaveBeenCalled());
    expect(dlcHostMock.mock.calls[0]?.[0]).toMatchObject({
      packId: "yunque.pack.dlc-demo",
      entry: "index.html",
      allowedRoutes: [{ method: "POST", path: "/v1/dlc-demo/ping" }],
      allowedNavPaths: ["/packs/dlc-demo"],
      allowedEventPaths: ["/v1/events/stream"],
    });
  });

  it("guides users back to the pack center when a dynamic route is not enabled", async () => {
    pathnameMock.value = "/packs/not-enabled";
    packsClientMock.enabled.mockResolvedValueOnce({ packs: [dlcDemoPack] });

    render(<PackRuntimeRouteClientPage />);

    expect(await screen.findByText("能力包入口未启用")).toBeInTheDocument();
    expect(screen.getByText("这个入口还没有在已启用能力包中声明，请先安装并启用对应能力。")).toBeInTheDocument();
    expect(screen.getByText("未找到可打开的能力包页面")).toBeInTheDocument();
    expect(screen.getByText(/需要先安装并启用对应能力包/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /返回能力包中心/ })).toHaveAttribute("href", "/packs");
    expect(screen.queryByText("Pack 路由未启用")).not.toBeInTheDocument();
    expect(dlcHostMock).not.toHaveBeenCalled();
  });
});
