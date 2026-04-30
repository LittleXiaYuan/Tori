"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { Avatar, Button, Tooltip } from "@heroui/react";
import {
  MessageCircle, Zap, BookOpen, ScanFace, Package, Settings,
  LogOut, Search, MailWarning, Puzzle, Brain, BrainCircuit,
  Shield, ShieldCheck, BarChart3, Globe, Blocks, HardDriveDownload,
  Terminal, Cpu, ChevronDown, HelpCircle, LayoutDashboard, Wrench,
  SmilePlus, HeartPulse, Lightbulb, Bot, GraduationCap,
  MessageSquareText, Users, Share2, Menu, X, Languages,
  PanelLeftClose, PanelLeftOpen, FolderGit2, Boxes, CircuitBoard,
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
  { id: "work", label: "工作", labelEn: "Work", icon: <Zap size={16} />, children: [
    { href: "/missions", label: "任务中心", labelEn: "Missions", icon: <Zap size={16} /> },
    { href: "/task-run", label: "执行视图", labelEn: "Execution", icon: <Terminal size={16} /> },
    { href: "/workflows", label: "工作流", labelEn: "Workflows", icon: <Blocks size={16} /> },
    { href: "/workers", label: "Worker", labelEn: "Workers", icon: <Cpu size={16} /> },
    { href: "/projects", label: "项目", labelEn: "Projects", icon: <FolderGit2 size={16} /> },
    { href: "/skills", label: "技能", labelEn: "Skills", icon: <Package size={16} /> },
    { href: "/plugins", label: "插件", labelEn: "Plugins", icon: <Puzzle size={16} /> },
    { href: "/cognis", label: "智体", labelEn: "Cognis", icon: <Boxes size={16} /> },
    { href: "/tools", label: "终端", labelEn: "Terminal", icon: <Wrench size={16} /> },
    { href: "/browser", label: "浏览器", labelEn: "Browser", icon: <Globe size={16} /> },
  ]},
  { id: "intelligence", label: "智能", labelEn: "Intelligence", icon: <Brain size={16} />, children: [
    { href: "/knowledge", label: "知识库", labelEn: "Knowledge", icon: <BookOpen size={16} /> },
    { href: "/memory", label: "记忆", labelEn: "Memory", icon: <Brain size={16} /> },
    { href: "/graph", label: "知识图谱", labelEn: "Graph", icon: <Share2 size={16} /> },
    { href: "/persona", label: "角色", labelEn: "Persona", icon: <ScanFace size={16} /> },
    { href: "/emotions", label: "情绪", labelEn: "Emotions", icon: <SmilePlus size={16} /> },
    { href: "/reflect", label: "反思", labelEn: "Reflection", icon: <Lightbulb size={16} /> },
    { href: "/reverie", label: "内心独白", labelEn: "Reverie", icon: <BrainCircuit size={16} /> },
    { href: "/lora", label: "LoRA 训练", labelEn: "LoRA", icon: <CircuitBoard size={16} /> },
    { href: "/heartbeat", label: "心跳", labelEn: "Heartbeat", icon: <HeartPulse size={16} /> },
  ]},
  { id: "system", label: "系统", labelEn: "System", icon: <ShieldCheck size={16} />, children: [
    { href: "/models", label: "模型", labelEn: "Models", icon: <Cpu size={16} /> },
    { href: "/metrics", label: "指标", labelEn: "Metrics", icon: <BarChart3 size={16} /> },
    { href: "/approvals", label: "审批", labelEn: "Approvals", icon: <ShieldCheck size={16} /> },
    { href: "/audit", label: "审计", labelEn: "Audit", icon: <Shield size={16} /> },
    { href: "/trust", label: "信任", labelEn: "Trust", icon: <ShieldCheck size={16} /> },
    { href: "/tenants", label: "租户", labelEn: "Tenants", icon: <Users size={16} /> },
    { href: "/backup", label: "备份", labelEn: "Backup", icon: <HardDriveDownload size={16} /> },
    { href: "/bots", label: "Bot", labelEn: "Bots", icon: <Bot size={16} /> },
    { href: "/settings", label: "设置", labelEn: "Settings", icon: <Settings size={16} /> },
  ]},
];

const iconMap: Record<string, React.ReactNode> = {
  bot: <Bot size={16} />, graduation: <GraduationCap size={16} />,
  message: <MessageSquareText size={16} />, puzzle: <Puzzle size={16} />,
  globe: <Globe size={16} />, zap: <Zap size={16} />, brain: <Brain size={16} />,
  package: <Package size={16} />, terminal: <Terminal size={16} />, blocks: <Blocks size={16} />,
};
function resolveIcon(name: string): React.ReactNode {
  return iconMap[name.toLowerCase()] || <Puzzle size={16} />;
}

const COLLAPSED_KEY = "yunque_sidebar_collapsed";
const EXPANDED_KEY = "yunque_sidebar_groups";

function getStoredGroups(): Set<string> {
  if (typeof window === "undefined") return new Set(["work"]);
  try {
    const v = localStorage.getItem(EXPANDED_KEY);
    return v ? new Set(JSON.parse(v)) : new Set(["work"]);
  } catch { return new Set(["work"]); }
}

export default function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const { locale, setLocale } = useI18n();
  const [collapsed, setCollapsed] = useState(false);
  const [extItems, setExtItems] = useState<NavItem[]>([]);
  const [online, setOnline] = useState<boolean | null>(null);
  const [version, setVersion] = useState("");
  const [mobileOpen, setMobileOpen] = useState(false);
  const [, startTransition] = useTransition();
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(getStoredGroups);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const stored = localStorage.getItem(COLLAPSED_KEY);
    if (stored !== null) setCollapsed(stored === "1");
    else setCollapsed(window.innerWidth < 1440);
  }, []);

  const toggleCollapsed = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev;
      localStorage.setItem(COLLAPSED_KEY, next ? "1" : "0");
      return next;
    });
  }, []);

  const toggleGroup = useCallback((id: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      localStorage.setItem(EXPANDED_KEY, JSON.stringify([...next]));
      return next;
    });
  }, []);

  useEffect(() => {
    let timer: NodeJS.Timeout;
    const probe = () => {
      api.version()
        .then((v) => { setOnline(true); setVersion(v?.version || ""); })
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
          href: `/ext/${t.key}`, label: t.label,
          labelEn: t.label_en || t.label, icon: resolveIcon(t.icon),
        })));
      }
    }).catch(() => {});
  }, []);

  const allCategories = useMemo(() => {
    if (extItems.length === 0) return categories;
    const extCategory: NavCategory = {
      id: "extensions", label: "扩展", labelEn: "Extensions",
      icon: <Blocks size={16} />, children: extItems,
    };
    return [...categories, extCategory];
  }, [extItems]);

  useEffect(() => {
    if (!pathname) return;
    for (const cat of allCategories) {
      if (cat.children?.some((c) => pathname === c.href || pathname.startsWith(c.href + "/"))) {
        setExpandedGroups((prev) => {
          if (prev.has(cat.id)) return prev;
          const next = new Set(prev);
          next.add(cat.id);
          localStorage.setItem(EXPANDED_KEY, JSON.stringify([...next]));
          return next;
        });
        break;
      }
    }
  }, [pathname, allCategories]);

  const handleLogout = useCallback(() => {
    localStorage.removeItem("yunque_token");
    localStorage.removeItem("yunque_api_key");
    router.replace("/login");
  }, [router]);

  useEffect(() => { setMobileOpen(false); }, [pathname]);

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
      collapse: zh ? "折叠侧边栏" : "Collapse",
      expand: zh ? "展开侧边栏" : "Expand",
      settings: zh ? "设置" : "Settings",
    };
  }, [locale]);

  const renderLink = (href: string, label: string, labelEn: string, icon: React.ReactNode, indent = false) => {
    const active = pathname === href || pathname?.startsWith(href + "/");
    const el = (
      <Link
        key={href}
        href={href}
        className={`sidebar-link${indent ? " sidebar-link-child" : ""}`}
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
    if (collapsed) {
      return (
        <Tooltip key={href} delay={0}>
          <Tooltip.Trigger>
            <Link href={href} className="sidebar-link" data-active={active || undefined}>
              <span className="sidebar-link-icon">{icon}</span>
              <span className="sidebar-link-label">{locale === "zh" ? label : labelEn}</span>
            </Link>
          </Tooltip.Trigger>
          <Tooltip.Content placement="right">{locale === "zh" ? label : labelEn}</Tooltip.Content>
        </Tooltip>
      );
    }
    return el;
  };

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
        {/* Brand */}
        <div className="sidebar-brand">
          <div className="flex items-center gap-2.5">
            <div className="relative flex-shrink-0">
              <Avatar size="sm" style={{ background: "linear-gradient(135deg, var(--yunque-accent), var(--yunque-success))" }}>
                <Avatar.Fallback className="text-white text-[10px] font-bold">YQ</Avatar.Fallback>
              </Avatar>
              <div
                className={online === true ? "online-dot" : ""}
                style={{
                  position: "absolute", bottom: -1, right: -1, width: 8, height: 8,
                  borderRadius: "50%",
                  background: online === true ? "var(--yunque-success)" : online === false ? "var(--yunque-danger)" : "var(--yunque-text-muted)",
                  border: "2px solid var(--yunque-sidebar)",
                }}
              />
            </div>
            <div className="min-w-0 flex-1 sidebar-brand-text">
              <div style={{ fontSize: "var(--text-md)", fontWeight: 600, color: "var(--yunque-text)" }}>{ui.title}</div>
              <div style={{
                fontSize: "var(--text-2xs)",
                color: online === true ? "var(--yunque-success)" : online === false ? "var(--yunque-danger)" : "var(--yunque-text-muted)",
              }}>
                {online === true ? `${ui.online}${version ? ` · v${version}` : ""}` : online === false ? ui.offline : ui.connecting}
              </div>
            </div>
            <button
              className="sidebar-inbox-btn sidebar-brand-text"
              onClick={() => { startTransition(() => { router.push("/inbox"); }); }}
              aria-label={locale === "zh" ? "收件箱" : "Inbox"}
            >
              <MailWarning size={15} />
            </button>
          </div>
        </div>

        {/* Search */}
        <div className="sidebar-search-wrap" style={{ padding: "0 12px 8px" }}>
          <button
            className="sidebar-search"
            onClick={() => { document.dispatchEvent(new CustomEvent("yunque:open-command-palette")); }}
          >
            <Search size={12} />
            <span className="sidebar-search-text">{ui.search}</span>
            <span className="ml-auto sidebar-search-text" style={{
              fontSize: "var(--text-2xs)", padding: "1px 5px",
              borderRadius: "4px", background: "rgba(255,255,255,0.05)",
            }}>
              {typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.userAgent) ? "⌘K" : "Ctrl+K"}
            </span>
          </button>
        </div>

        {/* Navigation */}
        <nav className="sidebar-nav-scroll">
          <div style={{ display: "flex", flexDirection: "column", gap: 2, padding: "0 8px" }}>
            {allCategories.map((cat) => {
              if (!cat.children) {
                return renderLink(cat.href!, cat.label, cat.labelEn, cat.icon);
              }

              const childActive = cat.children.some(
                (c) => pathname === c.href || pathname?.startsWith(c.href + "/"),
              );
              const isOpen = expandedGroups.has(cat.id);

              if (collapsed) {
                return (
                  <Tooltip key={cat.id} delay={0}>
                    <Tooltip.Trigger>
                      <button
                        className="sidebar-link"
                        data-active={childActive || undefined}
                        onClick={() => {
                          setCollapsed(false);
                          localStorage.setItem(COLLAPSED_KEY, "0");
                          setExpandedGroups((prev) => new Set([...prev, cat.id]));
                        }}
                      >
                        <span className="sidebar-link-icon">{cat.icon}</span>
                        <span className="sidebar-link-label">{locale === "zh" ? cat.label : cat.labelEn}</span>
                      </button>
                    </Tooltip.Trigger>
                    <Tooltip.Content placement="right">{locale === "zh" ? cat.label : cat.labelEn}</Tooltip.Content>
                  </Tooltip>
                );
              }

              return (
                <div key={cat.id} className="sidebar-group">
                  <button
                    className="sidebar-group-header"
                    data-active={childActive || undefined}
                    onClick={() => toggleGroup(cat.id)}
                  >
                    <span className="sidebar-link-icon">{cat.icon}</span>
                    <span className="sidebar-link-label">{locale === "zh" ? cat.label : cat.labelEn}</span>
                    <ChevronDown
                      size={12}
                      className="sidebar-link-label ml-auto"
                      style={{
                        opacity: 0.4,
                        transform: isOpen ? "rotate(0deg)" : "rotate(-90deg)",
                        transition: "transform 0.15s ease",
                      }}
                    />
                  </button>
                  {isOpen && (
                    <div className="sidebar-group-children">
                      {cat.children.map(({ href, label, labelEn, icon }) =>
                        renderLink(href, label, labelEn, icon, true)
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </nav>

        {/* Footer */}
        <div className="sidebar-footer">
          {collapsed ? (
            <div className="flex flex-col items-center gap-1 py-2">
              <Tooltip delay={0}>
                <Button size="sm" variant="ghost" isIconOnly className="min-w-0 rounded-lg" onPress={() => startTransition(() => router.push("/settings"))}>
                  <Settings size={14} style={{ opacity: 0.6 }} />
                </Button>
                <Tooltip.Content placement="right">{ui.settings}</Tooltip.Content>
              </Tooltip>
              <Tooltip delay={0}>
                <Button size="sm" variant="ghost" isIconOnly className="min-w-0 rounded-lg" onPress={toggleCollapsed}>
                  <PanelLeftOpen size={14} style={{ opacity: 0.6 }} />
                </Button>
                <Tooltip.Content placement="right">{ui.expand}</Tooltip.Content>
              </Tooltip>
            </div>
          ) : (
            <div className="flex items-center gap-1" style={{ padding: "4px 2px 0" }}>
              <Tooltip delay={0}>
                <Button size="sm" variant="ghost" isIconOnly className="min-w-0 rounded-lg" onPress={() => startTransition(() => router.push("/settings"))}>
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
