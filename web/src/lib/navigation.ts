/**
 * Shared navigation configuration for sidebar and top-nav.
 *
 * Single source of truth — both navigation components import from here
 * to avoid duplicated route definitions and icon mappings.
 */

import {
  Gauge,
  Terminal,
  ScanFace,
  MailWarning,
  Blocks,
  Package,
  BarChart3,
  Settings,
  BookOpen,
  Shield,
  HardDriveDownload,
  BrainCircuit,
  ListTodo,
  Lightbulb,
  Layers,
  FileDown,
  Puzzle,
  Wrench,
  Zap,
  Brain,
  Server,
  Palette,
  Globe,
  GitBranch,
  ShieldCheck,
  MessageCircle,
  Cpu,
  type LucideIcon,
} from "lucide-react";

export interface NavItem {
  href: string;
  key: string;
  icon: LucideIcon;
  _label?: string;
}

export interface NavGroup {
  key: string;
  icon: LucideIcon;
  items: NavItem[];
}

/** Icon lookup for plugin-provided tabs */
export const iconMap: Record<string, LucideIcon> = {
  Gauge, Terminal, ScanFace, MailWarning, Blocks, Package,
  BarChart3, Settings, BookOpen, Shield, HardDriveDownload,
  BrainCircuit, ListTodo, Lightbulb, Layers, FileDown, Puzzle, Palette, GitBranch,
};

// ── Normal mode: concise navigation for end users (like GPT/Claude/Feishu) ──
export const normalNavGroups: NavGroup[] = [
  {
    key: "nav.group.workbench",
    icon: Gauge,
    items: [
      { href: "/chat", key: "nav.chat", icon: MessageCircle },
      { href: "/missions", key: "nav.missions", icon: Zap },
      { href: "/knowledge", key: "nav.knowledge", icon: BookOpen },
      { href: "/inbox", key: "nav.inbox", icon: MailWarning },
    ],
  },
  {
    key: "nav.group.system",
    icon: Settings,
    items: [
      { href: "/persona", key: "nav.persona", icon: ScanFace },
      { href: "/skills", key: "nav.skills", icon: Package },
      { href: "/plugins", key: "nav.plugins", icon: Puzzle },
      { href: "/settings/providers", key: "nav.providers", icon: Cpu },
      { href: "/settings", key: "nav.settings", icon: Settings },
    ],
  },
];

// ── Dev mode: full navigation for developers and power users ──
export const devNavGroups: NavGroup[] = [
  {
    key: "nav.group.workbench",
    icon: Gauge,
    items: [
      { href: "/dashboard", key: "nav.dashboard", icon: Gauge },
      { href: "/chat", key: "nav.chat", icon: MessageCircle },
      { href: "/inbox", key: "nav.inbox", icon: MailWarning },
    ],
  },
  {
    key: "nav.group.missions",
    icon: Zap,
    items: [
      { href: "/missions", key: "nav.missions", icon: Zap },
    ],
  },
  {
    key: "nav.group.skills",
    icon: Wrench,
    items: [
      { href: "/skills", key: "nav.skills", icon: Package },
      { href: "/plugins", key: "nav.plugins", icon: Puzzle },
      { href: "/skill-policy", key: "nav.skillPolicy", icon: Shield },
      { href: "/skill-analytics", key: "nav.skillAnalytics", icon: BarChart3 },
      { href: "/docgen", key: "nav.docgen", icon: FileDown },
    ],
  },
  {
    key: "nav.group.cognition",
    icon: Brain,
    items: [
      { href: "/memory", key: "nav.memory", icon: Brain },
      { href: "/knowledge", key: "nav.knowledge", icon: BookOpen },
      { href: "/reverie", key: "nav.reverie", icon: BrainCircuit },
      { href: "/reflect", key: "nav.reflect", icon: Lightbulb },
      { href: "/persona", key: "nav.persona", icon: ScanFace },
    ],
  },
  {
    key: "nav.group.system",
    icon: Server,
    items: [
      { href: "/audit", key: "nav.audit", icon: Shield },
      { href: "/trust", key: "nav.trust", icon: ShieldCheck },
      { href: "/metrics", key: "nav.metrics", icon: BarChart3 },
      { href: "/browser", key: "nav.browser", icon: Globe },
      { href: "/tenants", key: "nav.tenants", icon: Blocks },
      { href: "/backup", key: "nav.backup", icon: HardDriveDownload },
      { href: "/settings/theme", key: "nav.theme", icon: Palette },
      { href: "/settings/providers", key: "nav.providers", icon: Cpu },
      { href: "/settings", key: "nav.settings", icon: Settings },
    ],
  },
];

/** Flatten all items for lookup (union of both modes) */
export const allNavItems = [...normalNavGroups, ...devNavGroups]
  .flatMap((g) => g.items)
  .filter((item, idx, arr) => arr.findIndex((i) => i.href === item.href) === idx);
