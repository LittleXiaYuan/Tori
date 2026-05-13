"use client";

import type React from "react";
import { HardDriveDownload, Package, Puzzle } from "lucide-react";
import { api, type InstalledPack, type PackFrontendMenu } from "@/lib/api";

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

export function resolvePackIcon(name?: string): React.ReactNode {
  if (!name) return <Package size={16} />;
  return packIconMap[name.toLowerCase()] || <Package size={16} />;
}

export async function fetchEnabledPacks(): Promise<InstalledPack[]> {
  const res = await api.packsEnabled();
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

