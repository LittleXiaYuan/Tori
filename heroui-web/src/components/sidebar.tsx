"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { Avatar, Button, Tooltip } from "@heroui/react";
import {
  MessageCircle,
  Zap,
  BookOpen,
  ScanFace,
  Package,
  Settings,
  LogOut,
  Search,
  MailWarning,
  Puzzle,
  Brain,
  BrainCircuit,
  Shield,
  ShieldCheck,
  BarChart3,
  Globe,
  Blocks,
  HardDriveDownload,
  Terminal,
  Cpu,
  ChevronRight,
  ChevronLeft,
  HelpCircle,
  LayoutDashboard,
  Wrench,
  SmilePlus,
  HeartPulse,
  Lightbulb,
  Bot,
  GraduationCap,
  MessageSquareText,
  Palette,
  Users,
  Share2,
  Menu,
  X,
  Plug,
  Bell,
  Languages,
  FlaskConical,
  PanelLeftClose,
  PanelLeftOpen,
} from "lucide-react";
import { useState, useCallback, useEffect, useMemo, useTransition } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";

interface NavItem {
  href: string;
  label: string;
  labelEn: string;
  icon: React.ReactNode;
}

interface NavCategory {
  id: string;
  label: string;
  labelEn: string;
  icon: React.ReactNode;
  href?: string;
  children?: NavItem[];
}

const categories: NavCategory[] = [
  { id: "dashboard", label: "概览", labelEn: "Overview", icon: <LayoutDashboard size={16} />, href: "/dashboard" },
  { id: "chat", label: "对话", labelEn: "Chat", icon: <MessageCircle size={16} />, href: "/chat" },
  { id: "tasks", label: "任务", labelEn: "Tasks", icon: <Zap size={16} />, children: [
    { href: "/missions", label: "任务中心", labelEn: "Mission Center", icon: <Zap size={16} /> },
    { href: "/task-run", label: "执行视图", labelEn: "Execution", icon: <Terminal size={16} /> },
    { href: "/workflows", label: "工作流", labelEn: "Workflows", icon: <Blocks size={16} /> },
  ]},
  { id: "knowledge", label: "知识", labelEn: "Knowledge", icon: <BookOpen size={16} />, children: [
    { href: "/knowledge", label: "知识库", labelEn: "Knowledge Base", icon: <BookOpen size={16} /> },
    { href: "/memory", label: "记忆", labelEn: "Memory", icon: <Brain size={16} /> },
    { href: "/graph", label: "知识图谱", labelEn: "Graph", icon: <Share2 size={16} /> },
    { href: "/persona", label: "角色", labelEn: "Persona", icon: <ScanFace size={16} /> },
    { href: "/emotions", label: "情绪", labelEn: "Emotion", icon: <SmilePlus size={16} /> },
  ]},
  { id: "tools", label: "工具", labelEn: "Tools", icon: <Package size={16} />, children: [
    { href: "/skills", label: "技能", labelEn: "Skills", icon: <Package size={16} /> },
    { href: "/plugins", label: "插件", labelEn: "Plugins", icon: <Puzzle size={16} /> },
    { href: "/tools", label: "终端", labelEn: "Terminal", icon: <Wrench size={16} /> },
    { href: "/browser", label: "浏览器", labelEn: "Browser", icon: <Globe size={16} /> },
  ]},
  { id: "lab", label: "实验室", labelEn: "Lab", icon: <FlaskConical size={16} />, children: [
    { href: "/reflect", label: "反思", labelEn: "Reflection", icon: <Lightbulb size={16} /> },
    { href: "/reverie", label: "内心独白", labelEn: "Reverie", icon: <BrainCircuit size={16} /> },
    { href: "/heartbeat", label: "心跳", labelEn: "Heartbeat", icon: <HeartPulse size={16} /> },
  ]},
  { id: "admin", label: "管理", labelEn: "Admin", icon: <ShieldCheck size={16} />, children: [
    { href: "/models", label: "模型管理", labelEn: "Models", icon: <Cpu size={16} /> },
    { href: "/settings/providers", label: "提供商", labelEn: "Providers", icon: <Globe size={16} /> },
    { href: "/metrics", label: "指标", labelEn: "Metrics", icon: <BarChart3 size={16} /> },
    { href: "/approvals", label: "审批", labelEn: "Approvals", icon: <ShieldCheck size={16} /> },
    { href: "/audit", label: "审计", labelEn: "Audit", icon: <Shield size={16} /> },
    { href: "/trust", label: "信任", labelEn: "Trust", icon: <ShieldCheck size={16} /> },
    { href: "/tenants", label: "租户", labelEn: "Tenants", icon: <Users size={16} /> },
    { href: "/backup", label: "备份", labelEn: "Backup", icon: <HardDriveDownload size={16} /> },
    { href: "/bots", label: "Bot 管理", labelEn: "Bots", icon: <Bot size={16} /> },
    { href: "/settings/connectors", label: "连接器", labelEn: "Connectors", icon: <Plug size={16} /> },
    { href: "/settings/notifications", label: "通知", labelEn: "Notifications", icon: <Bell size={16} /> },
    { href: "/settings/theme", label: "主题", labelEn: "Theme", icon: <Palette size={16} /> },
  ]},
  { id: "settings", label: "设置", labelEn: "Settings", icon: <Settings size={16} />, href: "/settings" },
];

const iconMap: Record<string, React.ReactNode> = {
  bot: <Bot size={16} />,
  graduation: <GraduationCap size={16} />,
  message: <MessageSquareText size={16} />,
  puzzle: <Puzzle size={16} />,
  globe: <Globe size={16} />,
  zap: <Zap size={16} />,
  brain: <Brain size={16} />,
  package: <Package size={16} />,
  terminal: <Terminal size={16} />,
  blocks: <Blocks size={16} />,
};
function resolveIcon(name: string): React.ReactNode {
  return iconMap[name.toLowerCase()] || <Puzzle size={16} />;
}

function findCategoryForPath(cats: NavCategory[], path: string): string | null {
  for (const cat of cats) {
    if (cat.children) {
      for (const child of cat.children) {
        if (path === child.href || path.startsWith(child.href + "/")) return cat.id;
      }
    }
  }
  return null;
}

const COLLAPSED_KEY = "yunque_sidebar_collapsed";

export default function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const { locale, setLocale } = useI18n();
  const [collapsed, setCollapsed] = useState(false);
  const [drillId, setDrillId] = useState<string | null>(null);
  const [extItems, setExtItems] = useState<NavItem[]>([]);
  const [online, setOnline] = useState<boolean | null>(null);
  const [version, setVersion] = useState("");
  const [mobileOpen, setMobileOpen] = useState(false);
  const [isPending, startTransition] = useTransition();

  useEffect(() => {
    if (typeof window === "undefined") return;
    // Clean up legacy dual-mode key
    localStorage.removeItem("yunque_simple_mode");
    const stored = localStorage.getItem(COLLAPSED_KEY);
    if (stored !== null) {
      setCollapsed(stored === "1");
    } else {
      setCollapsed(window.innerWidth < 1440);
    }
  }, []);

  const toggleCollapsed = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev;
      localStorage.setItem(COLLAPSED_KEY, next ? "1" : "0");
      if (next) setDrillId(null);
      return next;
    });
  }, []);

  useEffect(() => {
    let timer: NodeJS.Timeout;
    const probe = () => {
      api.version()
        .then((v) => {
          setOnline(true);
          setVersion(v?.version || "");
        })
        .catch(() => setOnline(false));
    };
    probe();
    const startPolling = () => { timer = setInterval(probe, 10000); };
    const stopPolling = () => clearInterval(timer);
    const handleVisibility = () => {
      if (document.hidden) stopPolling();
      else { probe(); startPolling(); }
    };
    document.addEventListener("visibilitychange", handleVisibility);
    startPolling();
    return () => { stopPolling(); document.removeEventListener("visibilitychange", handleVisibility); };
  }, []);

  useEffect(() => {
    api.pluginUITabs().then((res) => {
      if (res?.tabs?.length) {
        setExtItems(res.tabs.map((t) => ({
          href: `/ext/${t.key}`,
          label: t.label,
          labelEn: t.label_en || t.label,
          icon: resolveIcon(t.icon),
        })));
      }
    }).catch(() => {});
  }, []);

  const allCategories = useMemo(() => {
    if (extItems.length === 0) return categories;
    const extCategory: NavCategory = {
      id: "extensions",
      label: "扩展",
      labelEn: "Extensions",
      icon: <Blocks size={16} />,
      children: extItems,
    };
    const settingsIdx = categories.findIndex((c) => c.id === "settings");
    const result = [...categories];
    result.splice(settingsIdx >= 0 ? settingsIdx : result.length, 0, extCategory);
    return result;
  }, [extItems]);

  useEffect(() => {
    if (pathname) {
      const found = findCategoryForPath(allCategories, pathname);
      setDrillId(found);
    }
  }, []); // only on mount

  const handleLogout = useCallback(() => {
    localStorage.removeItem("yunque_token");
    localStorage.removeItem("yunque_api_key");
    router.replace("/login");
  }, [router]);

  useEffect(() => { setMobileOpen(false); }, [pathname]);

  const activeCategory = useMemo(() => allCategories.find((c) => c.id === drillId), [allCategories, drillId]);

  const ui = useMemo(() => {
    const zh = locale === "zh";
    return {
      title: zh ? "云雀 Agent" : "Yunque Agent",
      online: zh ? "在线" : "Online",
      offline: zh ? "离线" : "Offline",
      connecting: zh ? "连接中…" : "Connecting…",
      search: zh ? "搜索…" : "Search…",
      mobileOpen: zh ? "打开侧边栏" : "Open sidebar",
      mobileClose: zh ? "关闭侧边栏" : "Close sidebar",
      navAria: zh ? "主导航" : "Main navigation",
      help: zh ? "帮助" : "Help",
      logout: zh ? "退出" : "Logout",
      localeLabel: zh ? "EN" : "中文",
      back: zh ? "返回" : "Back",
      collapse: zh ? "折叠侧边栏" : "Collapse",
      expand: zh ? "展开侧边栏" : "Expand",
      settings: zh ? "设置" : "Settings",
    };
  }, [locale]);

  return (
    <>
      <button
        className="sidebar-toggle"
        onClick={() => setMobileOpen(!mobileOpen)}
        aria-label={mobileOpen ? ui.mobileClose : ui.mobileOpen}
      >
        {mobileOpen ? <X size={18} /> : <Menu size={18} />}
      </button>

      <div className="sidebar-overlay" data-open={mobileOpen || undefined} onClick={() => setMobileOpen(false)} />

      <aside className="sidebar animate-slide-in-left" data-open={mobileOpen || undefined} data-collapsed={collapsed || undefined} data-sidebar role="navigation" aria-label={ui.navAria}>
        <div className="sidebar-brand">
          <div className="flex items-center gap-2.5">
            <div className="relative flex-shrink-0">
              <Avatar size="sm" style={{ background: "linear-gradient(135deg, var(--yunque-accent), var(--yunque-success))" }}>
                <Avatar.Fallback className="text-white text-[10px] font-bold">YQ</Avatar.Fallback>
              </Avatar>
              <div
                className={online === true ? "online-dot" : ""}
                style={{
                  position: "absolute",
                  bottom: -1,
                  right: -1,
                  width: 8,
                  height: 8,
                  borderRadius: "50%",
                  background: online === true ? "var(--yunque-success)" : online === false ? "var(--yunque-danger)" : "var(--yunque-text-muted)",
                  border: "2px solid var(--yunque-sidebar)",
                }}
              />
            </div>
            <div className="min-w-0 flex-1 sidebar-brand-text">
              <div style={{ fontSize: "var(--text-md)", fontWeight: 600, color: "var(--yunque-text)" }}>{ui.title}</div>
              <div
                style={{
                  fontSize: "var(--text-2xs)",
                  color: online === true ? "var(--yunque-success)" : online === false ? "var(--yunque-danger)" : "var(--yunque-text-muted)",
                }}
              >
                {online === true ? `${ui.online}${version ? ` · v${version}` : ""}` : online === false ? ui.offline : ui.connecting}
              </div>
            </div>
            <button
              className="sidebar-inbox-btn sidebar-brand-text"
              onClick={() => { startTransition(() => { router.push("/inbox"); }); }}
              aria-label="Inbox"
            >
              <MailWarning size={15} />
            </button>
          </div>
        </div>

        <div className="sidebar-search-wrap" style={{ padding: "0 12px 8px" }}>
          <button
            className="sidebar-search"
            onClick={() => {
              document.dispatchEvent(new KeyboardEvent("keydown", { key: "k", metaKey: true, ctrlKey: true }));
            }}
          >
            <Search size={12} />
            <span className="sidebar-search-text">{ui.search}</span>
            <span
              className="ml-auto sidebar-search-text"
              style={{
                fontSize: "var(--text-2xs)",
                padding: "1px 5px",
                borderRadius: "4px",
                background: "rgba(255,255,255,0.05)",
              }}
            >
              ⌘K
            </span>
          </button>
        </div>

        <div className="sidebar-nav-container">
          {/* Main panel */}
          <nav
            className="sidebar-panel"
            data-hidden={drillId ? true : undefined}
          >
            <div style={{ display: "flex", flexDirection: "column", gap: 2, padding: "0 8px" }}>
              {allCategories.map((cat) => {
                if (cat.children) {
                  const childActive = cat.children.some(
                    (c) => pathname === c.href || pathname?.startsWith(c.href + "/"),
                  );
                  if (collapsed) {
                    const firstHref = cat.children[0]?.href;
                    return (
                      <Tooltip key={cat.id} delay={0} placement="right">
                        <Link
                          href={firstHref || "#"}
                          className="sidebar-link"
                          data-active={childActive || undefined}
                          onClick={(e) => {
                            e.preventDefault();
                            startTransition(() => { router.push(firstHref || "/"); });
                          }}
                        >
                          <span className="sidebar-link-icon">{cat.icon}</span>
                          <span className="sidebar-link-label">{locale === "zh" ? cat.label : cat.labelEn}</span>
                        </Link>
                        <Tooltip.Content>{locale === "zh" ? cat.label : cat.labelEn}</Tooltip.Content>
                      </Tooltip>
                    );
                  }
                  return (
                    <button
                      key={cat.id}
                      className="sidebar-link"
                      data-active={childActive || undefined}
                      onClick={() => setDrillId(cat.id)}
                    >
                      <span className="sidebar-link-icon">{cat.icon}</span>
                      <span className="sidebar-link-label">{locale === "zh" ? cat.label : cat.labelEn}</span>
                      <ChevronRight size={12} className="ml-auto sidebar-link-label" style={{ opacity: 0.3 }} />
                    </button>
                  );
                }
                const active = pathname === cat.href || (cat.href !== "/settings" && pathname?.startsWith(cat.href + "/"));
                const linkContent = (
                  <Link
                    key={cat.id}
                    href={cat.href!}
                    className="sidebar-link"
                    data-active={active || undefined}
                    onClick={(e) => {
                      if (active) { e.preventDefault(); return; }
                      e.preventDefault();
                      startTransition(() => { router.push(cat.href!); });
                    }}
                  >
                    <span className="sidebar-link-icon">{cat.icon}</span>
                    <span className="sidebar-link-label">{locale === "zh" ? cat.label : cat.labelEn}</span>
                  </Link>
                );
                if (collapsed) {
                  return (
                    <Tooltip key={cat.id} delay={0} placement="right">
                      {linkContent}
                      <Tooltip.Content>{locale === "zh" ? cat.label : cat.labelEn}</Tooltip.Content>
                    </Tooltip>
                  );
                }
                return linkContent;
              })}
            </div>
          </nav>

          {/* Drill-down panel (hidden when collapsed) */}
          {!collapsed && (
            <nav
              className="sidebar-panel sidebar-panel-sub"
              data-active={drillId ? true : undefined}
            >
              {activeCategory && (
                <>
                  <button
                    className="sidebar-back-btn"
                    onClick={() => setDrillId(null)}
                  >
                    <ChevronLeft size={14} />
                    <span className="sidebar-link-icon">{activeCategory.icon}</span>
                    <span style={{ fontWeight: 600 }}>{locale === "zh" ? activeCategory.label : activeCategory.labelEn}</span>
                  </button>
                  <div style={{ marginTop: 6, display: "flex", flexDirection: "column", gap: 2, padding: "0 8px" }}>
                    {activeCategory.children!.map(({ href, label, labelEn, icon }) => {
                      const active = pathname === href || pathname?.startsWith(href + "/");
                      return (
                        <Link
                          key={href}
                          href={href}
                          className="sidebar-link"
                          data-active={active || undefined}
                          onClick={(e) => {
                            if (active) { e.preventDefault(); return; }
                            e.preventDefault();
                            startTransition(() => { router.push(href); });
                          }}
                        >
                          <span className="sidebar-link-icon">{icon}</span>
                          <span className="sidebar-link-label">{locale === "zh" ? label : labelEn}</span>
                        </Link>
                      );
                    })}
                  </div>
                </>
              )}
            </nav>
          )}
        </div>

        <div className="sidebar-footer">
          {collapsed ? (
            <div className="flex flex-col items-center gap-1 py-2">
              <Tooltip delay={0} placement="right">
                <Button size="sm" variant="ghost" isIconOnly className="min-w-0 rounded-lg" onPress={() => window.dispatchEvent(new CustomEvent("yunque:open-settings"))}>
                  <Settings size={14} style={{ opacity: 0.6 }} />
                </Button>
                <Tooltip.Content>{ui.settings}</Tooltip.Content>
              </Tooltip>
              <Tooltip delay={0} placement="right">
                <Button size="sm" variant="ghost" isIconOnly className="min-w-0 rounded-lg" onPress={toggleCollapsed}>
                  <PanelLeftOpen size={14} style={{ opacity: 0.6 }} />
                </Button>
                <Tooltip.Content>{ui.expand}</Tooltip.Content>
              </Tooltip>
            </div>
          ) : (
            <div className="flex items-center gap-1" style={{ padding: "4px 2px 0" }}>
              <Tooltip delay={0}>
                <Button size="sm" variant="ghost" isIconOnly className="min-w-0 rounded-lg" onPress={() => window.dispatchEvent(new CustomEvent("yunque:open-settings"))}>
                  <Settings size={14} style={{ opacity: 0.6 }} />
                </Button>
                <Tooltip.Content>{ui.settings}</Tooltip.Content>
              </Tooltip>
              <Tooltip delay={0}>
                <Button size="sm" variant="ghost" isIconOnly className="min-w-0 rounded-lg" onPress={() => setLocale(locale === "zh" ? "en" : "zh")}>
                  <Languages size={14} style={{ opacity: 0.6 }} />
                </Button>
                <Tooltip.Content>{ui.localeLabel}</Tooltip.Content>
              </Tooltip>
              <Tooltip delay={0}>
                <Button size="sm" variant="ghost" isIconOnly className="min-w-0 rounded-lg" onPress={() => window.open("https://yunque.owo.today/", "_blank", "noopener,noreferrer")}>
                  <HelpCircle size={14} style={{ opacity: 0.6 }} />
                </Button>
                <Tooltip.Content>{ui.help}</Tooltip.Content>
              </Tooltip>
              <Tooltip delay={0}>
                <Button size="sm" variant="ghost" isIconOnly className="min-w-0 rounded-lg" onPress={handleLogout}>
                  <LogOut size={14} style={{ opacity: 0.6 }} />
                </Button>
                <Tooltip.Content>{ui.logout}</Tooltip.Content>
              </Tooltip>
              <div className="flex-1" />
              <Tooltip delay={0}>
                <Button size="sm" variant="ghost" isIconOnly className="min-w-0 rounded-lg" onPress={toggleCollapsed}>
                  <PanelLeftClose size={14} style={{ opacity: 0.6 }} />
                </Button>
                <Tooltip.Content>{ui.collapse}</Tooltip.Content>
              </Tooltip>
            </div>
          )}
        </div>
      </aside>
    </>
  );
}
