"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState, useMemo, useCallback } from "react";
import { motion, AnimatePresence } from "motion/react";
import { useI18n } from "@/lib/i18n";
import { api, type PluginUITab } from "@/lib/api";
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
  Clock,
  Shield,
  HardDriveDownload,
  BrainCircuit,
  ChevronDown,
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
  LogOut,
  PanelLeftClose,
  PanelLeftOpen,
  type LucideIcon,
} from "lucide-react";

const iconMap: Record<string, LucideIcon> = {
  Gauge, Terminal, ScanFace, MailWarning, Blocks, Package,
  BarChart3, Settings, BookOpen, Clock, Shield, HardDriveDownload,
  BrainCircuit, ListTodo, Lightbulb, Layers, FileDown, Puzzle, Palette, GitBranch,
};

interface NavItem {
  href: string;
  key: string;
  icon: LucideIcon;
  _label?: string;
}

interface NavGroup {
  key: string;
  icon: LucideIcon;
  items: NavItem[];
}

const navGroups: NavGroup[] = [
  {
    key: "nav.group.workbench",
    icon: Gauge,
    items: [
      { href: "/", key: "nav.dashboard", icon: Gauge },
      { href: "/chat", key: "nav.chat", icon: Terminal },
      { href: "/inbox", key: "nav.inbox", icon: MailWarning },
    ],
  },
  {
    key: "nav.group.tasks",
    icon: ListTodo,
    items: [
      { href: "/tasks", key: "nav.tasks", icon: ListTodo },
      { href: "/automation", key: "nav.automation", icon: Zap },
      { href: "/workflows", key: "nav.workflows", icon: GitBranch },
      { href: "/templates", key: "nav.templates", icon: Layers },
      { href: "/cron", key: "nav.cron", icon: Clock },
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
      { href: "/metrics", key: "nav.metrics", icon: BarChart3 },
      { href: "/browser", key: "nav.browser", icon: Globe },
      { href: "/tenants", key: "nav.tenants", icon: Blocks },
      { href: "/backup", key: "nav.backup", icon: HardDriveDownload },
      { href: "/settings/theme", key: "nav.theme", icon: Palette },
      { href: "/settings", key: "nav.settings", icon: Settings },
    ],
  },
];

const SIDEBAR_EXPANDED = 240;
const SIDEBAR_COLLAPSED = 64;

export function Sidebar() {
  const pathname = usePathname();
  const { locale, setLocale, t } = useI18n();
  const [collapsed, setCollapsed] = useState(false);
  const [openGroups, setOpenGroups] = useState<Set<string>>(() => new Set());
  const [pluginTabs, setPluginTabs] = useState<PluginUITab[]>([]);

  useEffect(() => {
    api.pluginUITabs()
      .then((res) => setPluginTabs(res.tabs || []))
      .catch(() => {});
  }, []);

  const groups = useMemo(() => {
    const result = navGroups.map((g) => ({ ...g, items: [...g.items] }));
    const skillGroup = result.find((g) => g.key === "nav.group.skills");
    if (skillGroup && pluginTabs.length > 0) {
      for (const tab of pluginTabs) {
        const href = `/ext/${tab.key}`;
        if (skillGroup.items.some((i) => i.href === href)) continue;
        const icon = iconMap[tab.icon] || Puzzle;
        skillGroup.items.push({
          href,
          key: `plugin.${tab.key}`,
          icon,
          _label: locale === "en" ? (tab.label_en || tab.label) : tab.label,
        });
      }
    }
    return result;
  }, [pluginTabs, locale]);

  useEffect(() => {
    const active = groups.find((g) => g.items.some((i) => i.href === pathname));
    if (active) {
      setOpenGroups((prev) => {
        const next = new Set(prev);
        next.add(active.key);
        return next;
      });
    }
  }, [pathname, groups]);

  const toggleGroup = useCallback((key: string) => {
    setOpenGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }, []);

  const handleLogout = useCallback(() => {
    document.cookie = "yunque_token=; path=/; max-age=0";
    localStorage.removeItem("yunque_token");
    window.location.href = "/login";
  }, []);

  const width = collapsed ? SIDEBAR_COLLAPSED : SIDEBAR_EXPANDED;

  return (
    <>
      {/* Spacer to push content */}
      <div style={{ width, flexShrink: 0, transition: "width 0.3s cubic-bezier(0.4, 0, 0.2, 1)" }} />

      {/* Fixed sidebar */}
      <motion.aside
        className="sidebar-root"
        animate={{ width }}
        transition={{ type: "spring", stiffness: 300, damping: 30 }}
        style={{
          position: "fixed",
          left: 0,
          top: 0,
          height: "100vh",
          zIndex: 50,
          display: "flex",
          flexDirection: "column",
          background: "var(--sidebar-bg, rgba(10, 10, 15, 0.95))",
          borderRight: "1px solid var(--border)",
          overflow: "hidden",
        }}
      >
        {/* Logo */}
        <div style={{
          padding: collapsed ? "20px 0" : "20px 16px",
          display: "flex",
          alignItems: "center",
          gap: 10,
          justifyContent: collapsed ? "center" : "flex-start",
          borderBottom: "1px solid var(--border)",
          minHeight: 64,
          flexShrink: 0,
        }}>
          <Link href="/" style={{ display: "flex", alignItems: "center", gap: 10, textDecoration: "none" }}>
            <div style={{
              width: 32,
              height: 32,
              borderRadius: 10,
              background: "var(--accent)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              color: "#fff",
              fontWeight: 700,
              fontSize: 14,
              flexShrink: 0,
            }}>
              Y
            </div>
            <AnimatePresence>
              {!collapsed && (
                <motion.span
                  initial={{ opacity: 0, width: 0 }}
                  animate={{ opacity: 1, width: "auto" }}
                  exit={{ opacity: 0, width: 0 }}
                  transition={{ duration: 0.2 }}
                  style={{
                    fontWeight: 600,
                    fontSize: 15,
                    letterSpacing: "-0.02em",
                    color: "var(--text)",
                    whiteSpace: "nowrap",
                    overflow: "hidden",
                  }}
                >
                  Yunque Agent
                </motion.span>
              )}
            </AnimatePresence>
          </Link>
        </div>

        {/* Navigation */}
        <nav className="sidebar-nav" style={{
          flex: 1,
          overflowY: "auto",
          overflowX: "hidden",
          padding: collapsed ? "8px 6px" : "8px 10px",
        }}>
          {groups.map((group) => {
            const GroupIcon = group.icon;
            const isOpen = openGroups.has(group.key);
            const hasActive = group.items.some((i) => i.href === pathname);

            return (
              <div key={group.key} style={{ marginBottom: 2 }}>
                <button
                  onClick={() => collapsed ? null : toggleGroup(group.key)}
                  title={collapsed ? t(group.key) : undefined}
                  className="sidebar-group-btn"
                  style={{
                    width: "100%",
                    display: "flex",
                    alignItems: "center",
                    gap: 10,
                    padding: collapsed ? "8px 0" : "8px 10px",
                    justifyContent: collapsed ? "center" : "flex-start",
                    borderRadius: 8,
                    border: "none",
                    background: hasActive ? "var(--bg-hover)" : "transparent",
                    color: hasActive ? "var(--text)" : "var(--text-muted)",
                    cursor: "pointer",
                    fontSize: 11,
                    fontWeight: 600,
                    letterSpacing: "0.05em",
                    textTransform: "uppercase",
                    transition: "all 0.15s ease",
                  }}
                >
                  <GroupIcon size={15} style={{ flexShrink: 0 }} />
                  {!collapsed && (
                    <>
                      <span style={{ flex: 1, textAlign: "left" }}>{t(group.key)}</span>
                      <motion.div
                        animate={{ rotate: isOpen ? 180 : 0 }}
                        transition={{ duration: 0.2 }}
                      >
                        <ChevronDown size={11} />
                      </motion.div>
                    </>
                  )}
                </button>

                <AnimatePresence initial={false}>
                  {(collapsed || isOpen) && (
                    <motion.div
                      initial={collapsed ? false : { height: 0, opacity: 0 }}
                      animate={{ height: "auto", opacity: 1 }}
                      exit={{ height: 0, opacity: 0 }}
                      transition={{ duration: 0.2, ease: [0.4, 0, 0.2, 1] }}
                      style={{ overflow: "hidden" }}
                    >
                      {group.items.map((item) => {
                        const Icon = item.icon;
                        const active = pathname === item.href;
                        const label = item._label || t(item.key);

                        return (
                          <Link
                            key={item.href}
                            href={item.href}
                            title={collapsed ? label : undefined}
                            className="sidebar-item"
                            style={{
                              display: "flex",
                              alignItems: "center",
                              gap: 10,
                              padding: collapsed ? "7px 0" : "7px 10px 7px 28px",
                              justifyContent: collapsed ? "center" : "flex-start",
                              borderRadius: 8,
                              fontSize: 13,
                              fontWeight: active ? 500 : 400,
                              color: active ? "var(--text)" : "var(--text-muted)",
                              background: active ? "var(--accent-subtle)" : "transparent",
                              textDecoration: "none",
                              transition: "all 0.15s ease",
                              position: "relative",
                            }}
                          >
                            {active && (
                              <motion.div
                                layoutId="sidebar-active-indicator"
                                style={{
                                  position: "absolute",
                                  left: collapsed ? "50%" : 10,
                                  top: "50%",
                                  transform: collapsed ? "translate(-50%, -50%)" : "translateY(-50%)",
                                  width: collapsed ? 4 : 3,
                                  height: 16,
                                  borderRadius: 2,
                                  background: "var(--accent)",
                                }}
                                transition={{ type: "spring", stiffness: 400, damping: 30 }}
                              />
                            )}
                            <Icon size={15} style={{ flexShrink: 0 }} />
                            {!collapsed && (
                              <span style={{ whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                                {label}
                              </span>
                            )}
                          </Link>
                        );
                      })}
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>
            );
          })}
        </nav>

        {/* Footer */}
        <div style={{
          borderTop: "1px solid var(--border)",
          padding: collapsed ? "8px 6px" : "8px 10px",
          flexShrink: 0,
          display: "flex",
          flexDirection: "column",
          gap: 1,
        }}>
          <SidebarButton
            icon={Globe}
            label={locale === "zh" ? "English" : "中文"}
            collapsed={collapsed}
            title={locale === "zh" ? "Switch to English" : "切换到中文"}
            onClick={() => setLocale(locale === "zh" ? "en" : "zh")}
          />
          <SidebarButton
            icon={LogOut}
            label={locale === "zh" ? "退出登录" : "Logout"}
            collapsed={collapsed}
            onClick={handleLogout}
          />
          <SidebarButton
            icon={collapsed ? PanelLeftOpen : PanelLeftClose}
            label={collapsed ? "展开" : "收起"}
            collapsed={collapsed}
            title={collapsed ? "展开侧边栏" : "收起侧边栏"}
            onClick={() => setCollapsed(!collapsed)}
          />
        </div>
      </motion.aside>
    </>
  );
}

function SidebarButton({
  icon: Icon,
  label,
  collapsed,
  title,
  onClick,
}: {
  icon: LucideIcon;
  label: string;
  collapsed: boolean;
  title?: string;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      title={title || (collapsed ? label : undefined)}
      className="sidebar-item"
      style={{
        display: "flex",
        alignItems: "center",
        gap: 10,
        padding: collapsed ? "7px 0" : "7px 10px",
        justifyContent: collapsed ? "center" : "flex-start",
        borderRadius: 8,
        border: "none",
        background: "transparent",
        color: "var(--text-muted)",
        cursor: "pointer",
        fontSize: 13,
        width: "100%",
        transition: "all 0.15s ease",
      }}
    >
      <Icon size={15} style={{ flexShrink: 0 }} />
      {!collapsed && <span>{label}</span>}
    </button>
  );
}
