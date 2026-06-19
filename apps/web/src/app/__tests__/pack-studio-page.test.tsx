import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PackStudioPage from "../packs/studio/page";

const packsClientMock = vi.hoisted(() => ({
  installed: vi.fn(),
  catalog: vi.fn(),
}));

const toastMock = vi.hoisted(() => vi.fn());

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

const wasmManifest = {
  id: "yunque.pack.wasm-plugin",
  name: "WASM 能力包",
  version: "0.1.0",
  description: "加载和调试 WASM 能力。",
  status: "alpha",
  backend: {
    capabilities: ["wasm.load", "wasm.execute"],
    permissions: ["wasm:execute", "network:download", "filesystem:write"],
    routes: ["/v1/wasm-plugin/run"],
    routeSpecs: [{ method: "POST", path: "/v1/wasm-plugin/run", description: "Run WASM" }],
  },
  frontend: {
    menus: [{ key: "wasm", label: "WASM", path: "/packs/wasm-plugin" }],
    routes: [{ path: "/packs/wasm-plugin", component: "WASMPluginPackPage", title: "WASM" }],
    assets: { type: "builtin" },
  },
  metadata: {
    usability: "experimental",
    primaryActionLabel: "检查 WASM 能力",
    primaryActionPath: "/packs/wasm-plugin",
    limitation: "当前只做审计和 dry-run。",
  },
};

const documentsManifest = {
  id: "yunque.pack.documents",
  name: "Documents",
  version: "0.1.0",
  description: "生成文档。",
  status: "beta",
  backend: {
    capabilities: ["documents.generate"],
    permissions: ["documents:write"],
    routes: [],
    routeSpecs: [],
  },
  frontend: {
    menus: [],
    routes: [],
    assets: { type: "builtin" },
  },
  metadata: {
    usability: "infrastructure",
    primaryActionLabel: "生成文档",
    primaryActionPath: "/chat?q=generate-doc",
  },
};

describe("PackStudioPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
    packsClientMock.installed.mockResolvedValue({
      packs: [
        { manifest: wasmManifest, status: "enabled" },
        { manifest: documentsManifest, status: "disabled" },
      ],
      count: 2,
    });
    packsClientMock.catalog.mockResolvedValue({
      generated_at: "2026-06-19T00:00:00Z",
      sources: [],
      source_reports: [],
      count: 0,
      installed: 2,
      enabled: 1,
      downloadable: 0,
      capabilities: 0,
      entries: [],
    });
  });

  it("turns real pack metadata into a guarded Xiaoyu modification task", async () => {
    render(<PackStudioPage />);

    expect(await screen.findByText("Pack Studio")).toBeInTheDocument();
    expect(screen.getByText("WASM 能力包")).toBeInTheDocument();
    expect(screen.getAllByText("Documents").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByText("WASM 能力包"));

    expect(screen.getByText("可以让小羽优先优化")).toBeInTheDocument();
    expect(screen.getByText("需要守住的边界")).toBeInTheDocument();
    expect(screen.getByText("不要反编译后硬改 WASM；需要源码、ABI 说明和 wasm-plugin 回归测试。")).toBeInTheDocument();
    expect(screen.getByText("这个包仍是实验能力，改造时不要把它包装成稳定承诺。")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("这次想补强什么"), { target: { value: "增加一个可查看运行结果的界面" } });

    const task = screen.getByLabelText("小羽改包任务") as HTMLTextAreaElement;
    expect(task.value).toContain("用户目标：增加一个可查看运行结果的界面");
    expect(task.value).toContain("POST /v1/wasm-plugin/run");
    expect(task.value).toContain("不要直接扩大权限或绕过签名");
    expect(task.value).toContain("go test ./internal/packs/wasmplugin ./internal/controlplane/gateway -run WASM -count=1");

    fireEvent.click(screen.getByRole("button", { name: "复制任务" }));

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(expect.stringContaining("请以“小羽改包”的方式改造能力包"));
    await waitFor(() => {
      expect(toastMock).toHaveBeenCalledWith("已复制小羽改包任务", "success");
    });
  });
});
