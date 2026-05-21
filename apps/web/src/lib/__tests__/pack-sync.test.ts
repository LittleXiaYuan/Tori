import { describe, expect, it, vi, afterEach } from "vitest";
import {
  buildPackBackendRouteBindings,
  buildPackNavItems,
  buildPackRouteBindings,
  buildPackSdkEntrypoints,
  fetchEnabledPacks,
  findPackRouteBinding,
  formatBackendRouteSpec,
  normalizePackRoutePath,
  packSdkImportSnippet,
} from "../pack-sync";
import type { InstalledPack } from "../pack-types";

const backupPack: InstalledPack = {
  status: "enabled",
  source: "packs/examples/backup-pack",
  manifest: {
    id: "yunque.pack.backup",
    name: "Backup Pack",
    version: "0.1.0",
    optional: true,
    backend: { routes: ["/v1/backup/info"], routeSpecs: [{ method: "GET", path: "/v1/backup/info", description: "Read backup status" }] },
    frontend: {
      menus: [{ key: "backup", label: "备份恢复", path: "/packs/backup", icon: "backup", order: 20 }],
      routes: [{ path: "/packs/backup", component: "backup/BackupPage", title: "备份恢复" }],
      assets: { type: "builtin", entry: "backup/BackupPage" },
    },
    sdk: { typescript: "yunque-client/backup" },
    distribution: {
      packageUrl: "https://packs.yunque.local/backup/backup-pack-0.1.0.tgz",
      frontendUrl: "https://packs.yunque.local/backup/frontend/remoteEntry.js",
      sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    },
  },
};

const laterPack: InstalledPack = {
  status: "enabled",
  source: "packs/examples/later-pack",
  manifest: {
    id: "yunque.pack.later",
    name: "Later Pack",
    version: "0.1.0",
    optional: true,
    backend: { routes: ["/v1/later/info"], routeSpecs: [{ method: "GET", path: "/v1/later/info" }] },
    frontend: {
      menus: [{ key: "later", label: "后置包", path: "/packs/later", icon: "package", order: 90 }],
      routes: [{ path: "/packs/later", component: "later/LaterPage" }],
    },
    sdk: {},
  },
};

const loraPack: InstalledPack = {
  status: "enabled",
  source: "packs/examples/lora-pack",
  manifest: {
    id: "yunque.pack.lora",
    name: "LoRA / LAA Evolution Pack",
    version: "0.1.0",
    optional: true,
    backend: {
      routes: ["/v1/lora/config"],
      routeSpecs: [
        { method: "GET", path: "/v1/lora/config" },
        { method: "PATCH", path: "/v1/lora/config" },
      ],
    },
    frontend: {
      menus: [{ key: "lora", label: "LoRA 训练", path: "/packs/lora", icon: "circuit-board", order: 95 }],
      routes: [{ path: "/packs/lora", component: "lora/LoRAPackPage", title: "LoRA 训练" }],
    },
    sdk: { typescript: "yunque-client/lora" },
  },
};

const cogniKernelPack: InstalledPack = {
  status: "enabled",
  source: "packs/examples/cogni-kernel-pack",
  manifest: {
    id: "yunque.pack.cogni-kernel",
    name: "Cogni Kernel Pack",
    version: "0.1.0",
    optional: true,
    backend: {
      routes: ["/v1/cognis", "/v1/cognis/"],
      routeSpecs: [
        { method: "GET", path: "/v1/cognis" },
        { method: "POST", path: "/v1/cognis" },
        { method: "GET", path: "/v1/cognis/" },
        { method: "POST", path: "/v1/cognis/" },
        { method: "DELETE", path: "/v1/cognis/" },
      ],
    },
    frontend: {
      menus: [{ key: "cognis", label: "智体内核", path: "/packs/cognis", icon: "brain-circuit", order: 80 }],
      routes: [{ path: "/packs/cognis", component: "cognis/CogniKernelPackPage", title: "智体内核" }],
    },
    sdk: { typescript: "yunque-client/cognis" },
  },
};

afterEach(() => {
  vi.restoreAllMocks();
});

describe("pack-sync frontend runtime", () => {
  it("builds sorted nav items from enabled pack menus", () => {
    const items = buildPackNavItems([laterPack, backupPack, cogniKernelPack]);

    expect(items.map((item) => item.href)).toEqual(["/packs/backup", "/packs/cognis", "/packs/later"]);
    expect(items[0]).toMatchObject({ packId: "yunque.pack.backup", label: "备份恢复", order: 20 });
    expect(items[0]?.keywords).toContain("yunque.pack.backup");
    expect(items[1]).toMatchObject({ packId: "yunque.pack.cogni-kernel", label: "智体内核", order: 80 });
  });

  it("builds sdk entrypoints and import snippets", () => {
    const entries = buildPackSdkEntrypoints(backupPack);

    expect(entries).toEqual([{ packId: "yunque.pack.backup", packName: "Backup Pack", language: "typescript", importPath: "yunque-client/backup" }]);
    expect(packSdkImportSnippet("typescript", "yunque-client/backup")).toBe('import * as packSdk from "yunque-client/backup";');
    expect(packSdkImportSnippet("python", "yunque_client.backup")).toBe("python:yunque_client.backup");
  });

  it("builds route bindings with assets, distribution, and sdk data", () => {
    const [binding] = buildPackRouteBindings([backupPack]);

    expect(binding).toMatchObject({
      packId: "yunque.pack.backup",
      packName: "Backup Pack",
      path: "/packs/backup",
      component: "backup/BackupPage",
      title: "备份恢复",
      assets: { type: "builtin", entry: "backup/BackupPage" },
      distribution: { frontendUrl: "https://packs.yunque.local/backup/frontend/remoteEntry.js" },
    });
    expect(binding?.pack).toBe(backupPack);
    expect(binding?.sdk[0]?.importPath).toBe("yunque-client/backup");
  });


  it("builds backend route bindings from manifest routeSpecs", () => {
    const bindings = buildPackBackendRouteBindings([backupPack, loraPack]);
    const [binding] = bindings;

    expect(binding).toMatchObject({
      packId: "yunque.pack.backup",
      packName: "Backup Pack",
      method: "GET",
      path: "/v1/backup/info",
    });
    expect(binding?.pack).toBe(backupPack);
    expect(formatBackendRouteSpec(binding!)).toBe("GET /v1/backup/info");
    expect(formatBackendRouteSpec("/v1/legacy/info")).toBe("/v1/legacy/info");
    expect(bindings.map(formatBackendRouteSpec)).toContain("PATCH /v1/lora/config");
    expect(buildPackBackendRouteBindings([cogniKernelPack]).map(formatBackendRouteSpec)).toContain("DELETE /v1/cognis/");
  });

  it("normalizes and resolves route bindings by pathname", () => {
    expect(normalizePackRoutePath("/packs/backup///")).toBe("/packs/backup");
    expect(normalizePackRoutePath("///")).toBe("/");

    const binding = findPackRouteBinding([backupPack], "/packs/backup/");
    expect(binding?.component).toBe("backup/BackupPage");
    expect(findPackRouteBinding([backupPack], "/packs/missing")).toBeUndefined();
  });

  it("fetches enabled packs from the backend registry source of truth", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ packs: [backupPack], count: 1 }), { status: 200 }),
    );

    const packs = await fetchEnabledPacks();

    expect(spy).toHaveBeenCalledOnce();
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/packs/enabled");
    expect(packs).toEqual([backupPack]);
  });
});
