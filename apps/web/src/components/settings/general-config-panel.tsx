"use client";

/**
 * GeneralConfigPanel — the schema-driven backend config, rendered inside the
 * Cherry settings modal instead of the old full-page /settings route.
 *
 * The old page wrapped every field in a large bordered card and rendered all
 * tiers' fields eagerly, which felt heavy and laggy. Here we reuse the modal's
 * dense `cherry-settings-row` language and only render the active tier, so it
 * stays light. Provider groups (core/multimodel) are excluded — those live in
 * the Models section.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, TextField, Input, Select, ListBox } from "@heroui/react";
import { Eye, EyeOff, Save, RefreshCw } from "lucide-react";
import { api, type ConfigField, type ConfigGroup } from "@/lib/api";
import { showToast } from "@/components/toast-provider";

type TierLevel = "common" | "advanced" | "expert";
const TIER_RANK: Record<string, number> = { common: 0, advanced: 1, expert: 2 };
const TIER_META: { level: TierLevel; label: string }[] = [
  { level: "common", label: "常用" },
  { level: "advanced", label: "高级" },
  { level: "expert", label: "专家" },
];

function fieldTier(field: ConfigField): TierLevel {
  const t = field.tier || "advanced";
  return t === "common" || t === "expert" ? t : "advanced";
}
function fieldVisibleAt(field: ConfigField, level: TierLevel): boolean {
  return TIER_RANK[fieldTier(field)] <= TIER_RANK[level];
}

interface GeneralConfigPanelProps {
  /** Only render these schema group keys. Omit to render all. */
  includeGroups?: string[];
  /** Hide the tier toggle + search bar (use when a panel is scoped tight). */
  hideToolbar?: boolean;
  /** Show group sub-headers above each group's fields. Default true. */
  showGroupHeaders?: boolean;
}

function buildVisibleSchema(
  groups: ConfigGroup[],
  level: TierLevel,
  query: string,
  include: Set<string> | null,
  allTiers: boolean,
): ConfigGroup[] {
  const q = query.trim().toLowerCase();
  return groups
    .filter((g) => !include || include.has(g.key))
    .map((g) => ({
      ...g,
      fields: (g.fields || []).filter((f) => {
        if (q) {
          const hay = `${f.key} ${f.label || ""} ${f.label_zh || ""} ${f.hint || ""}`.toLowerCase();
          return hay.includes(q);
        }
        // A scoped panel (includeGroups) shows every field in its groups —
        // the user already narrowed the scope by entering the section, so
        // tier-gating it again just hides everything and reads as "empty".
        return allTiers || fieldVisibleAt(f, level);
      }),
    }))
    .filter((g) => g.fields.length > 0);
}

export function GeneralConfigPanel({ includeGroups, hideToolbar, showGroupHeaders = true }: GeneralConfigPanelProps = {}) {
  // Scoped panels (with includeGroups) ignore tier filtering by default.
  const allTiers = Boolean(includeGroups);
  const includeSet = useMemo(() => (includeGroups ? new Set(includeGroups) : null), [includeGroups]);
  const [rawSchema, setRawSchema] = useState<ConfigGroup[]>([]);
  const [values, setValues] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [tier, setTier] = useState<TierLevel>("common");
  const [query, setQuery] = useState("");
  const [showPwd, setShowPwd] = useState<Set<string>>(new Set());
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [schemaRes, configRes] = await Promise.allSettled([api.getConfigSchema(), api.getConfig()]);
      const s = schemaRes.status === "fulfilled" ? schemaRes.value : { groups: [] as ConfigGroup[] };
      const c = configRes.status === "fulfilled" ? configRes.value : { values: {} };
      if (schemaRes.status === "rejected" || configRes.status === "rejected") {
        setError("部分配置未能加载，通常是登录态/权限过期；请重新登录或刷新。");
      }
      setRawSchema(s.groups || []);
      setValues(c.values || {});
    } catch (e) {
      setError(String((e as Error)?.message || e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const schema = useMemo(() => buildVisibleSchema(rawSchema, tier, query, includeSet, allTiers), [rawSchema, tier, query, includeSet, allTiers]);

  const upd = (key: string, val: string) => setValues((prev) => ({ ...prev, [key]: val }));
  const togglePwd = (key: string) =>
    setShowPwd((prev) => { const n = new Set(prev); n.has(key) ? n.delete(key) : n.add(key); return n; });

  const save = async () => {
    setSaving(true);
    try {
      await api.saveConfig(values);
      try {
        const reload = await api.configReload();
        showToast(reload.success ? "已保存并生效" : `已保存，部分配置需稍后生效`, reload.success ? "success" : "info");
      } catch {
        showToast("已保存，重载未生效", "info");
      }
    } catch (e) {
      showToast(String((e as Error)?.message || "保存失败"), "error");
    }
    setSaving(false);
  };

  return (
    <>
      {/* Tier + search. The tier toggle is meaningful only for the full
          (non-scoped) panel — a scoped panel already shows every tier, so
          showing a toggle that does nothing reads as broken. Hide it there. */}
      {!hideToolbar && (
        <div className="cherry-settings-section" style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
          {!allTiers && (
            <div className="cherry-segmented">
              {TIER_META.map((t) => (
                <button key={t.level} type="button" className={tier === t.level && !query ? "active" : ""} onClick={() => { setTier(t.level); setQuery(""); }}>
                  {t.label}
                </button>
              ))}
            </div>
          )}
          <TextField aria-label="搜索配置项" value={query} onChange={setQuery} className="flex-1 min-w-[140px]">
            <Input placeholder={allTiers ? "搜索配置项…" : "搜索配置项…（覆盖全部层级）"} />
          </TextField>
          <Button size="sm" variant="ghost" onPress={load} aria-label="刷新">
            <RefreshCw size={13} /> 刷新
          </Button>
        </div>
      )}

      {error && (
        <div className="cherry-settings-section" style={{ color: "var(--yunque-danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      {loading ? (
        <div className="cherry-settings-section" style={{ color: "var(--yunque-text-muted)", fontSize: 13 }}>加载中…</div>
      ) : schema.length === 0 ? (
        <div className="cherry-settings-section" style={{ color: "var(--yunque-text-muted)", fontSize: 13 }}>
          {query ? "没有匹配的配置项。" : allTiers ? "该分组暂无配置项。" : "当前层级没有可显示的配置项，切换到「高级 / 专家」试试。"}
        </div>
      ) : (
        schema.map((group) => (
          <div key={group.key} className="cherry-settings-section">
            {showGroupHeaders && (
              <div className="cherry-settings-row-label" style={{ marginBottom: 8, opacity: 0.7 }}>
                {group.label_zh || group.label || group.key}
              </div>
            )}
            {group.fields.map((field) => {
              const isPwd =
                field.type === "password" || field.sensitive ||
                field.key?.toLowerCase().includes("key") || field.key?.toLowerCase().includes("secret");
              const visible = showPwd.has(field.key);
              return (
                <div key={field.key} className="cherry-settings-row">
                  <div style={{ minWidth: 0, flex: 1 }}>
                    <div className="cherry-settings-row-label">
                      {field.label_zh || field.label || field.key}
                      {field.required && <span style={{ color: "var(--yunque-danger)", marginLeft: 3 }}>*</span>}
                    </div>
                    {field.hint && <div className="cherry-settings-row-desc">{field.hint}</div>}
                  </div>
                  <div className="cherry-settings-row-control">
                    {field.type === "select" && field.options ? (
                      <Select
                        aria-label={field.label_zh || field.label || field.key}
                        placeholder="请选择"
                        selectedKey={values[field.key] || null}
                        onSelectionChange={(k) => upd(field.key, k == null ? "" : String(k))}
                        className="config-field-input"
                      >
                        <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                        <Select.Popover>
                          <ListBox>
                            {field.options.map((opt) => (
                              <ListBox.Item key={opt} id={opt} textValue={opt}>{opt}<ListBox.ItemIndicator /></ListBox.Item>
                            ))}
                          </ListBox>
                        </Select.Popover>
                      </Select>
                    ) : (
                      <TextField
                        aria-label={field.label_zh || field.label || field.key}
                        type={isPwd && !visible ? "password" : "text"}
                        value={values[field.key] || ""}
                        onChange={(v) => upd(field.key, v)}
                        className="config-field-input"
                      >
                        <div style={{ position: "relative", display: "flex", alignItems: "center" }}>
                          <Input placeholder={field.placeholder || ""} style={{ paddingRight: isPwd ? 30 : undefined }} />
                          {isPwd && (
                            <button type="button" aria-label="切换可见" onClick={() => togglePwd(field.key)}
                              style={{ position: "absolute", right: 6, background: "transparent", border: "none", cursor: "pointer", color: "var(--yunque-text-muted)" }}>
                              {visible ? <EyeOff size={13} /> : <Eye size={13} />}
                            </button>
                          )}
                        </div>
                      </TextField>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        ))
      )}

      {/* Save lives at the bottom, after the fields — you save what you just
          edited, not before you've seen it. Sticky so it stays reachable on
          long config lists. */}
      {!loading && schema.length > 0 && (
        <div className="config-panel-footer">
          <Button size="sm" className="btn-accent" isPending={saving} onPress={save}>
            <Save size={13} /> 保存
          </Button>
        </div>
      )}
    </>
  );
}
