"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState, useMemo, useCallback } from "react";
import { motion, AnimatePresence } from "motion/react";
import { useI18n } from "@/lib/i18n";
import { api, type PluginUITab } from "@/lib/api";
import {
  Puzzle,
  ChevronRight,
  ChevronLeft,
  ChevronDown,
  Globe,
  Settings,
  Code2,
  type LucideIcon,
} from "lucide-react";
import {
  iconMap,
  normalNavGroups,
  devNavGroups,
  allNavItems,
  type NavItem,
  type NavGroup,
} from "@/lib/navigation";


export function TopNav() {
  const pathname = usePathname();
  const { locale, setLocale, t } = useI18n();
  const [expanded, setExpanded] = useState(false);
  const [openGroup, setOpenGroup] = useState<string | null>(null);
  const navRef = useRef<HTMLDivElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const [pluginTabs, setPluginTabs] = useState<PluginUITab[]>([]);
  const [devMode, setDevMode] = useState(() => {
    if (typeof window === "undefined") return false;
    return localStorage.getItem("yunque_dev_mode") === "1";
  });

  // Sync devMode across sidebar/top-nav via storage event
  useEffect(() => {
    const handler = () => {
      setDevMode(localStorage.getItem("yunque_dev_mode") === "1");
    };
    window.addEventListener("storage", handler);
    return () => window.removeEventListener("storage", handler);
  }, []);

  const navGroups = devMode ? devNavGroups : normalNavGroups;

  // Load plugin UI tabs once
  useEffect(() => {
    api.pluginUITabs()
      .then((res) => setPluginTabs(res.tabs || []))
      .catch(() => {});
  }, []);

  // Inject plugin tabs into the Skills group (if present in current mode)
  const groups = useMemo(() => {
    const result = navGroups.map((g: NavGroup) => ({ ...g, items: [...g.items] }));
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
  }, [pluginTabs, locale, navGroups]);

  // Find which group the current page belongs to
  const activeGroup = groups.find((g) => g.items.some((i) => i.href === pathname));
  const activeItem = allNavItems.find((i) => i.href === pathname) || allNavItems[0];
  const ActiveIcon = activeItem.icon;

  // Close dropdown on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node) &&
          navRef.current && !navRef.current.contains(e.target as Node)) {
        setOpenGroup(null);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  // Close dropdown on navigation
  useEffect(() => {
    setOpenGroup(null);
  }, [pathname]);

  const toggleGroup = useCallback((key: string) => {
    setOpenGroup((prev) => (prev === key ? null : key));
  }, []);

  return (
    <header
      className="fixed top-0 left-0 right-0 z-50 flex items-center justify-center"
      style={{ height: 64 }}
    >
      <motion.div
        initial={{ opacity: 0, y: -20, scale: 0.95 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ duration: 0.5, ease: [0.16, 1, 0.3, 1] }}
        className="mt-3"
      >
        <motion.nav
          ref={navRef}
          layout
          className="relative flex items-center gap-0.5 px-1.5 py-1.5 rounded-full border backdrop-blur-xl"
          style={{
            background: "rgba(10,10,10,0.85)",
            borderColor: "var(--border)",
          }}
          transition={{
            layout: {
              type: "spring",
              stiffness: 300,
              damping: 30,
            },
          }}
        >
          {/* Logo */}
          <Link
            href="/"
            className="flex items-center gap-1.5 px-3 py-1.5 mr-1 shrink-0"
          >
            <div
              className="w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-bold"
              style={{ background: "var(--text)", color: "var(--bg)" }}
            >
              Y
            </div>
          </Link>

          <div className="w-px h-4 mx-1" style={{ background: "var(--border)" }} />

          <AnimatePresence mode="wait">
            {expanded ? (
              <motion.div
                key="expanded"
                className="flex items-center gap-0.5"
                initial={{ opacity: 0, width: 0 }}
                animate={{ opacity: 1, width: "auto" }}
                exit={{ opacity: 0, width: 0 }}
                transition={{
                  width: { type: "spring", stiffness: 300, damping: 30 },
                  opacity: { duration: 0.2 },
                }}
                style={{ overflow: "visible" }}
              >
                {groups.map((group) => {
                  const GroupIcon = group.icon;
                  const isActiveGroup = activeGroup?.key === group.key;
                  const isOpen = openGroup === group.key;
                  return (
                    <div key={group.key} className="relative">
                      <button
                        onClick={() => toggleGroup(group.key)}
                        className="relative flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium transition-colors duration-200 cursor-pointer whitespace-nowrap z-10"
                        style={{
                          color: isActiveGroup ? "var(--text)" : "var(--text-muted)",
                          background: isOpen ? "var(--bg-hover)" : "transparent",
                        }}
                      >
                        <GroupIcon size={13} />
                        {t(group.key)}
                        <ChevronDown
                          size={10}
                          style={{
                            transform: isOpen ? "rotate(180deg)" : "rotate(0deg)",
                            transition: "transform 0.2s",
                          }}
                        />
                      </button>
                      <AnimatePresence>
                        {isOpen && (
                          <motion.div
                            ref={dropdownRef}
                            initial={{ opacity: 0, y: -4, scale: 0.95 }}
                            animate={{ opacity: 1, y: 0, scale: 1 }}
                            exit={{ opacity: 0, y: -4, scale: 0.95 }}
                            transition={{ duration: 0.15 }}
                            className="absolute top-full left-0 mt-2 py-1 rounded-xl border backdrop-blur-xl min-w-[160px]"
                            style={{
                              background: "rgba(10,10,10,0.95)",
                              borderColor: "var(--border)",
                            }}
                          >
                            {group.items.map((item) => {
                              const Icon = item.icon;
                              const active = pathname === item.href;
                              const label = item._label || t(item.key);
                              return (
                                <Link
                                  key={item.href}
                                  href={item.href}
                                  className="flex items-center gap-2 px-3 py-2 text-xs font-medium transition-colors duration-150 cursor-pointer"
                                  style={{
                                    color: active ? "var(--text)" : "var(--text-muted)",
                                    background: active ? "var(--bg-hover)" : "transparent",
                                  }}
                                  onMouseEnter={(e) => {
                                    if (!active) (e.currentTarget.style.background = "var(--bg-hover)");
                                  }}
                                  onMouseLeave={(e) => {
                                    if (!active) (e.currentTarget.style.background = "transparent");
                                  }}
                                >
                                  <Icon size={13} />
                                  {label}
                                </Link>
                              );
                            })}
                          </motion.div>
                        )}
                      </AnimatePresence>
                    </div>
                  );
                })}
              </motion.div>
            ) : (
              <motion.div
                key="collapsed"
                className="flex items-center gap-0.5"
                initial={{ opacity: 0, width: 0 }}
                animate={{ opacity: 1, width: "auto" }}
                exit={{ opacity: 0, width: 0 }}
                transition={{
                  width: { type: "spring", stiffness: 300, damping: 30 },
                  opacity: { duration: 0.2 },
                }}
                style={{ overflow: "hidden" }}
              >
                <Link
                  href={activeItem.href}
                  className="relative flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium whitespace-nowrap z-10"
                  style={{ color: "var(--text)" }}
                >
                  <ActiveIcon size={13} />
                  {(activeItem as any)._label || t(activeItem.key)}
                </Link>
              </motion.div>
            )}
          </AnimatePresence>

          <div className="w-px h-4 mx-1" style={{ background: "var(--border)" }} />

          {expanded && (
            <Link
              href="/settings"
              className="relative flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs transition-colors duration-200 cursor-pointer z-10"
              style={{
                color: pathname === "/settings" ? "var(--text)" : "var(--text-muted)",
              }}
            >
              <Settings size={13} />
            </Link>
          )}

          <button
            onClick={() => setLocale(locale === "zh" ? "en" : "zh")}
            className="relative flex items-center px-2.5 py-1.5 rounded-full text-xs font-medium transition-colors duration-200 cursor-pointer z-10"
            style={{ color: "var(--text-muted)" }}
            title={locale === "zh" ? "Switch to English" : "切换到中文"}
          >
            {locale === "zh" ? "EN" : "中"}
          </button>

          {/* Dev mode toggle */}
          <button
            onClick={() => {
              const next = !devMode;
              localStorage.setItem("yunque_dev_mode", next ? "1" : "0");
              setDevMode(next);
              window.dispatchEvent(new StorageEvent("storage", { key: "yunque_dev_mode" }));
            }}
            className="relative flex items-center px-2 py-1.5 rounded-full text-xs transition-colors duration-200 cursor-pointer z-10"
            style={{ color: devMode ? "var(--accent)" : "var(--text-muted)" }}
            title={devMode ? "切换到普通模式" : "切换到开发模式"}
          >
            <Code2 size={13} />
          </button>

          {/* Collapse / Expand toggle */}
          <button
            onClick={() => setExpanded(!expanded)}
            className="relative flex items-center px-2 py-1.5 rounded-full text-xs transition-colors duration-200 cursor-pointer z-10"
            style={{ color: "var(--text-muted)" }}
            title={expanded ? "收起导航" : "展开导航"}
          >
            <motion.div
              animate={{ rotate: expanded ? 0 : 180 }}
              transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
            >
              <ChevronLeft size={13} />
            </motion.div>
          </button>
        </motion.nav>
      </motion.div>
    </header>
  );
}
