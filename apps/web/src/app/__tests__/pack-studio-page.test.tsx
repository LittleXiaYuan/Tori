import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PackStudioPage from "../packs/studio/page";

const packsClientMock = vi.hoisted(() => ({
  installed: vi.fn(),
  catalog: vi.fn(),
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
        "不直接修改已签名或已安装包；先生成 diff 方案，用户确认后再打包为新 yqpack。",
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
        "diff 预览草案：",
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
      next_steps: ["让小羽只修改 editable_files 中的文件，先给 diff 预览。"],
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

    await waitFor(() => {
      expect(packsClientMock.studioPlan).toHaveBeenCalledWith({
        packId: "yunque.pack.wasm-plugin",
        goal: "增加一个可查看运行结果的界面",
      });
    });

    const task = screen.getByLabelText("小羽改包任务") as HTMLTextAreaElement;
    expect(task.value).toContain("用户目标：增加一个可查看运行结果的界面");
    expect(task.value).toContain("POST /v1/wasm-plugin/run");
    expect(task.value).toContain("可改文件候选：");
    expect(task.value).toContain("diff 预览草案：");
    expect(task.value).toContain("不要直接扩大权限或绕过签名");
    expect(task.value).toContain("重新打包与回滚：");
    expect(task.value).toContain("go test ./internal/packs/wasmplugin ./internal/controlplane/gateway -run WASM -count=1");

    const diffPreview = screen.getByLabelText("改包 diff 预览") as HTMLTextAreaElement;
    expect(diffPreview.value).toContain("diff --git a/packs/official/wasm-plugin-pack/pack.json");
    expect(diffPreview.value).toContain("\"description\": \"增加一个可查看运行结果的界面\"");
    expect(screen.getByText("packs/official/wasm-plugin-pack/pack.json")).toBeInTheDocument();
    expect(screen.getByText("internal/packs/wasmplugin/")).toBeInTheDocument();
    expect(screen.getByText("审计测试")).toBeInTheDocument();
    expect(screen.getByText("重新打包")).toBeInTheDocument();
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
    expect(await screen.findByText("Pack Studio 工作区")).toBeInTheDocument();
    expect(screen.getAllByText("yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa").length).toBeGreaterThan(0);
    expect(screen.getByText("go run ./cmd/yunque-plugin pack C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa --out dist\\packs\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack")).toBeInTheDocument();
    expect(screen.getByText("新包安装后若验证失败，执行 /v1/packs/disable 禁用新包。")).toBeInTheDocument();
    expect(screen.getByText("工作区是可编辑副本，不会启用能力包；安装新 yqpack 前仍需重新检查、测试和确认回滚路径。")).toBeInTheDocument();
    expect(screen.getByText("改包工作流状态")).toBeInTheDocument();
    expect(screen.getByText("小羽可以帮你生成计划和草稿，但每一步都必须经过 diff、审计、复检和显式安装确认。")).toBeInTheDocument();
    expect(screen.getByText("不自动应用")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "复制交付摘要" })).toBeInTheDocument();
    expect(screen.getByText("Plan / Draft")).toBeInTheDocument();
    expect(screen.getAllByText("下一步：载入草稿或交给小羽生成 Draft").length).toBeGreaterThan(0);

    expect(screen.getByText("小羽改造草稿队列")).toBeInTheDocument();
    expect(screen.getByText("从 Chat 导入 Patch Plan").closest("#import-plan")).not.toBeNull();
    expect(screen.getByText("从 Chat 导入 Patch Draft").closest("#import-draft")).not.toBeNull();
    expect(screen.getByText("小羽改造草稿队列").closest("#draft-queue")).not.toBeNull();
    expect(screen.getByText("C:\\yunque\\packs\\studio\\frontend\\index.html")).toBeInTheDocument();
    expect(screen.getByText("原因：manifest 是能力包契约入口，适合先补用户能理解的用途、入口、限制和回滚提示。")).toBeInTheDocument();
    expect(screen.getByText("原因：HTML 前端资源可在 yqpack 工作区内预览和替换，适合补独立界面、权限说明和结果区。")).toBeInTheDocument();
    expect(screen.getByText("Pack 可用性扫描")).toBeInTheDocument();
    expect(screen.getByText("复检 yqpack")).toBeInTheDocument();
    expect(screen.getByText("结构化计划只包含目标文件、风险、原因、门禁和内容摘要；真正内容仍需载入草稿后预览 diff。")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "复制 Patch Plan JSON" }));
    const patchPlanText = vi.mocked(navigator.clipboard.writeText).mock.calls.at(-1)?.[0] || "";
    const patchPlan = JSON.parse(patchPlanText);
    expect(patchPlan.kind).toBe("yunque.pack_studio.patch_plan.v1");
    expect(patchPlan.pack.id).toBe("yunque.pack.wasm-plugin");
    expect(patchPlan.workspace.id).toBe("yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa");
    expect(patchPlan.candidates).toHaveLength(2);
    expect(patchPlan.candidates[0]).toMatchObject({
      label: "manifest 草稿",
      risk_level: "low",
      applyable: true,
    });
    expect(patchPlan.candidates[1].gates).toContain("复检 yqpack");
    expect(patchPlan.candidates[1].content_summary.length).toBeGreaterThan(100);
    expect(patchPlanText).not.toContain("<!doctype html>");
    await waitFor(() => {
      expect(toastMock).toHaveBeenCalledWith("已复制结构化 Patch Plan", "success");
    });

    const patchPlanLink = screen.getByRole("link", { name: /交给 Chat 里的小羽（带 Patch Plan）/ });
    expect(patchPlanLink).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    const patchPlanQuery = new URL(patchPlanLink.getAttribute("href")!, "http://localhost").searchParams.get("q") || "";
    expect(patchPlanQuery).toContain("yunque.pack_studio.patch_plan.v1");
    expect(patchPlanQuery).toContain("manifest 草稿");
    expect(patchPlanQuery).toContain("预览 diff");
    expect(patchPlanQuery).toContain("运行内置审计");
    expect(patchPlanQuery).not.toContain("<!doctype html>");
    const patchPlanJson = patchPlanQuery.match(/```json\n([\s\S]+?)\n```/)?.[1] || "";
    const linkedPatchPlan = JSON.parse(patchPlanJson);
    expect(linkedPatchPlan.candidates[1].file_path).toBe("C:\\yunque\\packs\\studio\\frontend\\index.html");
    expect(linkedPatchPlan.candidates[1].content_summary.length).toBeGreaterThan(100);

    const draftRequestButtons = screen.getAllByRole("button", { name: "复制 Draft 请求" });
    fireEvent.click(draftRequestButtons[1]);
    const draftRequestPrompt = vi.mocked(navigator.clipboard.writeText).mock.calls.at(-1)?.[0] || "";
    expect(draftRequestPrompt).toContain("yunque.pack_studio.patch_draft_request.v1");
    expect(draftRequestPrompt).toContain("yunque.pack_studio.patch_draft.v1");
    expect(draftRequestPrompt).toContain("content 必须是完整的新文件内容，不要输出 diff、片段或解释文本");
    expect(draftRequestPrompt).toContain("starter_content");
    expect(draftRequestPrompt).toContain("<!doctype html>");
    expect(draftRequestPrompt).toContain("不要声称已经应用改动");
    await waitFor(() => {
      expect(toastMock).toHaveBeenCalledWith("已复制 Patch Draft 请求", "success");
    });

    const draftRequestLink = screen.getAllByRole("link", { name: /交给小羽生成 Draft/ })[1];
    expect(draftRequestLink).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    const draftRequestQuery = new URL(draftRequestLink.getAttribute("href")!, "http://localhost").searchParams.get("q") || "";
    expect(draftRequestQuery).toContain("yunque.pack_studio.patch_draft_request.v1");
    expect(draftRequestQuery).toContain("完整的新文件内容");
    const draftRequestJson = draftRequestQuery.match(/```json\n([\s\S]+?)\n```/)?.[1] || "";
    const linkedDraftRequest = JSON.parse(draftRequestJson);
    expect(linkedDraftRequest.target.file_path).toBe("C:\\yunque\\packs\\studio\\frontend\\index.html");
    expect(linkedDraftRequest.expected_output.kind).toBe("yunque.pack_studio.patch_draft.v1");

    const importedChatMessage = [
      "小羽整理好了 Pack Studio Patch Plan。",
      "",
      "```json",
      JSON.stringify(patchPlan, null, 2),
      "```",
    ].join("\n");
    fireEvent.change(screen.getByLabelText("导入 Patch Plan JSON"), { target: { value: importedChatMessage } });
    expect(screen.getByText("工作区匹配")).toBeInTheDocument();
    expect(screen.getByText("2 个候选")).toBeInTheDocument();
    expect(screen.getByText(/包：WASM 能力包/)).toBeInTheDocument();
    expect(screen.getByText("摘要：" + patchPlan.candidates[1].content_summary.hash)).toBeInTheDocument();
    const importButtons = screen.getAllByRole("button", { name: "填入文件" });
    fireEvent.click(importButtons[1]);
    expect(screen.getByDisplayValue("C:\\yunque\\packs\\studio\\frontend\\index.html")).toBeInTheDocument();
    expect((screen.getByLabelText("新的文件内容") as HTMLTextAreaElement).value).toBe("");
    expect(toastMock).toHaveBeenCalledWith("已填入 Patch Plan 目标文件；请补入新内容后再预览 diff", "success");

    const mismatchedPatchPlan = {
      ...patchPlan,
      workspace: {
        ...patchPlan.workspace,
        id: "other-workspace",
        path: "C:\\other\\pack",
        original_sha256: "f".repeat(64),
      },
    };
    fireEvent.change(screen.getByLabelText("导入 Patch Plan JSON"), {
      target: { value: `\`\`\`json\n${JSON.stringify(mismatchedPatchPlan, null, 2)}\n\`\`\`` },
    });
    expect(screen.getByText("工作区待确认")).toBeInTheDocument();
    expect(screen.getByText(/工作区或原始 SHA 与当前工作区不一致/)).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "填入文件" })[0]).toBeDisabled();

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
      content: "{\n  \"description\": \"Patch Draft 内容\"\n}\n",
      reason: "Chat 里的小羽补了一版 manifest 内容",
      risk_level: "low",
      gates: ["预览 diff", "内置审计"],
    };
    fireEvent.change(screen.getByLabelText("导入 Patch Draft JSON"), {
      target: { value: `\`\`\`json\n${JSON.stringify(patchDraft, null, 2)}\n\`\`\`` },
    });
    expect(screen.getByText("Draft 工作区匹配")).toBeInTheDocument();
    expect(screen.queryByText("2 chars")).not.toBeInTheDocument();
    expect(screen.getByText("原因：Chat 里的小羽补了一版 manifest 内容")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "载入 Draft" }));
    expect(screen.getByDisplayValue("C:\\yunque\\packs\\studio\\pack.json")).toBeInTheDocument();
    expect((screen.getByLabelText("新的文件内容") as HTMLTextAreaElement).value).toContain("Patch Draft 内容");
    expect(toastMock).toHaveBeenCalledWith("已载入 Patch Draft，请先预览 diff 再应用", "success");

    const mismatchedPatchDraft = {
      ...patchDraft,
      workspace: {
        ...patchDraft.workspace,
        id: "other-workspace",
        path: "C:\\other\\pack",
        original_sha256: "f".repeat(64),
      },
    };
    fireEvent.change(screen.getByLabelText("导入 Patch Draft JSON"), {
      target: { value: `\`\`\`json\n${JSON.stringify(mismatchedPatchDraft, null, 2)}\n\`\`\`` },
    });
    expect(screen.getByText("Draft 待确认")).toBeInTheDocument();
    expect(screen.getByText(/Patch Draft 的工作区或原始 SHA 与当前工作区不一致/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "载入 Draft" })).toBeDisabled();

    const draftButtons = screen.getAllByRole("button", { name: "载入草稿" });
    fireEvent.click(draftButtons[0]);
    const manifestDraft = screen.getByLabelText("新的文件内容") as HTMLTextAreaElement;
    const draftJSON = JSON.parse(manifestDraft.value);
    expect(draftJSON.description).toBe("增加一个可查看运行结果的界面");
    expect(draftJSON.metadata.primaryActionPath).toBe("/packs/wasm-plugin");
    expect(draftJSON.metadata.example3).toContain("保存到记忆或知识");
    expect(draftJSON.metadata.studioGoal).toBe("增加一个可查看运行结果的界面");
    expect(toastMock).toHaveBeenCalledWith("已生成 manifest 草稿，请先预览 diff 再应用", "success");
    expect(screen.getByText("草稿只会填入工作区改动框；真正写入仍需先预览 diff，并在应用后运行内置审计。")).toBeInTheDocument();

    fireEvent.click(draftButtons[1]);
    const frontendDraft = screen.getByLabelText("新的文件内容") as HTMLTextAreaElement;
    expect(screen.getByDisplayValue("C:\\yunque\\packs\\studio\\frontend\\index.html")).toBeInTheDocument();
    expect(frontendDraft.value).toContain("<title>WASM 能力包</title>");
    expect(frontendDraft.value).toContain("能力包界面草稿 · yunque.pack.wasm-plugin");
    expect(frontendDraft.value).toContain("接入真实 bridge/API 前必须先预览 diff、运行审计并重新打包");
    expect(screen.getAllByText("下一步：点击预览 diff").length).toBeGreaterThan(0);

    fireEvent.click(draftButtons[0]);
    fireEvent.change(screen.getByLabelText("新的文件内容"), { target: { value: "{\n  \"description\": \"更清楚\"\n}" } });
    fireEvent.click(screen.getByRole("button", { name: "预览 diff" }));
    await waitFor(() => {
      expect(packsClientMock.studioPatch).toHaveBeenCalledWith({
        workspacePath: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
        filePath: "C:\\yunque\\packs\\studio\\pack.json",
        content: "{\n  \"description\": \"更清楚\"\n}",
        reason: "增加一个可查看运行结果的界面",
        apply: false,
      });
    });
    const workspaceDiffPreview = await screen.findByLabelText("工作区 diff 预览") as HTMLTextAreaElement;
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

    fireEvent.click(screen.getByRole("button", { name: "安装新包" }));
    await waitFor(() => {
      expect(packsClientMock.install).toHaveBeenCalledWith({
        packagePath: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack",
        sha256: "d".repeat(64),
        source: "pack-studio:C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack",
      });
    });
    expect(await screen.findByText("已安装未启用")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /打开入口/ })).toHaveAttribute("href", "/packs/wasm-plugin");
    expect(screen.getByRole("link", { name: /查看详情/ })).toHaveAttribute("href", "/packs/detail?id=yunque.pack.wasm-plugin");
    expect(screen.getAllByText("下一步：确认权限后启用或回滚").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "启用" }));
    await waitFor(() => {
      expect(packsClientMock.enable).toHaveBeenCalledWith("yunque.pack.wasm-plugin");
    });
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "启用" })).toBeDisabled();
    });
    expect(screen.getAllByText("下一步：打开入口或查看详情").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "复制交付摘要" }));
    const deliverySummary = vi.mocked(navigator.clipboard.writeText).mock.calls.at(-1)?.[0] || "";
    expect(deliverySummary).toContain("# Pack Studio 改包交付摘要");
    expect(deliverySummary).toContain("- 改包目标：增加一个可查看运行结果的界面");
    expect(deliverySummary).toContain("- 审计：通过；风险：medium；改动：1；可改：1；需源码/专项审计：0");
    expect(deliverySummary).toContain("- 包路径：C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-studio.yqpack");
    expect(deliverySummary).toContain("- SHA256：" + "d".repeat(64));
    expect(deliverySummary).toContain("- 安装状态：enabled；安装包：WASM 能力包 (yunque.pack.wasm-plugin)");
    expect(deliverySummary).toContain("- 高风险或审计阻断改动不得继续打包/安装。");
    await waitFor(() => {
      expect(toastMock).toHaveBeenCalledWith("已复制改包交付摘要", "success");
    });

    fireEvent.click(screen.getByRole("button", { name: "复制任务" }));

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(expect.stringContaining("请以“小羽改包”的方式改造能力包"));
    await waitFor(() => {
      expect(toastMock).toHaveBeenCalledWith("已复制小羽改包任务", "success");
    });
  }, 15000);
});
