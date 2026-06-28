"use client";

/**
 * Cherry-style settings modal.
 *
 * Mirrors Cherry Studio's settings panel: a left sidebar of sections, a right
 * content pane. All common settings live here and open *on top of* the chat
 * canvas without navigating anywhere. Heavy / technical dashboards (training,
 * audit trail, planner graph) are left untouched under the HeroUI Classic
 * workbench, reachable via the "Advanced" escape hatch at the bottom.
 *
 * The sections deliberately wire to backend APIs sparingly: enough to feel
 * alive (theme, data path, provider list, about), while more elaborate editors
 * (per-provider key management, MCP config, skill market) still live in the
 * Classic workbench to avoid duplicating their full UI here. In Cherry mode
 * you can still get to them in one click via the "Open advanced" button on
 * each section.
 */

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Switch, Select, ListBox, ToggleButton, ToggleButtonGroup, Label, Modal, TextField, Input } from "@heroui/react";
import {
  Settings as SettingsIcon,
  User,
  Copy,
  Trash2,
  Cpu,
  Database,
  KeyRound,
  Monitor,
  Moon,
  Sun,
  Sparkles,
  Palette,
  Globe,
  Keyboard,
  Info,
  Wrench,
  ExternalLink,
  Search,
  MousePointer2,
  Plug,
  Bell,
} from "lucide-react";
import { CherryModal } from "./overlay";
import { api } from "@/lib/api";
import type { VersionInfo } from "@/lib/api-types";
import { createPacksClient } from "yunque-client/packs";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { loadTheme, patchAndApply, THEME_STORAGE_KEY } from "@/lib/theme-engine";
import { GeneralConfigPanel } from "@/components/settings/general-config-panel";
import { ThemePanel } from "@/components/settings/theme-panel";
import { ProvidersPanel } from "@/components/settings/providers-panel";
import { NotificationsPanel } from "@/components/settings/notifications-panel";
import { showToast } from "@/components/toast-provider";

export interface CherrySettingsModalProps {
  open: boolean;
  onClose: () => void;
  initialSection?: SectionId;
}

const packsClient = createPacksClient(createYunqueSDKClientOptions());
const BACKUP_PACK_ID = "yunque.pack.backup";

type SectionId =
  | "account"
  | "general"
  | "models"
  | "defaults"
  | "display"
  | "desktop"
  | "system"
  | "data"
  | "connectors"
  | "notifications"
  | "search"
  | "hotkeys"
  | "about";

const NAV: Array<{ id: SectionId; label: string; icon: typeof SettingsIcon; group: "基础" | "功能" | "系统" }> = [
  { id: "account", label: "账户管理", icon: User, group: "基础" },
  { id: "general", label: "通用", icon: SettingsIcon, group: "基础" },
  { id: "display", label: "显示", icon: Palette, group: "基础" },
  { id: "models", label: "模型服务", icon: Cpu, group: "功能" },
  { id: "defaults", label: "默认模型", icon: Sparkles, group: "功能" },
  { id: "connectors", label: "频道接入", icon: Plug, group: "功能" },
  { id: "notifications", label: "通知推送", icon: Bell, group: "功能" },
  { id: "search", label: "网络搜索", icon: Globe, group: "功能" },
  { id: "desktop", label: "桌面助手", icon: MousePointer2, group: "系统" },
  { id: "system", label: "系统配置", icon: Wrench, group: "系统" },
  { id: "data", label: "数据", icon: Database, group: "系统" },
  { id: "hotkeys", label: "快捷键", icon: Keyboard, group: "系统" },
  { id: "about", label: "关于", icon: Info, group: "系统" },
];

const VALID_SECTIONS = new Set<SectionId>(NAV.map((n) => n.id));
function coerceSection(id: SectionId | undefined): SectionId {
  return id && VALID_SECTIONS.has(id) ? id : "account";
}

export function CherrySettingsModal({
  open,
  onClose,
  initialSection = "account",
}: CherrySettingsModalProps) {
  const [section, setSection] = useState<SectionId>(coerceSection(initialSection));

  useEffect(() => {
    if (open) setSection(coerceSection(initialSection));
  }, [open, initialSection]);

  const grouped = useMemo(() => {
    const out: Record<string, typeof NAV> = {};
    for (const n of NAV) {
      if (!out[n.group]) out[n.group] = [];
      out[n.group].push(n);
    }
    return out;
  }, []);

  return (
    <CherryModal
      open={open}
      onClose={onClose}
      size="xl"
      bodyFlush
      ariaLabel="设置"
      header={
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <SettingsIcon size={16} style={{ color: "var(--yunque-accent)" }} />
          <span style={{ fontSize: 14, fontWeight: 600, color: "var(--yunque-text)" }}>设置</span>
        </div>
      }
    >
      <div className="cherry-settings">
        <nav className="cherry-settings-nav" aria-label="Settings sections">
          {Object.entries(grouped).map(([group, items]) => (
            <div key={group}>
              <div className="cherry-settings-nav-group">{group}</div>
              {items.map((n) => {
                const Icon = n.icon;
                return (
                  <button
                    key={n.id}
                    type="button"
                    className={`cherry-settings-nav-item ${section === n.id ? "active" : ""}`}
                    onClick={() => setSection(n.id)}
                  >
                    <Icon size={14} />
                    <span>{n.label}</span>
                  </button>
                );
              })}
            </div>
          ))}
        </nav>
        <div className="cherry-settings-content">
          <div key={section} className="cherry-settings-pane">
            {section === "account" && <AccountSection />}
            {section === "general" && <GeneralSection />}
            {section === "display" && <DisplaySection />}
            {section === "models" && <ModelsSection onClose={onClose} />}
            {section === "defaults" && <DefaultsSection />}
            {section === "connectors" && <ConnectorsSection />}
            {section === "notifications" && <NotificationsPanel />}
            {section === "search" && <SearchSection />}
            {section === "desktop" && <DesktopSection />}
            {section === "system" && <SystemSection />}
            {section === "data" && <DataSection />}
            {section === "hotkeys" && <HotkeysSection />}
            {section === "about" && <AboutSection />}
          </div>
        </div>
      </div>
    </CherryModal>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 通用 (General)
   ══════════════════════════════════════════════════════════════ */

type Preset = "auto" | "dark" | "light";

function readPresetTheme(): Preset {
  if (typeof window === "undefined") return "auto";
  const val = loadTheme().presetTheme;
  return val === "dark" || val === "light" ? val : "auto";
}

function writePresetTheme(next: Preset) {
  if (typeof window === "undefined") return;
  // Re-applies the *whole* theme so translucent bg/overlay vars set by a
  // previous light/dark run don't bleed into the new mode. Previously we
  // only flipped the html class, which left `--yunque-bg` stuck on
  // `rgba(...,alpha)` from the old theme's interfaceBgImage config.
  patchAndApply({ presetTheme: next });
}

function GeneralSection() {
  const [hasLegacy, setHasLegacy] = useState(false);
  useEffect(() => {
    // Detect legacy classic-theme customizations that clash with the flat
    // look (bg image / reduced opacity / non-default radius). Show the
    // "restore defaults" shortcut only when it would actually do something.
    const cfg = loadTheme();
    setHasLegacy(
      !!cfg.interfaceBgImage ||
      cfg.contentOpacity < 100 ||
      cfg.sidebarOpacity < 100 ||
      (cfg.radius !== "default" && cfg.radius !== "medium"),
    );
  }, []);

  const restoreCherryDefaults = () => {
    patchAndApply({
      interfaceBgImage: null,
      interfaceBgOpacity: 30,
      interfaceBgBlur: 8,
      contentOpacity: 100,
      sidebarOpacity: 100,
      radius: "default",
    });
    setHasLegacy(false);
  };

  return (
    <>
      <h2 className="cherry-settings-title">通用</h2>
      <p className="cherry-settings-subtitle">主题与外观定制</p>

      <div className="cherry-settings-section">
        {hasLegacy && (
          <div className="cherry-settings-row">
            <div>
              <div className="cherry-settings-row-label">还原默认外观</div>
              <div className="cherry-settings-row-desc">
                检测到经典工作台保存过的壁纸 / 半透明度 / 圆角设置。默认是无壁纸的纯净背景。
              </div>
            </div>
            <div className="cherry-settings-row-control">
              <Button size="sm" variant="ghost" onPress={restoreCherryDefaults}>
                一键还原
              </Button>
            </div>
          </div>
        )}
      </div>

      <ThemePanel />
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 显示 (Display)
   ══════════════════════════════════════════════════════════════ */

import { Avatar, Chip, Slider } from "@heroui/react";
import { usePreferences } from "@/lib/user-preferences";
import { useUserProfile, profileInitial, setNickname, setAvatar, fileToAvatarDataUrl } from "@/lib/user-profile";

const DENSITY_KEY = "yunque_chat_density";
type Density = "cozy" | "compact";

function DisplaySection() {
  const { preferences, updatePreferences } = usePreferences();
  const [density, setDensity] = useState<Density>("cozy");
  
  // Local state for immediate feedback
  const [fontScale, setFontScale] = useState(
    preferences?.interface?.fontSize === "small" ? 0 :
    preferences?.interface?.fontSize === "large" ? 2 : 1
  );

  useEffect(() => {
    if (typeof window === "undefined") return;
    setDensity((localStorage.getItem(DENSITY_KEY) as Density) || "cozy");
  }, []);
  
  useEffect(() => {
    const val = preferences?.interface?.fontSize;
    setFontScale(val === "small" ? 0 : val === "large" ? 2 : 1);
  }, [preferences?.interface?.fontSize]);

  const applyDensity = (d: Density) => {
    setDensity(d);
    localStorage.setItem(DENSITY_KEY, d);
    document.documentElement.setAttribute("data-density", d);
  };

  const handleFontChange = (val: number | number[]) => {
    const v = Array.isArray(val) ? val[0] : val;
    setFontScale(v);
    const fontSize = v === 0 ? "small" : v === 2 ? "large" : "default";
    updatePreferences("interface", { fontSize });
    
    // Dispatch custom event to trigger immediate update in AppShell
    window.dispatchEvent(new Event("yunque:preferences-updated"));
  };

  return (
    <>
      <h2 className="cherry-settings-title">显示</h2>
      <p className="cherry-settings-subtitle">消息排版、字号、头像显示</p>

      <div className="cherry-settings-section">
        <div className="cherry-settings-row">
          <div style={{ flex: 1, paddingRight: 32 }}>
            <div className="cherry-settings-row-label">全局字号</div>
            <div className="cherry-settings-row-desc mb-6">调整界面和聊天消息的文字大小。</div>
            <Slider
              className="w-full"
              step={1}
              maxValue={2} 
              minValue={0} 
              value={fontScale}
              onChange={handleFontChange}
              aria-label="全局字号"
            >
              <Slider.Track className="h-1.5 rounded-full bg-[var(--yunque-bg-muted)]">
                <Slider.Fill className="bg-[var(--yunque-accent)]" />
                <Slider.Thumb className="size-4 rounded-full border-2 border-[var(--yunque-surface-1)] bg-[var(--yunque-accent)]" />
              </Slider.Track>
            </Slider>
            <div className="mt-3 flex justify-between text-xs" style={{ color: "var(--yunque-text-muted)" }}>
              <span>小</span>
              <span>标准</span>
              <span>大</span>
            </div>
          </div>
        </div>

        <div className="cherry-settings-row">
          <div>
            <div className="cherry-settings-row-label">消息密度</div>
            <div className="cherry-settings-row-desc">紧凑模式会压缩段落间距、缩小头像。</div>
          </div>
          <div className="cherry-settings-row-control">
            <ToggleButtonGroup
              selectionMode="single"
              disallowEmptySelection
              selectedKeys={new Set([density])}
              onSelectionChange={(keys) => { const k = [...keys][0]; if (k) applyDensity(String(k) as "cozy" | "compact"); }}
              aria-label="消息密度"
            >
              <ToggleButton id="cozy" variant="ghost">宽松</ToggleButton>
              <ToggleButton id="compact" variant="ghost"><ToggleButtonGroup.Separator />紧凑</ToggleButton>
            </ToggleButtonGroup>
          </div>
        </div>
      </div>
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 模型服务 (Models)
   ══════════════════════════════════════════════════════════════ */

function ModelsSection({ onClose }: { onClose: () => void }) {
  return (
    <>
      <h2 className="cherry-settings-title">模型服务</h2>
      <p className="cherry-settings-subtitle">接入模式、提供商密钥、Tori 中转与路由</p>
      <ProvidersPanel onNavigateChat={onClose} />
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 默认模型 (Defaults)
   ══════════════════════════════════════════════════════════════ */

function DefaultsSection() {
  return (
    <>
      <h2 className="cherry-settings-title">默认模型</h2>
      <p className="cherry-settings-subtitle">向量嵌入模型与多模型池（快速 / 专家）配置</p>
      <GeneralConfigPanel includeGroups={["embedding", "multimodel"]} hideToolbar showGroupHeaders />
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 网络搜索 (Search)
   MCP and Memory sections were removed from settings: MCP is a power-user /
   Cogni concern (lives at /mcp), and Memory is a main-path surface reachable
   from the rail/dashboard — neither belongs as a settings jump-out.
   ══════════════════════════════════════════════════════════════ */

const SEARCH_KEY = "yunque_web_search_enabled";

function SearchSection() {
  const [enabled, setEnabled] = useState(false);
  useEffect(() => {
    if (typeof window === "undefined") return;
    setEnabled(localStorage.getItem(SEARCH_KEY) === "1");
  }, []);
  const toggle = () => {
    const next = !enabled;
    setEnabled(next);
    localStorage.setItem(SEARCH_KEY, next ? "1" : "0");
  };

  return (
    <>
      <h2 className="cherry-settings-title">网络搜索</h2>
      <p className="cherry-settings-subtitle">让模型在需要时主动搜索互联网，并配置搜索引擎</p>

      <div className="cherry-settings-section">
        <div className="cherry-settings-row">
          <div>
            <div className="cherry-settings-row-label">允许网络搜索</div>
            <div className="cherry-settings-row-desc">
              开启后，输入栏地球图标会变成激活态，工具调用会出现「web_search」节点。
            </div>
          </div>
          <div className="cherry-settings-row-control">
            <Switch isSelected={enabled} onChange={toggle} aria-label="允许网络搜索">
              <Switch.Control><Switch.Thumb /></Switch.Control>
            </Switch>
          </div>
        </div>
      </div>

      {/* Real engine config — SEARXNG_URL etc. live in the schema's "other"
          group; surface them here so search is actually configurable, not
          just a hollow on/off. */}
      <GeneralConfigPanel includeGroups={["other"]} hideToolbar showGroupHeaders={false} />
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 数据 (Data)
   ══════════════════════════════════════════════════════════════ */

function DataSection() {
  const router = useRouter();
  const [systemInfo, setSystemInfo] = useState<{ data_dir?: string; db_size_mb?: number } | null>(null);
  const [backupPackStatus, setBackupPackStatus] = useState<"enabled" | "disabled" | "missing" | "loading">("loading");

  useEffect(() => {
    api
      .systemInfo()
      .then((info) => setSystemInfo(info as { data_dir?: string; db_size_mb?: number }))
      .catch(() => setSystemInfo(null));
    let alive = true;
    packsClient
      .installed()
      .then((res) => {
        if (!alive) return;
        const pack = res.packs.find((item) => item.manifest.id === BACKUP_PACK_ID);
        setBackupPackStatus(pack?.status === "enabled" ? "enabled" : pack ? "disabled" : "missing");
      })
      .catch(() => {
        if (alive) setBackupPackStatus("missing");
      });
    return () => {
      alive = false;
    };
  }, []);

  return (
    <>
      <h2 className="cherry-settings-title">数据</h2>
      <p className="cherry-settings-subtitle">本地数据目录、备份与恢复</p>

      <div className="cherry-settings-section">
        <div className="cherry-settings-row">
          <div>
            <div className="cherry-settings-row-label">数据目录</div>
            <div className="cherry-settings-row-desc" style={{ fontFamily: "ui-monospace, monospace", wordBreak: "break-all" }}>
              {systemInfo?.data_dir || "（加载中…）"}
            </div>
          </div>
          <div className="cherry-settings-row-control">
            <Button
              size="sm"
              variant="ghost"
              isDisabled={!systemInfo?.data_dir}
              onPress={() => { if (systemInfo?.data_dir) navigator.clipboard?.writeText(systemInfo.data_dir); }}
            >
              复制路径
            </Button>
          </div>
        </div>

        <div className="cherry-settings-row">
          <div>
            <div className="cherry-settings-row-label">备份恢复 Pack</div>
            <div className="cherry-settings-row-desc">
              备份能力属于可选能力包。主设置只显示状态和入口，导入、导出、回滚由 Pack Runtime 页面承载。
              <div style={{ marginTop: 4, color: backupPackStatus === "enabled" ? "var(--yunque-accent)" : "var(--yunque-text-muted)" }}>
                当前状态：{backupPackStatus === "loading" ? "检查中…" : backupPackStatus === "enabled" ? "已启用" : backupPackStatus === "disabled" ? "已安装但未启用" : "未安装"}
              </div>
            </div>
          </div>
          <div className="cherry-settings-row-control">
            <Button size="sm" className="btn-accent" onPress={() => router.push(backupPackStatus === "missing" ? "/packs" : "/packs/backup")}>
              {backupPackStatus === "missing" ? "安装 Pack" : "打开备份 Pack"}
            </Button>
          </div>
        </div>
      </div>
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 快捷键 (Hotkeys)
   ══════════════════════════════════════════════════════════════ */

function HotkeysSection() {
  const rows: Array<[string, string]> = [
    ["Ctrl / Cmd + N", "新建话题"],
    ["Ctrl / Cmd + K", "命令面板"],
    ["Ctrl / Cmd + ,", "打开设置"],
    ["Enter", "发送"],
    ["Shift + Enter", "换行"],
    ["Esc", "关闭弹窗"],
  ];
  return (
    <>
      <h2 className="cherry-settings-title">快捷键</h2>
      <p className="cherry-settings-subtitle">常用操作的键盘快捷</p>
      <div className="cherry-settings-section">
        {rows.map(([k, v], i) => (
          <div key={k} className="cherry-settings-row" style={{ borderTopColor: i === 0 ? "transparent" : undefined }}>
            <div>
              <div className="cherry-settings-row-label">{v}</div>
            </div>
            <div className="cherry-settings-row-control">
              <code
                style={{
                  fontSize: 12,
                  padding: "3px 8px",
                  borderRadius: 6,
                  background: "var(--yunque-bg-muted)",
                  border: "1px solid var(--yunque-border)",
                  color: "var(--yunque-text)",
                  fontFamily: "ui-monospace, monospace",
                }}
              >
                {k}
              </code>
            </div>
          </div>
        ))}
      </div>
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 关于 (About)
   ══════════════════════════════════════════════════════════════ */

function AboutSection() {
  const [version, setVersion] = useState<VersionInfo | null>(null);
  useEffect(() => {
    api.version().then(setVersion).catch(() => setVersion(null));
  }, []);

  return (
    <>
      <h2 className="cherry-settings-title">关于云雀 Agent</h2>
      <p className="cherry-settings-subtitle">版本与环境信息</p>

      <div className="cherry-settings-section">
        <div className="cherry-settings-row">
          <div className="cherry-settings-row-label">应用版本</div>
          <div className="cherry-settings-row-control">
            <code
              style={{
                fontSize: 12,
                padding: "3px 8px",
                borderRadius: 6,
                background: "var(--yunque-bg-muted)",
                color: "var(--yunque-text)",
                fontFamily: "ui-monospace, monospace",
              }}
            >
              {version?.version || "0.1.0-dev"}
            </code>
          </div>
        </div>
        <div className="cherry-settings-row">
          <div className="cherry-settings-row-label">构建</div>
          <div className="cherry-settings-row-control">
            <code
              style={{
                fontSize: 12,
                padding: "3px 8px",
                borderRadius: 6,
                background: "var(--yunque-bg-muted)",
                color: "var(--yunque-text)",
                fontFamily: "ui-monospace, monospace",
              }}
            >
              {version?.git_commit?.slice(0, 7) || "unknown"} · {version?.go_version || ""}
            </code>
          </div>
        </div>
        <div className="cherry-settings-row">
          <div className="cherry-settings-row-label">平台</div>
          <div className="cherry-settings-row-control">
            <code
              style={{
                fontSize: 12,
                padding: "3px 8px",
                borderRadius: 6,
                background: "var(--yunque-bg-muted)",
                color: "var(--yunque-text)",
                fontFamily: "ui-monospace, monospace",
              }}
            >
              {version?.os || "?"}/{version?.arch || "?"}
            </code>
          </div>
        </div>
      </div>
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Helpers
   ══════════════════════════════════════════════════════════════ */

function OpenInAdvancedInline({
  path,
  onClose,
  label = "详细设置",
}: {
  path: string;
  onClose: () => void;
  label?: string;
}) {
  const router = useRouter();
  return (
    <Button size="sm" variant="ghost" onPress={() => { onClose(); router.push(path); }}>
      <Wrench size={12} />
      {label}
      <ExternalLink size={11} style={{ marginLeft: 4, opacity: 0.6 }} />
    </Button>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 系统配置 (System) — true system-level .env config only
   ══════════════════════════════════════════════════════════════ */

function SystemSection() {
  // Terminal users don't touch JWT/RateLimit/CORS (security) or DB path/mode
  // (storage) — those are self-host .env concerns and stay out of the desktop
  // UI (the backend schema still defines them so .env round-trips work).
  // filesystem/sandbox_cloud/advanced/other will move to their owning sections
  // / Packs per SETTINGS-PACK-DELEGATION-DESIGN; kept here until then.
  return (
    <>
      <h2 className="cherry-settings-title">系统配置</h2>
      <p className="cherry-settings-subtitle">文件系统、云沙箱、心跳等运行参数</p>
      <GeneralConfigPanel includeGroups={["filesystem", "sandbox_cloud", "advanced"]} />
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 频道接入 (Connectors) — channel tokens (Telegram, Feishu…)
   ══════════════════════════════════════════════════════════════ */

function ConnectorsSection() {
  return (
    <>
      <h2 className="cherry-settings-title">频道接入</h2>
      <p className="cherry-settings-subtitle">把云雀接到 Telegram / 飞书 / Discord / Slack / QQ 等渠道</p>
      <GeneralConfigPanel includeGroups={["channels"]} hideToolbar showGroupHeaders={false} />
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 桌面助手 (Desktop) — OS-level helpers, desktop-only
   ══════════════════════════════════════════════════════════════ */

function invokeDesktop<T>(cmd: string, args?: Record<string, unknown>): Promise<T | null> {
  if (typeof window === "undefined") return Promise.resolve(null);
  const invoke = (window as unknown as { __TAURI_INTERNALS__?: { invoke?: (c: string, a?: Record<string, unknown>) => Promise<unknown> } }).__TAURI_INTERNALS__?.invoke;
  if (!invoke) return Promise.resolve(null);
  return invoke(cmd, args).then((v) => v as T).catch(() => null);
}

function DesktopSection() {
  const [isDesktop, setIsDesktop] = useState(false);
  const [selection, setSelection] = useState<boolean | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const desktop = typeof window !== "undefined" && Boolean((window as unknown as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__);
    setIsDesktop(desktop);
    if (desktop) {
      invokeDesktop<boolean>("get_selection_assistant_enabled").then((v) => setSelection(typeof v === "boolean" ? v : false));
    }
  }, []);

  const toggleSelection = async (next: boolean) => {
    if (saving) return;
    setSaving(true);
    await invokeDesktop("set_selection_assistant_enabled", { enabled: next });
    setSelection(next);
    showToast(next ? "已开启划词助手" : "已关闭划词助手", "success");
    setSaving(false);
  };

  return (
    <>
      <h2 className="cherry-settings-title">桌面助手</h2>
      <p className="cherry-settings-subtitle">在云雀窗口之外工作的能力，默认关闭</p>
      <div className="cherry-settings-section">
        {!isDesktop ? (
          <div className="cherry-settings-row-desc">这些能力只在桌面客户端可用。</div>
        ) : (
          <div className="cherry-settings-row">
            <div style={{ minWidth: 0, flex: 1 }}>
              <div className="cherry-settings-row-label">划词助手</div>
              <div className="cherry-settings-row-desc">
                在任意应用里选中文字后弹出云雀工具栏（搜索 / 翻译 / 解释 / 存储）。
                开启会监听全局鼠标划选；关闭则只在云雀窗口内生效。
              </div>
            </div>
            <div className="cherry-settings-row-control">
              <Switch
                isSelected={selection === true}
                isDisabled={selection === null || saving}
                onChange={(v) => toggleSelection(Boolean(v))}
                aria-label="划词助手开关"
              >
                <Switch.Control><Switch.Thumb /></Switch.Control>
              </Switch>
            </div>
          </div>
        )}
      </div>
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 账户与仪表盘 (Account)
   ══════════════════════════════════════════════════════════════ */

import type { CostSummary, SystemInfo as SysInfo } from "@/lib/api";

function AccountSection() {
  const [costSummary, setCostSummary] = useState<CostSummary | null>(null);
  const [sysInfo, setSysInfo] = useState<SysInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const profile = useUserProfile();
  const avatarInputRef = useRef<HTMLInputElement>(null);
  const displayName = profile.nickname || "云雀本地开发者";
  const [nameOpen, setNameOpen] = useState(false);
  const [nameDraft, setNameDraft] = useState("");
  const editName = () => {
    setNameDraft(profile.nickname || "");
    setNameOpen(true);
  };
  const saveName = () => {
    setNickname(nameDraft);
    setNameOpen(false);
  };
  const pickAvatar = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    try {
      setAvatar(await fileToAvatarDataUrl(file));
    } catch {
      /* ignore decode errors */
    }
  };

  useEffect(() => {
    let alive = true;
    Promise.all([
      api.costSummary().catch(() => null),
      api.systemInfo().catch(() => null),
    ]).then(([cost, sys]) => {
      if (!alive) return;
      setCostSummary(cost);
      setSysInfo(sys);
      setLoading(false);
    });
    return () => { alive = false; };
  }, []);

  return (
    <>
      <h2 className="cherry-settings-title">账户管理</h2>
      <p className="cherry-settings-subtitle">查看身份信息、运行资源与配额</p>

      <div className="cherry-settings-section" style={{ padding: 0, background: "transparent", border: "none", marginTop: 24 }}>
        
        {/* User Card */}
        <div style={{ background: "var(--yunque-surface-1)", borderRadius: 12, padding: "20px 24px", display: "flex", justifyContent: "space-between", alignItems: "center", border: "1px solid var(--yunque-border)" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
            <button type="button" onClick={() => avatarInputRef.current?.click()} title="更换头像" style={{ borderRadius: "9999px", cursor: "pointer", lineHeight: 0 }}>
              <Avatar className="size-12 text-large bg-[#4caf50]">
                {profile.avatar && <Avatar.Image alt={displayName} src={profile.avatar} />}
                <Avatar.Fallback>{profileInitial(profile.nickname)}</Avatar.Fallback>
              </Avatar>
            </button>
            <input ref={avatarInputRef} type="file" accept="image/*" hidden onChange={pickAvatar} />
            <div>
              <div style={{ fontSize: 16, fontWeight: 600, color: "var(--yunque-text)" }}>{displayName}</div>
              <div style={{ fontSize: 13, color: "var(--yunque-text-muted)" }}>Workspace Owner</div>
            </div>
          </div>
          <Button size="sm" variant="ghost" onPress={editName}>管理身份</Button>
        </div>

        {/* 改名 Modal（HeroUI 原生，替代浏览器 prompt）。
            z-index 高于 cherry 设置弹窗(10000)，否则会被压在它下面一层。 */}
        <Modal.Backdrop isOpen={nameOpen} onOpenChange={setNameOpen} variant="blur" style={{ zIndex: 10001 }}>
          <Modal.Container placement="center" size="sm">
            <Modal.Dialog>
              <Modal.CloseTrigger />
              <Modal.Header>
                <Modal.Heading>设置称呼</Modal.Heading>
              </Modal.Header>
              <Modal.Body>
                <TextField value={nameDraft} onChange={setNameDraft} autoFocus>
                  <Label>怎么称呼你？</Label>
                  <Input placeholder="例如：夏鸢" onKeyDown={(e) => { if (e.key === "Enter") saveName(); }} />
                </TextField>
              </Modal.Body>
              <Modal.Footer>
                <Button variant="ghost" slot="close">取消</Button>
                <Button className="btn-accent" onPress={saveName}>保存</Button>
              </Modal.Footer>
            </Modal.Dialog>
          </Modal.Container>
        </Modal.Backdrop>

        {/* Dashboard / Credits Card — theme-token colors so it works in both
            light and dark; data is real (costSummary / sysInfo). */}
        <div style={{ marginTop: 16, background: "var(--yunque-surface-1)", borderRadius: 12, border: "1px solid var(--yunque-border)", overflow: "hidden", color: "var(--yunque-text)" }}>

          <div style={{ padding: "16px 20px", display: "flex", justifyContent: "space-between", alignItems: "center", borderBottom: "1px solid var(--yunque-border)" }}>
            <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
              <span style={{ fontSize: 15, fontWeight: 600, color: "var(--yunque-text)" }}>API 运行大盘</span>
              <Chip size="sm" variant="soft">本地版</Chip>
            </div>
          </div>

          <div style={{ padding: "20px", display: "flex", flexDirection: "column", gap: 24 }}>
            {/* Total Cost */}
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-end" }}>
              <div>
                <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 8 }}>
                  <Sparkles size={14} style={{ color: "var(--yunque-accent)" }} />
                  <span style={{ fontSize: 13, color: "var(--yunque-text-muted)" }}>本月预估成本 (USD)</span>
                </div>
                <div style={{ color: "var(--yunque-text-muted)", fontSize: 12 }}>
                  包含所有后端 LLM 的流式输出。
                </div>
              </div>
              <div style={{ textAlign: "right" }}>
                <div style={{ fontSize: 12, color: "var(--yunque-text-muted)", marginBottom: 4 }}>累计支出</div>
                <div style={{ fontSize: 24, fontWeight: 700, fontFamily: "ui-monospace, monospace", lineHeight: 1, color: "var(--yunque-text)" }}>
                  ${costSummary ? costSummary.total_cost_usd?.toFixed(4) : "0.0000"}
                </div>
              </div>
            </div>

            {/* Packages */}
            <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
              <div style={{ display: "flex", justifyContent: "space-between", fontSize: 13 }}>
                <div>
                  <div style={{ fontWeight: 600, marginBottom: 4, color: "var(--yunque-text)" }}>基础模型调用</div>
                  <div style={{ color: "var(--yunque-text-muted)", fontSize: 11 }}>通过云雀 Agent 产生的 API 调用总数</div>
                </div>
                <div style={{ textAlign: "right", fontFamily: "ui-monospace, monospace", color: "var(--yunque-text)" }}>
                  <div style={{ color: "var(--yunque-text-muted)", marginBottom: 2 }}>总量: 无限制</div>
                  <div style={{ fontWeight: 600 }}>已用: {costSummary ? costSummary.total_calls?.toLocaleString() : "0"}</div>
                </div>
              </div>

              <div style={{ display: "flex", justifyContent: "space-between", fontSize: 13 }}>
                <div>
                  <div style={{ fontWeight: 600, marginBottom: 4, color: "var(--yunque-text)" }}>系统资源占用</div>
                  <div style={{ color: "var(--yunque-text-muted)", fontSize: 11 }}>Agent 后台进程 (Go)</div>
                </div>
                <div style={{ textAlign: "right", fontFamily: "ui-monospace, monospace", color: "var(--yunque-text)" }}>
                  <div style={{ color: "var(--yunque-text-muted)", marginBottom: 2 }}>内存: {sysInfo ? sysInfo.memory_mb?.toLocaleString() : "0"} MB</div>
                  <div style={{ fontWeight: 600 }}>CPU: {sysInfo ? sysInfo.cpu_count : "0"} 核</div>
                </div>
              </div>
            </div>

          </div>
        </div>

      </div>
    </>
  );
}
