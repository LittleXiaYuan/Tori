import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PackStudioPage from "../packs/studio/page";

const packsClientMock = vi.hoisted(() => ({
  installed: vi.fn(),
  catalog: vi.fn(),
  releaseCatalog: vi.fn(),
  install: vi.fn(),
  enable: vi.fn(),
  disable: vi.fn(),
  rollback: vi.fn(),
  studioPlan: vi.fn(),
  studioInspect: vi.fn(),
  studioWorkspace: vi.fn(),
  studioPatch: vi.fn(),
  studioAudit: vi.fn(),
  studioRepack: vi.fn(),
}));

const toastMock = vi.hoisted(() => vi.fn());
const navigationMock = vi.hoisted(() => ({
  query: "",
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(navigationMock.query),
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
    navigationMock.query = "";
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
    packsClientMock.releaseCatalog.mockResolvedValue({
      generated_at: "2026-06-19T00:00:00Z",
      releases: [],
      count: 0,
      entries: [],
    });
    packsClientMock.studioPlan.mockImplementation(({ packId, goal }: { packId: string; goal: string }) => Promise.resolve({
      generated_at: "2026-06-19T00:00:00Z",
      pack_id: packId,
      pack_name: packId === "yunque.pack.wasm-plugin" ? "WASM 能力包" : "Documents",
      version: "0.1.0",
      source: "test",
      status: packId === "yunque.pack.wasm-plugin" ? "enabled" : "disabled",
      installed: true,
      enabled: packId === "yunque.pack.wasm-plugin",
      goal,
      risk_level: packId === "yunque.pack.wasm-plugin" ? "high" : "medium",
      summary: "后端只读改包计划",
      capabilities: packId === "yunque.pack.wasm-plugin" ? ["wasm.load", "wasm.execute"] : ["documents.generate"],
      permissions: packId === "yunque.pack.wasm-plugin" ? ["wasm:execute", "network:download", "filesystem:write"] : ["documents:write"],
      frontend_paths: packId === "yunque.pack.wasm-plugin" ? ["/packs/wasm-plugin"] : [],
      backend_routes: packId === "yunque.pack.wasm-plugin" ? ["POST /v1/wasm-plugin/run"] : [],
      surfaces: packId === "yunque.pack.wasm-plugin" ? ["frontend", "backend", "wasm"] : ["manifest"],
      editable: ["用途说明、起手示例、入口文案、可用度分层和权限解释可以从 manifest/前端展示层优化。"],
      guarded: [
        "不直接修改已签名或已安装包；先生成差异方案，用户确认后再打包为新 yqpack。",
        "不要反编译后硬改 WASM；需要源码、ABI 说明和 wasm-plugin 回归测试。",
      ],
      warnings: ["这个包仍是实验能力，改造时不要把它包装成稳定承诺。"],
      editable_files: [
        "packs/official/wasm-plugin-pack/pack.json",
        "apps/web/src/app/packs/wasm-plugin/page.tsx",
        "internal/packs/wasmplugin/",
      ],
      diff_preview: `diff --git a/packs/official/wasm-plugin-pack/pack.json b/packs/official/wasm-plugin-pack/pack.json\n+ "description": "${goal}"`,
      audit_steps: [
        "node scripts\\check-pack-usability.mjs --strict",
        "go test ./internal/packs/wasmplugin ./internal/controlplane/gateway -run WASM -count=1",
      ],
      package_steps: [
        "go run ./cmd/yunque-plugin pack packs\\official\\wasm-plugin-pack --out dist\\packs\\wasm-plugin-0.1.0.yqpack",
      ],
      rollback_steps: ["新包作为 fork/local 版本安装；验证失败时禁用新版本并回滚上一版本。"],
      cogni_use: ["WASM 包只能使用 host 允许的 ABI。"],
      xiaoyu_prompt: [
        "请以“小羽改包”的方式改造能力包 WASM 能力包",
        `用户目标：${goal}`,
        "POST /v1/wasm-plugin/run",
        "可改文件候选：",
        "差异预览草案：",
        "不要直接扩大权限或绕过签名",
        "重新打包与回滚：",
        "go test ./internal/packs/wasmplugin ./internal/controlplane/gateway -run WASM -count=1",
      ].join("\n"),
    }));
    packsClientMock.studioInspect.mockImplementation(({ packagePath }: { packagePath?: string }) => Promise.resolve({
      generated_at: "2026-06-19T00:00:00Z",
      source: packagePath || "C:\\packs\\wasm-plugin.yqpack",
      sha256: packagePath ? "d".repeat(64) : "a".repeat(64),
      expected_sha256: packagePath ? "d".repeat(64) : "a".repeat(64),
      sha256_match: true,
      size_bytes: 4096,
      manifest: wasmManifest,
      entries: [
        { path: "pack.json", kind: "manifest", size_bytes: 512, editable: true, reason: "能力包 manifest，可改用途、入口、权限说明和发行元数据。" },
        { path: "frontend/index.html", kind: "frontend", size_bytes: 1024, editable: true, reason: "iframe/DLC 前端资源，可在沙箱边界内优化界面。" },
        { path: "backend/plugin.wasm", kind: "wasm", size_bytes: 2048, editable: false, needs_source: true, reason: "WASM 二进制不能硬改；需要源码、ABI 说明和 wasm 回归测试。" },
      ],
      entry_count: 3,
      editable_count: 2,
      guarded_count: 1,
      warnings: [],
      plan: {
        generated_at: "2026-06-19T00:00:00Z",
        pack_id: wasmManifest.id,
        pack_name: wasmManifest.name,
        version: wasmManifest.version,
        installed: false,
        enabled: false,
        goal: "增加一个可查看运行结果的界面",
        risk_level: "high",
        summary: "只读检查",
        surfaces: ["frontend", "wasm"],
        editable: [],
        guarded: [],
        editable_files: [],
        diff_preview: "",
        audit_steps: [],
        package_steps: [],
        rollback_steps: [],
        cogni_use: [],
        xiaoyu_prompt: "",
      },
    }));
    packsClientMock.studioWorkspace.mockResolvedValue({
      generated_at: "2026-06-19T00:00:00Z",
      workspace_path: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
      workspace_id: "yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
      package_source: "C:\\packs\\wasm-plugin.yqpack",
      original_sha256: "a".repeat(64),
      expected_sha256: "a".repeat(64),
      sha256_match: true,
      manifest: wasmManifest,
      inspect: {} as never,
      editable_files: ["C:\\yunque\\packs\\studio\\pack.json", "C:\\yunque\\packs\\studio\\frontend\\index.html"],
      guarded_files: ["C:\\yunque\\packs\\studio\\backend\\plugin.wasm"],
      audit_commands: ["node scripts\\check-pack-usability.mjs --strict"],
      repack_commands: [
        "go run ./cmd/yunque-plugin pack C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa --out dist\\packs\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack",
      ],
      rollback_commands: ["新包安装后若验证失败，执行 /v1/packs/disable 禁用新包。"],
      next_steps: ["让小羽只修改 editable_files 中的文件，先给差异预览。"],
      warnings: [],
    });
    packsClientMock.studioPatch.mockImplementation(({ apply }: { apply?: boolean }) => Promise.resolve({
      generated_at: "2026-06-19T00:00:00Z",
      workspace_path: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
      file_path: "C:\\yunque\\packs\\studio\\pack.json",
      relative_path: "pack.json",
      applied: Boolean(apply),
      reason: "增加一个可查看运行结果的界面",
      old_sha256: "b".repeat(64),
      new_sha256: "c".repeat(64),
      diff_preview: "diff --git a/pack.json b/pack.json\n+  \"description\": \"更清楚\"",
      warnings: [],
      next_steps: ["运行 audit_commands"],
    }));
    packsClientMock.studioAudit.mockResolvedValue({
      generated_at: "2026-06-19T00:00:00Z",
      workspace_path: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
      workspace_id: "yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
      original_sha256: "a".repeat(64),
      current_sha256: "e".repeat(64),
      manifest: wasmManifest,
      change_count: 1,
      editable_change_count: 1,
      guarded_change_count: 0,
      allowed: true,
      risk_level: "medium",
      changes: [
        { path: "pack.json", kind: "manifest", status: "modified", editable: true },
      ],
      warnings: [],
      next_steps: ["若 guarded_change_count 为 0，可以继续重新打包并复检新 yqpack。"],
    });
    packsClientMock.studioRepack.mockResolvedValue({
      generated_at: "2026-06-19T00:00:00Z",
      workspace_path: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
      package_path: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack",
      sha256: "d".repeat(64),
      size_bytes: 4096,
      manifest: wasmManifest,
      inspect: { sha256_match: true } as never,
      warnings: [],
      next_steps: ["重新运行只读检查，确认新 yqpack 的 manifest、sha256 和文件分类。"],
    });
    packsClientMock.install.mockResolvedValue({
      pack: { manifest: wasmManifest, status: "installed" },
      status: "installed",
    });
    packsClientMock.enable.mockResolvedValue({
      pack: { manifest: wasmManifest, status: "enabled" },
      status: "enabled",
    });
    packsClientMock.disable.mockResolvedValue({
      pack: { manifest: wasmManifest, status: "disabled" },
      status: "disabled",
    });
    packsClientMock.rollback.mockResolvedValue({
      pack: { manifest: wasmManifest, status: "installed" },
      status: "installed",
    });
  });

  it("opens the selected pack and goal from store links", async () => {
    navigationMock.query = new URLSearchParams({
      packId: "yunque.pack.wasm-plugin",
      goal: "补一个结果面板",
      packageUrl: "https://oss.example.com/wasm-plugin.yqpack",
      sha256: "9".repeat(64),
    }).toString();

    render(<PackStudioPage />);

    expect((await screen.findAllByText("WASM 能力包")).length).toBeGreaterThan(0);
    await waitFor(() => {
      expect(packsClientMock.studioPlan).toHaveBeenCalledWith({
        packId: "yunque.pack.wasm-plugin",
        goal: "补一个结果面板",
      });
    });
    expect((screen.getByLabelText("这次想补强什么") as HTMLInputElement).value).toBe("补一个结果面板");
    expect((screen.getByLabelText("OSS / Release URL") as HTMLInputElement).value).toBe("https://oss.example.com/wasm-plugin.yqpack");
    expect((screen.getByLabelText("SHA256") as HTMLInputElement).value).toBe("9".repeat(64));
    expect(screen.getByText("当前能力包：WASM 能力包")).toBeInTheDocument();
    expect(screen.getByText("已带 yqpack 来源")).toBeInTheDocument();
    expect(screen.getByText(/候选来源：已启用；本机状态：已启用；尚未对当前 yqpack 做只读检查/)).toBeInTheDocument();
    expect(screen.getByText("先在这里做只读检查、工作区、差异预览、审计和重新打包；完成后回详情确认权限，或回能力包中心刷新入口与状态。")).toBeInTheDocument();
    expect(screen.getByText("当前阶段：只读检查 · 下一步：填写路径/URL 后点击只读检查。小羽只生成计划和草稿，不会自动应用改动。")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /查看详情/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.wasm-plugin");
    expect(screen.getByRole("link", { name: /打开能力入口/ })).toHaveAttribute("href", "/packs/wasm-plugin");
    expect(screen.getByText("改包流程导轨")).toBeInTheDocument();
    expect(screen.getByText("从只读检查到启用/回滚都在同一条路径里推进；小羽只生成计划和草稿，真正写入、打包、安装都需要你确认。")).toBeInTheDocument();
    expect(screen.getByText("当前包：WASM 能力包")).toBeInTheDocument();
    expect(screen.getByText("0/8 已完成")).toBeInTheDocument();
    expect(screen.getByText("最终验收出口：")).toBeInTheDocument();
    expect(screen.getByText(/回中心确认状态，进详情复查权限，再打开入口复验。/)).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /能力包中心/ }).some((link) => link.getAttribute("href") === "/packs?q=yunque.pack.wasm-plugin&from=studio")).toBe(true);
    expect(screen.getAllByRole("link", { name: /权限与详情/ }).some((link) => link.getAttribute("href") === "/packs/detail?id=yunque.pack.wasm-plugin")).toBe(true);
    expect(screen.getAllByRole("link", { name: /^打开入口/ }).some((link) => link.getAttribute("href") === "/packs/wasm-plugin")).toBe(true);
    expect(screen.getByText("权限：沙箱、联网、写入；需要授权后使用")).toBeInTheDocument();
    expect(screen.getByText("权限：写入；启用前建议确认")).toBeInTheDocument();
    expect(screen.getAllByText("下一步：填写路径/URL 后点击只读检查").length).toBeGreaterThan(0);
    expect(screen.getByRole("link", { name: /跳到当前操作/ })).toHaveAttribute("href", "#yqpack-check");
    expect(screen.getByText("已从能力包中心接入这个 yqpack")).toBeInTheDocument();
    expect(screen.getByText("不用回到商店手动找包；先在这里做只读检查，再进入工作区、差异预览、审计和重新打包。这一步只校验 SHA、能力声明与文件分类，不会安装、启用或改动本地能力包。")).toBeInTheDocument();
    expect(screen.getByText("URL: https://oss.example.com/wasm-plugin.yqpack")).toBeInTheDocument();
    expect(screen.getByText(`SHA256: ${"9".repeat(64)}`)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "立即只读检查" }));

    await waitFor(() => {
      expect(packsClientMock.studioInspect).toHaveBeenCalledWith({
        packagePath: undefined,
        packageUrl: "https://oss.example.com/wasm-plugin.yqpack",
        sha256: "9".repeat(64),
        goal: "补一个结果面板",
      });
    });
  });

  it("selects a release-only pack from official sources", async () => {
    packsClientMock.installed.mockResolvedValueOnce({
      packs: [{ manifest: documentsManifest, status: "disabled" }],
      count: 1,
    });
    packsClientMock.catalog.mockResolvedValueOnce({
      generated_at: "2026-06-19T00:00:00Z",
      sources: [],
      source_reports: [],
      count: 0,
      installed: 1,
      enabled: 0,
      downloadable: 0,
      capabilities: 0,
      entries: [],
    });
    packsClientMock.releaseCatalog.mockResolvedValueOnce({
      generated_at: "2026-06-19T00:00:00Z",
      releases: ["https://example.com/releases/tag/pack%2Fwasm-plugin%2Fv0.1.0"],
      count: 1,
      entries: [{
        release_url: "https://example.com/releases/tag/pack%2Fwasm-plugin%2Fv0.1.0",
        release_tag: "pack/wasm-plugin/v0.1.0",
        package_url: "https://oss.example.com/wasm-plugin.yqpack",
        package_name: "wasm-plugin.yqpack",
        sha256: "9".repeat(64),
        size_bytes: 4096,
        installed: false,
        enabled: false,
        status: "disabled",
        update_action: "install",
        downloadable: true,
        manifest: wasmManifest,
      }],
    });
    navigationMock.query = new URLSearchParams({
      packId: "yunque.pack.wasm-plugin",
      goal: "补一个结果面板",
    }).toString();

    render(<PackStudioPage />);

    expect((await screen.findAllByText("WASM 能力包")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("官方源").length).toBeGreaterThan(0);
    await waitFor(() => {
      expect((screen.getByLabelText("OSS / Release URL") as HTMLInputElement).value).toBe("https://oss.example.com/wasm-plugin.yqpack");
      expect((screen.getByLabelText("SHA256") as HTMLInputElement).value).toBe("9".repeat(64));
    });
    expect(screen.getByText("已从能力包中心接入这个 yqpack")).toBeInTheDocument();
    expect(screen.getByText("当前能力包：WASM 能力包")).toBeInTheDocument();
    expect(screen.getByText("已带 yqpack 来源")).toBeInTheDocument();
    expect(screen.getByText(/候选来源：官方源；本机状态：未安装；尚未对当前 yqpack 做只读检查/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /查看详情/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.wasm-plugin");
    await waitFor(() => {
      expect(packsClientMock.studioPlan).toHaveBeenCalledWith({
        packId: "yunque.pack.wasm-plugin",
        goal: "补一个结果面板",
      });
    });
    const task = screen.getByLabelText("小羽改包任务") as HTMLTextAreaElement;
    expect(task.value).toContain("请以“小羽改包”的方式改造能力包 WASM 能力包");

    fireEvent.click(screen.getByRole("button", { name: /Documents/ }));

    expect((screen.getByLabelText("OSS / Release URL") as HTMLInputElement).value).toBe("");
    expect((screen.getByLabelText("SHA256") as HTMLInputElement).value).toBe("");
    expect(screen.queryByText("已从能力包中心接入这个 yqpack")).not.toBeInTheDocument();
  });

  it("syncs Studio context to the manifest found by yqpack inspection", async () => {
    navigationMock.query = new URLSearchParams({
      packId: "yunque.pack.documents",
      goal: "检查一个外部 yqpack",
      packageUrl: "https://oss.example.com/wasm-plugin.yqpack",
      sha256: "9".repeat(64),
    }).toString();

    render(<PackStudioPage />);

    expect(await screen.findByText("当前能力包：Documents")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /查看详情/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.documents");

    fireEvent.click(screen.getByRole("button", { name: "立即只读检查" }));

    await waitFor(() => {
      expect(packsClientMock.studioInspect).toHaveBeenCalledWith({
        packagePath: undefined,
        packageUrl: "https://oss.example.com/wasm-plugin.yqpack",
        sha256: "9".repeat(64),
        goal: "检查一个外部 yqpack",
      });
    });
    expect(screen.getByText("当前能力包：WASM 能力包")).toBeInTheDocument();
    expect(screen.getByText("已同步检查结果")).toBeInTheDocument();
    expect(screen.getByText(/候选来源：已启用；本机状态：已启用；只读检查已匹配当前能力包，SHA 匹配，3 个文件/)).toBeInTheDocument();
    expect(screen.getByText(/只读检查已对齐候选/)).toBeInTheDocument();
    expect(screen.getByText(/检查结果已同步到 WASM 能力包，来源是 已启用/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /跳到工作区准备/ })).toHaveAttribute("href", "#yqpack-check");
    expect(screen.getByRole("link", { name: /查看详情/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.wasm-plugin");
    expect(screen.getByRole("link", { name: /打开能力入口/ })).toHaveAttribute("href", "/packs/wasm-plugin");
  });

  it("prefills yqpack source for private catalog candidates", async () => {
    const privateManifest = {
      ...documentsManifest,
      id: "yunque.pack.private-docs",
      name: "Private Docs",
      description: "来自私有源的文档能力包。",
    };
    packsClientMock.installed.mockResolvedValueOnce({
      packs: [{ manifest: wasmManifest, status: "enabled" }],
      count: 1,
    });
    packsClientMock.catalog.mockResolvedValueOnce({
      generated_at: "2026-06-19T00:00:00Z",
      sources: ["https://oss.example.com/catalog.json"],
      source_reports: [],
      count: 1,
      installed: 1,
      enabled: 1,
      downloadable: 1,
      capabilities: 1,
      entries: [{
        source: "https://oss.example.com/catalog.json",
        package_url: "https://oss.example.com/private-docs.yqpack",
        sha256: "8".repeat(64),
        installed: false,
        enabled: false,
        manifest: privateManifest,
      }],
    });
    navigationMock.query = new URLSearchParams({
      packId: "yunque.pack.private-docs",
      goal: "补齐私有源入口",
    }).toString();

    render(<PackStudioPage />);

    expect((await screen.findAllByText("Private Docs")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("私有源").length).toBeGreaterThan(0);
    await waitFor(() => {
      expect((screen.getByLabelText("OSS / Release URL") as HTMLInputElement).value).toBe("https://oss.example.com/private-docs.yqpack");
      expect((screen.getByLabelText("SHA256") as HTMLInputElement).value).toBe("8".repeat(64));
    });
    expect(screen.getByText("已从能力包中心接入这个 yqpack")).toBeInTheDocument();
  });

  it("imports a batch readiness request and lets users continue pack by pack", async () => {
    const batchRequest = {
      kind: "yunque.pack_studio.batch_draft_request.v1",
      goal: "批量把这些能力包补齐用途和入口。",
      batch: { page: 2, page_count: 5, total: 26, page_size: 6 },
      rules: ["不要自动应用改动。", "回到能力包工坊预览差异 / 审计 / 重新打包。"],
      packs: [
        {
          id: "yunque.pack.wasm-plugin",
          name: "WASM 能力包",
          version: "0.1.0",
          status: "alpha",
          source: "官方源",
          priority: {
            level: "P0",
            label: "P0 先补授权边界",
            reason: "涉及高风险授权，但交付路径还不够清楚。",
          },
          missing: ["使用示例", "用户感知位置"],
          readiness: "需补说明",
          risk: {
            level: "high",
            label: "需要授权",
            requires_authorization: true,
          },
          permission_summary: "权限：沙箱、联网、写入；需要授权后使用",
          delivery: {
            level: "plan_only",
            label: "实验/计划",
            description: "可以体验、验证边界或生成计划，但不应包装成稳定可交付能力。",
            next_step: "先保留限制说明。",
          },
          polish_guidance: {
            reason: "体检缺口：使用示例、用户感知位置。",
            first_edit: "先补 metadata.example1-3，用真实用户动作描述它能产出什么结果。",
            verify: "改完回到 /packs/wasm-plugin 验证入口、提示、结果位置和回滚路径是否可见。",
            handoff: "只读检查 -> 准备工作区 -> 预览差异 -> 审计 -> 重新打包 -> 复检 SHA -> 安装/启用/回滚。",
          },
          handoff_links: {
            center: "/packs?q=yunque.pack.wasm-plugin&from=studio",
            detail: "/packs/detail?id=yunque.pack.wasm-plugin",
            open: "/packs/wasm-plugin",
            studio: "/packs/studio?packId=yunque.pack.wasm-plugin&goal=%E8%A1%A5%E9%BD%90%20WASM%20%E7%94%A8%E9%80%94",
          },
          studio_url: "/packs/studio?packId=yunque.pack.wasm-plugin&goal=%E8%A1%A5%E9%BD%90%20WASM%20%E7%94%A8%E9%80%94",
          package_url: "https://oss.example.com/wasm-plugin.yqpack",
          sha256: "9".repeat(64),
        },
        {
          id: "yunque.pack.documents",
          name: "Documents",
          version: "0.1.0",
          status: "beta",
          source: "已安装",
          priority: {
            level: "P1",
            label: "P1 补用户理解",
            reason: "用户还缺少场景、示例或结果位置来判断价值。",
          },
          missing: ["用户感知位置"],
          readiness: "需补说明",
          handoff_links: {
            center: "/packs?q=yunque.pack.documents&from=studio",
            detail: "/packs/detail?id=yunque.pack.documents",
            open: "/chat?q=generate-doc",
            studio: "/packs/studio?packId=yunque.pack.documents",
          },
          studio_url: "/packs/studio?packId=yunque.pack.documents",
        },
      ],
    };
    navigationMock.query = new URLSearchParams({
      batch: `小羽收到批量补肉任务。\n\n\`\`\`json\n${JSON.stringify(batchRequest, null, 2)}\n\`\`\``,
    }).toString();

    render(<PackStudioPage />);

    expect(await screen.findByText("导入批量补肉任务")).toBeInTheDocument();
    expect((screen.getByLabelText("导入批量补肉任务 JSON") as HTMLTextAreaElement).value).toContain("yunque.pack_studio.batch_draft_request.v1");
    expect(screen.getByText("2 个包")).toBeInTheDocument();
    expect(screen.getByText("第 2 / 5 批")).toBeInTheDocument();
    expect(screen.getByText("逐包处理")).toBeInTheDocument();
    expect(screen.getByText("目标：批量把这些能力包补齐用途和入口。")).toBeInTheDocument();
    expect(screen.getByText("规则：不要自动应用改动。；回到能力包工坊预览差异 / 审计 / 重新打包。")).toBeInTheDocument();
    expect(screen.getByText("来自补肉队列第 2 / 5 批：本批 2 个，队列总计 26 个，每批最多 6 个。")).toBeInTheDocument();
    expect(screen.getByText("高风险/需授权：1")).toBeInTheDocument();
    expect(screen.getByText("P0：1")).toBeInTheDocument();
    expect(screen.getByText("P1：1")).toBeInTheDocument();
    expect(screen.getByText("P2：0")).toBeInTheDocument();
    expect(screen.getByText("实验/计划：1")).toBeInTheDocument();
    expect(screen.getByText("待补肉：2")).toBeInTheDocument();
    expect(screen.getByText("缺说明/示例：2")).toBeInTheDocument();
    expect(screen.getByText("缺入口/后端：0")).toBeInTheDocument();
    expect(screen.getByText("批量处理进度")).toBeInTheDocument();
    expect(screen.getByText("逐包载入、逐包检查、逐包重打包；能力包工坊不会把批量任务自动应用到多个能力包。")).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getAllByText((_, element) => /批量进度：\s*1\s*\/\s*2/.test(element?.textContent || "")).length).toBeGreaterThan(0);
    });
    expect(screen.getByText("已推进：1 / 2")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /上一包/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: /下一包/ })).toBeEnabled();
    expect(screen.getByRole("button", { name: /跳到下一个 P0/ })).toBeDisabled();
    expect(screen.getByText("当前处理：WASM 能力包")).toBeInTheDocument();
    expect(screen.getAllByText("P0 先补授权边界").length).toBeGreaterThan(0);
    expect(screen.getByText("优先级：P0 先补授权边界。涉及高风险授权，但交付路径还不够清楚。")).toBeInTheDocument();
    expect(screen.getByText("优先级：P1 补用户理解。用户还缺少场景、示例或结果位置来判断价值。")).toBeInTheDocument();
    expect(screen.getAllByText((_, element) => /本页状态：\s*本页已载入/.test(element?.textContent || "")).length).toBeGreaterThan(0);
    expect(screen.getByRole("link", { name: /跳到下一步/ })).toHaveAttribute("href", "#yqpack-check");
    expect(screen.getByText("补：使用示例")).toBeInTheDocument();
    expect(screen.getAllByText("补：用户感知位置")).toHaveLength(2);
    expect(screen.getAllByText("实验/计划").length).toBeGreaterThan(0);
    expect(screen.getByText("风险：需要授权")).toBeInTheDocument();
    expect(screen.getAllByText("权限：沙箱、联网、写入；需要授权后使用").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText(/交付状态：可以体验、验证边界或生成计划/)).toBeInTheDocument();
    expect(screen.getByText(/下一步：先保留限制说明/)).toBeInTheDocument();
    expect(screen.getByText("为什么进队列：")).toBeInTheDocument();
    expect(screen.getByText("体检缺口：使用示例、用户感知位置。")).toBeInTheDocument();
    expect(screen.getByText("优先修改：")).toBeInTheDocument();
    expect(screen.getByText("先补 metadata.example1-3，用真实用户动作描述它能产出什么结果。")).toBeInTheDocument();
    expect(screen.getByText("验收路径：")).toBeInTheDocument();
    expect(screen.getByText("改完回到 /packs/wasm-plugin 验证入口、提示、结果位置和回滚路径是否可见。")).toBeInTheDocument();
    expect(screen.getAllByText(/验收出口：/).length).toBeGreaterThan(1);
    expect(screen.getAllByText(/回中心确认状态，进详情复查权限，再打开入口复验。/).length).toBeGreaterThan(1);
    expect(screen.getAllByText("本页已载入").length).toBeGreaterThan(0);
    expect(screen.getAllByText("已安装").length).toBeGreaterThan(0);
    expect(screen.getByText("yqpack：https://oss.example.com/wasm-plugin.yqpack")).toBeInTheDocument();
    expect(screen.getByText(`SHA：${"9".repeat(64)}`)).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /打开工坊/ })[0]).toHaveAttribute("href", expect.stringContaining("packId=yunque.pack.wasm-plugin"));
    expect(screen.getAllByRole("link", { name: /查看详情/ }).some((link) => link.getAttribute("href") === "/packs/detail?id=yunque.pack.wasm-plugin")).toBe(true);
    expect(screen.getAllByRole("link", { name: /回中心/ }).some((link) => link.getAttribute("href") === "/packs?q=yunque.pack.wasm-plugin&from=studio")).toBe(true);
    expect(screen.getAllByRole("link", { name: /打开入口/ }).some((link) => link.getAttribute("href") === "/packs/wasm-plugin")).toBe(true);
    expect(screen.getAllByRole("link", { name: /查看详情/ }).some((link) => link.getAttribute("href") === "/packs/detail?id=yunque.pack.documents")).toBe(true);
    expect(screen.getAllByRole("link", { name: /回中心/ }).some((link) => link.getAttribute("href") === "/packs?q=yunque.pack.documents&from=studio")).toBe(true);
    expect(screen.getAllByRole("link", { name: /打开入口/ }).some((link) => link.getAttribute("href") === "/chat?q=generate-doc")).toBe(true);

    fireEvent.click(screen.getAllByRole("button", { name: "载入本页" })[0]);

    expect((screen.getByLabelText("这次想补强什么") as HTMLInputElement).value).toBe("补齐 WASM 用途");
    expect((screen.getByLabelText("OSS / Release URL") as HTMLInputElement).value).toBe("https://oss.example.com/wasm-plugin.yqpack");
    expect((screen.getByLabelText("SHA256") as HTMLInputElement).value).toBe("9".repeat(64));
    expect(screen.getByText("已从能力包中心接入这个 yqpack")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "立即只读检查" }));
    await waitFor(() => {
      expect(packsClientMock.studioInspect).toHaveBeenCalledWith({
        packagePath: undefined,
        packageUrl: "https://oss.example.com/wasm-plugin.yqpack",
        sha256: "9".repeat(64),
        goal: "补齐 WASM 用途",
      });
    });
    expect(screen.getAllByText((_, element) => /本页状态：\s*只读已检查/.test(element?.textContent || "")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("只读已检查").length).toBeGreaterThan(0);
    expect(screen.getByText("已推进：2 / 2")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /下一包/ }));

    expect(screen.getByText("当前能力包：Documents")).toBeInTheDocument();
    expect(screen.getByText("当前处理：Documents")).toBeInTheDocument();
    expect(screen.getByText("已记录：只读已检查")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /上一包/ })).toBeEnabled();
    expect(screen.getByRole("button", { name: /下一包/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: /跳到下一个 P0/ })).toBeEnabled();
    expect((screen.getByLabelText("这次想补强什么") as HTMLInputElement).value).toBe("批量把这些能力包补齐用途和入口。");
    expect((screen.getByLabelText("OSS / Release URL") as HTMLInputElement).value).toBe("");
    expect((screen.getByLabelText("SHA256") as HTMLInputElement).value).toBe("");
    expect(screen.queryByText("已同步检查结果")).not.toBeInTheDocument();
    expect(screen.getAllByText((_, element) => /本页状态：\s*本页已载入/.test(element?.textContent || "")).length).toBeGreaterThan(0);
    expect(screen.getAllByRole("link", { name: /查看详情/ }).some((link) => link.getAttribute("href") === "/packs/detail?id=yunque.pack.documents")).toBe(true);
  });

  it("keeps Studio candidate selection searchable and paginated", async () => {
    const manyPacks = Array.from({ length: 14 }, (_, index) => ({
      manifest: {
        ...documentsManifest,
        id: `yunque.pack.bulk-${index + 1}`,
        name: `Bulk Pack ${index + 1}`,
        description: `批量候选 ${index + 1}`,
      },
      status: "disabled",
    }));
    packsClientMock.installed.mockResolvedValueOnce({
      packs: manyPacks,
      count: manyPacks.length,
    });

    render(<PackStudioPage />);

    expect(await screen.findByText("匹配 14 个 · 第 1 / 2 页")).toBeInTheDocument();
    expect(screen.getByText("工坊候选 · 每页 12 个")).toBeInTheDocument();
    expect(screen.getAllByText("待补肉").length).toBeGreaterThan(0);
    expect(screen.getAllByText(/交给小羽先补/).length).toBeGreaterThan(0);
    expect(screen.getAllByText("Bulk Pack 1").length).toBeGreaterThan(0);
    expect(screen.queryByText("Bulk Pack 9")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "后台支撑" }));
    expect(screen.getByText("匹配 0 个 · 第 1 / 1 页")).toBeInTheDocument();
    expect(screen.getByText("没有匹配的能力包。可以清除搜索，或切换来源筛选。")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "待补肉" }));
    expect(screen.getByText("匹配 14 个 · 第 1 / 2 页")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));

    expect(screen.getByText("匹配 14 个 · 第 2 / 2 页")).toBeInTheDocument();
    expect(screen.getByText("Bulk Pack 9")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("搜索工坊能力包"), { target: { value: "bulk-14" } });

    expect(screen.getByText("匹配 1 个 · 第 1 / 1 页")).toBeInTheDocument();
    expect(screen.getByText("Bulk Pack 14")).toBeInTheDocument();
  });

  it("turns real pack metadata into a guarded Xiaoyu modification task", async () => {
    render(<PackStudioPage />);

    expect(await screen.findByText("能力包工坊")).toBeInTheDocument();
    expect(screen.getByText("匹配 2 个 · 第 1 / 1 页")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("搜索工坊能力包"), { target: { value: "documents" } });
    expect(screen.getByText("匹配 1 个 · 第 1 / 1 页")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "清除搜索" }));
    expect(screen.getByText("WASM 能力包")).toBeInTheDocument();
    expect(screen.getAllByText("Documents").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByText("WASM 能力包"));

    expect(screen.getByText("可以让小羽优先优化")).toBeInTheDocument();
    expect(screen.getByText("需要守住的边界")).toBeInTheDocument();
    expect(screen.getByText("能力包体检")).toBeInTheDocument();
    expect(screen.getByText("交付状态")).toBeInTheDocument();
    expect(screen.getAllByText("待补肉").length).toBeGreaterThan(0);
    expect(screen.getByText("小羽会优先补齐：使用示例、用户感知位置。")).toBeInTheDocument();
    expect(screen.getByText("能力包体检缺口：使用示例、用户感知位置，优先补齐这些用户可感知信息。")).toBeInTheDocument();
    expect(screen.getByText(/交付状态仍是待补肉/)).toBeInTheDocument();
    expect(screen.getByText("不要反编译后硬改 WASM；需要源码、ABI 说明和 wasm-plugin 回归测试。")).toBeInTheDocument();
    expect(screen.getByText(/这个包仍是实验能力，改造时不要把它包装成稳定承诺。/)).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("这次想补强什么"), { target: { value: "增加一个可查看运行结果的界面" } });

    await waitFor(() => {
      expect(packsClientMock.studioPlan).toHaveBeenCalledWith({
        packId: "yunque.pack.wasm-plugin",
        goal: "增加一个可查看运行结果的界面",
      });
    });

    const task = screen.getByLabelText("小羽改包任务") as HTMLTextAreaElement;
    expect(task.value).toContain("用户目标：增加一个可查看运行结果的界面");
    expect(task.value).toContain("能力包体检：需补说明；还缺 使用示例、用户感知位置");
    expect(task.value).toContain("交付状态：待补肉");
    expect(task.value).toContain("验证交付状态是否改善");
    expect(task.value).toContain("POST /v1/wasm-plugin/run");
    expect(task.value).toContain("可改文件候选：");
    expect(task.value).toContain("差异预览草案：");
    expect(task.value).toContain("不要直接扩大权限或绕过签名");
    expect(task.value).toContain("重新打包与回滚：");
    expect(task.value).toContain("go test ./internal/packs/wasmplugin ./internal/controlplane/gateway -run WASM -count=1");

    const diffPreview = screen.getByLabelText("改包差异预览") as HTMLTextAreaElement;
    expect(diffPreview.value).toContain("diff --git a/packs/official/wasm-plugin-pack/pack.json");
    expect(diffPreview.value).toContain("\"description\": \"增加一个可查看运行结果的界面\"");
    expect(screen.getByText("packs/official/wasm-plugin-pack/pack.json")).toBeInTheDocument();
    expect(screen.getByText("internal/packs/wasmplugin/")).toBeInTheDocument();
    expect(screen.getByText("审计测试")).toBeInTheDocument();
    expect(screen.getAllByText("重新打包").length).toBeGreaterThan(0);
    expect(screen.getByText("回滚策略")).toBeInTheDocument();
    expect(screen.getByText("go run ./cmd/yunque-plugin pack packs\\official\\wasm-plugin-pack --out dist\\packs\\wasm-plugin-0.1.0.yqpack")).toBeInTheDocument();
    expect(screen.getByText("新包作为 fork/local 版本安装；验证失败时禁用新版本并回滚上一版本。")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("本地 yqpack 路径"), { target: { value: "C:\\packs\\wasm-plugin.yqpack" } });
    fireEvent.change(screen.getByLabelText("SHA256"), { target: { value: "a".repeat(64) } });
    fireEvent.click(screen.getByRole("button", { name: "只读检查" }));

    await waitFor(() => {
      expect(packsClientMock.studioInspect).toHaveBeenCalledWith({
        packagePath: "C:\\packs\\wasm-plugin.yqpack",
        packageUrl: undefined,
        sha256: "a".repeat(64),
        goal: "增加一个可查看运行结果的界面",
      });
    });
    expect(await screen.findByText("SHA 匹配")).toBeInTheDocument();
    expect(screen.getByText("3 个文件")).toBeInTheDocument();
    expect(screen.getByText("2 可改")).toBeInTheDocument();
    expect(screen.getByText("1 需源码/审计")).toBeInTheDocument();
    expect(screen.getByText("frontend/index.html")).toBeInTheDocument();
    expect(screen.getByText("backend/plugin.wasm")).toBeInTheDocument();
    expect(screen.getByText("只读检查不会安装能力包；它只告诉小羽真实包内有哪些文件、哪些能改、哪些必须保留边界。")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "准备工作区" }));
    await waitFor(() => {
      expect(packsClientMock.studioWorkspace).toHaveBeenCalledWith({
        packagePath: "C:\\packs\\wasm-plugin.yqpack",
        packageUrl: undefined,
        sha256: "a".repeat(64),
        goal: "增加一个可查看运行结果的界面",
      });
    });
    expect(await screen.findByText("能力包工坊工作区")).toBeInTheDocument();
    expect(screen.getAllByText("yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa").length).toBeGreaterThan(0);
    expect(screen.getByText("go run ./cmd/yunque-plugin pack C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa --out dist\\packs\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack")).toBeInTheDocument();
    expect(screen.getByText("新包安装后若验证失败，执行 /v1/packs/disable 禁用新包。")).toBeInTheDocument();
    expect(screen.getByText("工作区是可编辑副本，不会启用能力包；安装新 yqpack 前仍需重新检查、测试和确认回滚路径。")).toBeInTheDocument();
    expect(screen.getByText("改包工作流状态")).toBeInTheDocument();
    expect(screen.getByText("小羽可以帮你生成计划和草稿，但每一步都必须经过差异预览、审计、复检和显式安装确认。")).toBeInTheDocument();
    expect(screen.getByText("不自动应用")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "复制交付摘要" })).toBeInTheDocument();
    expect(screen.getByText("交付闭环")).toBeInTheDocument();
    expect(screen.getByText("改包完成不是停在差异预览；新 yqpack 需要复检、安装验证，再回能力包中心刷新来源与入口。")).toBeInTheDocument();
    expect(screen.getByText("还需继续检查")).toBeInTheDocument();
    expect(screen.getByText("本地复检")).toBeInTheDocument();
    expect(screen.getByText("本地安装验证")).toBeInTheDocument();
    expect(screen.getByText("刷新能力包中心")).toBeInTheDocument();
    expect(screen.getByText("上传 OSS 前检查清单")).toBeInTheDocument();
    expect(screen.getByText("全部就绪后再把新 yqpack 放到 Release 或 OSS；清单不会替你上传，也不会自动启用能力包。")).toBeInTheDocument();
    expect(screen.getByText("继续检查")).toBeInTheDocument();
    expect(screen.getByText("回滚路径已记录")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /回详情验收/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.wasm-plugin");
    expect(screen.getByRole("link", { name: /打开入口复验/ })).toHaveAttribute("href", "/packs/wasm-plugin");
    expect(screen.getAllByText("小羽草稿").length).toBeGreaterThan(0);
    expect(screen.getAllByText("下一步：载入草稿或交给小羽生成草稿").length).toBeGreaterThan(0);
    expect(screen.getByRole("link", { name: /跳到当前操作/ })).toHaveAttribute("href", "#draft-queue");

    expect(screen.getByText("小羽改造草稿队列")).toBeInTheDocument();
    expect(screen.getByText("从 Chat 导入改包计划").closest("#import-plan")).not.toBeNull();
    expect(screen.getByText("从 Chat 导入改包草稿").closest("#import-draft")).not.toBeNull();
    expect(screen.getByText("小羽改造草稿队列").closest("#draft-queue")).not.toBeNull();
    expect(screen.getByText("C:\\yunque\\packs\\studio\\frontend\\index.html")).toBeInTheDocument();
    expect(screen.getByText("原因：能力声明是能力包契约入口，适合先补用户能理解的用途、入口、限制和回滚提示。")).toBeInTheDocument();
    expect(screen.getByText("原因：HTML 前端资源可在 yqpack 工作区内预览和替换，适合补独立界面、权限说明和结果区。")).toBeInTheDocument();
    expect(screen.getByText("能力包可用性扫描")).toBeInTheDocument();
    expect(screen.getByText("复检 yqpack")).toBeInTheDocument();
    expect(screen.getByText("结构化计划只包含目标文件、风险、原因、门禁和内容摘要；真正内容仍需载入草稿后预览差异。")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "复制改包计划" }));
    const patchPlanText = vi.mocked(navigator.clipboard.writeText).mock.calls.at(-1)?.[0] || "";
    const patchPlan = JSON.parse(patchPlanText);
    expect(patchPlan.kind).toBe("yunque.pack_studio.patch_plan.v1");
    expect(patchPlan.pack.id).toBe("yunque.pack.wasm-plugin");
    expect(patchPlan.workspace.id).toBe("yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa");
    expect(patchPlan.candidates).toHaveLength(2);
    expect(patchPlan.candidates[0]).toMatchObject({
      label: "体检缺口能力声明草稿",
      risk_level: "low",
      applyable: true,
    });
    expect(patchPlan.candidates[0].gates).toContain("复跑体检缺口");
    expect(patchPlan.candidates[1].gates).toContain("复检 yqpack");
    expect(patchPlan.candidates[1].gates).toContain("复跑体检缺口");
    expect(patchPlan.candidates[1].content_summary.length).toBeGreaterThan(100);
    expect(patchPlanText).not.toContain("<!doctype html>");
    await waitFor(() => {
      expect(toastMock).toHaveBeenCalledWith("已复制结构化改包计划", "success");
    });

    const patchPlanLink = screen.getByRole("link", { name: /交给 Chat 里的小羽（带改包计划）/ });
    expect(patchPlanLink).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    const patchPlanQuery = new URL(patchPlanLink.getAttribute("href")!, "http://localhost").searchParams.get("q") || "";
    expect(patchPlanQuery).toContain("yunque.pack_studio.patch_plan.v1");
    expect(patchPlanQuery).toContain("体检缺口能力声明草稿");
    expect(patchPlanQuery).toContain("预览差异");
    expect(patchPlanQuery).toContain("运行内置审计");
    expect(patchPlanQuery).not.toContain("<!doctype html>");
    const patchPlanJson = patchPlanQuery.match(/```json\n([\s\S]+?)\n```/)?.[1] || "";
    const linkedPatchPlan = JSON.parse(patchPlanJson);
    expect(linkedPatchPlan.candidates[1].file_path).toBe("C:\\yunque\\packs\\studio\\frontend\\index.html");
    expect(linkedPatchPlan.candidates[1].content_summary.length).toBeGreaterThan(100);

    const draftRequestButtons = screen.getAllByRole("button", { name: "复制草稿请求" });
    fireEvent.click(draftRequestButtons[1]);
    const draftRequestPrompt = vi.mocked(navigator.clipboard.writeText).mock.calls.at(-1)?.[0] || "";
    expect(draftRequestPrompt).toContain("yunque.pack_studio.patch_draft_request.v1");
    expect(draftRequestPrompt).toContain("yunque.pack_studio.patch_draft.v1");
    expect(draftRequestPrompt).toContain("这次必须优先补齐体检缺口：使用示例、用户感知位置");
    expect(draftRequestPrompt).toContain("\"delivery\"");
    expect(draftRequestPrompt).toContain("本包交付状态：待补肉");
    expect(draftRequestPrompt).toContain("readiness_gaps");
    expect(draftRequestPrompt).toContain("content 必须是完整的新文件内容，不要输出差异补丁、片段或解释文本");
    expect(draftRequestPrompt).toContain("starter_content");
    expect(draftRequestPrompt).toContain("<!doctype html>");
    expect(draftRequestPrompt).toContain("不要声称已经应用改动");
    await waitFor(() => {
      expect(toastMock).toHaveBeenCalledWith("已复制改包草稿请求", "success");
    });

    const draftRequestLinks = screen.getAllByRole("link", { name: /交给小羽生成草稿/ });
    const draftRequestLink = draftRequestLinks[draftRequestLinks.length - 1];
    expect(draftRequestLink).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    const draftRequestQuery = new URL(draftRequestLink.getAttribute("href")!, "http://localhost").searchParams.get("q") || "";
    expect(draftRequestQuery).toContain("yunque.pack_studio.patch_draft_request.v1");
    expect(draftRequestQuery).toContain("完整的新文件内容");
    const draftRequestJson = draftRequestQuery.match(/```json\n([\s\S]+?)\n```/)?.[1] || "";
    const linkedDraftRequest = JSON.parse(draftRequestJson);
    expect(linkedDraftRequest.target.file_path).toBe("C:\\yunque\\packs\\studio\\frontend\\index.html");
    expect(linkedDraftRequest.target.readiness_gaps).toEqual(["使用示例", "用户感知位置"]);
    expect(linkedDraftRequest.expected_output.kind).toBe("yunque.pack_studio.patch_draft.v1");
    const readinessDraftLink = screen.getByRole("link", { name: /按体检缺口交给小羽生成草稿/ });
    expect(readinessDraftLink).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    const readinessDraftQuery = new URL(readinessDraftLink.getAttribute("href")!, "http://localhost").searchParams.get("q") || "";
    expect(readinessDraftQuery).toContain("这次必须优先补齐体检缺口：使用示例、用户感知位置");
    expect(readinessDraftQuery).toContain("体检缺口能力声明草稿");

    const importedChatMessage = [
      "小羽整理好了能力包工坊改包计划。",
      "",
      "```json",
      JSON.stringify(patchPlan, null, 2),
      "```",
    ].join("\n");
    fireEvent.change(screen.getByLabelText("导入改包计划 JSON"), { target: { value: importedChatMessage } });
    expect(screen.getByText("工作区匹配")).toBeInTheDocument();
    expect(screen.getByText("2 个候选")).toBeInTheDocument();
    expect(screen.getAllByText(/包：WASM 能力包/).length).toBeGreaterThan(0);
    expect(screen.getByText("摘要：" + patchPlan.candidates[1].content_summary.hash)).toBeInTheDocument();
    const importButtons = screen.getAllByRole("button", { name: "填入文件" });
    fireEvent.click(importButtons[1]);
    expect(screen.getByDisplayValue("C:\\yunque\\packs\\studio\\frontend\\index.html")).toBeInTheDocument();
    expect((screen.getByLabelText("新的文件内容") as HTMLTextAreaElement).value).toBe("");
    expect(toastMock).toHaveBeenCalledWith("已填入改包计划目标文件；请补入新内容后再预览差异", "success");

    const mismatchedPatchPlan = {
      ...patchPlan,
      workspace: {
        ...patchPlan.workspace,
        id: "other-workspace",
        path: "C:\\other\\pack",
        original_sha256: "f".repeat(64),
      },
    };
    fireEvent.change(screen.getByLabelText("导入改包计划 JSON"), {
      target: { value: `\`\`\`json\n${JSON.stringify(mismatchedPatchPlan, null, 2)}\n\`\`\`` },
    });
    expect(screen.getByText("工作区待确认")).toBeInTheDocument();
    expect(screen.getByText(/工作区或原始 SHA 与当前工作区不一致/)).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "填入文件" })[0]).toBeDisabled();

    fireEvent.change(screen.getByLabelText("导入改包草稿 JSON"), {
      target: { value: draftRequestPrompt },
    });
    expect(screen.getByText("这是草稿请求，还不是可导入草稿")).toBeInTheDocument();
    expect(screen.getByText("请求工作区匹配")).toBeInTheDocument();
    expect(screen.getByText(/生成出的改包草稿才能载入差异预览/)).toBeInTheDocument();
    expect(screen.getByText(/starter [\d,]+ chars/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /交给 Chat 生成草稿/ })).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    expect(screen.queryByText("未识别到可导入的改包草稿。草稿必须包含 file_path 和 content。")).not.toBeInTheDocument();

    const patchDraft = {
      kind: "yunque.pack_studio.patch_draft.v1",
      pack: {
        id: wasmManifest.id,
        name: wasmManifest.name,
        version: wasmManifest.version,
      },
      goal: "增加一个可查看运行结果的界面",
      workspace: patchPlan.workspace,
      file_path: "C:\\yunque\\packs\\studio\\pack.json",
      content: "{\n  \"description\": \"改包草稿内容\"\n}\n",
      reason: "Chat 里的小羽补了一版能力声明内容",
      risk_level: "low",
      gates: ["预览差异", "内置审计"],
    };
    fireEvent.change(screen.getByLabelText("导入改包草稿 JSON"), {
      target: { value: `\`\`\`json\n${JSON.stringify(patchDraft, null, 2)}\n\`\`\`` },
    });
    expect(screen.getByText("草稿工作区匹配")).toBeInTheDocument();
    expect(screen.queryByText("2 chars")).not.toBeInTheDocument();
    expect(screen.getByText("原因：Chat 里的小羽补了一版能力声明内容")).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "载入草稿" })[0]);
    expect(screen.getByDisplayValue("C:\\yunque\\packs\\studio\\pack.json")).toBeInTheDocument();
    expect((screen.getByLabelText("新的文件内容") as HTMLTextAreaElement).value).toContain("改包草稿内容");
    expect(toastMock).toHaveBeenCalledWith("已载入改包草稿，请先预览差异再应用", "success");

    const mismatchedPatchDraft = {
      ...patchDraft,
      workspace: {
        ...patchDraft.workspace,
        id: "other-workspace",
        path: "C:\\other\\pack",
        original_sha256: "f".repeat(64),
      },
    };
    fireEvent.change(screen.getByLabelText("导入改包草稿 JSON"), {
      target: { value: `\`\`\`json\n${JSON.stringify(mismatchedPatchDraft, null, 2)}\n\`\`\`` },
    });
    expect(screen.getByText("草稿待确认")).toBeInTheDocument();
    expect(screen.getByText(/改包草稿的工作区或原始 SHA 与当前工作区不一致/)).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "载入草稿" })[0]).toBeDisabled();

    const draftButtons = screen.getAllByRole("button", { name: "载入草稿" }).slice(1);
    fireEvent.click(draftButtons[0]);
    const manifestDraft = screen.getByLabelText("新的文件内容") as HTMLTextAreaElement;
    const draftJSON = JSON.parse(manifestDraft.value);
    expect(draftJSON.description).toBe("增加一个可查看运行结果的界面");
    expect(draftJSON.metadata.primaryActionPath).toBe("/packs/wasm-plugin");
    expect(draftJSON.metadata.usageSurface).toContain("/packs/wasm-plugin");
    expect(draftJSON.metadata.example3).toContain("保存到记忆或知识");
    expect(draftJSON.metadata.studioGoal).toBe("增加一个可查看运行结果的界面");
    expect(toastMock).toHaveBeenCalledWith("已生成 体检缺口能力声明草稿，请先预览差异再应用", "success");
    expect(screen.getByText("草稿只会填入工作区改动框；真正写入仍需先预览差异，并在应用后运行内置审计。")).toBeInTheDocument();

    fireEvent.click(draftButtons[1]);
    const frontendDraft = screen.getByLabelText("新的文件内容") as HTMLTextAreaElement;
    expect(screen.getByDisplayValue("C:\\yunque\\packs\\studio\\frontend\\index.html")).toBeInTheDocument();
    expect(frontendDraft.value).toContain("<title>WASM 能力包</title>");
    expect(frontendDraft.value).toContain("能力包界面草稿 · yunque.pack.wasm-plugin");
    expect(frontendDraft.value).toContain("这次补齐的体检缺口");
    expect(frontendDraft.value).toContain("接入真实调用前必须先预览差异、运行审计并重新打包");
    expect(screen.getAllByText("下一步：点击预览差异").length).toBeGreaterThan(0);

    fireEvent.click(draftButtons[0]);
    fireEvent.change(screen.getByLabelText("新的文件内容"), { target: { value: "{\n  \"description\": \"更清楚\"\n}" } });
    fireEvent.click(screen.getByRole("button", { name: "预览差异" }));
    await waitFor(() => {
      expect(packsClientMock.studioPatch).toHaveBeenCalledWith({
        workspacePath: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
        filePath: "C:\\yunque\\packs\\studio\\pack.json",
        content: "{\n  \"description\": \"更清楚\"\n}",
        reason: "增加一个可查看运行结果的界面",
        apply: false,
      });
    });
    const workspaceDiffPreview = await screen.findByLabelText("工作区差异预览") as HTMLTextAreaElement;
    expect(workspaceDiffPreview.value).toContain("\"description\": \"更清楚\"");
    expect(screen.getByText("仅预览")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "应用到工作区" }));
    await waitFor(() => {
      expect(packsClientMock.studioPatch).toHaveBeenCalledWith(expect.objectContaining({ apply: true }));
    });
    expect(await screen.findByText("已应用")).toBeInTheDocument();
    expect(screen.getAllByText("下一步：运行内置审计").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "运行内置审计" }));
    await waitFor(() => {
      expect(packsClientMock.studioAudit).toHaveBeenCalledWith({
        workspacePath: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
        goal: "增加一个可查看运行结果的界面",
      });
    });
    expect(await screen.findByText("审计通过")).toBeInTheDocument();
    expect(screen.getByText("1 个改动 · 1 可改 · 0 需源码/专项审计")).toBeInTheDocument();
    expect(screen.getAllByText("下一步：可以重新打包").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "重新打包" }));
    await waitFor(() => {
      expect(packsClientMock.studioRepack).toHaveBeenCalledWith({
        workspacePath: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
        goal: "增加一个可查看运行结果的界面",
      });
    });
    expect(await screen.findByText("新 yqpack 已生成")).toBeInTheDocument();
    expect(screen.getByText("C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack")).toBeInTheDocument();
    expect(screen.getByText("SHA256：" + "d".repeat(64))).toBeInTheDocument();
    expect(screen.getByText("发布与验证路径")).toBeInTheDocument();
    expect(screen.getByText("新 yqpack 不会自动上传或启用；先本地复检，再安装验证，最后更新发布源并回能力包中心刷新。")).toBeInTheDocument();
    expect(screen.getAllByText("上传 package_path 并保留 SHA256：" + "d".repeat(64)).length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("更新 catalog/release 后回 /packs 刷新官方源/私有源。").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByRole("link", { name: /回能力包中心/ }).some((link) => link.getAttribute("href") === "/packs?q=yunque.pack.wasm-plugin&from=studio")).toBe(true);
    expect(screen.getAllByRole("link", { name: /回详情验收/ }).some((link) => link.getAttribute("href") === "/packs/detail?id=yunque.pack.wasm-plugin")).toBe(true);
    expect(screen.getAllByRole("link", { name: /打开入口复验/ }).some((link) => link.getAttribute("href") === "/packs/wasm-plugin")).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "复制新包信息" }));
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(`package=C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack\nsha256=${"d".repeat(64)}`);
    expect(screen.getAllByText("下一步：复检新包 SHA").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "复检新包" }));
    await waitFor(() => {
      expect(packsClientMock.studioInspect).toHaveBeenCalledWith({
        packagePath: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack",
        sha256: "d".repeat(64),
        goal: "增加一个可查看运行结果的界面",
      });
    });
    expect(await screen.findByText("复检 SHA 匹配")).toBeInTheDocument();
    expect(screen.getAllByText("下一步：安装新包").length).toBeGreaterThan(0);
    expect(screen.getByText("可上传/发布")).toBeInTheDocument();
    expect(screen.getAllByText("SHA 匹配，3 个文件").length).toBeGreaterThanOrEqual(2);

    fireEvent.click(screen.getByRole("button", { name: "安装新包" }));
    await waitFor(() => {
      expect(packsClientMock.install).toHaveBeenCalledWith({
        packagePath: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack",
        sha256: "d".repeat(64),
        source: "pack-studio:C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack",
      });
    });
    expect((await screen.findAllByText("已安装未启用")).length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("WASM 能力包 · 已安装未启用").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("已安装").length).toBeGreaterThan(0);
    expect(screen.getAllByText("需要授权").length).toBeGreaterThan(0);
    expect(screen.getAllByText("权限：沙箱、联网、写入；需要授权后使用").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByRole("link", { name: /^打开入口$/ }).some((link) => link.getAttribute("href") === "/packs/wasm-plugin")).toBe(true);
    expect(screen.getByRole("link", { name: /查看权限与来源/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.wasm-plugin");
    expect(screen.getByRole("link", { name: /回中心管理/ })).toHaveAttribute("href", "/packs?q=yunque.pack.wasm-plugin&from=studio");
    expect(screen.getByText("下一步：确认权限后再启用")).toBeInTheDocument();
    expect(screen.getByText("新包已经安装但未启用。先确认权限、来源和风险；确认后启用，或回中心继续管理这个包。")).toBeInTheDocument();
    expect(screen.getByText("安装后怎么验收")).toBeInTheDocument();
    expect(screen.getByText(/先触发一次：从「检查 WASM 能力」进入/)).toBeInTheDocument();
    expect(screen.getByText(/看结果在哪：回到 \/packs\/wasm-plugin 查看页面状态、结果或下一步动作/)).toBeInTheDocument();
    expect(screen.getByText("复验失败怎么退")).toBeInTheDocument();
    expect(screen.getByText("1. 先禁用新包，停止它继续影响入口、Chat 或任务流程。")).toBeInTheDocument();
    expect(screen.getByText("2. 再执行回滚，恢复上一版本或原始 yqpack 的安装记录。")).toBeInTheDocument();
    expect(screen.getAllByText("下一步：确认权限后启用或回滚").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "启用" }));
    await waitFor(() => {
      expect(packsClientMock.enable).toHaveBeenCalledWith("yunque.pack.wasm-plugin");
    });
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "启用" })).toBeDisabled();
    });
    expect(screen.getAllByText("已启用").length).toBeGreaterThan(0);
    expect(screen.getByText("下一步：打开入口验证，或回详情确认权限")).toBeInTheDocument();
    expect(screen.getByText("新包已经启用。建议先打开入口跑一遍主路径；如果结果不符合预期，可以回到这里禁用或回滚。")).toBeInTheDocument();
    expect(screen.getAllByText("下一步：打开入口或查看详情").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "回滚" }));
    await waitFor(() => {
      expect(packsClientMock.rollback).toHaveBeenCalledWith("yunque.pack.wasm-plugin");
    });
    expect(await screen.findByText("下一步：确认权限后再启用")).toBeInTheDocument();
    expect(toastMock).toHaveBeenCalledWith("已回滚能力包", "success");

    fireEvent.click(screen.getByRole("button", { name: "复制交付摘要" }));
    const deliverySummary = vi.mocked(navigator.clipboard.writeText).mock.calls.at(-1)?.[0] || "";
    expect(deliverySummary).toContain("# 能力包工坊改包交付摘要");
    expect(deliverySummary).toContain("- 改包目标：增加一个可查看运行结果的界面");
    expect(deliverySummary).toContain("- 审计：通过；风险：medium；改动：1；可改：1；需源码/专项审计：0");
    expect(deliverySummary).toContain("- 包路径：C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack");
    expect(deliverySummary).toContain("- SHA256：" + "d".repeat(64));
    expect(deliverySummary).toContain("- 安装状态：installed；安装包：WASM 能力包 (yunque.pack.wasm-plugin)");
    expect(deliverySummary).toContain("## 产品验收入口");
    expect(deliverySummary).toContain("- 能力包中心：/packs?q=yunque.pack.wasm-plugin&from=studio（聚焦这个包，确认来源、版本、安装/启用状态和侧栏入口）");
    expect(deliverySummary).toContain("- 权限与来源详情：/packs/detail?id=yunque.pack.wasm-plugin（复查权限、风险、入口、回滚和小羽补齐建议）");
    expect(deliverySummary).toContain("- 打开入口复验：/packs/wasm-plugin（按用户主路径触发一次，确认结果、产物或状态变化可见）");
    expect(deliverySummary).toContain("- 当前安装状态：已安装未启用，先确认权限后启用；不符合预期则禁用或回滚。");
    expect(deliverySummary).toContain("## 验证路径");
    expect(deliverySummary).toContain("- 先触发一次：从「检查 WASM 能力」进入");
    expect(deliverySummary).toContain("- 看结果在哪：回到 /packs/wasm-plugin 查看页面状态、结果或下一步动作");
    expect(deliverySummary).toContain("- 高风险或审计阻断改动不得继续打包/安装。");
    await waitFor(() => {
      expect(toastMock).toHaveBeenCalledWith("已复制改包交付摘要", "success");
    });

    fireEvent.click(screen.getByRole("button", { name: "复制任务" }));

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(expect.stringContaining("请以“小羽改包”的方式改造能力包"));
    await waitFor(() => {
      expect(toastMock).toHaveBeenCalledWith("已复制小羽改包任务", "success");
    });

    fireEvent.click(screen.getByRole("button", { name: "只读检查" }));
    await waitFor(() => {
      expect(packsClientMock.studioInspect).toHaveBeenCalledWith({
        packagePath: "C:\\packs\\wasm-plugin.yqpack",
        packageUrl: undefined,
        sha256: "a".repeat(64),
        goal: "增加一个可查看运行结果的界面",
      });
    });
    expect(await screen.findByText("当前阶段：准备工作区 · 下一步：SHA 匹配后准备工作区。小羽只生成计划和草稿，不会自动应用改动。")).toBeInTheDocument();
    expect(screen.queryByText("新包已经安装但未启用。先确认权限、来源和风险；确认后启用，或回中心继续管理这个包。")).not.toBeInTheDocument();
    expect(screen.queryByText("安装后怎么验收")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "启用" })).not.toBeInTheDocument();
  }, 30000);
});
