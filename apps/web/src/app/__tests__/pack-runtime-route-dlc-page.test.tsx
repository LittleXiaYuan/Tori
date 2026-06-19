import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PackRuntimeRouteClientPage from "../packs/[...slug]/client-page";

const packsClientMock = vi.hoisted(() => ({
  enabled: vi.fn(),
}));

const dlcHostMock = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  usePathname: () => "/packs/dlc-demo",
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
    packsClientMock.enabled.mockResolvedValue({ packs: [dlcDemoPack] });
  });

  it("explains iframe-bundle DLC boundaries before rendering the sandbox host", async () => {
    render(<PackRuntimeRouteClientPage />);

    expect(await screen.findByText("这个能力包能帮你做什么")).toBeInTheDocument();
    expect(screen.getByText("可直接使用")).toBeInTheDocument();
    expect(screen.getByText("需补说明")).toBeInTheDocument();
    expect(screen.getByText("还没有写清使用示例，可以交给小羽补齐。")).toBeInTheDocument();
    expect(screen.getByText(/还缺：使用示例、用户感知位置/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /问云雀怎么用/ })).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    expect(screen.getByRole("link", { name: /权限与详情/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.dlc-demo");
    expect(screen.getByRole("link", { name: /交给小羽补齐/ })).toHaveAttribute("href", expect.stringContaining("/packs/studio?packId=yunque.pack.dlc-demo"));
    expect(await screen.findByText("这个能力界面来自能力包本身")).toBeInTheDocument();
    expect(screen.getAllByText("独立界面包").length).toBeGreaterThan(0);
    expect(screen.getByText("iframe 沙箱")).toBeInTheDocument();
    expect(screen.getByText("按声明路由调用")).toBeInTheDocument();
    expect(screen.getByText(/随能力包一起下载的 DLC\/iframe 前端/)).toBeInTheDocument();
    expect(screen.getByText("沙箱边界")).toBeInTheDocument();
    expect(screen.getByText("iframe 没有宿主 token，不能读取云雀本地登录态。")).toBeInTheDocument();
    expect(screen.getByText("backend.call 只能访问该能力包 manifest 声明的后端路由。")).toBeInTheDocument();
    expect(screen.getByText("越权 bridge 调用会被拒绝并写入审计线索。")).toBeInTheDocument();
    expect(screen.getByText("这是 DLC/iframe/WASM 的参考包，用于验证热插拔界面和 bridge 权限，不是业务功能。")).toBeInTheDocument();
    expect(screen.getByTestId("pack-dlc-host")).toBeInTheDocument();

    await waitFor(() => expect(dlcHostMock).toHaveBeenCalled());
    expect(dlcHostMock.mock.calls[0]?.[0]).toMatchObject({
      packId: "yunque.pack.dlc-demo",
      entry: "index.html",
      allowedRoutes: [{ method: "POST", path: "/v1/dlc-demo/ping" }],
      allowedNavPaths: ["/packs/dlc-demo"],
      allowedEventPaths: ["/v1/events/stream"],
    });
  });
});
