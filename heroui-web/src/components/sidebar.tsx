"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { Switch, Avatar, Button } from "@heroui/react";
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

const normalCategories: NavCategory[] = [
  { id: "dashboard", label: "概览", labelEn: "Overview", icon: <LayoutDashboard size={16} />, href: "/dashboard" },
  { id: "chat", label: "对话", labelEn: "Chat", icon: <MessageCircle size={16} />, href: "/chat" },
  { id: "tasks", label: "任务", labelEn: "Tasks", icon: <Zap size={16} />, href: "/missions" },
  { id: "knowledge", label: "知识", labelEn: "Knowledge", icon: <BookOpen size={16} />, children: [
    { href: "/knowledge", label: "知识库", labelEn: "Knowledge Base", icon: <BookOpen size={16} /> },
    { href: "/memory", label: "记忆", labelEn: "Memory", icon: <Brain size={16} /> },
    { href: "/persona", label: "角色", labelEn: "Persona", icon: <ScanFace size={16} /> },
  ]},
  { id: "tools", label: "工具", labelEn: "Tools", icon: <Package size={16} />, children: [
    { href: "/skills", label: "技能", labelEn: "Skills", icon: <Package size={16} /> },
    { href: "/workflows", label: "工作流", labelEn: "Workflows", icon: <Blocks size={16} /> },
  ]},
  { id: "settings", label: "设置", labelEn: "Settings", icon: <Settings size={16} />, href: "/settings" },
];

const devCategories: NavCategory[] = [
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

function findCategoryForPath(categories: NavCategory[], path: string): string | null {
  for (const cat of categories) {
    if (cat.children) {
      for (const child of cat.children) {
        if (path === child.href || path.startsWith(child.href + "/")) return cat.id;
      }
    }
  }
  return null;
}

export default function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const { locale, setLocale } = useI18n();
  const [devMode, setDevMode] = useState(false);
  const [drillId, setDrillId] = useState<string | null>(null);
  const [extItems, setExtItems] = useState<NavItem[]>([]);
  const [online, setOnline] = useState<boolean | null>(null);
  const [version, setVersion] = useState("");
  const [mobileOpen, setMobileOpen] = useState(false);
  const [isPending, startTransition] = useTransition();

  useEffect(() => {
    if (typeof window !== "undefined") {
      setDevMode(localStorage.getItem("yunque_lab_mode") === "1");
    }
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

  const toggleDevMode = useCallback((val: boolean) => {
    setDevMode(val);
    setDrillId(null);
    localStorage.setItem("yunque_lab_mode", val ? "1" : "0");
  }, []);

  const categories = useMemo(() => {
    const base = devMode ? devCategories : normalCategories;
    if (extItems.length === 0) return base;
    const extCategory: NavCategory = {
      id: "extensions",
      label: "扩展",
      labelEn: "Extensions",
      icon: <Blocks size={16} />,
      children: extItems,
    };
    const settingsIdx = base.findIndex((c) => c.id === "settings");
    const result = [...base];
    result.splice(settingsIdx >= 0 ? settingsIdx : result.length, 0, extCategory);
    return result;
  }, [devMode, extItems]);

  useEffect(() => {
    if (pathname) {
      const found = findCategoryForPath(categories, pathname);
      setDrillId(found);
    }
  }, []); // only on mount — don't override user clicks

  const handleLogout = useCallback(() => {
    localStorage.removeItem("yunque_token");
    localStorage.removeItem("yunque_api_key");
    router.replace("/login");
  }, [router]);

  useEffect(() => { setMobileOpen(false); }, [pathname]);

  const activeCategory = useMemo(() => categories.find((c) => c.id === drillId), [categories, drillId]);

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
      devMode: zh ? "高级功能" : "Advanced features",
      help: zh ? "帮助" : "Help",
      logout: zh ? "退出" : "Logout",
      language: zh ? "语言" : "Language",
      localeLabel: zh ? "EN" : "中文",
      back: zh ? "返回" : "Back",
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

      <aside className="sidebar animate-slide-in-left" data-open={mobileOpen || undefined} data-sidebar role="navigation" aria-label={ui.navAria}>
        <div className="sidebar-brand">
          <div className="flex items-center gap-2.5">
            <div className="relative">
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
            <div className="min-w-0 flex-1">
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
              className="sidebar-inbox-btn"
              onClick={() => { startTransition(() => { router.push("/inbox"); }); }}
              aria-label="Inbox"
            >
              <MailWarning size={15} />
            </button>
          </div>
        </div>

        <div style={{ padding: "0 12px 12px" }}>
          <button
            className="sidebar-search"
            onClick={() => {
              document.dispatchEvent(new KeyboardEvent("keydown", { key: "k", metaKey: true, ctrlKey: true }));
            }}
          >
            <Search size={12} />
            <span>{ui.search}</span>
            <span
              className="ml-auto"
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
          {/* ── Main panel ── */}
          <nav
            className="sidebar-panel"
            data-hidden={drillId ? true : undefined}
          >
            <div style={{ display: "flex", flexDirection: "column", gap: 2, padding: "0 8px" }}>
              {categories.map((cat) => {
                if (cat.children) {
                  const childActive = cat.children.some(
                    (c) => pathname === c.href || pathname?.startsWith(c.href + "/"),
                  );
                  return (
                    <button
                      key={cat.id}
                      className="sidebar-link"
                      data-active={childActive || undefined}
                      onClick={() => setDrillId(cat.id)}
                    >
                      <span className="sidebar-link-icon">{cat.icon}</span>
                      <span>{locale === "zh" ? cat.label : cat.labelEn}</span>
                      <ChevronRight size={12} className="ml-auto" style={{ opacity: 0.3 }} />
                    </button>
                  );
                }
                const active = pathname === cat.href || (cat.href !== "/settings" && pathname?.startsWith(cat.href + "/"));
                return (
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
                    <span>{locale === "zh" ? cat.label : cat.labelEn}</span>
                  </Link>
                );
              })}
            </div>
          </nav>

          {/* ── Drill-down panel ── */}
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
                        <span>{locale === "zh" ? label : labelEn}</span>
                      </Link>
                    );
                  })}
                </div>
              </>
            )}
          </nav>
        </div>

        <div className="sidebar-footer">
          <div className="flex items-center justify-between gap-2" style={{ padding: "6px 4px" }}>
            <span style={{ fontSize: "var(--text-xs)", fontWeight: 500, color: "var(--yunque-text-muted)" }}>{ui.devMode}</span>
            <Switch isSelected={devMode} onChange={() => toggleDevMode(!devMode)}>
              <Switch.Control><Switch.Thumb /></Switch.Control>
            </Switch>
          </div>

          <div className="flex items-center justify-between gap-2" style={{ padding: "6px 4px" }}>
            <div className="inline-flex items-center gap-2" style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)" }}>
              <Languages size={14} style={{ opacity: 0.7 }} />
              <span>{ui.language}</span>
            </div>
            <Button
              size="sm"
              variant="ghost"
              className="min-w-0 rounded-lg px-2"
              onPress={() => setLocale(locale === "zh" ? "en" : "zh")}
            >
              {ui.localeLabel}
            </Button>
          </div>

          <a href="https://yunque.owo.today/" target="_blank" rel="noopener noreferrer" className="sidebar-footer-btn">
            <HelpCircle size={14} style={{ opacity: 0.5 }} />
            <span>{ui.help}</span>
          </a>

          <button onClick={handleLogout} className="sidebar-footer-btn" type="button">
            <LogOut size={14} style={{ opacity: 0.5 }} />
            <span>{ui.logout}</span>
          </button>
        </div>
      </aside>
    </>
  );
}
