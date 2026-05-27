"use client";

import { useState, useEffect, useCallback } from "react";
import { api, type ConfigGroup, getApiKey, setApiKey } from "@/lib/api";
import { showToast } from "@/components/toast-provider";
import {
  Button, Spinner, TextField, Input, Label,
  Select, ListBox, Chip, Separator,
} from "@heroui/react";
import {
  Key, Save, Check, AlertTriangle, RefreshCw, Settings, Cpu, Layers,
  Shield, Plug, Database, Eye, EyeOff, Rocket, FolderOpen, FolderCheck,
  Smile, Heart, ExternalLink, Cloud, Search, User,
} from "lucide-react";
import { PreferencesPanel } from "@/components/preferences-panel";
import Link from "next/link";

const groupMeta: Record<string, { icon: React.ElementType; color: string }> = {
  preferences: { icon: User,       color: "var(--yunque-accent)" },
  core:        { icon: Cpu,        color: "var(--yunque-accent)" },
  multimodel:  { icon: Layers,     color: "#8b5cf6" },
  advanced:    { icon: Settings,   color: "#f59e0b" },
  embedding:   { icon: Database,   color: "#06b6d4" },
  channels:    { icon: Plug,       color: "#10b981" },
  filesystem:  { icon: FolderOpen, color: "#f97316" },
  security:    { icon: Shield,     color: "#ef4444" },
  emotion:     { icon: Smile,      color: "#ec4899" },
  storage:     { icon: Heart,      color: "#8b5cf6" },
  sandbox_cloud: { icon: Cloud,   color: "#22d3ee" },
  other:       { icon: Settings,   color: "var(--yunque-text-muted)" },
};

export default function SettingsPage() {
  const [clientKey, setClientKey] = useState("");
  const [savedKey, setSavedKey] = useState(false);
  const [schema, setSchema] = useState<ConfigGroup[]>([]);
  const [values, setValues] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saveResult, setSaveResult] = useState<{ ok: boolean; msg: string } | null>(null);
  const [showPwd, setShowPwd] = useState<Set<string>>(new Set());
  const [setupNeeded, setSetupNeeded] = useState(false);
  const [activeGroup, setActiveGroup] = useState("");
  const [detectedDirs, setDetectedDirs] = useState<Array<{ label: string; label_zh: string; path: string; exists: boolean; kind: string }>>([]);
  const [detectLoading, setDetectLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");

  useEffect(() => { setClientKey(getApiKey()); loadConfig(); }, []);

  const providerGroups = new Set(["core", "multimodel"]);
  const preferencesGroup = { key: "preferences", label: "个性化", label_zh: "个性化", fields: [] };

  const loadConfig = useCallback(async () => {
    setLoading(true);
    try {
      const [s, c] = await Promise.all([
        api.getConfigSchema().catch(() => ({ groups: [] as ConfigGroup[] })),
        api.getConfig().catch(() => ({ values: {} as Record<string, string> })),
      ]);
      const filtered = s.groups.filter(g => !providerGroups.has(g.key));
      const withPreferences = [preferencesGroup, ...filtered];
      setSchema(withPreferences);
      setValues(c.values);
      if (withPreferences.length && !activeGroup) setActiveGroup("preferences");
      try { const chk = await api.checkSetup(); setSetupNeeded(chk.setup_needed); } catch { /* no auth */ }
    } finally { setLoading(false); }
  }, [activeGroup]);

  const saveKey = () => { setApiKey(clientKey); setSavedKey(true); setTimeout(() => setSavedKey(false), 2000); };

  const handleSaveConfig = async () => {
    setSaving(true);
    setSaveResult(null);
    try {
      await api.saveConfig(values);
      try {
        const reload = await api.configReload();
        setSaveResult({ ok: true, msg: reload.success ? "已保存并生效" : `已保存，但生效失败：${reload.error || "请检查 LLM 配置"}` });
        if (reload.success) setSetupNeeded(false);
      } catch { setSaveResult({ ok: true, msg: "已保存，重载未生效" }); }
    } catch (e: unknown) {
      setSaveResult({ ok: false, msg: String((e as Error)?.message || "保存失败") });
    }
    setSaving(false);
    setTimeout(() => setSaveResult(null), 4000);
  };

  const handleDetectDirs = async () => {
    setDetectLoading(true);
    try {
      const res = await api.detectDirs();
      setDetectedDirs(res.dirs || []);
      if (res.default_paths?.length && !values["HOST_READ_PATHS"]) {
        setValues(prev => ({ ...prev, HOST_READ_PATHS: res.default_paths.join(",") }));
      }
    } catch (e) { showToast(e instanceof Error ? e.message : "目录探测失败", "error"); }
    setDetectLoading(false);
  };

  const addDirToReadPaths = (path: string) => {
    const parts = (values["HOST_READ_PATHS"] || "").split(",").map(s => s.trim()).filter(Boolean);
    if (!parts.includes(path)) {
      parts.push(path);
      setValues(prev => ({ ...prev, HOST_READ_PATHS: parts.join(",") }));
    }
  };

  const togglePwd = (key: string) => {
    setShowPwd(prev => { const n = new Set(prev); prev.has(key) ? n.delete(key) : n.add(key); return n; });
  };

  const upd = (key: string, val: string) => { setValues(prev => ({ ...prev, [key]: val })); setSaveResult(null); };

  if (loading) return <div className="flex-1 flex items-center justify-center"><Spinner size="lg" /></div>;

  const q = searchQuery.toLowerCase().trim();
  const currentGroup = q
    ? (() => {
        const allFields = schema.flatMap(g => (g.fields || []).map(f => ({ ...f, groupLabel: g.label_zh || g.label })));
        const matched = allFields.filter(f =>
          (f.key || "").toLowerCase().includes(q) ||
          (f.label || "").toLowerCase().includes(q) ||
          (f.label_zh || "").toLowerCase().includes(q) ||
          (f.hint || "").toLowerCase().includes(q)
        );
        return matched.length ? { key: "_search", label: "搜索结果", label_zh: "搜索结果", fields: matched } as ConfigGroup : null;
      })()
    : schema.find(g => g.key === activeGroup);

  return (
    <div>
      {/* Actions bar */}
      <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", marginBottom: "var(--sp-4)" }}>
        {saveResult && (
          <Chip size="sm" variant={saveResult.ok ? "soft" : "soft"}
            style={{ color: saveResult.ok ? "var(--yunque-success)" : "var(--yunque-danger)" }}>
            {saveResult.ok ? <Check size={12} /> : <AlertTriangle size={12} />} {saveResult.msg}
          </Chip>
        )}
        <div className="flex-1" />
        <a href="https://yunque.owo.today/zh/guide/configuration" target="_blank" rel="noopener noreferrer"
          style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", textDecoration: "none", padding: "4px 8px", borderRadius: "var(--radius-sm)", border: "1px solid var(--yunque-border)", transition: "border-color 0.15s ease" }}>
          <ExternalLink size={11} /> 配置说明
        </a>
        <Button variant="ghost" size="sm" onPress={loadConfig}><RefreshCw size={13} /></Button>
        <Button size="sm" isPending={saving} onPress={handleSaveConfig}
          style={{ background: "var(--yunque-accent)", color: "#fff", fontWeight: 600 }}>
          <Save size={13} /> 保存
        </Button>
      </div>

      {/* Setup Banner */}
      {setupNeeded && (
        <div className="section-card" style={{
          borderLeft: "3px solid var(--yunque-warning)",
          background: "var(--yunque-warning-muted)",
          display: "flex", alignItems: "flex-start", gap: "var(--sp-3)",
          padding: "var(--card-pad-sm)",
        }}>
          <Rocket size={18} style={{ color: "var(--yunque-warning)", marginTop: 2, flexShrink: 0 }} />
          <div>
            <div style={{ fontSize: "var(--text-md)", fontWeight: 600 }}>首次设置</div>
            <p style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-secondary)", marginTop: "var(--sp-1)" }}>
              在「核心大模型」中填写 API 地址和密钥，点击「保存」即可开始使用。
            </p>
          </div>
        </div>
      )}

      {/* Provider redirect card */}
      <Link href="/settings/providers" className="section-card" style={{
        padding: "var(--card-pad-sm)", display: "flex", alignItems: "center", gap: "var(--sp-3)",
        textDecoration: "none", cursor: "pointer", transition: "border-color 0.15s ease",
        borderLeft: "3px solid var(--yunque-accent)",
      }}>
        <Cpu size={16} style={{ color: "var(--yunque-accent)", flexShrink: 0 }} />
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--yunque-text)" }}>模型与提供商配置</div>
          <p style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", marginTop: 2 }}>
            LLM 接入模式、API 密钥、Tori 中转等设置已移至专属页面
          </p>
        </div>
        <ExternalLink size={14} style={{ color: "var(--yunque-text-muted)", flexShrink: 0 }} />
      </Link>

      {/* API Key */}
      <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: "var(--sp-2)" }}>
          <Key size={13} style={{ color: "var(--yunque-accent)" }} />
          <span style={{ fontSize: "var(--text-sm)", fontWeight: 600 }}>本地 API 密钥</span>
          <span className="kpi-sub" style={{ marginLeft: "auto" }}>存储在 localStorage</span>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
          <TextField className="flex-1" name="client-api-key" type={showPwd.has("api-key") ? "text" : "password"}
            value={clientKey} onChange={setClientKey}>
            <Label className="sr-only">API Key</Label>
            <div style={{ position: "relative" }}>
              <Input placeholder="输入 API Key" style={{ fontFamily: "monospace" }} />
              <Button isIconOnly aria-label="切换密码可见" variant="ghost" size="sm"
                onPress={() => togglePwd("api-key")} style={{
                  position: "absolute", right: 6, top: "50%", transform: "translateY(-50%)",
                }}>
                {showPwd.has("api-key") ? <EyeOff size={13} /> : <Eye size={13} />}
              </Button>
            </div>
          </TextField>
          <Button size="sm" onPress={saveKey} style={{
            background: savedKey ? "var(--yunque-success-muted)" : "var(--yunque-accent)",
            color: savedKey ? "var(--yunque-success)" : "#fff", fontWeight: 600, flexShrink: 0,
          }}>
            {savedKey ? <><Check size={12} /> 已存</> : <><Save size={12} /> 保存</>}
          </Button>
        </div>
      </div>

      {/* Main settings area: sidebar + content */}
      {schema.length > 0 && (
        <div className="settings-layout">
          {/* Left navigation */}
          <nav className="settings-nav">
            <div className="flex items-center gap-2 rounded-lg px-2.5 py-1.5 mb-2 text-xs"
              style={{ background: "var(--yunque-bg-muted)", color: "var(--yunque-text-muted)" }}>
              <Search size={12} />
              <input
                className="bg-transparent outline-none flex-1 text-xs"
                placeholder="搜索配置项…"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                style={{ color: "var(--yunque-text)" }}
              />
            </div>
            {schema.map(group => {
              const meta = groupMeta[group.key] || groupMeta.other;
              const Icon = meta.icon;
              const active = activeGroup === group.key;
              return (
                <button key={group.key} onClick={() => setActiveGroup(group.key)}
                  className="settings-nav-item" data-active={active || undefined}>
                  <Icon size={15} style={{ color: active ? meta.color : "var(--yunque-text-muted)", flexShrink: 0 }} />
                  <span className="settings-nav-label">{group.label_zh || group.label}</span>
                  <span className="settings-nav-count">{group.fields?.length || 0}</span>
                </button>
              );
            })}
          </nav>

          {/* Right content */}
          <div className="settings-content">
            {currentGroup && currentGroup.key === "preferences" && (
              <PreferencesPanel />
            )}
            {currentGroup && currentGroup.key !== "preferences" && (
              <>
                <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)", marginBottom: "var(--sp-5)", paddingBottom: "var(--sp-3)", borderBottom: "1px solid var(--yunque-border)" }}>
                  {(() => { const M = (groupMeta[currentGroup.key] || groupMeta.other); const I = M.icon; return <I size={17} style={{ color: M.color }} />; })()}
                  <div style={{ flex: 1 }}>
                    <span style={{ fontSize: "var(--text-md)", fontWeight: 600 }}>{currentGroup.label_zh || currentGroup.label}</span>
                    <span style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", marginLeft: "var(--sp-2)" }}>{currentGroup.fields?.length || 0} 项配置</span>
                  </div>
                </div>

                <div className="settings-fields">
                  {currentGroup.fields?.map(field => {
                    const isPwd = field.type === "password" || field.sensitive
                      || field.key?.toLowerCase().includes("key")
                      || field.key?.toLowerCase().includes("secret");
                    const visible = showPwd.has(field.key);

                    if (field.type === "select" && field.options) {
                      return (
                        <div key={field.key} className="settings-field-card">
                          <Select className="w-full"
                            placeholder="请选择"
                            selectedKey={values[field.key] || ""}
                            onSelectionChange={k => upd(field.key, String(k))}>
                            <Label>
                              {field.label_zh || field.label || field.key}
                              {field.required && <span style={{ color: "var(--yunque-danger)", marginLeft: 2 }}>*</span>}
                            </Label>
                            <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                            <Select.Popover>
                              <ListBox>
                                {field.options.map(opt => (
                                  <ListBox.Item key={opt} id={opt} textValue={opt}>{opt}<ListBox.ItemIndicator /></ListBox.Item>
                                ))}
                              </ListBox>
                            </Select.Popover>
                          </Select>
                          {field.hint && <div className="settings-field-hint">{field.hint}</div>}
                        </div>
                      );
                    }

                    return (
                      <div key={field.key} className="settings-field-card">
                        <TextField name={field.key}
                          type={isPwd && !visible ? "password" : "text"}
                          isRequired={field.required}
                          value={values[field.key] || ""}
                          onChange={v => upd(field.key, v)}>
                          <Label>
                            {field.label_zh || field.label || field.key}
                            {field.required && <span style={{ color: "var(--yunque-danger)", marginLeft: 2 }}>*</span>}
                          </Label>
                          <div style={{ position: "relative" }}>
                            <Input placeholder={field.placeholder || ""} />
                            {isPwd && (
                              <Button isIconOnly aria-label="切换密码可见" variant="ghost" size="sm"
                                onPress={() => togglePwd(field.key)} style={{
                                  position: "absolute", right: 6, top: "50%", transform: "translateY(-50%)",
                                }}>
                                {visible ? <EyeOff size={13} /> : <Eye size={13} />}
                              </Button>
                            )}
                          </div>
                        </TextField>
                        {field.hint && <div className="settings-field-hint">{field.hint}</div>}
                      </div>
                    );
                  })}
                </div>

                {/* Filesystem: directory detection panel */}
                {currentGroup.key === "filesystem" && (
                  <>
                    <Separator style={{ margin: "var(--sp-4) 0" }} />
                    <div>
                      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "var(--sp-3)" }}>
                        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                          <FolderCheck size={14} style={{ color: "var(--yunque-accent)" }} />
                          <span style={{ fontSize: "var(--text-sm)", fontWeight: 600 }}>自动探测目录</span>
                        </div>
                        <Button size="sm" variant="outline" isPending={detectLoading} onPress={handleDetectDirs}>
                          <FolderOpen size={12} /> 探测
                        </Button>
                      </div>
                      <p style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-secondary)", marginBottom: "var(--sp-3)" }}>
                        自动发现桌面、文档、下载等目录，点击可添加到只读访问路径。
                      </p>
                      {detectedDirs.length > 0 && (
                        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(240px, 1fr))", gap: "var(--sp-2)" }}>
                          {detectedDirs.map(d => {
                            const isAdded = (values["HOST_READ_PATHS"] || "").split(",").map(s => s.trim()).includes(d.path);
                            return (
                              <button key={d.kind} onClick={() => d.exists && !isAdded && addDirToReadPaths(d.path)}
                                disabled={!d.exists || isAdded}
                                className="settings-dir-item" data-added={isAdded || undefined}>
                                <FolderOpen size={15} style={{ color: isAdded ? "var(--yunque-success)" : "var(--yunque-accent)", flexShrink: 0 }} />
                                <div style={{ minWidth: 0, flex: 1 }}>
                                  <div style={{ fontSize: "var(--text-sm)", fontWeight: 500 }}>{d.label_zh || d.label}</div>
                                  <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                                    {d.path || "未发现"}
                                  </div>
                                </div>
                                {isAdded && <Check size={13} style={{ color: "var(--yunque-success)", flexShrink: 0 }} />}
                              </button>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  </>
                )}
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
