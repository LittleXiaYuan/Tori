"use client";

import type React from "react";
import { BrainCircuit, CircuitBoard, HardDriveDownload, Package, Puzzle } from "lucide-react";
import { createPacksClient } from "yunque-client/packs";
import type { InstalledPack, PackBackendRouteSpec, PackDistributionManifest, PackFrontendAssets, PackFrontendMenu } from "yunque-client/packs";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { packSafeOpenPath } from "@/lib/pack-presentation";


export interface PackSdkEntrypoint {
  packId: string;
  packName: string;
  language: string;
  importPath: string;
}

export interface PackRouteBinding {
  pack: InstalledPack;
  packId: string;
  packName: string;
  path: string;
  component: string;
  title?: string;
  assets?: PackFrontendAssets;
  distribution?: PackDistributionManifest;
  sdk: PackSdkEntrypoint[];
}

export interface PackBackendRouteBinding extends PackBackendRouteSpec {
  pack: InstalledPack;
  packId: string;
  packName: string;
}

export interface PackNavItem {
  href: string;
  label: string;
  labelEn: string;
  icon: React.ReactNode;
  packId: string;
  packName: string;
  order: number;
  keywords: string;
}

const packIconMap: Record<string, React.ReactNode> = {
  backup: <HardDriveDownload size={16} />,
  brain: <BrainCircuit size={16} />,
  "brain-circuit": <BrainCircuit size={16} />,
  cognis: <BrainCircuit size={16} />,
  "cogni-kernel": <BrainCircuit size={16} />,
  "circuit-board": <CircuitBoard size={16} />,
  circuit: <CircuitBoard size={16} />,
  lora: <CircuitBoard size={16} />,
  package: <Package size={16} />,
  plugin: <Puzzle size={16} />,
  puzzle: <Puzzle size={16} />,
};

const packsClient = createPacksClient(createYunqueSDKClientOptions());

export function resolvePackIcon(name?: string): React.ReactNode {
  if (!name) return <Package size={16} />;
  return packIconMap[name.toLowerCase()] || <Package size={16} />;
}

export async function fetchEnabledPacks(): Promise<InstalledPack[]> {
  const res = await packsClient.enabled();
  return Array.isArray(res?.packs) ? res.packs : [];
}

/** Thin-shell introspection packs whose underlying subsystems always run; they
 *  are surfaced via the static NAV_ITEMS catalog (group 扩展) instead of being
 *  re-emitted here, so enabling them never double-lists every surface. */
const CORE_PROMOTED_PACK_IDS = new Set<string>([
  "yunque.pack.inner-life",
  "yunque.pack.night-school",
  "yunque.pack.experience",
  "yunque.pack.world-model",
  "yunque.pack.micro-agent",
]);

/** Packs whose surfaces are rendered via the static NAV_ITEMS catalog (gated by
 *  filterNavItemsByEnabledPacks), so their manifest menus must NOT also be
 *  emitted here — otherwise enabling the pack would double-list every surface. */
const STATIC_NAV_PACK_IDS = new Set<string>([
  "yunque.pack.control-plane",
  "yunque.pack.work",
  "yunque.pack.skills",
  "yunque.pack.mcp-dispatch",
  // Default-preloaded packs: rendered via static core nav; their manifest
  // menus must not double-list when enabled.
  "yunque.pack.memory",
  "yunque.pack.knowledge",
  "yunque.pack.cogni-console",
  "yunque.pack.workspace",
]);

export function buildPackNavItems(packs: InstalledPack[]): PackNavItem[] {
  return packs
    .filter((pack) => !CORE_PROMOTED_PACK_IDS.has(pack.manifest.id) && !STATIC_NAV_PACK_IDS.has(pack.manifest.id))
    .flatMap((pack) => {
      const manifest = pack.manifest;
      const menus = manifest.frontend?.menus || [];
      return menus
        .map((menu: PackFrontendMenu) => {
          const href = packSafeOpenPath(manifest, menu.path);
          if (!href) return null;
          return {
            href,
            label: menu.label,
            labelEn: menu.label,
            icon: resolvePackIcon(menu.icon),
            packId: manifest.id,
            packName: manifest.name,
            order: menu.order ?? 999,
            keywords: `${manifest.id} ${manifest.name} ${manifest.description || ""} ${menu.key} ${menu.label} pack 能力包`,
          };
        })
        .filter((item): item is PackNavItem => Boolean(item));
    })
    .sort((a, b) => a.order - b.order || a.label.localeCompare(b.label));
}



export function normalizePackRoutePath(path: string): string {
  const trimmed = path.trim().replace(/\/+$/, "");
  return trimmed || "/";
}

export function buildPackSdkEntrypoints(pack: InstalledPack): PackSdkEntrypoint[] {
  return Object.entries(pack.manifest.sdk || {})
    .filter((entry): entry is [string, string] => typeof entry[1] === "string" && entry[1].trim().length > 0)
    .map(([language, importPath]) => ({
      packId: pack.manifest.id,
      packName: pack.manifest.name,
      language,
      importPath,
    }));
}

export function buildPackBackendRouteBindings(packs: InstalledPack[]): PackBackendRouteBinding[] {
  return packs.flatMap((pack) => (pack.manifest.backend?.routeSpecs || []).map((route) => ({
    ...route,
    pack,
    packId: pack.manifest.id,
    packName: pack.manifest.name,
  })));
}

export function formatBackendRouteSpec(route: string | PackBackendRouteSpec): string {
  if (typeof route === "string") return route;
  return `${route.method} ${route.path}`;
}

export function buildPackRouteBindings(packs: InstalledPack[]): PackRouteBinding[] {
  return packs.flatMap((pack) => {
    const sdk = buildPackSdkEntrypoints(pack);
    return (pack.manifest.frontend?.routes || []).map((route) => ({
      pack,
      packId: pack.manifest.id,
      packName: pack.manifest.name,
      path: route.path,
      component: route.component,
      title: route.title,
      assets: pack.manifest.frontend?.assets,
      distribution: pack.manifest.distribution,
      sdk,
    }));
  });
}

export function findPackRouteBinding(packs: InstalledPack[], pathname: string): PackRouteBinding | undefined {
  const current = normalizePackRoutePath(pathname);
  return buildPackRouteBindings(packs).find((route) => normalizePackRoutePath(route.path) === current);
}


export function packSdkImportSnippet(language: string, importPath: string): string {
  if (language === "typescript") return `import * as packSdk from "${importPath}";`;
  return `${language}:${importPath}`;
}
