"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { api, type ConfigField, type ConfigGroup, getApiKey, setApiKey } from "@/lib/api";
import { showToast } from "@/components/toast-provider";
import {
  Button, Spinner, TextField, Input, Label,
  Select, ListBox, Chip, Separator,
} from "@heroui/react";
import {
  Key, Save, Check, AlertTriangle, RefreshCw, Settings, Cpu, Layers,
  Shield, Plug, Database, Eye, EyeOff, Rocket, FolderOpen, FolderCheck,
  Heart, ExternalLink, Cloud, Search, User,
} from "lucide-react";
import { PreferencesPanel } from "@/components/preferences-panel";
import { SettingsCard } from "./_components/settings-card";

const groupMeta: Record<string, { icon: React.ElementType; color: string }> = {
  preferences: { icon: User,       color: "var(--yunque-accent)" },
  core:        { icon: Cpu,        color: "var(--yunque-accent)" },
  multimodel:  { icon: Layers,     color: "#8b5cf6" },
  advanced:    { icon: Settings,   color: "#f59e0b" },
  embedding:   { icon: Database,   color: "#06b6d4" },
  channels:    { icon: Plug,       color: "#10b981" },
  filesystem:  { icon: FolderOpen, color: "#f97316" },
  security:    { icon: Shield,     color: "#ef4444" },
  storage:     { icon: Heart,      color: "#8b5cf6" },
  sandbox_cloud: { icon: Cloud,   color: "#22d3ee" },
  other:       { icon: Settings,   color: "var(--yunque-text-muted)" },
};

const providerGroups = new Set(["core", "multimodel"]);
const preferencesGroup: ConfigGroup = { key: "preferences", label: "Preferences", label_zh: "个性化", fields: [] };

// Visibility is driven per-field by `tier` (set in the backend schema), not
// per-group. An empty/unknown tier is treated as "advanced" so a newly added
// field is never accidentally surfaced as common or buried as expert.
type TierLevel = "common" | "advanced" | "expert";
const TIER_RANK: Record<string, number> = { common: 0, advanced: 1, expert: 2 };
const TIER_META: { level: TierLevel; label: string }[] = [
  { level: "common", label: "常用" },
  { level: "advanced", label: "高级" },
  { level: "expert", label: "专家" },
];

function fieldTier(field: ConfigField): TierLevel {
  const t = field.tier || "advanced";
  return (t === "common" || t === "expert") ? t : "advanced";
}

function fieldVisibleAt(field: ConfigField, level: TierLevel): boolean {
  return TIER_RANK[fieldTier(field)] <= TIER_RANK[level];
}

// buildVisibleSchema drops provider groups (configured on /settings/providers)
// and filters each group's fields down to the current tier level. Groups left
// empty after filtering are dropped, except the synthetic preferences group
// which is always available.
function buildVisibleSchema(groups: ConfigGroup[], level: TierLevel): ConfigGroup[] {
  return groups
    .filter(group => !providerGroups.has(group.key))
    .map(group => group.key === "preferences"
      ? group
      : { ...group, fields: (group.fields || []).filter(f => fieldVisibleAt(f, level)) })
    .filter(group => group.key === "preferences" || group.fields.length > 0);
}

export default function SettingsPage() {
  const [clientKey, setClientKey] = useState("");
  const [savedKey, setSavedKey] = useState(false);
  const [rawSchema, setRawSchema] = useState<ConfigGroup[]>([]);
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
  const [tierLevel, setTierLevel] = useState<TierLevel>("common");
  const [configError, setConfigError] = useState<string | null>(null);

  useEffect(() => { setClientKey(getApiKey()); loadConfig(); }, []);

  const showAdvanced = tierLevel !== "common";
  const schema = useMemo(() => buildVisibleSchema(rawSchema, tierLevel), [rawSchema, tierLevel]);

  // When lowering the tier, the active group may no longer be visible; fall
  // back to the first available group so the content pane never goes blank.
  useEffect(() => {
    if (!schema.length) return;
    if (!schema.some(g => g.key === activeGroup)) {
      setActiveGroup(schema[0].key);
    }
  }, [schema, activeGroup]);

  const loadConfig = useCallback(async () => {
    setLoading(true);
    setConfigError(null);
    try {
      const [schemaRes, configRes] = await Promise.allSettled([
        api.getConfigSchema(),
        api.getConfig(),
      ]);

      const s = schemaRes.status === "fulfilled" ? schemaRes.value : { groups: [] as ConfigGroup[] };
      const c = configRes.status === "fulfilled" ? configRes.value : { values: values };
      const errors = [
        schemaRes.status === "rejected" ? `配置项加载失败：${String((schemaRes.reason as Error)?.message || schemaRes.reason)}` : "",
        configRes.status === "rejected" ? `配置值加载失败：${String((configRes.reason as Error)?.message || configRes.reason)}` : "",
      ].filter(Boolean);
      if (errors.length) setConfigError(errors.join("；"));

      const filtered = s.groups.filter(g => !providerGroups.has(g.key));
      const withPreferences = [preferencesGroup, ...filtered];
      setRawSchema(withPreferences);
      setValues(c.values);
      if (!activeGroup) setActiveGroup("preferences");
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
        setSaveResult({ ok: true, msg: reload.success ? "已保存并生效" : `已保存，部分配置需稍后生效：${reload.error || "请检查模型配置"}` });
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

  const selectTier = (level: TierLevel) => {
    setTierLevel(level);
    setSearchQuery("");
  };

  if (loading) return <div className="flex-1 flex items-center justify-center"><Spinner size="lg" /></div>;

  const q = searchQuery.toLowerCase().trim();
  // Search spans every tier (not just the visible one) so a buried field is
  // always findable; the tier toggle only governs default browsing.
  const currentGroup = q
    ? (() => {
        const allFields = rawSchema
          .filter(g => !providerGroups.has(g.key) && g.key !== "preferences")
          .flatMap(g => (g.fields || []).map(f => ({ ...f, groupLabel: g.label_zh || g.label })));
        const matched = allFields.filter(f =>
          (f.key || "").toLowerCase().includes(q) ||
          (f.label || "").toLowerCase().includes(q) ||
          (f.label_zh || "").toLowerCase().includes(q) ||
          (f.hint || "").toLowerCase().includes(q)
        );
        return matched.length ? { key: "_search", label: "搜索结果", label_zh: "搜索结果", fields: matched } as ConfigGroup : null;
      })()
    : schema.find(g => g.key === activeGroup);
  // Count fields not visible at the current tier, so the toggle can hint how
  // much is still folded away.
  const hiddenAdvancedCount = rawSchema.reduce((count, group) => {
    if (providerGroups.has(group.key) || group.key === "preferences") return count;
    const hidden = (group.fields || []).filter(f => !fieldVisibleAt(f, tierLevel)).length;
    return count + hidden;
  }, 0);

  return (
    <div>

      {/* Actions bar */}
      <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", margin: "var(--sp-4) 0" }}>
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

      {/* Top guide cards: setup / provider / api-key, laid out side by side */}
      <div className="settings-card-grid">
        {setupNeeded && (
          <SettingsCard
            tone="warning"
            href="/setup"
            icon={<Rocket size={18} style={{ color: "var(--yunque-warning)" }} />}
            title="首次设置"
            desc="用一分钟向导配好模型，完成后自动进入对话。"
            action={<span className="kpi-sub">去配置 →</span>}
          />
        )}

        <SettingsCard
          tone="accent"
          href="/settings/providers"
          icon={<Cpu size={16} style={{ color: "var(--yunque-accent)" }} />}
          title="模型与提供商配置"
          desc="选择 DeepSeek / Qwen / OpenAI / Tori 等模型来源；这里不再混放底层环境变量"
          action={<ExternalLink size={14} style={{ color: "var(--yunque-text-muted)" }} />}
        />

        {showAdvanced && (
          <SettingsCard
            tone="accent"
            icon={<Key size={14} style={{ color: "var(--yunque-accent)" }} />}
            title="访问凭证（高级）"
            desc="只有在你手动调用受保护 API 或调试认证时才需要修改；普通桌面使用可以忽略。"
            action={<span className="kpi-sub">仅保存在本机浏览器</span>}
          >
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
          </SettingsCard>
        )}
      </div>

      {/* Tier selector: 常用 / 高级 / 专家 */}
      <SettingsCard
        tone="accent"
        icon={<Settings size={15} style={{ color: showAdvanced ? "var(--yunque-accent)" : "var(--yunque-text-muted)" }} />}
        title="设置显示层级"
        desc="常用：思考深度、心跳、文件访问、个性化。高级：嵌入、IM 频道、云沙箱。专家：安全、存储、监听地址。"
        action={
          <div style={{ display: "inline-flex", gap: "var(--sp-1)" }}>
            {TIER_META.map(t => {
              const active = tierLevel === t.level;
              // How many extra fields this tier reveals beyond the current one.
              const reveals = TIER_RANK[t.level] > TIER_RANK[tierLevel]
                ? rawSchema.reduce((n, g) => {
                    if (providerGroups.has(g.key) || g.key === "preferences") return n;
                    return n + (g.fields || []).filter(f =>
                      !fieldVisibleAt(f, tierLevel) && fieldVisibleAt(f, t.level)).length;
                  }, 0)
                : 0;
              return (
                <Button key={t.level} size="sm"
                  variant={active ? "outline" : "ghost"}
                  onPress={() => selectTier(t.level)}
                  style={active ? { borderColor: "var(--yunque-accent)", color: "var(--yunque-accent)", fontWeight: 600 } : undefined}>
                  {t.label}{reveals ? ` +${reveals}` : ""}
                </Button>
              );
            })}
          </div>
        }
      />

      {configError && (
        <SettingsCard
          tone="danger"
          icon={<AlertTriangle size={14} style={{ color: "var(--yunque-danger)" }} />}
          title={<span style={{ color: "var(--yunque-danger)" }}>设置配置没有加载完整</span>}
          desc={`${configError}。这不是功能被删除，通常是登录态/权限过期；请重新登录或点击右上角刷新。`}
        />
      )}

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
            {!showAdvanced && !configError && (
              <div className="settings-nav-hint">
                当前为常用层级；切换到高级/专家可见更多，搜索始终覆盖全部配置。
              </div>
            )}
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
                <div className="settings-section-head">
                  {(() => { const M = (groupMeta[currentGroup.key] || groupMeta.other); const I = M.icon; return <I size={17} style={{ color: M.color }} />; })()}
                  <div style={{ flex: 1 }}>
                    <span className="settings-section-head__title">{currentGroup.label_zh || currentGroup.label}</span>
                    <span className="settings-section-head__count">{currentGroup.fields?.length || 0} 项配置</span>
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
                      <div className="settings-section-head" style={{ marginBottom: "var(--sp-3)", paddingBottom: 0, borderBottom: "none" }}>
                        <FolderCheck size={14} style={{ color: "var(--yunque-accent)" }} />
                        <span className="settings-section-head__title" style={{ fontSize: "var(--text-sm)" }}>自动探测目录</span>
                        <Button className="settings-section-head__action" size="sm" variant="outline" isPending={detectLoading} onPress={handleDetectDirs}>
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
