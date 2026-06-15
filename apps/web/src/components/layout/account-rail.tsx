"use client";

/**
 * AccountRail —— 极窄左侧账号栏（64px 宽）
 *
 * 取代原 `sidebar.tsx` 的全功能侧栏。沿用其业务逻辑：
 *  - 后端在线状态轮询 + 在线点
 *  - 登出（清 localStorage 然后跳 /login）
 *  - 语言切换 (i18n locale)
 *  - 命令面板触发 (Cmd+K)
 *
 * 五大类功能性导航全部撤掉，转交命令面板。该 rail 上只放：头像 / 搜索 /
 * 主题切换 / 设置 / 帮助 / 语言 / 登出。
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Avatar, Button, Tooltip } from "@heroui/react";
import {
  Brain,
  Languages,
  LayoutGrid,
  LogOut,
  MessageCircle,
  Moon,
  Puzzle,
  Search,
  Settings,
  Sun,
  Zap,
} from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { loadTheme, patchAndApply } from "@/lib/theme-engine";
import AccountRailFlyout from "@/components/layout/account-rail-flyout";
import { WindowControls } from "@/components/title-bar";
import { buildPackNavItems, fetchEnabledPacks } from "@/lib/pack-sync";
import { DEFAULT_ENABLED_PACK_IDS, type NavItem } from "@/lib/nav-items";
import { useNavigationPreferences } from "@/hooks/use-user-preferences";

const FLY_ENTER_DELAY_MS = 300;
const FLY_LEAVE_DELAY_MS = 200;

interface QuickLink {
  href: string;
  icon: React.ReactNode;
  zh: string;
  en: string;
}

/**
 * 常用功能直达，少而精。启用后的能力包入口默认只进入「主路径」弹层
 * 和 ⌘K；只有用户手动固定后才出现在这条窄侧边栏中。
 */
const QUICK_LINKS: QuickLink[] = [
  { href: "/chat",      icon: <MessageCircle size={16} />, zh: "对话",   en: "Chat" },
  { href: "/missions",  icon: <Zap size={16} />,           zh: "任务中心", en: "Missions" },
  { href: "/memory",    icon: <Brain size={16} />,         zh: "记忆",   en: "Memory" },
];

export default function AccountRail() {
  const router = useRouter();
  const pathname = usePathname();
  const { locale, setLocale } = useI18n();
  const navigationPrefs = useNavigationPreferences();
  const [online, setOnline] = useState<boolean | null>(null);
  const [version, setVersion] = useState("");
  const [themeMode, setThemeMode] = useState<"dark" | "light">("dark");
  const [flyoutOpen, setFlyoutOpen] = useState(false);
  const [extItems, setExtItems] = useState<NavItem[]>([]);
  const [packItems, setPackItems] = useState<NavItem[]>([]);
  const [enabledPackIds, setEnabledPackIds] = useState<Set<string>>(() => new Set(DEFAULT_ENABLED_PACK_IDS));
  const [anchorTop, setAnchorTop] = useState(0);
  const enterTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const leaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const moreBtnRef = useRef<HTMLButtonElement | null>(null);

  // 加载插件 UI tab，作为「扩展」分组并入 flyout（与 command-palette 同源）。
  useEffect(() => {
    api
      .pluginUITabs()
      .then((res) => {
        if (!res?.tabs?.length) return;
        setExtItems(
          res.tabs.map((t) => ({
            id: `ext-${t.key}`,
            href: `/ext/${t.key}`,
            label: t.label,
            group: "扩展" as const,
            layer: "pack" as const,
            icon: <Puzzle size={16} />,
            keywords: `${t.label} ${t.label_en || ""} extension ${t.key}`,
          })),
        );
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    fetchEnabledPacks()
      .then((packs) => {
        setEnabledPackIds(new Set(packs.map((p) => p.manifest.id)));
        setPackItems(
          buildPackNavItems(packs).map((item) => ({
            id: `pack-${item.packId}-${item.href}`,
            href: item.href,
            label: item.label,
            group: "扩展" as const,
            layer: "pack" as const,
            defaultVisible: true,
            icon: item.icon,
            keywords: item.keywords,
          })),
        );
      })
      .catch(() => {
        setEnabledPackIds(new Set());
        setPackItems([]);
      });
  }, []);

  // hover 进入/离开延时控制
  const clearTimers = useCallback(() => {
    if (enterTimer.current) {
      clearTimeout(enterTimer.current);
      enterTimer.current = null;
    }
    if (leaveTimer.current) {
      clearTimeout(leaveTimer.current);
      leaveTimer.current = null;
    }
  }, []);
  const updateAnchor = useCallback(() => {
    const btn = moreBtnRef.current;
    if (!btn) return;
    const r = btn.getBoundingClientRect();
    // flyout 顶部对齐 More 按钮顶部，向上抬 4px 以减小鼠标移动时的跳动空隙。
    setAnchorTop(Math.max(8, r.top - 4));
  }, []);
  const scheduleOpenFlyout = useCallback(() => {
    clearTimers();
    enterTimer.current = setTimeout(() => {
      updateAnchor();
      setFlyoutOpen(true);
    }, FLY_ENTER_DELAY_MS);
  }, [clearTimers, updateAnchor]);
  const scheduleCloseFlyout = useCallback(() => {
    clearTimers();
    leaveTimer.current = setTimeout(() => setFlyoutOpen(false), FLY_LEAVE_DELAY_MS);
  }, [clearTimers]);
  const cancelCloseFlyout = useCallback(() => {
    if (leaveTimer.current) {
      clearTimeout(leaveTimer.current);
      leaveTimer.current = null;
    }
  }, []);
  const closeFlyoutNow = useCallback(() => {
    clearTimers();
    setFlyoutOpen(false);
  }, [clearTimers]);
  useEffect(() => () => clearTimers(), [clearTimers]);

  // 路由切换后自动收起 flyout，避免 SPA 跳转后仍残留打开状态。
  useEffect(() => {
    closeFlyoutNow();
  }, [pathname, closeFlyoutNow]);

  // flyout 打开时，Esc 关闭。（Tab 由 flyout 自身的 `inert` 在闭合时阻挡。）
  useEffect(() => {
    if (!flyoutOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeFlyoutNow();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [flyoutOpen, closeFlyoutNow]);

  // 后端心跳轮询（10s）。可见性变化时暂停。
  // 只用轻量 /healthz 判断“在线/离线”：版本接口偶发失败、鉴权状态或更新检查
  // 不应该让桌面壳显示成离线。
  useEffect(() => {
    let timer: ReturnType<typeof setInterval> | undefined;
    const probe = () => {
      api
        .healthz()
        .then((health) => {
          setOnline(true);
          setVersion(health?.version || "");
        })
        .catch(() => setOnline(false));
    };
    probe();
    const start = () => {
      timer = setInterval(probe, 10000);
    };
    const stop = () => {
      if (timer) clearInterval(timer);
    };
    const onVis = () => {
      if (document.hidden) stop();
      else {
        probe();
        start();
      }
    };
    document.addEventListener("visibilitychange", onVis);
    start();
    return () => {
      stop();
      document.removeEventListener("visibilitychange", onVis);
    };
  }, []);

  // 当前显示的明/暗模式（用于切换图标）
  useEffect(() => {
    if (typeof document === "undefined") return;
    const detect = () => {
      const t = document.documentElement.getAttribute("data-theme");
      setThemeMode(t === "light" ? "light" : "dark");
    };
    detect();
    const obs = new MutationObserver(detect);
    obs.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme", "class"] });
    return () => obs.disconnect();
  }, []);

  const handleLogout = useCallback(() => {
    localStorage.removeItem("yunque_token");
    localStorage.removeItem("yunque_api_key");
    router.replace("/login");
  }, [router]);

  const toggleTheme = useCallback(() => {
    const cur = loadTheme();
    const next = cur.presetTheme === "light" ? "dark" : "light";
    patchAndApply({ presetTheme: next });
  }, []);

  const toggleLocale = useCallback(() => {
    setLocale(locale === "zh" ? "en" : "zh");
  }, [locale, setLocale]);

  const openCommandPalette = useCallback(() => {
    closeFlyoutNow();
    document.dispatchEvent(new CustomEvent("yunque:open-command-palette"));
  }, [closeFlyoutNow]);

  useEffect(() => {
    const handler = () => closeFlyoutNow();
    document.addEventListener("yunque:open-command-palette", handler);
    return () => document.removeEventListener("yunque:open-command-palette", handler);
  }, [closeFlyoutNow]);

  const ui = useMemo(() => {
    const zh = locale === "zh";
    return {
      online: zh ? "在线" : "Online",
      offline: zh ? "离线" : "Offline",
      connecting: zh ? "连接中…" : "Connecting…",
      search: zh ? "搜索 (⌘K)" : "Search (⌘K)",
      settings: zh ? "设置" : "Settings",
      help: zh ? "帮助" : "Help",
      logout: zh ? "退出" : "Logout",
      locale: zh ? "English" : "中文",
      themeLight: zh ? "切换到亮色" : "Switch to Light",
      themeDark: zh ? "切换到暗色" : "Switch to Dark",
    };
  }, [locale]);

  const onlineColor =
    online === true
      ? "var(--yunque-success)"
      : online === false
      ? "var(--yunque-danger)"
      : "var(--yunque-text-muted)";

  return (
    <>
    <nav
      className="account-rail"
      data-sidebar
      aria-label={locale === "zh" ? "主导航" : "Main navigation"}
    >
      <div className="account-rail-top">
        <WindowControls />
        <div className="account-rail-divider" aria-hidden="true" />
        <Tooltip delay={0}>
          <Tooltip.Trigger>
            <button
              className="account-rail-avatar-wrap"
              onClick={() => router.push("/dashboard")}
              aria-label={locale === "zh" ? "概览" : "Overview"}
            >
              <Avatar
                size="sm"
                style={{ background: "linear-gradient(135deg, var(--yunque-accent), var(--yunque-success))" }}
              >
                <Avatar.Fallback className="text-white text-[10px] font-bold">YQ</Avatar.Fallback>
              </Avatar>
              <span
                className={online === true ? "online-dot" : ""}
                style={{
                  position: "absolute",
                  bottom: -1,
                  right: -1,
                  width: 8,
                  height: 8,
                  borderRadius: "50%",
                  background: onlineColor,
                  border: "2px solid var(--yunque-sidebar)",
                }}
              />
            </button>
          </Tooltip.Trigger>
          <Tooltip.Content placement="right">
            {online === true
              ? `${ui.online}${version ? ` · v${version}` : ""}`
              : online === false
              ? ui.offline
              : ui.connecting}
          </Tooltip.Content>
        </Tooltip>

        <Tooltip delay={0}>
          <Tooltip.Trigger>
            <Button
              size="sm"
              variant="ghost"
              isIconOnly
              className="account-rail-btn"
              onPress={openCommandPalette}
              aria-label={ui.search}
            >
              <Search size={16} />
            </Button>
          </Tooltip.Trigger>
          <Tooltip.Content placement="right">{ui.search}</Tooltip.Content>
        </Tooltip>

        <div className="account-rail-divider" aria-hidden="true" />

        {[...QUICK_LINKS, ...packItems.filter((item) => navigationPrefs.pinnedItems.includes(item.id)).map((item) => ({
          href: item.href,
          icon: item.icon,
          zh: item.label,
          en: item.label,
        }))].map((link) => {
          const active = pathname === link.href || (pathname?.startsWith(link.href + "/") ?? false);
          return (
            <Tooltip key={link.href} delay={0}>
              <Tooltip.Trigger>
                <Button
                  size="sm"
                  variant="ghost"
                  isIconOnly
                  className="account-rail-btn"
                  data-active={active || undefined}
                  onPress={() => router.push(link.href)}
                  aria-label={locale === "zh" ? link.zh : link.en}
                >
                  {link.icon}
                </Button>
              </Tooltip.Trigger>
              <Tooltip.Content placement="right">
                {locale === "zh" ? link.zh : link.en}
              </Tooltip.Content>
            </Tooltip>
          );
        })}

        {/* More: 全功能 hover/点击 触发器（专用入口，避免 hover 5 个快捷图标时误弹） */}
        <Tooltip delay={0}>
          <Tooltip.Trigger>
            <button
              ref={moreBtnRef}
              type="button"
              className="account-rail-btn account-rail-btn--more"
              data-active={flyoutOpen || undefined}
              onMouseEnter={scheduleOpenFlyout}
              onMouseLeave={scheduleCloseFlyout}
              onClick={() => {
                if (flyoutOpen) {
                  closeFlyoutNow();
                } else {
                  updateAnchor();
                  setFlyoutOpen(true);
                }
              }}
              aria-label={locale === "zh" ? "全部功能" : "All features"}
              aria-expanded={flyoutOpen}
            >
              <LayoutGrid size={16} />
            </button>
          </Tooltip.Trigger>
          <Tooltip.Content placement="right">
            {locale === "zh" ? "全部功能" : "All features"}
          </Tooltip.Content>
        </Tooltip>

      </div>

      <div className="account-rail-bottom">
        <Tooltip delay={0}>
          <Tooltip.Trigger>
            <Button
              size="sm"
              variant="ghost"
              isIconOnly
              className="account-rail-btn"
              onPress={toggleTheme}
              aria-label={themeMode === "light" ? ui.themeDark : ui.themeLight}
            >
              {themeMode === "light" ? <Moon size={16} /> : <Sun size={16} />}
            </Button>
          </Tooltip.Trigger>
          <Tooltip.Content placement="right">
            {themeMode === "light" ? ui.themeDark : ui.themeLight}
          </Tooltip.Content>
        </Tooltip>

        <Tooltip delay={0}>
          <Tooltip.Trigger>
            <Button
              size="sm"
              variant="ghost"
              isIconOnly
              className="account-rail-btn"
              onPress={toggleLocale}
              aria-label={ui.locale}
            >
              <Languages size={16} />
            </Button>
          </Tooltip.Trigger>
          <Tooltip.Content placement="right">{ui.locale}</Tooltip.Content>
        </Tooltip>

        <Tooltip delay={0}>
          <Tooltip.Trigger>
            <Button
              size="sm"
              variant="ghost"
              isIconOnly
              className="account-rail-btn"
              data-active={pathname?.startsWith("/settings") || undefined}
              onPress={() => router.push("/settings")}
              aria-label={ui.settings}
            >
              <Settings size={16} />
            </Button>
          </Tooltip.Trigger>
          <Tooltip.Content placement="right">{ui.settings}</Tooltip.Content>
        </Tooltip>

        <Tooltip delay={0}>
          <Tooltip.Trigger>
            <Button
              size="sm"
              variant="ghost"
              isIconOnly
              className="account-rail-btn account-rail-btn--danger"
              onPress={handleLogout}
              aria-label={ui.logout}
            >
              <LogOut size={16} />
            </Button>
          </Tooltip.Trigger>
          <Tooltip.Content placement="right">{ui.logout}</Tooltip.Content>
        </Tooltip>
      </div>
    </nav>

    <AccountRailFlyout
      open={flyoutOpen}
      extItems={[...packItems, ...extItems]}
      enabledPackIds={enabledPackIds}
      anchorTop={anchorTop}
      onMouseEnter={cancelCloseFlyout}
      onMouseLeave={scheduleCloseFlyout}
      onPick={closeFlyoutNow}
    />
    </>
  );
}
