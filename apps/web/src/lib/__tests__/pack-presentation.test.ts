import { describe, expect, it } from "vitest";
import type { PackManifest } from "yunque-client/packs";
import {
  packBoundarySummary,
  capabilitySurfaceLabels,
  catalogActionForEntry,
  entryInstallRequest,
  formatPackInstallError,
  groupPackPermissions,
  packPermissionSummary,
  packDeliveryProfile,
  packInstallChecklist,
  packInstallTroubleshooting,
  packFeatureFlags,
  packReadiness,
  packUsageExplanation,
  packUsability,
  packVerificationSteps,
  riskProfileForPack,
} from "../pack-presentation";

describe("pack-presentation", () => {
  it("groups backend permissions into user readable categories", () => {
    const groups = groupPackPermissions([
      "knowledge:read",
      "memory:write",
      "network:download",
      "browser:intent",
      "computer:plan",
      "sandbox:execute",
      "control-plane:manage",
      "custom:odd",
    ]);

    expect(groups.map((group) => group.key)).toEqual([
      "memoryKnowledge",
      "network",
      "browser",
      "computer",
      "sandbox",
      "admin",
      "other",
    ]);
    expect(groups.find((group) => group.key === "memoryKnowledge")?.permissions).toEqual(["knowledge:read", "memory:write"]);
    expect(groups.find((group) => group.key === "computer")?.label).toBe("电脑使用");
  });

  it("marks computer-use as high risk while explaining plan-only behavior", () => {
    const manifest: PackManifest = {
      id: "yunque.pack.computer-use",
      name: "Computer Use",
      version: "0.1.0",
      backend: {
        capabilities: ["computer.use.plan"],
        permissions: ["computer:plan", "browser:read"],
      },
    };

    expect(riskProfileForPack(manifest)).toMatchObject({
      level: "high",
      label: "需要授权",
      requiresAuthorization: true,
    });
    expect(packUsageExplanation(manifest).join(" ")).toContain("当前阶段只做计划");
    expect(packUsageExplanation(manifest).join(" ")).toContain("不执行本机控制");
    expect(packInstallChecklist(manifest, { sourceLabel: "官方发布源 · example.com" })).toMatchObject([
      { key: "source", tone: "safe" },
      { key: "permissions", tone: "warning" },
      { key: "boundary", tone: "danger" },
      { key: "rollback", tone: "warning" },
    ]);
    expect(packInstallChecklist(manifest)[2].detail).toContain("不执行本机桌面控制");
    expect(packPermissionSummary(manifest)).toBe("权限：电脑使用、浏览器；需要授权后使用");
  });

  it("summarizes low-risk packs without raw permission jargon", () => {
    const manifest: PackManifest = {
      id: "yunque.pack.simple",
      name: "Simple",
      version: "0.1.0",
      backend: {
        permissions: [],
      },
    };

    expect(packPermissionSummary(manifest)).toBe("权限：未声明额外权限；低风险");
  });

  it("marks financial trading permissions as high risk", () => {
    const manifest: PackManifest = {
      id: "trading",
      name: "量化交易",
      version: "0.1.0",
      backend: {
        capabilities: ["trading.signal.analyze", "trading.order.propose"],
        permissions: ["network:read", "approval:required", "finance:trade:propose"],
      },
    };

    expect(riskProfileForPack(manifest)).toMatchObject({
      level: "high",
      label: "需要授权",
      requiresAuthorization: true,
    });
  });

  it("detects iframe bundle and wasm surfaces", () => {
    const manifest: PackManifest = {
      id: "yunque.pack.dlc-demo",
      name: "DLC Demo",
      version: "0.1.0",
      frontend: {
        routes: [{ path: "/packs/dlc-demo", component: "iframe", title: "DLC Demo" }],
        assets: { type: "iframe-bundle", entry: "index.html" },
      },
      backend: {
        capabilities: ["wasm.plugin.execute"],
        permissions: ["wasm:execute"],
      },
      sdk: { typescript: "yunque-client/dlc-demo" },
      update: { rollback: true },
    };

    expect(packFeatureFlags(manifest)).toMatchObject({
      hasFrontend: true,
      hasBackend: true,
      hasWasm: true,
      isIframeBundle: true,
      hasSdk: true,
      canRollback: true,
    });
    expect(capabilitySurfaceLabels(manifest)).toEqual(["独立界面包", "有后端能力", "含 WASM", "有 SDK", "可回滚"]);
  });

  it("builds install requests for package, remote manifest and local manifest entries", () => {
    const manifest: PackManifest = { id: "yunque.pack.demo", name: "Demo", version: "0.1.0" };

    expect(entryInstallRequest({
      manifest,
      package_url: "https://oss.example.com/demo.yqpack",
      sha256: "abc",
      source: "official",
      downloadable: true,
    })).toEqual({ packageUrl: "https://oss.example.com/demo.yqpack", sha256: "abc", source: "official", download: true });
    expect(entryInstallRequest({
      manifest,
      manifest_url: "https://oss.example.com/pack.json",
      source: "private",
      downloadable: true,
    })).toEqual({ manifestUrl: "https://oss.example.com/pack.json", source: "private", download: true });
    expect(entryInstallRequest({
      manifest,
      manifest_path: "packs/official/demo/pack.json",
      source: "local",
      downloadable: false,
    })).toEqual({ manifestPath: "packs/official/demo/pack.json", source: "local", download: false });
  });

  it("maps catalog entries to install, enable, update and use actions", () => {
    const manifest: PackManifest = { id: "yunque.pack.demo", name: "Demo", version: "0.1.0" };

    expect(catalogActionForEntry({
      manifest,
      package_url: "https://oss.example.com/demo.yqpack",
      downloadable: true,
      installed: false,
      enabled: false,
      update_action: "install",
    })).toEqual({ kind: "install", label: "安装", disabled: false, needsInstallSource: true });
    expect(catalogActionForEntry({
      manifest,
      installed: true,
      enabled: false,
      update_action: "enable",
      downloadable: false,
    })).toEqual({ kind: "enable", label: "启用", disabled: false, needsInstallSource: false });
    expect(catalogActionForEntry({
      manifest,
      package_url: "https://oss.example.com/demo.yqpack",
      installed: true,
      enabled: true,
      update_action: "update",
      downloadable: true,
    })).toEqual({ kind: "update", label: "更新", disabled: false, needsInstallSource: true });
    expect(catalogActionForEntry({
      manifest,
      package_url: "https://oss.example.com/demo.yqpack",
      installed: true,
      enabled: false,
      update_action: "update",
      downloadable: true,
    })).toEqual({ kind: "update", label: "更新", disabled: false, needsInstallSource: true });
  });

  it("turns install failures into clear user-facing reasons", () => {
    expect(formatPackInstallError(new Error("sha256 checksum mismatch"))).toContain("SHA256 校验不一致");
    expect(formatPackInstallError(new Error("signature verification failed"))).toContain("签名验证未通过");
    expect(formatPackInstallError(new Error("manifest schema invalid"))).toContain("能力声明不合法");
    expect(formatPackInstallError(new Error("unsupported platform"))).toContain("当前平台不支持");
    expect(formatPackInstallError(new Error("download timeout"))).toContain("下载失败");
  });

  it("explains install troubleshooting without raw backend errors", () => {
    const items = packInstallTroubleshooting();
    expect(items.map((item) => item.key)).toEqual(["download", "sha", "signature", "manifest", "platform"]);
    expect(items.find((item) => item.key === "download")?.detail).toContain("网络、源地址和访问权限");
    expect(items.find((item) => item.key === "sha")?.detail).toContain("不要继续安装");
    expect(items.find((item) => item.key === "signature")?.tone).toBe("danger");
    expect(items.find((item) => item.key === "manifest")?.detail).toContain("工坊只读检查");
  });

  it("summarizes pack usability from metadata and frontend entries", () => {
    const actionable: PackManifest = {
      id: "yunque.pack.memory",
      name: "Memory",
      version: "0.1.0",
      metadata: {
        usability: "actionable",
        primaryActionLabel: "查看和整理记忆",
        primaryActionPath: "/memory",
      },
    };
    const experimental: PackManifest = {
      id: "yunque.pack.computer-use",
      name: "Computer Use",
      version: "0.1.0",
      status: "alpha",
      metadata: {
        primaryActionLabel: "查看电脑使用计划",
        primaryActionPath: "/packs/computer-use",
        limitation: "当前只生成计划，不执行本机桌面控制。",
      },
    };
    const infrastructure: PackManifest = {
      id: "yunque.pack.state",
      name: "State",
      version: "0.1.0",
      metadata: {
        usability: "infrastructure",
        primaryActionLabel: "由任务和对话自动使用",
        primaryActionPath: "/missions",
        usageSurface: "任务中心与 Chat 任务进度",
      },
    };

    expect(packUsability(actionable)).toMatchObject({
      kind: "actionable",
      label: "可直接使用",
      primaryActionLabel: "查看和整理记忆",
      primaryActionPath: "/memory",
    });
    expect(packUsability(experimental)).toMatchObject({
      kind: "experimental",
      label: "实验能力",
      primaryActionLabel: "查看电脑使用计划",
      primaryActionPath: "/packs/computer-use",
      limitation: "当前只生成计划，不执行本机桌面控制。",
    });
    expect(packUsability(infrastructure)).toMatchObject({
      kind: "infrastructure",
      label: "后台支撑",
      primaryActionPath: "/missions",
    });
    expect(packUsageExplanation(infrastructure).join(" ")).toContain("任务中心与 Chat 任务进度");
  });

  it("separates delivery depth from basic manifest readiness", () => {
    const ready: PackManifest = {
      id: "yunque.pack.memory",
      name: "Memory",
      version: "0.1.0",
      metadata: {
        usability: "actionable",
        primaryActionPath: "/memory",
        usageSurface: "记忆页和 Chat 个性化上下文",
        example1: "查看并整理长期偏好。",
      },
      backend: { capabilities: ["memory.recall"] },
    };
    const support: PackManifest = {
      id: "yunque.pack.state",
      name: "State",
      version: "0.1.0",
      metadata: {
        usability: "infrastructure",
        primaryActionPath: "/missions",
        usageSurface: "任务中心与 Chat 任务进度",
        example1: "为任务中心提供结构化状态。",
      },
      backend: { capabilities: ["state.read"] },
    };
    const planOnly: PackManifest = {
      id: "yunque.pack.computer-use",
      name: "Computer Use",
      version: "0.1.0",
      status: "alpha",
      metadata: {
        primaryActionPath: "/packs/computer-use",
        usageSurface: "电脑使用页和 Chat 电脑使用计划",
        example1: "把自然语言目标转成需审批的电脑使用计划。",
        limitation: "当前只生成电脑使用计划，不执行本机桌面控制。",
      },
      backend: { capabilities: ["computer.use.plan"] },
    };
    const needsMeat: PackManifest = {
      id: "yunque.pack.unclear",
      name: "Unclear",
      version: "0.1.0",
      metadata: {},
    };

    expect(packDeliveryProfile(ready)).toMatchObject({ level: "ready", label: "可直接交付" });
    expect(packDeliveryProfile(support)).toMatchObject({ level: "support", label: "后台支撑" });
    expect(packDeliveryProfile(planOnly)).toMatchObject({ level: "plan_only", label: "实验/计划" });
    expect(packDeliveryProfile(needsMeat)).toMatchObject({ level: "needs_meat", label: "需打磨" });
    expect(packDeliveryProfile(needsMeat).nextStep).toContain("交给小羽");
  });

  it("summarizes whether pack cards have enough user-facing context", () => {
    const complete: PackManifest = {
      id: "yunque.pack.complete",
      name: "Complete",
      version: "0.1.0",
      metadata: {
        primaryActionPath: "/packs/complete",
        usageSurface: "能力包中心和 Chat 产物区",
        example1: "打开能力界面查看结果。",
      },
      backend: { capabilities: ["complete.run"] },
    };
    const missingContext: PackManifest = {
      id: "yunque.pack.context",
      name: "Context",
      version: "0.1.0",
      metadata: {
        primaryActionPath: "/packs/context",
        example1: "运行一次检查。",
      },
      backend: { capabilities: ["context.run"] },
    };
    const missingEntry: PackManifest = {
      id: "yunque.pack.entry",
      name: "Entry",
      version: "0.1.0",
      metadata: {
        usageSurface: "后台自动使用",
        example1: "由云雀自动调度。",
      },
    };

    expect(packReadiness(complete)).toMatchObject({ level: "complete", label: "说明完整", missing: [] });
    expect(packReadiness(missingContext)).toMatchObject({ level: "needs_context", missing: ["用户感知位置"] });
    expect(packReadiness(missingEntry)).toMatchObject({
      level: "needs_entry",
      missing: ["打开/使用入口", "后端能力声明"],
    });
  });

  it("builds user-facing verification steps for actionable, support and plan-only packs", () => {
    const actionable: PackManifest = {
      id: "yunque.pack.memory",
      name: "Memory",
      version: "0.1.0",
      metadata: {
        usability: "actionable",
        primaryActionLabel: "查看和整理记忆",
        primaryActionPath: "/memory",
        usageSurface: "记忆页和 Chat 个性化上下文",
        example1: "查看并整理长期偏好。",
      },
      backend: { capabilities: ["memory.recall"] },
    };
    const support: PackManifest = {
      id: "yunque.pack.state",
      name: "State",
      version: "0.1.0",
      metadata: {
        usability: "infrastructure",
        usageSurface: "任务中心与 Chat 任务进度",
        example1: "发起一个任务并观察状态变化。",
      },
      backend: { capabilities: ["state.read"] },
    };
    const planOnly: PackManifest = {
      id: "yunque.pack.computer-use",
      name: "Computer Use",
      version: "0.1.0",
      status: "alpha",
      metadata: {
        primaryActionPath: "/packs/computer-use",
        usageSurface: "电脑使用页和 Chat 电脑使用计划",
        example1: "把目标转成需审批的电脑使用计划。",
        limitation: "当前只生成电脑使用计划，不执行本机桌面控制。",
      },
      backend: { capabilities: ["computer.use.plan"] },
    };

    expect(packVerificationSteps(actionable)[0]).toMatchObject({
      label: "先触发一次",
      href: "/memory",
    });
    expect(packVerificationSteps(actionable)[1].detail).toContain("记忆页和 Chat 个性化上下文");
    expect(packVerificationSteps(support)[0].detail).toContain("通常不需要单独打开");
    expect(packVerificationSteps(support)[1].detail).toContain("任务中心与 Chat 任务进度");
    expect(packVerificationSteps(planOnly)[1].detail).toContain("实验/计划能力");
    expect(packVerificationSteps(planOnly)[2].detail).toContain("当前只生成电脑使用计划");
  });

  it("builds explicit boundary summaries for high-risk and sandboxed packs", () => {
    const computerUse: PackManifest = {
      id: "yunque.pack.computer-use",
      name: "Computer Use",
      version: "0.1.0",
      status: "alpha",
      metadata: {
        primaryActionPath: "/packs/computer-use",
        usageSurface: "电脑使用页和 Chat 电脑使用计划",
        example1: "把目标转成需审批的电脑使用计划。",
        limitation: "当前只生成电脑使用计划，不执行本机桌面控制。",
      },
      backend: {
        permissions: ["computer:control", "browser:read", "sandbox:desktop"],
        capabilities: ["computer.intent.plan"],
      },
      update: { rollback: true },
    };
    const wasm: PackManifest = {
      id: "yunque.pack.wasm-plugin",
      name: "WASM Plugin",
      version: "0.1.0",
      status: "alpha",
      metadata: {
        primaryActionPath: "/packs/wasm-plugin",
        usageSurface: "WASM 插件页、远程安装审批与证据导出",
        example1: "生成远程安装审批计划。",
        limitation: "当前远程安装和真实运行时 host 仍受严格门禁。",
      },
      backend: {
        permissions: ["wasm:execute", "wasm:write"],
        capabilities: ["wasm.remote_install.plan"],
      },
    };
    const dlc: PackManifest = {
      id: "yunque.pack.dlc-demo",
      name: "DLC Demo",
      version: "0.1.0",
      status: "alpha",
      metadata: {
        primaryActionPath: "/packs/dlc-demo",
        usageSurface: "DLC 演示页和 iframe 沙箱界面",
        example1: "通过 postMessage 桥调用白名单路由。",
      },
      frontend: {
        routes: [{ path: "/packs/dlc-demo", component: "PackDlcHost" }],
        assets: { type: "iframe-bundle", entry: "index.html" },
      },
      backend: { permissions: ["dlc:demo"], capabilities: ["dlc.demo.ping"] },
    };

    expect(packBoundarySummary(computerUse).find((item) => item.key === "doesNot")?.detail).toContain("不会执行本机桌面控制");
    expect(packBoundarySummary(computerUse).find((item) => item.key === "rollback")?.detail).toContain("回滚");
    expect(packBoundarySummary(wasm).find((item) => item.key === "doesNot")?.detail).toContain("不会跳过审批直接写入或运行未知模块");
    expect(packBoundarySummary(dlc).find((item) => item.key === "doesNot")?.detail).toContain("拿不到云雀 token");
  });
});
