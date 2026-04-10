"use client";

import { useState, useEffect, useCallback } from "react";
import { api, getApiKey } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  Cpu, Globe, Link2, Unlink, Search, Plus, Check, AlertTriangle,
  RefreshCw, ChevronRight, Zap, Brain, Sparkles,
} from "lucide-react";

interface PresetModel {
  id: string;
  name: string;
  type: string;
  tier?: string;
  capabilities?: string[];
}

interface Preset {
  id: string;
  name: string;
  description?: string;
  base_url: string;
  models: PresetModel[];
  docs_url?: string;
}

interface ProviderStatus {
  id: string;
  display_name?: string;
  type: string;
  source?: string;
  model: string;
  base_url: string;
  enabled: boolean;
  tier?: string;
  priority: number;
  key_count: number;
  breaker_state: string;
  preset_id?: string;
}

interface ToriStatus {
  bound: boolean;
  tori_base_url?: string;
  username?: string;
  email?: string;
  has_api_key?: boolean;
}

const tierIcon: Record<string, React.ElementType> = {
  fast: Zap,
  smart: Brain,
  expert: Sparkles,
};

const tierColor: Record<string, string> = {
  fast: "#22c55e",
  smart: "#3b82f6",
  expert: "#a855f7",
};

export default function ProvidersPage() {
  const { locale } = useI18n();
  const zh = locale === "zh";

  const [presets, setPresets] = useState<Preset[]>([]);
  const [providers, setProviders] = useState<ProviderStatus[]>([]);
  const [mode, setMode] = useState("hybrid");
  const [toriStatus, setToriStatus] = useState<ToriStatus>({ bound: false });
  const [loading, setLoading] = useState(true);
  const [addingPreset, setAddingPreset] = useState<string | null>(null);
  const [addForm, setAddForm] = useState({ api_key: "", model: "" });
  const [toriUrl, setToriUrl] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const key = getApiKey();
      const headers = key ? { Authorization: `Bearer ${key}` } : {};
      const [presetsRes, providersRes, toriRes] = await Promise.all([
        fetch("/api/providers/presets", { headers }).then(r => r.json()).catch(() => ({ presets: [] })),
        fetch("/api/providers", { headers }).then(r => r.json()).catch(() => ({ providers: [], mode: "hybrid" })),
        fetch("/v1/tori/status", { headers }).then(r => r.json()).catch(() => ({ bound: false })),
      ]);
      setPresets(presetsRes.presets || []);
      setProviders(providersRes.providers || []);
      setMode(providersRes.mode || "hybrid");
      setToriStatus(toriRes);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const changeMode = async (m: string) => {
    const key = getApiKey();
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (key) headers.Authorization = `Bearer ${key}`;
    await fetch("/api/providers/mode", { method: "POST", headers, body: JSON.stringify({ mode: m }) });
    setMode(m);
  };

  const registerProvider = async (presetId: string) => {
    const key = getApiKey();
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (key) headers.Authorization = `Bearer ${key}`;
    await fetch("/api/providers/register", {
      method: "POST", headers,
      body: JSON.stringify({ preset_id: presetId, api_key: addForm.api_key, model: addForm.model }),
    });
    setAddingPreset(null);
    setAddForm({ api_key: "", model: "" });
    load();
  };

  const bindTori = async () => {
    if (!toriUrl) return;
    const key = getApiKey();
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (key) headers.Authorization = `Bearer ${key}`;
    await fetch("/v1/tori/bind", { method: "POST", headers, body: JSON.stringify({ tori_url: toriUrl }) });
    setTimeout(load, 3000);
  };

  const unbindTori = async () => {
    const key = getApiKey();
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (key) headers.Authorization = `Bearer ${key}`;
    await fetch("/v1/tori/unbind", { method: "POST", headers });
    load();
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <RefreshCw size={20} className="animate-spin" style={{ color: "var(--text-muted)" }} />
      </div>
    );
  }

  return (
    <div className="max-w-4xl space-y-6">
      <BlurFade delay={0}>
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold">{zh ? "模型提供商" : "Model Providers"}</h1>
            <p className="text-sm mt-1" style={{ color: "var(--text-muted)" }}>
              {zh ? "配置 LLM 接入方式：自带 Key、Tori 中转或智能混合" : "Configure LLM access: bring your own key, Tori relay, or smart hybrid"}
            </p>
          </div>
        </div>
      </BlurFade>

      {/* Mode Selector */}
      <BlurFade delay={0.05}>
        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="text-sm font-medium mb-3">{zh ? "接入模式" : "Provider Mode"}</div>
          <div className="grid grid-cols-3 gap-3">
            {([
              { key: "local", icon: Cpu, label: zh ? "自带 Key" : "Local Keys", desc: zh ? "直连 API 提供商" : "Direct API access" },
              { key: "tori", icon: Globe, label: zh ? "Tori 中转" : "Tori Relay", desc: zh ? "通过 Tori 统一路由" : "Route through Tori" },
              { key: "hybrid", icon: Sparkles, label: zh ? "智能混合" : "Smart Hybrid", desc: zh ? "优先本地，降级 Tori" : "Local first, Tori fallback" },
            ] as const).map(m => (
              <button key={m.key} onClick={() => changeMode(m.key)}
                className="rounded-xl border p-4 text-left transition-all cursor-pointer"
                style={{
                  background: mode === m.key ? "var(--accent-bg)" : "var(--bg-hover)",
                  borderColor: mode === m.key ? "var(--accent)" : "var(--border)",
                  boxShadow: mode === m.key ? "0 0 0 1px var(--accent)" : "none",
                }}>
                <m.icon size={18} style={{ color: mode === m.key ? "var(--accent)" : "var(--text-muted)" }} />
                <div className="text-sm font-medium mt-2">{m.label}</div>
                <div className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>{m.desc}</div>
                {mode === m.key && <Check size={14} className="mt-2" style={{ color: "var(--accent)" }} />}
              </button>
            ))}
          </div>
        </div>
      </BlurFade>

      {/* Tori Binding */}
      <BlurFade delay={0.1}>
        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="flex items-center gap-2 mb-3">
            <Globe size={16} style={{ color: toriStatus.bound ? "var(--success)" : "var(--text-muted)" }} />
            <span className="text-sm font-medium">{zh ? "Tori 绑定" : "Tori Binding"}</span>
            {toriStatus.bound && (
              <span className="text-xs px-2 py-0.5 rounded-full" style={{ background: "#22c55e20", color: "var(--success)" }}>
                {zh ? "已绑定" : "Connected"}
              </span>
            )}
          </div>
          {toriStatus.bound ? (
            <div className="space-y-2">
              <div className="text-sm" style={{ color: "var(--text-secondary)" }}>
                {toriStatus.username && <span className="font-medium">{toriStatus.username}</span>}
                {toriStatus.email && <span className="ml-2 text-xs" style={{ color: "var(--text-muted)" }}>{toriStatus.email}</span>}
              </div>
              <div className="text-xs" style={{ color: "var(--text-muted)" }}>
                {toriStatus.tori_base_url} • API Key: {toriStatus.has_api_key ? "✓" : "✗"}
              </div>
              <button onClick={unbindTori}
                className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs mt-2 cursor-pointer"
                style={{ background: "var(--bg-hover)", color: "var(--danger)" }}>
                <Unlink size={12} /> {zh ? "解除绑定" : "Unbind"}
              </button>
            </div>
          ) : (
            <div className="space-y-3">
              <p className="text-xs" style={{ color: "var(--text-muted)" }}>
                {zh ? "绑定 Tori 账号后可自动获取 API Key 和可用模型列表" : "Bind your Tori account to auto-get API keys and model list"}
              </p>
              <div className="flex gap-2">
                <input value={toriUrl} onChange={e => setToriUrl(e.target.value)}
                  placeholder="https://tori.example.com"
                  className="flex-1 rounded-lg px-3 py-2 text-sm border outline-none"
                  style={{ background: "var(--bg-hover)", borderColor: "var(--border)", color: "var(--text)" }} />
                <button onClick={bindTori} disabled={!toriUrl}
                  className="flex items-center gap-1.5 rounded-lg px-4 py-2 text-sm font-medium cursor-pointer disabled:opacity-40"
                  style={{ background: "var(--accent)", color: "white" }}>
                  <Link2 size={14} /> {zh ? "绑定" : "Bind"}
                </button>
              </div>
            </div>
          )}
        </div>
      </BlurFade>

      {/* Active Providers */}
      {providers.length > 0 && (
        <BlurFade delay={0.15}>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-sm font-medium mb-3">{zh ? "已配置的提供商" : "Active Providers"}</div>
            <div className="space-y-2">
              {providers.map(p => {
                const TierIcon = tierIcon[p.tier || ""] || Cpu;
                return (
                  <div key={p.id} className="flex items-center gap-3 rounded-lg p-3"
                    style={{ background: "var(--bg-hover)" }}>
                    <TierIcon size={16} style={{ color: tierColor[p.tier || ""] || "var(--text-muted)" }} />
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium truncate">{p.display_name || p.id}</div>
                      <div className="text-xs truncate" style={{ color: "var(--text-muted)" }}>
                        {p.model} • {p.source || "direct"}
                      </div>
                    </div>
                    <span className="text-xs px-2 py-0.5 rounded-full"
                      style={{
                        background: p.enabled ? "#22c55e20" : "#ef444420",
                        color: p.enabled ? "var(--success)" : "var(--danger)",
                      }}>
                      {p.enabled ? (zh ? "启用" : "Active") : (zh ? "停用" : "Disabled")}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
        </BlurFade>
      )}

      {/* Provider Presets */}
      <BlurFade delay={0.2}>
        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="flex items-center gap-2 mb-4">
            <Plus size={16} style={{ color: "var(--accent)" }} />
            <span className="text-sm font-medium">{zh ? "添加提供商" : "Add Provider"}</span>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
            {presets.filter(p => p.id !== "custom").map(preset => {
              const isAdding = addingPreset === preset.id;
              const already = providers.some(p => p.preset_id === preset.id);
              return (
                <div key={preset.id}>
                  <button
                    onClick={() => setAddingPreset(isAdding ? null : preset.id)}
                    disabled={already}
                    className="w-full rounded-xl border p-4 text-left transition-all cursor-pointer disabled:opacity-40"
                    style={{
                      background: isAdding ? "var(--accent-bg)" : "var(--bg-hover)",
                      borderColor: isAdding ? "var(--accent)" : "var(--border)",
                    }}>
                    <div className="text-sm font-medium">{preset.name}</div>
                    {preset.description && (
                      <div className="text-xs mt-1 truncate" style={{ color: "var(--text-muted)" }}>{preset.description}</div>
                    )}
                    <div className="text-xs mt-2 flex items-center gap-1" style={{ color: "var(--text-muted)" }}>
                      {preset.models.length} {zh ? "个模型" : "models"}
                      {already && <> • <Check size={10} /> {zh ? "已添加" : "Added"}</>}
                    </div>
                  </button>

                  {isAdding && !already && (
                    <div className="mt-2 rounded-lg border p-3 space-y-2" style={{ background: "var(--bg-hover)", borderColor: "var(--border)" }}>
                      <select value={addForm.model} onChange={e => setAddForm(f => ({ ...f, model: e.target.value }))}
                        className="w-full rounded-lg px-3 py-2 text-sm border outline-none"
                        style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }}>
                        <option value="">{zh ? "选择模型" : "Select model"}</option>
                        {preset.models.map(m => <option key={m.id} value={m.id}>{m.name}</option>)}
                      </select>
                      <input value={addForm.api_key} onChange={e => setAddForm(f => ({ ...f, api_key: e.target.value }))}
                        type="password" placeholder="API Key"
                        className="w-full rounded-lg px-3 py-2 text-sm border outline-none"
                        style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }} />
                      <button onClick={() => registerProvider(preset.id)}
                        disabled={!addForm.model || !addForm.api_key}
                        className="w-full rounded-lg px-3 py-2 text-sm font-medium cursor-pointer disabled:opacity-40"
                        style={{ background: "var(--accent)", color: "white" }}>
                        <Plus size={14} className="inline mr-1" /> {zh ? "添加" : "Add"}
                      </button>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </BlurFade>
    </div>
  );
}
