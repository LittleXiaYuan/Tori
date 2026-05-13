"use client";

import type React from "react";
import { HardDriveDownload, Package, Puzzle } from "lucide-react";
import { createPacksClient } from "@/lib/packs-client";
import type { InstalledPack, PackDistributionManifest, PackFrontendAssets, PackFrontendMenu } from "@/lib/api";


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
  package: <Package size={16} />,
  plugin: <Puzzle size={16} />,
  puzzle: <Puzzle size={16} />,
};

const packsClient = createPacksClient();

export function resolvePackIcon(name?: string): React.ReactNode {
  if (!name) return <Package size={16} />;
  return packIconMap[name.toLowerCase()] || <Package size={16} />;
}

export async function fetchEnabledPacks(): Promise<InstalledPack[]> {
  const res = await packsClient.enabled();
  return Array.isArray(res?.packs) ? res.packs : [];
}

export function buildPackNavItems(packs: InstalledPack[]): PackNavItem[] {
  return packs
    .flatMap((pack) => {
      const manifest = pack.manifest;
      const menus = manifest.frontend?.menus || [];
      return menus.map((menu: PackFrontendMenu) => ({
        href: menu.path,
        label: menu.label,
        labelEn: menu.label,
        icon: resolvePackIcon(menu.icon),
        packId: manifest.id,
        packName: manifest.name,
        order: menu.order ?? 999,
        keywords: `${manifest.id} ${manifest.name} ${manifest.description || ""} ${menu.key} ${menu.label} pack 增量包`,
      }));
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
