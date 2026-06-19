import { describe, expect, it } from "vitest";
import type { PackManifest } from "yunque-client/packs";
import {
  capabilitySurfaceLabels,
  catalogActionForEntry,
  entryInstallRequest,
  formatPackInstallError,
  groupPackPermissions,
  packFeatureFlags,
  packUsageExplanation,
  packUsability,
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
    expect(formatPackInstallError(new Error("manifest schema invalid"))).toContain("manifest 不合法");
    expect(formatPackInstallError(new Error("unsupported platform"))).toContain("当前平台不支持");
    expect(formatPackInstallError(new Error("download timeout"))).toContain("下载失败");
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
      limitation: "当前只生成计划，不执行本机桌面控制。",
    });
    expect(packUsability(infrastructure)).toMatchObject({
      kind: "infrastructure",
      label: "后台支撑",
      primaryActionPath: "/missions",
    });
    expect(packUsageExplanation(infrastructure).join(" ")).toContain("任务中心与 Chat 任务进度");
  });
});
