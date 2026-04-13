"use client";

import { useState, useCallback, useEffect } from "react";
import { Card, Button, Spinner, Chip, Tooltip, TextField, Input, Label, Switch, Tabs } from "@heroui/react";
import {
  Cpu, Cloud, Zap, Link2, Unlink, RefreshCw, Plus,
  CheckCircle2, AlertCircle, Globe, Server, Wifi, WifiOff,
  Key, ChevronRight, ExternalLink, CloudOff, ArrowDownToLine, Activity, Database, Hash,
  Brain, Wrench,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { useApiData } from "@/lib/use-api-data";
import { api, type ProviderInfo, type ProviderPreset, type ToriBindingStatus, type ToriHealthStatus, type ToriUsageSummary } from "@/lib/api";
import { showToast } from "@/components/toast-provider";

type ProviderModeType = "local" | "tori" | "hybrid";

const modeConfig: Record<ProviderModeType, { icon: React.ElementType; label: string; desc: string; color: string }> = {
  local:  { icon: Key,   label: "自带 Key",     desc: "使用你自己的 API 密钥直连各大模型提供商", color: "#3b82f6" },
  tori:   { icon: Cloud, label: "Tori 中转",    desc: "通过 Tori 平台统一代理，免配置即用", color: "#8b5cf6" },
  hybrid: { icon: Zap,   label: "智能混合",     desc: "优先直连，故障自动回退 Tori 中转", color: "#22c55e" },
};

const presetColors: Record<string, string> = {
  deepseek: "#4d6bfe", openai: "#10a37f", anthropic: "#d4a574", google: "#4285f4",
  doubao: "#3370ff", qwen: "#6236ff", zhipu: "#2563eb", moonshot: "#1a1a2e",
  minimax: "#ff6600", ollama: "#ffffff", openrouter: "#6366f1", custom: "#6b7280",
  siliconflow: "#00b4d8", gitcode: "#fc5531",
};

const capMeta: Record<string, { label: string; color: string; icon: string }> = {
  vision:           { label: "视觉",   color: "#a855f7", icon: "👁" },
  reasoning:        { label: "推理",   color: "#f59e0b", icon: "🧠" },
  function_calling: { label: "工具",   color: "#3b82f6", icon: "🔧" },
  structured_output:{ label: "结构化", color: "#06b6d4", icon: "📋" },
  long_context:     { label: "长文本", color: "#10b981", icon: "📜" },
  web_search:       { label: "搜索",   color: "#ef4444", icon: "🔍" },
  code_interpreter: { label: "代码",   color: "#8b5cf6", icon: "💻" },
  computer_use:     { label: "操控",   color: "#ec4899", icon: "🖥" },
  audio_in:         { label: "语音",   color: "#14b8a6", icon: "🎙" },
  video_in:         { label: "视频",   color: "#f97316", icon: "🎬" },
  image_gen:        { label: "生图",   color: "#d946ef", icon: "🎨" },
  streaming:        { label: "流式",   color: "#64748b", icon: "⚡" },
  prompt_caching:   { label: "缓存",   color: "#84cc16", icon: "💾" },
  mcp:              { label: "MCP",    color: "#6366f1", icon: "🔌" },
};

const keyCapabilities = ["vision", "reasoning", "web_search", "code_interpreter", "computer_use", "audio_in", "video_in", "image_gen", "mcp"];

function CapBadges({ caps, max = 5 }: { caps?: string[]; max?: number }) {
  if (!caps?.length) return null;
  const important = caps.filter(c => keyCapabilities.includes(c));
  const shown = important.slice(0, max);
  const extra = important.length - shown.length;
  return (
    <>
      {shown.map(c => {
        const m = capMeta[c];
        if (!m) return null;
        return (
          <span key={c} title={c} style={{
            fontSize: "var(--text-2xs)", padding: "1px 5px",
            borderRadius: 4, background: `${m.color}14`, color: m.color,
            whiteSpace: "nowrap",
          }}>
            {m.icon} {m.label}
          </span>
        );
      })}
      {extra > 0 && (
        <span style={{ fontSize: "var(--text-2xs)", color: "var(--yunque-text-muted)" }}>+{extra}</span>
      )}
    </>
  );
}

export default function ProvidersPage() {
  const { data, loading, refresh } = useApiData(
    async () => {
      const [providersRes, modeRes, presetsRes, toriRes, execRes] = await Promise.all([
        api.providerList().catch(() => ({ providers: [] as ProviderInfo[], count: 0 })),
        api.providerMode().catch(() => ({ mode: "local" })),
        api.providerPresets().catch(() => ({ presets: [] as ProviderPreset[] })),
        api.toriStatus().catch(() => ({ bound: false } as ToriBindingStatus)),
        api.execProvider().catch(() => ({ exec_provider: "", available_providers: [] as string[] })),
      ]);
      let toriHealth: ToriHealthStatus = { status: "unknown" };
      let toriUsage: ToriUsageSummary = {};
      if (toriRes.bound) {
        [toriHealth, toriUsage] = await Promise.all([
          api.toriHealth().catch(() => ({ status: "unreachable" } as ToriHealthStatus)),
          api.toriUsage().catch(() => ({} as ToriUsageSummary)),
        ]);
      }
      return {
        providers: providersRes.providers || [],
        mode: (modeRes.mode || "local") as ProviderModeType,
        presets: presetsRes.presets || [],
        tori: toriRes,
        toriHealth,
        toriUsage,
        execProvider: execRes.exec_provider || "",
        availableProviders: execRes.available_providers || [],
      };
    },
    { providers: [] as ProviderInfo[], mode: "local" as ProviderModeType, presets: [] as ProviderPreset[], tori: { bound: false } as ToriBindingStatus, toriHealth: { status: "unknown" } as ToriHealthStatus, toriUsage: {} as ToriUsageSummary, execProvider: "", availableProviders: [] as string[] },
  );

  const { providers, mode: serverMode, presets, tori, toriHealth, toriUsage, execProvider: serverExecProvider, availableProviders } = data;
  const [localMode, setLocalMode] = useState<ProviderModeType | null>(null);
  const mode = localMode ?? serverMode;
  const [expandedPreset, setExpandedPreset] = useState<string | null>(null);
  const [bindUrl, setBindUrl] = useState("https://tori.owo.today");
  const [binding, setBinding] = useState(false);
  const [unbinding, setUnbinding] = useState(false);
  const [discovering, setDiscovering] = useState(false);
  const [registerForm, setRegisterForm] = useState({ apiKey: "", model: "", baseUrl: "" });
  const [registering, setRegistering] = useState(false);
  const [tab, setTab] = useState("mode");
  const [modeError, setModeError] = useState<string | null>(null);
  const [savingExec, setSavingExec] = useState(false);

  const setMode = useCallback(async (m: ProviderModeType) => {
    setLocalMode(m);
    setModeError(null);
    try {
      await api.setProviderMode(m);
      refresh();
    } catch (e: unknown) {
      setLocalMode(null);
      setModeError(String((e as Error)?.message || "切换模式失败"));
      setTimeout(() => setModeError(null), 5000);
    }
  }, [refresh]);

  const handleBind = useCallback(async () => {
    if (!bindUrl.trim()) return;
    setBinding(true);
    try {
      const res = await api.toriBind(bindUrl.trim());
      if (res.auth_url) window.open(res.auth_url, "_blank");
      setTimeout(refresh, 3000);
    } catch (e) { showToast(e instanceof Error ? e.message : "绑定失败", "error"); }
    setBinding(false);
  }, [bindUrl, refresh]);

  const handleUnbind = useCallback(async () => {
    setUnbinding(true);
    try { await api.toriUnbind(); } catch (e) { showToast(e instanceof Error ? e.message : "解绑失败", "error"); }
    setUnbinding(false);
    refresh();
  }, [refresh]);

  const handleDiscover = useCallback(async () => {
    setDiscovering(true);
    try {
      const res = await api.toriDiscover(true);
      const count = res.models?.length ?? 0;
      const reg = res.registered ?? 0;
      if (count === 0) {
        showToast("Tori 上暂无可用模型", "warning");
      } else {
        showToast(`发现 ${count} 个模型，注册 ${reg} 个`, "success");
      }
    } catch (e) { showToast(e instanceof Error ? e.message : "发现失败", "error"); }
    setDiscovering(false);
    refresh();
  }, [refresh]);

  const [regResult, setRegResult] = useState<{ ok: boolean; msg: string } | null>(null);

  const handleRegisterModel = useCallback(async (presetId: string, modelId: string, tier?: string) => {
    setRegistering(true);
    setRegResult(null);
    try {
      await api.providerRegister({
        preset_id: presetId,
        api_key: registerForm.apiKey || undefined,
        base_url: registerForm.baseUrl || undefined,
        model: modelId || registerForm.model || undefined,
        tier: tier || undefined,
      });
      setRegResult({ ok: true, msg: `已添加 ${modelId || registerForm.model}` });
      setRegisterForm({ apiKey: "", model: "", baseUrl: "" });
      refresh();
    } catch (e: unknown) {
      setRegResult({ ok: false, msg: String((e as Error)?.message || "添加失败") });
    }
    setRegistering(false);
    setTimeout(() => setRegResult(null), 4000);
  }, [registerForm, refresh]);

  const handleSetExecProvider = useCallback(async (pid: string) => {
    setSavingExec(true);
    try {
      await api.setExecProvider(pid);
      showToast("执行层模型已更新", "success");
      refresh();
    } catch (e) { showToast(e instanceof Error ? e.message : "设置失败", "error"); }
    setSavingExec(false);
  }, [refresh]);

  const activeProviders = providers.filter(p => p.enabled);

  const [firstTime, setFirstTime] = useState(false);
  useEffect(() => {
    api.checkSetup().then((chk) => { if (chk.setup_needed) setFirstTime(true); }).catch(() => {});
  }, []);

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader
        icon={<Cpu size={20} />}
        title="模型提供商"
        description="管理 LLM 接入方式、API 密钥与 Tori 中转"
        onRefresh={refresh}
      />

      {firstTime && (
        <Card className="section-card p-5 border-l-4" style={{ borderLeftColor: "var(--yunque-accent)" }}>
          <div className="flex items-center justify-between">
            <div>
              <div className="flex items-center gap-2 font-semibold" style={{ color: "var(--yunque-text)" }}>
                <Zap size={16} style={{ color: "var(--yunque-accent)" }} /> 欢迎使用云雀 Agent！
              </div>
              <p className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                首次使用请先配置模型提供商。推荐使用 <strong>Tori 中转</strong>（免 Key 即用）或填入你自己的 API Key。
              </p>
            </div>
            <Button
              size="sm"
              variant="outline"
              onPress={() => { sessionStorage.setItem("yunque_setup_skipped", "1"); window.location.href = "/chat"; }}
            >
              跳过，直接聊天
            </Button>
          </div>
        </Card>
      )}

      {/* KPI Strip */}
      <div className="kpi-grid stagger-children">
        <Card className="section-card p-4 hover-lift">
          <div className="kpi-label stat-card-header"><Wifi size={13} style={{ color: "#22c55e" }} />活跃提供商</div>
          <div className="kpi-value">{activeProviders.length}</div>
        </Card>
        <Card className="section-card p-4 hover-lift">
          <div className="kpi-label stat-card-header"><Cpu size={13} style={{ color: "var(--yunque-accent)" }} />提供商总数</div>
          <div className="kpi-value">{providers.length}</div>
        </Card>
        <Card className="section-card p-4 hover-lift">
          <div className="kpi-label stat-card-header"><Globe size={13} style={{ color: "#8b5cf6" }} />可用预置</div>
          <div className="kpi-value">{presets.length}</div>
        </Card>
        <Card className="section-card p-4 hover-lift">
          <div className="kpi-label stat-card-header"><Cloud size={13} style={{ color: tori.bound ? "#22c55e" : "var(--yunque-text-muted)" }} />Tori</div>
          <div className="kpi-value" style={{ fontSize: "var(--text-lg)" }}>{tori.bound ? "已绑定" : "未绑定"}</div>
        </Card>
      </div>

      <Tabs selectedKey={tab} onSelectionChange={k => setTab(k as string)}>
        <Tabs.ListContainer>
          <Tabs.List aria-label="提供商设置">
            <Tabs.Tab id="mode">接入模式<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="routing"><Tabs.Separator />模型分配<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="tori"><Tabs.Separator />Tori 平台<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="providers"><Tabs.Separator />提供商列表 <Chip style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)", fontSize: "var(--text-2xs)" }}>{providers.length}</Chip><Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="presets"><Tabs.Separator />添加提供商<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>

        {/* ── Mode Selection ─── */}
        <Tabs.Panel id="mode">
          <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: "var(--sp-4)" }}>
            {(Object.entries(modeConfig) as [ProviderModeType, typeof modeConfig[ProviderModeType]][]).map(([key, cfg]) => {
              const Icon = cfg.icon;
              const active = mode === key;
              return (
                <button
                  key={key}
                  onClick={() => setMode(key)}
                  style={{
                    display: "flex", flexDirection: "column", alignItems: "flex-start",
                    padding: "var(--card-pad)", gap: "var(--sp-3)",
                    borderRadius: "var(--radius-lg)",
                    background: active ? `${cfg.color}10` : "var(--yunque-card)",
                    border: `2px solid ${active ? cfg.color : "var(--yunque-border)"}`,
                    cursor: "pointer",
                    transition: "all var(--duration-base) ease",
                    textAlign: "left",
                  }}
                >
                  <div style={{
                    width: 40, height: 40, borderRadius: "var(--radius-md)",
                    display: "flex", alignItems: "center", justifyContent: "center",
                    background: active ? `${cfg.color}20` : "var(--yunque-surface-2)",
                  }}>
                    <Icon size={20} style={{ color: active ? cfg.color : "var(--yunque-text-muted)" }} />
                  </div>
                  <div>
                    <div style={{ fontSize: "var(--text-md)", fontWeight: 600, color: "var(--yunque-text)", display: "flex", alignItems: "center", gap: 6 }}>
                      {cfg.label}
                      {active && <CheckCircle2 size={14} style={{ color: cfg.color }} />}
                    </div>
                    <div style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-secondary)", marginTop: "var(--sp-1)" }}>
                      {cfg.desc}
                    </div>
                  </div>
                </button>
              );
            })}
          </div>
          {modeError && (
            <div style={{
              marginTop: "var(--sp-3)", padding: "var(--sp-2) var(--sp-3)", borderRadius: "var(--radius-md)",
              background: "var(--yunque-danger-muted)", color: "var(--yunque-danger)",
              fontSize: "var(--text-sm)", fontWeight: 500,
            }}>
              {modeError}
            </div>
          )}
          <div className="section-card" style={{ marginTop: "var(--sp-4)", padding: "var(--card-pad-sm)", display: "flex", alignItems: "center", gap: "var(--sp-3)" }}>
            <AlertCircle size={14} style={{ color: "var(--yunque-text-muted)", flexShrink: 0 }} />
            <span style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-secondary)" }}>
              {mode === "local" && "所有请求将直接发送到你配置的各个 LLM API，需要自行管理 API 密钥。"}
              {mode === "tori" && "所有请求经 Tori 平台中转，由 Tori 负责密钥管理与负载均衡。需先绑定 Tori 账号。"}
              {mode === "hybrid" && "优先使用本地直连提供商，当直连不可用时自动回退到 Tori 中转，确保高可用。"}
            </span>
          </div>
        </Tabs.Panel>

        {/* ── Model Routing (Cognitive / Execution) ─── */}
        <Tabs.Panel id="routing">
          <div className="space-y-4">
            <Card className="section-card p-5">
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: "var(--sp-3)" }}>
                <AlertCircle size={14} style={{ color: "var(--yunque-accent)" }} />
                <span style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-secondary)" }}>
                  认知层（Planner）负责理解意图和规划，执行层（Exec Agent）负责调用工具完成任务。为它们分配不同的模型可以优化成本与效率。
                </span>
              </div>
            </Card>

            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--sp-4)" }}>
              {/* Cognitive Layer */}
              <Card className="section-card p-5">
                <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-3)" }}>
                  <div style={{
                    width: 36, height: 36, borderRadius: "var(--radius-md)",
                    display: "flex", alignItems: "center", justifyContent: "center",
                    background: "rgba(139,92,246,0.12)",
                  }}>
                    <Brain size={18} style={{ color: "#8b5cf6" }} />
                  </div>
                  <div>
                    <div style={{ fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--yunque-text)" }}>认知层 · Planner</div>
                    <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)" }}>理解意图 → 规划 → 委派任务</div>
                  </div>
                </div>
                <div style={{
                  padding: "var(--sp-3)", borderRadius: "var(--radius-md)",
                  background: "rgba(139,92,246,0.06)", border: "1px solid rgba(139,92,246,0.15)",
                }}>
                  <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", marginBottom: "var(--sp-1)" }}>当前模型池</div>
                  <div style={{ fontSize: "var(--text-md)", fontWeight: 600, color: "#8b5cf6" }}>smart</div>
                  <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", marginTop: "var(--sp-1)" }}>
                    使用 smart 池中的最高优先级提供商
                  </div>
                </div>
              </Card>

              {/* Execution Layer */}
              <Card className="section-card p-5">
                <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-3)" }}>
                  <div style={{
                    width: 36, height: 36, borderRadius: "var(--radius-md)",
                    display: "flex", alignItems: "center", justifyContent: "center",
                    background: "rgba(34,197,94,0.12)",
                  }}>
                    <Wrench size={18} style={{ color: "#22c55e" }} />
                  </div>
                  <div>
                    <div style={{ fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--yunque-text)" }}>执行层 · Exec Agents</div>
                    <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)" }}>浏览器 / 文件 / 代码 / 搜索 / 通用</div>
                  </div>
                </div>
                <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-2)" }}>
                  <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)" }}>选择执行层使用的提供商</div>
                  <div style={{ display: "flex", flexWrap: "wrap", gap: "var(--sp-2)" }}>
                    {["smart", ...availableProviders.filter(p => p !== "smart")].map(pid => {
                      const isActive = (serverExecProvider || "smart") === pid;
                      return (
                        <button
                          key={pid}
                          disabled={savingExec}
                          onClick={() => handleSetExecProvider(pid)}
                          style={{
                            padding: "6px 14px", borderRadius: "var(--radius-md)",
                            fontSize: "var(--text-sm)", fontWeight: isActive ? 600 : 400,
                            background: isActive ? "rgba(34,197,94,0.12)" : "var(--yunque-surface-2)",
                            color: isActive ? "#22c55e" : "var(--yunque-text)",
                            border: `1.5px solid ${isActive ? "#22c55e" : "var(--yunque-border)"}`,
                            cursor: savingExec ? "wait" : "pointer",
                            transition: "all var(--duration-fast) ease",
                          }}
                        >
                          {pid}
                          {isActive && <CheckCircle2 size={12} style={{ marginLeft: 4, verticalAlign: "middle" }} />}
                        </button>
                      );
                    })}
                  </div>
                  {availableProviders.length === 0 && (
                    <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", fontStyle: "italic" }}>
                      尚无可用提供商，请先在「添加提供商」中配置
                    </div>
                  )}
                </div>
              </Card>
            </div>

            {/* Agent breakdown */}
            <Card className="section-card p-5">
              <div style={{ fontSize: "var(--text-sm)", fontWeight: 600, marginBottom: "var(--sp-3)", color: "var(--yunque-text)" }}>
                执行代理一览
              </div>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))", gap: "var(--sp-2)" }}>
                {[
                  { name: "browser_exec", label: "浏览器", icon: "🌐", desc: "搜索/导航/点击/输入" },
                  { name: "file_exec", label: "文件", icon: "📄", desc: "Word/Excel/PPT/PDF" },
                  { name: "code_exec", label: "代码", icon: "💻", desc: "Python/Shell 执行" },
                  { name: "research_exec", label: "研究", icon: "🔍", desc: "网络搜索/信息收集" },
                  { name: "general_exec", label: "通用", icon: "⚙️", desc: "图片/翻译/邮件等" },
                ].map(agent => (
                  <div key={agent.name} style={{
                    padding: "var(--sp-3)", borderRadius: "var(--radius-md)",
                    background: "var(--yunque-surface-2)", border: "1px solid var(--yunque-border)",
                  }}>
                    <div style={{ fontSize: "var(--text-md)", marginBottom: 2 }}>{agent.icon}</div>
                    <div style={{ fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--yunque-text)" }}>{agent.label}</div>
                    <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)" }}>{agent.desc}</div>
                  </div>
                ))}
              </div>
            </Card>
          </div>
        </Tabs.Panel>

        {/* ── Tori Platform ─── */}
        <Tabs.Panel id="tori">
          <div className="space-y-4">
            {/* Binding Status */}
            <Card className="section-card p-5">
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)" }}>
                  <div style={{
                    width: 44, height: 44, borderRadius: "var(--radius-lg)",
                    display: "flex", alignItems: "center", justifyContent: "center",
                    background: tori.bound ? "var(--yunque-success-muted)" : "var(--yunque-surface-2)",
                  }}>
                    {tori.bound
                      ? <Cloud size={22} style={{ color: "var(--yunque-success)" }} />
                      : <CloudOff size={22} style={{ color: "var(--yunque-text-muted)" }} />
                    }
                  </div>
                  <div>
                    <div style={{ fontSize: "var(--text-md)", fontWeight: 600, color: "var(--yunque-text)", display: "flex", alignItems: "center", gap: 8 }}>
                      Tori 平台
                      <Chip size="sm" style={{
                        background: tori.bound ? "var(--yunque-success-muted)" : "var(--yunque-danger-muted)",
                        color: tori.bound ? "var(--yunque-success)" : "var(--yunque-danger)",
                        fontSize: "var(--text-2xs)",
                      }}>
                        {tori.bound ? "已绑定" : "未绑定"}
                      </Chip>
                    </div>
                    {tori.bound && tori.username && (
                      <div style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-secondary)", marginTop: 2 }}>
                        {tori.username}{tori.tori_url && <span style={{ color: "var(--yunque-text-muted)" }}> · {tori.tori_url}</span>}
                      </div>
                    )}
                    {!tori.bound && (
                      <div style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)", marginTop: 2 }}>
                        绑定 Tori 账号后可使用中转、数据同步等功能
                      </div>
                    )}
                  </div>
                </div>
                {tori.bound && (
                  <Button size="sm" variant="ghost" isPending={unbinding} onPress={handleUnbind} style={{ color: "var(--yunque-danger)" }}>
                    <Unlink size={13} /> 解绑
                  </Button>
                )}
              </div>
            </Card>

            {/* Bind Form */}
            {!tori.bound && (
              <Card className="section-card p-5 animate-scale-in">
                <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: "var(--sp-3)" }}>
                  <Link2 size={14} style={{ color: "var(--yunque-accent)" }} />
                  <span style={{ fontSize: "var(--text-sm)", fontWeight: 600 }}>绑定 Tori 账号</span>
                </div>
                <div style={{ display: "flex", gap: "var(--sp-2)" }}>
                  <div style={{ flex: 1 }}>
                    <TextField value={bindUrl} onChange={setBindUrl}>
                      <Label>Tori 平台地址</Label>
                      <Input placeholder="https://tori.example.com" />
                    </TextField>
                  </div>
                  <Button
                    size="sm" isPending={binding} onPress={handleBind}
                    className="btn-accent" style={{ alignSelf: "flex-end", marginBottom: 2 }}
                  >
                    <ExternalLink size={13} /> 授权绑定
                  </Button>
                </div>
                <p style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", marginTop: "var(--sp-2)" }}>
                  将打开 Tori 平台进行 OAuth2 授权，授权后自动完成绑定
                </p>
              </Card>
            )}

            {/* Tori Health & Usage */}
            {tori.bound && (
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--sp-3)" }}>
                <Card className="section-card p-4">
                  <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: "var(--sp-2)" }}>
                    <Activity size={13} style={{ color: toriHealth.status === "ok" ? "#22c55e" : "#f59e0b" }} />
                    <span style={{ fontSize: "var(--text-xs)", fontWeight: 600 }}>服务状态</span>
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <Chip size="sm" style={{
                      background: toriHealth.status === "ok" ? "var(--yunque-success-muted)" : "var(--yunque-warning-muted)",
                      color: toriHealth.status === "ok" ? "var(--yunque-success)" : "var(--yunque-warning)",
                      fontSize: "var(--text-2xs)",
                    }}>
                      {toriHealth.status === "ok" ? "正常" : toriHealth.status === "degraded" ? "降级" : "不可达"}
                    </Chip>
                    {toriHealth.version && <span style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)" }}>{toriHealth.version}</span>}
                  </div>
                  {toriHealth.uptime != null && toriHealth.uptime > 0 && (
                    <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", marginTop: 4 }}>
                      运行 {Math.floor(toriHealth.uptime / 3600)}h {Math.floor((toriHealth.uptime % 3600) / 60)}m
                    </div>
                  )}
                </Card>
                <Card className="section-card p-4">
                  <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: "var(--sp-2)" }}>
                    <Database size={13} style={{ color: "var(--yunque-accent)" }} />
                    <span style={{ fontSize: "var(--text-xs)", fontWeight: 600 }}>用量统计</span>
                  </div>
                  {toriUsage.request_count != null ? (
                    <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 4 }}>
                      <div>
                        <div style={{ fontSize: "var(--text-2xs)", color: "var(--yunque-text-muted)" }}>请求数</div>
                        <div style={{ fontSize: "var(--text-sm)", fontWeight: 600 }}>{toriUsage.request_count?.toLocaleString()}</div>
                      </div>
                      <div>
                        <div style={{ fontSize: "var(--text-2xs)", color: "var(--yunque-text-muted)" }}>总 Token</div>
                        <div style={{ fontSize: "var(--text-sm)", fontWeight: 600 }}>{toriUsage.total_tokens?.toLocaleString()}</div>
                      </div>
                      <div>
                        <div style={{ fontSize: "var(--text-2xs)", color: "var(--yunque-text-muted)" }}>已用配额</div>
                        <div style={{ fontSize: "var(--text-sm)", fontWeight: 600 }}>{toriUsage.used_quota?.toLocaleString()}</div>
                      </div>
                      <div>
                        <div style={{ fontSize: "var(--text-2xs)", color: "var(--yunque-text-muted)" }}>剩余配额</div>
                        <div style={{ fontSize: "var(--text-sm)", fontWeight: 600 }}>{toriUsage.remain_quota?.toLocaleString()}</div>
                      </div>
                    </div>
                  ) : (
                    <span style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)" }}>暂无数据</span>
                  )}
                </Card>
              </div>
            )}

            {/* Tori Discover */}
            {tori.bound && (
              <Card className="section-card p-5">
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "var(--sp-3)" }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                    <ArrowDownToLine size={14} style={{ color: "var(--yunque-accent)" }} />
                    <span style={{ fontSize: "var(--text-sm)", fontWeight: 600 }}>发现 Tori 模型</span>
                  </div>
                  <Button size="sm" variant="outline" isPending={discovering} onPress={handleDiscover}>
                    <RefreshCw size={12} /> 自动发现并注册
                  </Button>
                </div>
                <p style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)" }}>
                  从 Tori 平台自动发现可用模型并注册为提供商，无需手动配置密钥
                </p>
              </Card>
            )}
          </div>
        </Tabs.Panel>

        {/* ── Provider List ─── */}
        <Tabs.Panel id="providers">
          <div className="flex items-center justify-between mb-3">
            <span style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)" }}>已配置的模型提供商</span>
            <Button size="sm" variant="ghost" onPress={async () => {
              try { const r = await api.breakerReset(); alert(`已重置 ${r.reset_count} 个熔断器`); refresh(); } catch (e: any) { alert(e.message); }
            }} style={{ fontSize: "var(--text-xs)" }}>
              重置熔断器
            </Button>
          </div>
          <div className="space-y-3 stagger-children">
            {providers.length === 0 ? (
              <Card className="section-card p-12 text-center">
                <WifiOff size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
                <div style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)" }}>暂无提供商，前往「添加提供商」标签页配置</div>
              </Card>
            ) : providers.map(p => {
              const healthy = p.breaker_state === "closed";
              const statusColor = healthy ? "#22c55e" : p.breaker_state === "half-open" ? "#f59e0b" : "#ef4444";
              return (
                <Card key={p.id} className="section-card p-5 hover-lift transition-all duration-200">
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                    <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)" }}>
                      <div style={{
                        width: 36, height: 36, borderRadius: "var(--radius-md)",
                        display: "flex", alignItems: "center", justifyContent: "center",
                        background: "var(--yunque-surface-2)", border: "1px solid var(--yunque-border)",
                        position: "relative",
                      }}>
                        <Server size={16} style={{ color: p.enabled ? statusColor : "var(--yunque-text-muted)" }} />
                        <span style={{
                          position: "absolute", bottom: -2, right: -2,
                          width: 8, height: 8, borderRadius: "50%",
                          background: p.enabled ? statusColor : "#4b5563",
                          border: "2px solid var(--yunque-card)",
                        }} />
                      </div>
                      <div>
                        <div style={{ display: "flex", alignItems: "center", gap: 6, fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--yunque-text)" }}>
                          {p.display_name || p.id}
                          <Chip size="sm" style={{ background: `${statusColor}15`, color: statusColor, fontSize: "var(--text-2xs)" }}>
                            {healthy ? "healthy" : p.breaker_state || "unknown"}
                          </Chip>
                          {p.tier && <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>{p.tier}</Chip>}
                        </div>
                        <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", marginTop: 2, display: "flex", alignItems: "center", gap: 6 }}>
                          <span>{p.type}</span>
                          {p.base_url && <><span>·</span><span className="font-mono" style={{ maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{p.base_url}</span></>}
                          {p.model && <><span>·</span><span>{p.model}</span></>}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Switch isSelected={p.enabled} onChange={() => p.enabled ? api.providerDisable(p.id).then(refresh) : api.providerEnable(p.id).then(refresh)} size="sm">
                        <Switch.Control><Switch.Thumb /></Switch.Control>
                      </Switch>
                      <Tooltip delay={0}>
                        <Button isIconOnly variant="ghost" size="sm" onPress={async () => {
                          try {
                            await api.providerDelete(p.id);
                            showToast("已删除", "success");
                            refresh();
                          } catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); }
                        }} style={{ color: "#ef4444" }}>
                          <Unlink size={14} />
                        </Button>
                        <Tooltip.Content>删除模型</Tooltip.Content>
                      </Tooltip>
                    </div>
                  </div>
                </Card>
              );
            })}
          </div>
        </Tabs.Panel>

        {/* ── Add from Presets ─── */}
        <Tabs.Panel id="presets">
          {regResult && (
            <div style={{
              padding: "var(--sp-2) var(--sp-3)", marginBottom: "var(--sp-3)", borderRadius: "var(--radius-md)",
              fontSize: "var(--text-sm)", fontWeight: 500,
              background: regResult.ok ? "var(--yunque-success-muted)" : "var(--yunque-danger-muted)",
              color: regResult.ok ? "var(--yunque-success)" : "var(--yunque-danger)",
            }}>
              {regResult.msg}
            </div>
          )}
          {presets.length === 0 && (
            <Card className="section-card p-12 text-center">
              <Plus size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
              <div style={{ fontSize: "var(--text-md)", fontWeight: 600, marginBottom: "var(--sp-1)" }}>暂无预置提供商</div>
              <p style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)" }}>
                后端暂未返回预置列表。你可以在「提供商列表」Tab 中手动管理已有的提供商。
              </p>
            </Card>
          )}
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(260px, 1fr))", gap: "var(--sp-3)" }} className="stagger-children">
            {presets.map(preset => {
              const expanded = expandedPreset === preset.id;
              const color = presetColors[preset.id] || "#6b7280";
              return (
                <div key={preset.id}>
                  <button
                    onClick={() => setExpandedPreset(expanded ? null : preset.id)}
                    style={{
                      width: "100%", textAlign: "left",
                      display: "flex", alignItems: "center", gap: "var(--sp-3)",
                      padding: "var(--card-pad-sm)",
                      borderRadius: expanded ? "var(--radius-lg) var(--radius-lg) 0 0" : "var(--radius-lg)",
                      background: expanded ? "var(--yunque-accent-soft)" : "var(--yunque-card)",
                      border: `1px solid ${expanded ? "var(--yunque-accent)" : "var(--yunque-border)"}`,
                      borderBottom: expanded ? "none" : undefined,
                      cursor: "pointer",
                      transition: "all var(--duration-fast) ease",
                    }}
                  >
                    <span style={{
                      width: 32, height: 32, borderRadius: "var(--radius-md)",
                      display: "flex", alignItems: "center", justifyContent: "center",
                      background: `${color}18`, color, fontSize: "var(--text-sm)", fontWeight: 700,
                      flexShrink: 0,
                    }}>{preset.name.charAt(0)}</span>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--yunque-text)", display: "flex", alignItems: "center", gap: 6 }}>
                        {preset.name}
                        {preset.is_aggregator && (
                          <span style={{ fontSize: "var(--text-2xs)", padding: "1px 5px", borderRadius: 4, background: "rgba(99,102,241,0.12)", color: "#6366f1" }}>聚合</span>
                        )}
                      </div>
                      {preset.base_url && (
                        <div style={{ fontSize: "var(--text-2xs)", color: "var(--yunque-text-muted)", fontFamily: "monospace", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                          {preset.base_url}
                        </div>
                      )}
                    </div>
                    <ChevronRight size={14} style={{
                      color: "var(--yunque-text-muted)",
                      transform: expanded ? "rotate(90deg)" : "rotate(0)",
                      transition: "transform var(--duration-fast) ease",
                    }} />
                  </button>

                  {expanded && (
                    <div
                      className="animate-scale-in"
                      style={{
                        padding: "var(--card-pad-sm)",
                        borderRadius: "0 0 var(--radius-lg) var(--radius-lg)",
                        background: "var(--yunque-card)",
                        border: "1px solid var(--yunque-accent)",
                        borderTop: "1px solid var(--yunque-border)",
                      }}
                    >
                      <div className="space-y-3">
                        {/* API Key — always shown */}
                        <TextField value={registerForm.apiKey} onChange={v => setRegisterForm(f => ({ ...f, apiKey: v }))}>
                          <Label>API Key</Label>
                          <Input placeholder="sk-..." type="password" />
                        </TextField>

                        {/* Custom: base URL + model name */}
                        {(preset.id === "custom" || preset.id === "ollama" || preset.id === "openrouter") && (
                          <>
                            {preset.id === "custom" && (
                              <TextField value={registerForm.baseUrl} onChange={v => setRegisterForm(f => ({ ...f, baseUrl: v }))}>
                                <Label>Base URL</Label>
                                <Input placeholder="https://api.example.com/v1" />
                              </TextField>
                            )}
                            <TextField value={registerForm.model} onChange={v => setRegisterForm(f => ({ ...f, model: v }))}>
                              <Label>模型名称</Label>
                              <Input placeholder="model-name" />
                            </TextField>
                            <div style={{ display: "flex", justifyContent: "flex-end" }}>
                              <Button size="sm" isPending={registering}
                                onPress={() => handleRegisterModel(preset.id, registerForm.model)}
                                className="btn-accent">
                                <Plus size={13} /> 添加
                              </Button>
                            </div>
                          </>
                        )}

                        {/* Presets with model list: clickable chips */}
                        {preset.models?.length > 0 && (
                          <div>
                            <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", marginBottom: "var(--sp-2)" }}>
                              点击模型即可添加（需先填写 API Key）
                            </div>
                            <div style={{ display: "flex", flexWrap: "wrap", gap: "var(--sp-2)" }}>
                              {preset.models.map(m => {
                                const tierColor = m.tier === "expert" ? "#f59e0b" : m.tier === "smart" ? "#3b82f6" : "#22c55e";
                                const alreadyAdded = providers.some(p => p.model === m.id && p.base_url?.includes(preset.base_url?.replace(/\/v\d.*/, "") || "---"));
                                return (
                                  <button
                                    key={m.id}
                                    disabled={registering || alreadyAdded}
                                    onClick={() => handleRegisterModel(preset.id, m.id, m.tier)}
                                    style={{
                                      display: "flex", alignItems: "center", gap: 6,
                                      padding: "6px 12px", borderRadius: "var(--radius-md)",
                                      background: alreadyAdded ? "var(--yunque-surface-2)" : "var(--yunque-card)",
                                      border: `1px solid ${alreadyAdded ? "var(--yunque-success)" : "var(--yunque-border)"}`,
                                      cursor: alreadyAdded ? "default" : "pointer",
                                      opacity: alreadyAdded ? 0.6 : 1,
                                      transition: "all var(--duration-fast) ease",
                                      fontSize: "var(--text-sm)",
                                    }}
                                  >
                                    <span style={{ fontWeight: 500, color: "var(--yunque-text)" }}>{m.name}</span>
                                    {m.tier && (
                                      <span style={{
                                        fontSize: "var(--text-2xs)", padding: "1px 5px",
                                        borderRadius: 4, background: `${tierColor}15`, color: tierColor,
                                      }}>
                                        {m.tier}
                                      </span>
                                    )}
                                    <CapBadges caps={m.capabilities} max={3} />
                                    {m.context_window ? (
                                      <span style={{
                                        fontSize: "var(--text-2xs)", padding: "1px 4px",
                                        borderRadius: 3, background: "rgba(100,116,139,0.1)", color: "#64748b",
                                      }}>
                                        {m.context_window >= 1024 ? `${m.context_window / 1024}M` : `${m.context_window}K`}
                                      </span>
                                    ) : null}
                                    {alreadyAdded ? (
                                      <CheckCircle2 size={12} style={{ color: "var(--yunque-success)" }} />
                                    ) : (
                                      <Plus size={12} style={{ color: "var(--yunque-text-muted)" }} />
                                    )}
                                  </button>
                                );
                              })}
                            </div>
                            {/* Manual input fallback */}
                            <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginTop: "var(--sp-3)" }}>
                              <div style={{ flex: 1 }}>
                                <TextField value={registerForm.model} onChange={v => setRegisterForm(f => ({ ...f, model: v }))}>
                                  <Label className="sr-only">自定义模型</Label>
                                  <Input placeholder="或手动输入模型名…" />
                                </TextField>
                              </div>
                              <Button size="sm" isPending={registering}
                                isDisabled={!registerForm.model}
                                onPress={() => handleRegisterModel(preset.id, registerForm.model)}
                                variant="outline" style={{ flexShrink: 0 }}>
                                <Plus size={13} /> 添加
                              </Button>
                            </div>
                          </div>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </Tabs.Panel>
      </Tabs>
    </div>
  );
}
