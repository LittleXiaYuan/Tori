"use client";

import { useState, useEffect } from "react";
import {
  api,
  getApiKey,
  setApiKey,
  type ConfigGroup,
} from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  Key,
  Save,
  Check,
  AlertTriangle,
  RefreshCw,
  ChevronDown,
  ChevronRight,
  Settings,
  Cpu,
  Layers,
  Shield,
  Plug,
  Database,
  Eye,
  EyeOff,
} from "lucide-react";

const groupIcons: Record<string, React.ElementType> = {
  core: Cpu,
  multimodel: Layers,
  advanced: Settings,
  embedding: Database,
  channels: Plug,
  security: Shield,
  other: Settings,
};

export default function SettingsPage() {
  const { locale } = useI18n();
  const [clientKey, setClientKey] = useState("");
  const [savedKey, setSavedKey] = useState(false);

  const [schema, setSchema] = useState<ConfigGroup[]>([]);
  const [values, setValues] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saveResult, setSaveResult] = useState<{ ok: boolean; msg: string } | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set(["core"]));
  const [showPwd, setShowPwd] = useState<Set<string>>(new Set());
  const [setupNeeded, setSetupNeeded] = useState(false);

  useEffect(() => {
    setClientKey(getApiKey());
    loadConfig();
  }, []);

  const loadConfig = async () => {
    setLoading(true);
    try {
      try {
        const chk = await api.checkSetup();
        setSetupNeeded(chk.setup_needed);
        if (chk.setup_needed) setExpanded(new Set(["core", "multimodel", "advanced", "embedding", "channels", "security", "other"]));
      } catch { /* no auth yet */ }
      const [s, c] = await Promise.all([
        api.getConfigSchema().catch(() => ({ groups: [] as ConfigGroup[] })),
        api.getConfig().catch(() => ({ values: {} as Record<string, string> })),
      ]);
      setSchema(s.groups);
      setValues(c.values);
    } finally {
      setLoading(false);
    }
  };

  const saveKey = () => {
    setApiKey(clientKey);
    setSavedKey(true);
    setTimeout(() => setSavedKey(false), 2000);
  };

  const saveConfig = async () => {
    setSaving(true);
    setSaveResult(null);
    try {
      const res = await api.saveConfig(values);
      setSaveResult({ ok: res.success, msg: res.message || res.error || "" });
      if (res.success) {
        const c = await api.getConfig().catch(() => ({ values: {} }));
        setValues(c.values);
      }
    } catch (e: unknown) {
      setSaveResult({ ok: false, msg: e instanceof Error ? e.message : "Save failed" });
    } finally {
      setSaving(false);
    }
  };

  const toggle = (k: string) => setExpanded(p => { const n = new Set(p); n.has(k) ? n.delete(k) : n.add(k); return n; });
  const togglePwd = (k: string) => setShowPwd(p => { const n = new Set(p); n.has(k) ? n.delete(k) : n.add(k); return n; });
  const upd = (k: string, v: string) => { setValues(p => ({ ...p, [k]: v })); setSaveResult(null); };

  return (
    <div className="max-w-3xl">
      <BlurFade delay={0}>
        <h1 className="text-xl font-semibold mb-6">{locale === "zh" ? "设置" : "Settings"}</h1>
      </BlurFade>

      {/* Setup banner */}
      {setupNeeded && (
        <BlurFade delay={0.05}>
          <div className="rounded-xl border p-5 mb-6 flex items-start gap-3"
            style={{ background: "var(--warning-bg)", borderColor: "var(--warning)" }}>
            <AlertTriangle size={20} style={{ color: "var(--warning)" }} className="shrink-0 mt-0.5" />
            <div>
              <div className="text-sm font-medium" style={{ color: "var(--warning)" }}>
                {locale === "zh" ? "首次设置" : "Initial Setup Required"}
              </div>
              <div className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>
                {locale === "zh"
                  ? "请填写核心大模型配置后保存，即可开始使用云雀 Agent。"
                  : "Fill in Core LLM configuration below and save to get started."}
              </div>
            </div>
          </div>
        </BlurFade>
      )}

      {/* Client API Key */}
      <BlurFade delay={0.05}>
        <div className="rounded-xl border p-5 mb-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="flex items-center gap-2 mb-3">
            <Key size={16} style={{ color: "var(--accent)" }} />
            <span className="text-sm font-medium">{locale === "zh" ? "客户端 API Key" : "Client API Key"}</span>
          </div>
          <p className="text-xs mb-3" style={{ color: "var(--text-muted)" }}>
            {locale === "zh" ? "浏览器认证用，保存在 localStorage。" : "Browser auth key, stored in localStorage."}
          </p>
          <div className="flex gap-2">
            <input type="password" value={clientKey} onChange={e => setClientKey(e.target.value)} placeholder="ya_..."
              className="flex-1 rounded-lg px-3 py-2 text-sm outline-none border"
              style={{ background: "var(--bg-hover)", borderColor: "var(--border)", color: "var(--text)" }} />
            <button onClick={saveKey}
              className="flex items-center gap-1.5 rounded-lg px-3 py-2 text-sm font-medium cursor-pointer"
              style={{ background: savedKey ? "#22c55e20" : "var(--accent)", color: savedKey ? "var(--success)" : "white" }}>
              {savedKey ? <><Check size={14} /> Saved</> : <><Save size={14} /> Save</>}
            </button>
          </div>
        </div>
      </BlurFade>

      {/* Config groups */}
      {loading ? (
        <div className="flex items-center justify-center py-16">
          <RefreshCw size={20} className="animate-spin" style={{ color: "var(--text-muted)" }} />
        </div>
      ) : (
        schema.map((g, gi) => {
          const Icon = groupIcons[g.key] || Settings;
          const open = expanded.has(g.key);
          return (
            <BlurFade key={g.key} delay={0.1 + gi * 0.03}>
              <div className="rounded-xl border mb-3 overflow-hidden" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                <button onClick={() => toggle(g.key)}
                  className="w-full flex items-center gap-3 px-5 py-4 text-left cursor-pointer">
                  <Icon size={16} style={{ color: "var(--text-muted)" }} />
                  <span className="text-sm font-medium flex-1">{locale === "zh" ? g.label_zh : g.label}</span>
                  {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                </button>
                {open && (
                  <div className="px-5 pb-5 space-y-3 border-t pt-3" style={{ borderColor: "var(--border)" }}>
                    {g.fields.map(f => (
                      <div key={f.key}>
                        <label className="text-xs font-medium mb-1.5 flex items-center gap-1.5">
                          <span style={{ color: "var(--text-secondary)" }}>{locale === "zh" ? f.label_zh : f.label}</span>
                          {f.required && <span style={{ color: "var(--danger)" }}>*</span>}
                          <span className="text-[10px] font-mono" style={{ color: "var(--text-muted)" }}>{f.key}</span>
                        </label>
                        {f.type === "select" ? (
                          <select value={values[f.key] || ""} onChange={e => upd(f.key, e.target.value)}
                            className="w-full rounded-lg px-3 py-2 text-sm border outline-none"
                            style={{ background: "var(--bg-hover)", borderColor: "var(--border)", color: "var(--text)" }}>
                            <option value="" style={{ background: "var(--bg-card)", color: "var(--text)" }}>—</option>
                            {f.options?.map(o => <option key={o} value={o} style={{ background: "var(--bg-card)", color: "var(--text)" }}>{o}</option>)}
                          </select>
                        ) : (
                          <div className="relative">
                            <input
                              type={f.type === "password" && !showPwd.has(f.key) ? "password" : "text"}
                              value={values[f.key] || ""} onChange={e => upd(f.key, e.target.value)}
                              placeholder={f.placeholder}
                              className="w-full rounded-lg px-3 py-2 text-sm border outline-none pr-9"
                              style={{ background: "var(--bg-hover)", borderColor: "var(--border)", color: "var(--text)" }} />
                            {f.type === "password" && (
                              <button type="button" onClick={() => togglePwd(f.key)}
                                className="absolute right-2.5 top-1/2 -translate-y-1/2 cursor-pointer"
                                style={{ color: "var(--text-muted)" }}>
                                {showPwd.has(f.key) ? <EyeOff size={14} /> : <Eye size={14} />}
                              </button>
                            )}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </BlurFade>
          );
        })
      )}

      {/* Save */}
      {schema.length > 0 && (
        <BlurFade delay={0.3}>
          <div className="flex items-center gap-3 mt-4">
            <button onClick={saveConfig} disabled={saving}
              className="btn-glow flex items-center gap-2 rounded-xl px-5 py-2.5 text-sm font-medium cursor-pointer">
              {saving ? <RefreshCw size={14} className="animate-spin" /> : <Save size={14} />}
              {locale === "zh" ? "保存配置" : "Save Configuration"}
            </button>
            {saveResult && (
              <span className="text-xs flex items-center gap-1.5"
                style={{ color: saveResult.ok ? "var(--success)" : "var(--danger)" }}>
                {saveResult.ok ? <Check size={14} /> : <AlertTriangle size={14} />}
                {saveResult.msg}
              </span>
            )}
          </div>
        </BlurFade>
      )}
    </div>
  );
}
