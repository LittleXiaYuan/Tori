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
  ChevronDown,
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

interface NavGroup {
  label: string;
  labelEn: string;
  items: NavItem[];
}

const normalGroups: NavGroup[] = [
  {
    label: "核心",
    labelEn: "Core",
    items: [
      { href: "/dashboard", label: "概览", labelEn: "Overview", icon: <LayoutDashboard size={16} /> },
      { href: "/chat", label: "对话", labelEn: "Chat", icon: <MessageCircle size={16} /> },
      { href: "/missions", label: "任务", labelEn: "Tasks", icon: <Zap size={16} /> },
      { href: "/skills", label: "技能", labelEn: "Skills", icon: <Package size={16} /> },
      { href: "/workflows", label: "工作流", labelEn: "Workflows", icon: <Blocks size={16} /> },
    ],
  },
  {
    label: "更多",
    labelEn: "More",
    items: [
      { href: "/inbox", label: "收件箱", labelEn: "Inbox", icon: <MailWarning size={16} /> },
      { href: "/knowledge", label: "知识库", labelEn: "Knowledge", icon: <BookOpen size={16} /> },
      { href: "/memory", label: "记忆", labelEn: "Memory", icon: <Brain size={16} /> },
      { href: "/persona", label: "角色", labelEn: "Persona", icon: <ScanFace size={16} /> },
      { href: "/settings", label: "设置", labelEn: "Settings", icon: <Settings size={16} /> },
    ],
  },
];

const devGroups: NavGroup[] = [
  {
    label: "核心",
    labelEn: "Core",
    items: [
      { href: "/dashboard", label: "概览", labelEn: "Overview", icon: <LayoutDashboard size={16} /> },
      { href: "/chat", label: "对话", labelEn: "Chat", icon: <MessageCircle size={16} /> },
      { href: "/inbox", label: "收件箱", labelEn: "Inbox", icon: <MailWarning size={16} /> },
    ],
  },
  {
    label: "任务",
    labelEn: "Tasking",
    items: [
      { href: "/missions", label: "任务中心", labelEn: "Mission Center", icon: <Zap size={16} /> },
      { href: "/task-run", label: "执行视图", labelEn: "Execution", icon: <Terminal size={16} /> },
      { href: "/workflows", label: "工作流", labelEn: "Workflows", icon: <Blocks size={16} /> },
    ],
  },
  {
    label: "智能",
    labelEn: "Cognition",
    items: [
      { href: "/knowledge", label: "知识库", labelEn: "Knowledge Base", icon: <BookOpen size={16} /> },
      { href: "/memory", label: "记忆", labelEn: "Memory", icon: <Brain size={16} /> },
      { href: "/graph", label: "知识图谱", labelEn: "Graph", icon: <Share2 size={16} /> },
      { href: "/reflect", label: "反思", labelEn: "Reflection", icon: <Lightbulb size={16} /> },
      { href: "/persona", label: "角色", labelEn: "Persona", icon: <ScanFace size={16} /> },
      { href: "/emotions", label: "情绪", labelEn: "Emotion", icon: <SmilePlus size={16} /> },
      { href: "/reverie", label: "内心独白", labelEn: "Reverie", icon: <BrainCircuit size={16} /> },
      { href: "/heartbeat", label: "心跳", labelEn: "Heartbeat", icon: <HeartPulse size={16} /> },
    ],
  },
  {
    label: "工具",
    labelEn: "Tools",
    items: [
      { href: "/skills", label: "技能", labelEn: "Skills", icon: <Package size={16} /> },
      { href: "/plugins", label: "插件", labelEn: "Plugins", icon: <Puzzle size={16} /> },
      { href: "/tools", label: "终端", labelEn: "Terminal", icon: <Wrench size={16} /> },
      { href: "/browser", label: "浏览器", labelEn: "Browser", icon: <Globe size={16} /> },
    ],
  },
  {
    label: "管理",
    labelEn: "Admin",
    items: [
      { href: "/models", label: "模型管理", labelEn: "Models", icon: <Cpu size={16} /> },
      { href: "/settings/providers", label: "提供商", labelEn: "Providers", icon: <Globe size={16} /> },
      { href: "/metrics", label: "指标", labelEn: "Metrics", icon: <BarChart3 size={16} /> },
      { href: "/approvals", label: "审批", labelEn: "Approvals", icon: <ShieldCheck size={16} /> },
      { href: "/audit", label: "审计", labelEn: "Audit", icon: <Shield size={16} /> },
      { href: "/trust", label: "信任", labelEn: "Trust", icon: <ShieldCheck size={16} /> },
      { href: "/tenants", label: "租户", labelEn: "Tenants", icon: <Users size={16} /> },
      { href: "/backup", label: "备份", labelEn: "Backup", icon: <HardDriveDownload size={16} /> },
      { href: "/bots", label: "Bot 管理", labelEn: "Bots", icon: <Bot size={16} /> },
      { href: "/settings", label: "设置", labelEn: "Settings", icon: <Settings size={16} /> },
      { href: "/settings/connectors", label: "连接器", labelEn: "Connectors", icon: <Plug size={16} /> },
      { href: "/settings/notifications", label: "通知", labelEn: "Notifications", icon: <Bell size={16} /> },
      { href: "/settings/theme", label: "主题", labelEn: "Theme", icon: <Palette size={16} /> },
    ],
  },
  {
    label: "扩展",
    labelEn: "Extensions",
    items: [
      { href: "/ext/airi", label: "Airi 桥接", labelEn: "Airi Bridge", icon: <Bot size={16} /> },
      { href: "/ext/paper", label: "论文助手", labelEn: "Paper Assistant", icon: <GraduationCap size={16} /> },
      { href: "/ext/qqchat", label: "QQ 分析", labelEn: "QQ Analysis", icon: <MessageSquareText size={16} /> },
    ],
  },
];

export default function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const { locale, setLocale } = useI18n();
  const [devMode, setDevMode] = useState(false);
  const [openGroups, setOpenGroups] = useState<Set<string>>(() => new Set());
  const [online, setOnline] = useState<boolean | null>(null);
  const [version, setVersion] = useState("");
  const [mobileOpen, setMobileOpen] = useState(false);
  const [isPending, startTransition] = useTransition();

  useEffect(() => {
    if (typeof window !== "undefined") {
      setDevMode(localStorage.getItem("yunque_dev_mode") === "1");
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
    const startPolling = () => {
      timer = setInterval(probe, 10000);
    };
    const stopPolling = () => clearInterval(timer);
    const handleVisibility = () => {
      if (document.hidden) stopPolling();
      else {
        probe();
        startPolling();
      }
    };
    document.addEventListener("visibilitychange", handleVisibility);
    startPolling();
    return () => {
      stopPolling();
      document.removeEventListener("visibilitychange", handleVisibility);
    };
  }, []);

  const toggleDevMode = useCallback((val: boolean) => {
    setDevMode(val);
    localStorage.setItem("yunque_dev_mode", val ? "1" : "0");
  }, []);

  const groups = devMode ? devGroups : normalGroups;

  useEffect(() => {
    setOpenGroups(new Set(groups.map((g) => g.label)));
  }, [devMode, locale]); // keep expanded after label language changes

  const toggleGroup = useCallback((label: string) => {
    setOpenGroups((prev) => {
      const next = new Set(prev);
      if (next.has(label)) next.delete(label);
      else next.add(label);
      return next;
    });
  }, []);

  const handleLogout = useCallback(() => {
    localStorage.removeItem("yunque_token");
    localStorage.removeItem("yunque_api_key");
    router.replace("/login");
  }, [router]);

  useEffect(() => {
    setMobileOpen(false);
  }, [pathname]);

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
      devMode: zh ? "开发模式" : "Developer mode",
      help: zh ? "帮助" : "Help",
      logout: zh ? "退出" : "Logout",
      language: zh ? "语言" : "Language",
      localeLabel: zh ? "EN" : "中文",
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

      <aside className="sidebar animate-slide-in-left" data-open={mobileOpen || undefined} role="navigation" aria-label={ui.navAria}>
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

        <nav className="sidebar-nav">
          {groups.map((group, gi) => {
            const groupKey = group.label;
            const isOpen = openGroups.has(groupKey);
            return (
              <div key={groupKey} style={{ marginTop: gi > 0 ? 16 : 4 }}>
                <button onClick={() => toggleGroup(groupKey)} className="sidebar-group-label">
                  <ChevronDown
                    size={9}
                    style={{
                      transition: "transform var(--duration-fast) ease",
                      transform: isOpen ? "rotate(0)" : "rotate(-90deg)",
                    }}
                  />
                  {locale === "zh" ? group.label : group.labelEn}
                </button>
                {isOpen && (
                  <div style={{ marginTop: 2, display: "flex", flexDirection: "column", gap: 1 }}>
                    {group.items.map(({ href, label, labelEn, icon }) => {
                      const active = pathname === href || (href !== "/settings" && pathname?.startsWith(href + "/"));
                      return (
                        <Link
                          key={href}
                          href={href}
                          className="sidebar-link"
                          data-active={active || undefined}
                          data-pending={isPending && !active ? "" : undefined}
                          onClick={(e) => {
                            if (active || isPending) { e.preventDefault(); return; }
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
                )}
              </div>
            );
          })}
        </nav>

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

          <button className="sidebar-footer-btn" type="button">
            <HelpCircle size={14} style={{ opacity: 0.5 }} />
            <span>{ui.help}</span>
          </button>

          <button onClick={handleLogout} className="sidebar-footer-btn" type="button">
            <LogOut size={14} style={{ opacity: 0.5 }} />
            <span>{ui.logout}</span>
          </button>
        </div>
      </aside>
    </>
  );
}
