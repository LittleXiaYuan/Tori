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

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Settings as SettingsIcon,
  Cpu,
  Database,
  KeyRound,
  Monitor,
  Moon,
  Sun,
  Sparkles,
  Palette,
  Brain,
  Network,
  Globe,
  Keyboard,
  Info,
  Wrench,
  ExternalLink,
  Search,
} from "lucide-react";
import { CherryModal } from "./overlay";
import { api } from "@/lib/api";
import type { VersionInfo } from "@/lib/api-types";
import { createPacksClient } from "yunque-client/packs";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { loadTheme, patchAndApply, THEME_STORAGE_KEY } from "@/lib/theme-engine";

export interface CherrySettingsModalProps {
  open: boolean;
  onClose: () => void;
  initialSection?: SectionId;
}

const packsClient = createPacksClient(createYunqueSDKClientOptions());
const BACKUP_PACK_ID = "yunque.pack.backup";

type SectionId =
  | "general"
  | "models"
  | "defaults"
  | "display"
  | "data"
  | "mcp"
  | "memory"
  | "search"
  | "hotkeys"
  | "about";

const NAV: Array<{ id: SectionId; label: string; icon: typeof SettingsIcon; group: "基础" | "功能" | "系统" }> = [
  { id: "general", label: "通用", icon: SettingsIcon, group: "基础" },
  { id: "display", label: "显示", icon: Palette, group: "基础" },
  { id: "models", label: "模型服务", icon: Cpu, group: "功能" },
  { id: "defaults", label: "默认模型", icon: Sparkles, group: "功能" },
  { id: "mcp", label: "MCP 服务器", icon: Network, group: "功能" },
  { id: "search", label: "网络搜索", icon: Globe, group: "功能" },
  { id: "memory", label: "全局记忆", icon: Brain, group: "功能" },
  { id: "data", label: "数据", icon: Database, group: "系统" },
  { id: "hotkeys", label: "快捷键", icon: Keyboard, group: "系统" },
  { id: "about", label: "关于", icon: Info, group: "系统" },
];

export function CherrySettingsModal({
  open,
  onClose,
  initialSection = "general",
}: CherrySettingsModalProps) {
  const [section, setSection] = useState<SectionId>(initialSection);

  useEffect(() => {
    if (open) setSection(initialSection);
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
          {section === "general" && <GeneralSection />}
          {section === "display" && <DisplaySection />}
          {section === "models" && <ModelsSection onClose={onClose} />}
          {section === "defaults" && <DefaultsSection onClose={onClose} />}
          {section === "mcp" && <MCPSection onClose={onClose} />}
          {section === "search" && <SearchSection />}
          {section === "memory" && <MemorySection onClose={onClose} />}
          {section === "data" && <DataSection />}
          {section === "hotkeys" && <HotkeysSection />}
          {section === "about" && <AboutSection />}
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
  const [theme, setTheme] = useState<Preset>("auto");
  const [hasLegacy, setHasLegacy] = useState(false);
  useEffect(() => {
    setTheme(readPresetTheme());
    // Detect legacy classic-theme customizations that clash with Cherry's
    // flat look (bg image / reduced opacity / non-default radius). Show the
    // "restore Cherry defaults" shortcut only when it would actually do
    // something.
    const cfg = loadTheme();
    setHasLegacy(
      !!cfg.interfaceBgImage ||
      cfg.contentOpacity < 100 ||
      cfg.sidebarOpacity < 100 ||
      (cfg.radius !== "default" && cfg.radius !== "medium"),
    );
  }, []);

  const select = (next: Preset) => {
    setTheme(next);
    writePresetTheme(next);
  };

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
      <p className="cherry-settings-subtitle">语言、外观主题、首屏行为</p>

      <div className="cherry-settings-section">
        <div className="cherry-settings-row">
          <div>
            <div className="cherry-settings-row-label">外观主题</div>
            <div className="cherry-settings-row-desc">跟随系统切换深浅色，或手动锁定。</div>
          </div>
          <div className="cherry-settings-row-control">
            <div className="cherry-segmented">
              <button type="button" className={theme === "auto" ? "active" : ""} onClick={() => select("auto")}>
                <Monitor size={12} style={{ marginRight: 4, verticalAlign: "-2px" }} />
                系统
              </button>
              <button type="button" className={theme === "light" ? "active" : ""} onClick={() => select("light")}>
                <Sun size={12} style={{ marginRight: 4, verticalAlign: "-2px" }} />
                浅色
              </button>
              <button type="button" className={theme === "dark" ? "active" : ""} onClick={() => select("dark")}>
                <Moon size={12} style={{ marginRight: 4, verticalAlign: "-2px" }} />
                深色
              </button>
            </div>
          </div>
        </div>
        <div className="cherry-settings-row">
          <div>
            <div className="cherry-settings-row-label">界面语言</div>
            <div className="cherry-settings-row-desc">当前：简体中文（其他语言即将推出）</div>
          </div>
          <div className="cherry-settings-row-control">
            <select className="cherry-select" defaultValue="zh-CN" style={{ width: 140 }}>
              <option value="zh-CN">简体中文</option>
              <option value="en-US" disabled>English（WIP）</option>
            </select>
          </div>
        </div>
        {hasLegacy && (
          <div className="cherry-settings-row">
            <div>
              <div className="cherry-settings-row-label">还原 Cherry 默认外观</div>
              <div className="cherry-settings-row-desc">
                检测到经典工作台中保存过的壁纸 / 半透明度 / 圆角设置。Cherry 默认是无壁纸的纯净背景。
              </div>
            </div>
            <div className="cherry-settings-row-control">
              <button type="button" className="cherry-btn" onClick={restoreCherryDefaults}>
                一键还原
              </button>
            </div>
          </div>
        )}
      </div>

      <OpenInAdvancedHint label="更多主题定制（颜色、圆角、壁纸）" path="/settings/theme" />
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 显示 (Display)
   ══════════════════════════════════════════════════════════════ */

const DENSITY_KEY = "yunque_chat_density";
type Density = "cozy" | "compact";

function DisplaySection() {
  const [density, setDensity] = useState<Density>("cozy");
  useEffect(() => {
    if (typeof window === "undefined") return;
    setDensity((localStorage.getItem(DENSITY_KEY) as Density) || "cozy");
  }, []);
  const applyDensity = (d: Density) => {
    setDensity(d);
    localStorage.setItem(DENSITY_KEY, d);
    document.documentElement.setAttribute("data-density", d);
  };

  return (
    <>
      <h2 className="cherry-settings-title">显示</h2>
      <p className="cherry-settings-subtitle">消息排版、字号、头像显示</p>

      <div className="cherry-settings-section">
        <div className="cherry-settings-row">
          <div>
            <div className="cherry-settings-row-label">消息密度</div>
            <div className="cherry-settings-row-desc">紧凑模式会压缩段落间距、缩小头像。</div>
          </div>
          <div className="cherry-settings-row-control">
            <div className="cherry-segmented">
              <button type="button" className={density === "cozy" ? "active" : ""} onClick={() => applyDensity("cozy")}>
                宽松
              </button>
              <button type="button" className={density === "compact" ? "active" : ""} onClick={() => applyDensity("compact")}>
                紧凑
              </button>
            </div>
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
  const router = useRouter();
  const [mode, setMode] = useState<string>("smart");
  const [presets, setPresets] = useState<Array<{ id: string; name: string }>>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const [m, p] = await Promise.all([
          api.providerMode().catch(() => ({ mode: "smart" })),
          api.providerPresets().catch(() => ({ presets: [] })),
        ]);
        if (!alive) return;
        setMode(m.mode);
        setPresets(
          (p.presets || []).map((x) => ({
            id: x.id,
            name: x.name || x.id,
          }))
        );
      } finally {
        if (alive) setLoading(false);
      }
    })();
    return () => {
      alive = false;
    };
  }, []);

  return (
    <>
      <h2 className="cherry-settings-title">模型服务</h2>
      <p className="cherry-settings-subtitle">选择接入模式、管理提供商密钥与 Tori 中转绑定</p>

      <div className="cherry-settings-section">
        <div className="cherry-settings-row">
          <div>
            <div className="cherry-settings-row-label">接入模式</div>
            <div className="cherry-settings-row-desc">
              智能混合：优先直连，故障自动回退 Tori。
            </div>
          </div>
          <div className="cherry-settings-row-control">
            <div className="cherry-segmented">
              {["direct", "tori", "smart"].map((m) => (
                <button
                  key={m}
                  type="button"
                  className={mode === m ? "active" : ""}
                  onClick={async () => {
                    const next = await api.setProviderMode(m).catch(() => null);
                    if (next) setMode(next.mode);
                  }}
                >
                  {m === "direct" ? "自带 Key" : m === "tori" ? "Tori 中转" : "智能混合"}
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className="cherry-settings-section">
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 10 }}>
          <div style={{ fontSize: 13, fontWeight: 500, color: "var(--yunque-text)" }}>
            内置提供商预设（{loading ? "…" : presets.length}）
          </div>
          <button
            type="button"
            className="cherry-btn"
            onClick={() => {
              onClose();
              router.push("/settings/providers");
            }}
          >
            <KeyRound size={12} />
            管理密钥
            <ExternalLink size={11} style={{ marginLeft: 4, opacity: 0.6 }} />
          </button>
        </div>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
          {loading && <span style={{ color: "var(--yunque-text-muted)", fontSize: 12 }}>加载中…</span>}
          {!loading && presets.length === 0 && (
            <span style={{ color: "var(--yunque-text-muted)", fontSize: 12 }}>
              暂无预设，点「管理密钥」去添加。
            </span>
          )}
          {presets.slice(0, 14).map((p) => (
            <span
              key={p.id}
              style={{
                fontSize: 11.5,
                padding: "3px 8px",
                borderRadius: 999,
                background: "var(--yunque-bg-muted)",
                color: "var(--yunque-text-secondary)",
              }}
            >
              {p.name}
            </span>
          ))}
          {!loading && presets.length > 14 && (
            <span style={{ fontSize: 11.5, color: "var(--yunque-text-muted)", padding: "3px 4px" }}>
              +{presets.length - 14}
            </span>
          )}
        </div>
      </div>

      <OpenInAdvancedHint label="详细的 Provider / Tori / 配额管理" path="/settings/providers" />
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 默认模型 (Defaults)
   ══════════════════════════════════════════════════════════════ */

function DefaultsSection({ onClose }: { onClose: () => void }) {
  return (
    <>
      <h2 className="cherry-settings-title">默认模型</h2>
      <p className="cherry-settings-subtitle">为不同场景指派默认模型（会话 / 代码 / 视觉）</p>

      <div className="cherry-settings-section">
        <div className="cherry-settings-row">
          <div>
            <div className="cherry-settings-row-label">分场景路由</div>
            <div className="cherry-settings-row-desc">
              Cherry 目前把所有场景走同一主模型；细粒度分配在高级工作台里配置。
            </div>
          </div>
          <div className="cherry-settings-row-control">
            <OpenInAdvancedInline path="/settings/models" onClose={onClose} />
          </div>
        </div>
      </div>
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: MCP
   ══════════════════════════════════════════════════════════════ */

function MCPSection({ onClose }: { onClose: () => void }) {
  return (
    <>
      <h2 className="cherry-settings-title">MCP 服务器</h2>
      <p className="cherry-settings-subtitle">挂载 MCP 插件（工具 / 资源 / 提示词）</p>

      <div className="cherry-settings-section">
        <div className="cherry-settings-row">
          <div>
            <div className="cherry-settings-row-label">连接状态</div>
            <div className="cherry-settings-row-desc">
              MCP 的完整列表、启停、鉴权流程在工作台里会更顺手，因为要看日志和 JSON 配置。
            </div>
          </div>
          <div className="cherry-settings-row-control">
            <OpenInAdvancedInline path="/mcp" onClose={onClose} />
          </div>
        </div>
      </div>
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 网络搜索 (Search)
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
      <p className="cherry-settings-subtitle">让模型在需要时主动搜索互联网引用</p>

      <div className="cherry-settings-section">
        <div className="cherry-settings-row">
          <div>
            <div className="cherry-settings-row-label">允许网络搜索</div>
            <div className="cherry-settings-row-desc">
              开启后，输入栏地球图标会变成激活态，工具调用会出现「web_search」节点。
            </div>
          </div>
          <div className="cherry-settings-row-control">
            <button
              type="button"
              className={`cherry-switch ${enabled ? "on" : ""}`}
              aria-pressed={enabled}
              aria-label="Toggle web search"
              onClick={toggle}
            />
          </div>
        </div>
      </div>
    </>
  );
}

/* ══════════════════════════════════════════════════════════════
   Section: 记忆 (Memory)
   ══════════════════════════════════════════════════════════════ */

function MemorySection({ onClose }: { onClose: () => void }) {
  return (
    <>
      <h2 className="cherry-settings-title">全局记忆</h2>
      <p className="cherry-settings-subtitle">跨会话记住事实、偏好、上下文</p>

      <div className="cherry-settings-section">
        <div className="cherry-settings-row">
          <div>
            <div className="cherry-settings-row-label">记忆编辑器</div>
            <div className="cherry-settings-row-desc">
              提取、审阅、手工编辑记忆块，并查看 GraphRAG 社区视图。
            </div>
          </div>
          <div className="cherry-settings-row-control">
            <OpenInAdvancedInline path="/memory" onClose={onClose} />
          </div>
        </div>
      </div>
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
            <button
              type="button"
              className="cherry-btn"
              onClick={() => {
                if (systemInfo?.data_dir) navigator.clipboard?.writeText(systemInfo.data_dir);
              }}
              disabled={!systemInfo?.data_dir}
            >
              复制路径
            </button>
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
            <button type="button" className="cherry-btn primary" onClick={() => router.push(backupPackStatus === "missing" ? "/packs" : "/packs/backup")}>
              {backupPackStatus === "missing" ? "安装 Pack" : "打开备份 Pack"}
            </button>
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
    <button
      type="button"
      className="cherry-btn"
      onClick={() => {
        onClose();
        router.push(path);
      }}
    >
      <Wrench size={12} />
      {label}
      <ExternalLink size={11} style={{ marginLeft: 4, opacity: 0.6 }} />
    </button>
  );
}

function OpenInAdvancedHint({ label, path }: { label: string; path: string }) {
  const router = useRouter();
  return (
    <div
      style={{
        marginTop: 8,
        padding: "10px 12px",
        background: "var(--yunque-bg)",
        border: "1px dashed var(--yunque-border)",
        borderRadius: 10,
        fontSize: 12,
        color: "var(--yunque-text-muted)",
        display: "flex",
        alignItems: "center",
        gap: 10,
      }}
    >
      <Search size={13} style={{ flexShrink: 0 }} />
      <span style={{ flex: 1 }}>{label}</span>
      <button
        type="button"
        className="cherry-btn"
        onClick={() => router.push(path)}
        style={{ padding: "4px 10px" }}
      >
        前往
        <ExternalLink size={11} style={{ marginLeft: 4, opacity: 0.6 }} />
      </button>
    </div>
  );
}
