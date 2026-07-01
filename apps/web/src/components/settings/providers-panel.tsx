"use client";

/**
 * ProvidersPanel — minimal, OpenAI/Anthropic-style custom model manager.
 *
 * Replaces the previous 1396-line multi-tab panel (presets / mode / routing /
 * Tori). The model now is dead simple: a list of custom models you add by hand
 * — each with your own name, a base URL, an API key and a model id — plus the
 * ability to enable/disable, test and delete them. Anything fancier (presets,
 * smart routing, Tori binding) is intentionally gone.
 *
 * Shared by the settings modal (Models section) and the legacy
 * /settings/providers route; both render the same component.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, TextField, Input, Label, Switch, Tooltip } from "@heroui/react";
import { Cpu, Plus, Trash2, Zap, Loader2 } from "lucide-react";
import { api, type ProviderInfo } from "@/lib/api";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";

interface DraftModel {
  name: string;
  baseUrl: string;
  apiKey: string;
  model: string;
}

const EMPTY_DRAFT: DraftModel = { name: "", baseUrl: "", apiKey: "", model: "" };

function slugify(name: string, model: string): string {
  const base = `${name || model || "custom"}`
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 40);
  // Suffix with a short time-based token so two same-named entries don't clash.
  const suffix = Date.now().toString(36).slice(-4);
  return `${base || "custom"}-${suffix}`;
}

export function ProvidersPanel(_props?: {
  initialTab?: string;
  focusProviderId?: string | null;
  onNavigateChat?: () => void;
}) {
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [draft, setDraft] = useState<DraftModel>(EMPTY_DRAFT);
  const [submitting, setSubmitting] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);
  const [activeId, setActiveId] = useState<string>("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [list, exec] = await Promise.all([
        api.providerList().catch(() => ({ providers: [] as ProviderInfo[], count: 0 })),
        api.execProvider().catch(() => ({ exec_provider: "", available_providers: [] as string[] })),
      ]);
      setProviders(list.providers || []);
      setActiveId(exec.exec_provider || "");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const canSubmit = draft.baseUrl.trim() !== "" && draft.model.trim() !== "";

  const submit = async () => {
    if (!canSubmit || submitting) return;
    setSubmitting(true);
    try {
      await api.providerRegister({
        id: slugify(draft.name, draft.model),
        name: draft.name.trim() || draft.model.trim(),
        base_url: draft.baseUrl.trim(),
        api_key: draft.apiKey.trim(),
        model: draft.model.trim(),
      });
      showToast("已添加模型", "success");
      setDraft(EMPTY_DRAFT);
      setAdding(false);
      await load();
    } catch (e) {
      showToast(formatErrorMessage(e, "添加失败"), "error");
    }
    setSubmitting(false);
  };

  const remove = async (p: ProviderInfo) => {
    try {
      await api.providerDelete(p.id);
      showToast("已删除", "success");
      await load();
    } catch (e) {
      showToast(formatErrorMessage(e, "删除失败"), "error");
    }
  };

  const toggle = async (p: ProviderInfo) => {
    try {
      if (p.enabled) await api.providerDisable(p.id);
      else await api.providerEnable(p.id);
      await load();
    } catch (e) {
      showToast(formatErrorMessage(e, "切换失败"), "error");
    }
  };

  const test = async (p: ProviderInfo) => {
    setTesting(p.id);
    try {
      const res = await api.providerTest(p.id);
      const ok = res.status === "ok" || res.status === "healthy";
      showToast(ok ? `连接正常 · ${res.latency_ms}ms` : `连接失败：${res.error || res.status}`, ok ? "success" : "error");
    } catch (e) {
      showToast(formatErrorMessage(e, "测试失败"), "error");
    }
    setTesting(null);
  };

  const useForChat = async (p: ProviderInfo) => {
    try {
      await api.setExecProvider(p.id);
      setActiveId(p.id);
      showToast(`已切换主模型：${p.display_name || p.model}`, "success");
    } catch (e) {
      showToast(formatErrorMessage(e, "切换失败"), "error");
    }
  };

  const sorted = useMemo(
    () => [...providers].sort((a, b) => (a.display_name || a.id).localeCompare(b.display_name || b.id)),
    [providers],
  );

  return (
    <div className="provider-min">
      {/* Add form */}
      {adding ? (
        <div className="provider-min__form">
          <div className="provider-min__form-grid">
            <TextField className="provider-min__field" value={draft.name} onChange={(v) => setDraft({ ...draft, name: v })}>
              <Label>名称（自定义）</Label>
              <Input placeholder="比如：我的 GPT" />
            </TextField>
            <TextField className="provider-min__field" isRequired value={draft.model} onChange={(v) => setDraft({ ...draft, model: v })}>
              <Label>模型 ID</Label>
              <Input placeholder="gpt-4.1 / deepseek-chat / claude-..." />
            </TextField>
            <TextField className="provider-min__field provider-min__field--wide" isRequired value={draft.baseUrl} onChange={(v) => setDraft({ ...draft, baseUrl: v })}>
              <Label>API 地址</Label>
              <Input placeholder="https://api.openai.com/v1" />
            </TextField>
            <TextField className="provider-min__field provider-min__field--wide" value={draft.apiKey} onChange={(v) => setDraft({ ...draft, apiKey: v })}>
              <Label>API Key</Label>
              <Input type="password" placeholder="sk-..." />
            </TextField>
          </div>
          <div className="provider-min__form-actions">
            <Button size="sm" variant="ghost" onPress={() => { setAdding(false); setDraft(EMPTY_DRAFT); }}>取消</Button>
            <Button size="sm" className="btn-accent" isPending={submitting} isDisabled={!canSubmit} onPress={submit}>添加</Button>
          </div>
        </div>
      ) : (
        <Button size="sm" variant="outline" className="self-start" onPress={() => setAdding(true)}>
          <Plus size={15} /> 添加模型
        </Button>
      )}

      {/* List */}
      {loading ? (
        <div className="provider-min__empty">加载中…</div>
      ) : sorted.length === 0 ? (
        <div className="provider-min__empty">还没有模型。点「添加模型」填入 API 地址和模型 ID 即可。</div>
      ) : (
        <div className="provider-min__list">
          {sorted.map((p) => {
            const isActive = p.id === activeId;
            return (
              <div key={p.id} className={`provider-min__row ${p.enabled ? "" : "is-off"}`}>
                <span className="provider-min__icon"><Cpu size={15} /></span>
                <div className="provider-min__meta">
                  <div className="provider-min__name">
                    {p.display_name || p.model}
                    {isActive && <span className="provider-min__active">主模型</span>}
                  </div>
                  <div className="provider-min__sub">{p.model} · {p.base_url}</div>
                </div>
                <div className="provider-min__actions">
                  {!isActive && p.enabled && (
                    <Button size="sm" variant="ghost" onPress={() => useForChat(p)}>设为主模型</Button>
                  )}
                  <Tooltip delay={0}>
                    <Tooltip.Trigger>
                      <Button isIconOnly size="sm" variant="ghost" aria-label="测试连接" isDisabled={testing === p.id} onPress={() => test(p)}>
                        {testing === p.id ? <Loader2 size={14} className="provider-min__spin" /> : <Zap size={14} />}
                      </Button>
                    </Tooltip.Trigger>
                    <Tooltip.Content>测试连接</Tooltip.Content>
                  </Tooltip>
                  <Switch isSelected={p.enabled} onChange={() => toggle(p)} aria-label={p.enabled ? "禁用" : "启用"}>
                    <Switch.Control><Switch.Thumb /></Switch.Control>
                  </Switch>
                  <Tooltip delay={0}>
                    <Tooltip.Trigger>
                      <Button isIconOnly size="sm" variant="ghost" aria-label="删除" onPress={() => remove(p)} style={{ color: "var(--yunque-text-muted)" }}>
                        <Trash2 size={14} />
                      </Button>
                    </Tooltip.Trigger>
                    <Tooltip.Content>删除</Tooltip.Content>
                  </Tooltip>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
