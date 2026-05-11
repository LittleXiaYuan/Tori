"use client";

import { useState } from "react";
import { Card, Button, Spinner, Chip, Tooltip, TextField, Input, Label, Switch, Select, ListBox } from "@heroui/react";
import { api, type ModelInfo, type ProviderInfo } from "@/lib/api";
import { Cpu, Plus, Trash2, Sparkles, RefreshCw, Download, Wifi, ChevronDown, ChevronUp } from "lucide-react";
import PageHeader from "@/components/page-header";
import { useApiData } from "@/lib/use-api-data";
import { showErrorToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";

const clientLabels: Record<string, string> = {
  openai: "OpenAI", anthropic: "Anthropic", google: "Google", ollama: "Ollama",
};

export default function ModelsPage() {
  const { data, loading, refresh } = useApiData(
    async () => {
      const [modelsRes, providersRes] = await Promise.all([
        api.getModels().catch(() => ({ models: [] as ModelInfo[] })),
        api.providerList().catch(() => ({ providers: [] as ProviderInfo[], count: 0 })),
      ]);
      return { models: modelsRes.models || [], providers: providersRes.providers || [] };
    },
    { models: [] as ModelInfo[], providers: [] as ProviderInfo[] },
  );
  const { models, providers } = data;
  const [showAdd, setShowAdd] = useState(false);
  const [importing, setImporting] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; msg: string } | null>(null);
  const [form, setForm] = useState({ model_id: "", name: "", type: "chat", client_type: "openai", supports_reasoning: false, dimensions: 0 });

  const existingModelIds = new Set(models.map(m => m.model_id));
  const importableProviders = providers.filter(p => p.enabled && p.model && !existingModelIds.has(p.model));

  const importFromProviders = async () => {
    setImporting(true);
    setResult(null);
    let imported = 0;
    const errors: string[] = [];
    for (const p of importableProviders) {
      try {
        await api.addModel({
          id: p.model.replace(/[^a-zA-Z0-9_-]/g, "-"),
          model_id: p.model,
          name: p.display_name ? `${p.display_name} - ${p.model}` : p.model,
          type: "chat",
          client_type: p.type || "openai",
          base_url: p.base_url,
          supports_reasoning: false,
        });
        imported++;
      } catch (e: unknown) {
        errors.push(`${p.model}: ${formatErrorMessage(e, "导入失败")}`);
      }
    }
    setImporting(false);
    if (imported > 0) {
      setResult({ ok: true, msg: `成功导入 ${imported} 个模型` });
    } else if (errors.length > 0) {
      setResult({ ok: false, msg: errors.join("; ") });
    } else {
      setResult({ ok: false, msg: "无可导入的模型" });
    }
    setTimeout(() => setResult(null), 6000);
    refresh();
  };

  const addModel = async () => {
    if (!form.model_id) return;
    setResult(null);
    try {
      const id = form.model_id.replace(/[^a-zA-Z0-9_-]/g, "-");
      await api.addModel({ ...form, id, dimensions: form.type === "embedding" ? form.dimensions : undefined });
      setForm({ model_id: "", name: "", type: "chat", client_type: "openai", supports_reasoning: false, dimensions: 0 });
      setShowAdd(false);
      setResult({ ok: true, msg: "模型已添加" });
      refresh();
    } catch (e: unknown) {
      setResult({ ok: false, msg: formatErrorMessage(e, "添加失败") });
    }
    setTimeout(() => setResult(null), 4000);
  };

  const removeModel = async (id: string) => {
    try { await api.deleteModel(id); refresh(); } catch (e) { showErrorToast(e, "删除失败"); }
  };

  const chatModels = models.filter((m) => m.type === "chat");
  const embeddingModels = models.filter((m) => m.type === "embedding");

  if (loading) return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader
        icon={<Cpu size={20} />}
        title="模型管理"
        description="管理对话模型与嵌入模型的注册"
        onRefresh={refresh}
        actions={
          <div style={{ display: "flex", gap: "var(--sp-2)" }}>
            {importableProviders.length > 0 && (
              <Button size="sm" variant="outline" isPending={importing} onPress={importFromProviders}>
                <Download size={13} /> 从提供商导入 ({importableProviders.length})
              </Button>
            )}
            <Button size="sm" onPress={() => setShowAdd(!showAdd)} className="btn-accent">
              <Plus size={14} /> 手动添加
            </Button>
          </div>
        }
      />

      {result && (
        <div style={{
          padding: "var(--sp-2) var(--sp-3)", borderRadius: "var(--radius-md)",
          fontSize: "var(--text-sm)", fontWeight: 500,
          background: result.ok ? "var(--yunque-success-muted)" : "var(--yunque-danger-muted)",
          color: result.ok ? "var(--yunque-success)" : "var(--yunque-danger)",
        }}>
          {result.msg}
        </div>
      )}

      <div className="kpi-grid stagger-children">
        <Card className="section-card p-4 hover-lift">
          <div className="kpi-label stat-card-header"><Cpu size={13} style={{ color: "var(--yunque-accent)" }} />模型总数</div>
          <div className="kpi-value">{models.length}</div>
        </Card>
        <Card className="section-card p-4 hover-lift">
          <div className="kpi-label stat-card-header"><Cpu size={13} style={{ color: "#3b82f6" }} />对话模型</div>
          <div className="kpi-value">{chatModels.length}</div>
        </Card>
        <Card className="section-card p-4 hover-lift">
          <div className="kpi-label stat-card-header"><Sparkles size={13} style={{ color: "var(--yunque-warning)" }} />嵌入模型</div>
          <div className="kpi-value">{embeddingModels.length}</div>
        </Card>
        <Card className="section-card p-4 hover-lift">
          <div className="kpi-label stat-card-header"><Wifi size={13} style={{ color: "#22c55e" }} />活跃提供商</div>
          <div className="kpi-value">{providers.filter(p => p.enabled).length}</div>
        </Card>
      </div>

      {/* Quick add form */}
      {showAdd && (
        <Card className="section-card p-5 space-y-3 animate-scale-in">
          <div style={{ fontSize: "var(--text-sm)", fontWeight: 600, marginBottom: "var(--sp-1)" }}>手动添加模型</div>
          <div className="grid grid-cols-2 gap-3">
            <TextField value={form.model_id} onChange={(v) => setForm({ ...form, model_id: v })}>
              <Label>模型 ID</Label><Input placeholder="e.g. gpt-4o, deepseek-chat" />
            </TextField>
            <TextField value={form.name} onChange={(v) => setForm({ ...form, name: v })}>
              <Label>显示名称（可选）</Label><Input placeholder="自动使用模型 ID" />
            </TextField>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Select selectedKey={form.client_type} onSelectionChange={(key) => setForm({ ...form, client_type: key as string })} className="w-full">
                <Label>提供商类型</Label>
                <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                <Select.Popover><ListBox>
                  <ListBox.Item id="openai" textValue="OpenAI 兼容">OpenAI 兼容<ListBox.ItemIndicator /></ListBox.Item>
                  <ListBox.Item id="anthropic" textValue="Anthropic">Anthropic<ListBox.ItemIndicator /></ListBox.Item>
                  <ListBox.Item id="google" textValue="Google">Google<ListBox.ItemIndicator /></ListBox.Item>
                  <ListBox.Item id="ollama" textValue="Ollama 本地">Ollama 本地<ListBox.ItemIndicator /></ListBox.Item>
                </ListBox></Select.Popover>
              </Select>
            </div>
            <div>
              <Select selectedKey={form.type} onSelectionChange={(key) => setForm({ ...form, type: key as string })} className="w-full">
                <Label>模型类型</Label>
                <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                <Select.Popover><ListBox>
                  <ListBox.Item id="chat" textValue="对话">对话<ListBox.ItemIndicator /></ListBox.Item>
                  <ListBox.Item id="embedding" textValue="嵌入">嵌入<ListBox.ItemIndicator /></ListBox.Item>
                </ListBox></Select.Popover>
              </Select>
            </div>
          </div>
          {form.type === "embedding" && (
            <TextField value={String(form.dimensions || "")} onChange={(v) => setForm({ ...form, dimensions: parseInt(v) || 0 })}>
              <Label>向量维度</Label><Input placeholder="e.g. 1536" />
            </TextField>
          )}
          <div className="flex items-center justify-between">
            <Switch isSelected={form.supports_reasoning} onChange={(v: boolean) => setForm({ ...form, supports_reasoning: v })}>
              <Switch.Control><Switch.Thumb /></Switch.Control>
              <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>支持推理</span>
            </Switch>
            <div className="flex gap-2">
              <Button variant="ghost" size="sm" onPress={() => setShowAdd(false)}>取消</Button>
              <Button size="sm" onPress={addModel} className="btn-accent">添加</Button>
            </div>
          </div>
        </Card>
      )}

      {/* Provider-sourced models hint */}
      {models.length === 0 && importableProviders.length > 0 && (
        <Card className="section-card p-6 text-center animate-scale-in">
          <Download size={32} className="mx-auto mb-3" style={{ color: "var(--yunque-accent)" }} />
          <div style={{ fontSize: "var(--text-md)", fontWeight: 600, marginBottom: "var(--sp-1)" }}>发现 {importableProviders.length} 个可导入的提供商模型</div>
          <p style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)", marginBottom: "var(--sp-3)" }}>
            {importableProviders.map(p => p.model).join("、")}
          </p>
          <Button size="sm" isPending={importing} onPress={importFromProviders} className="btn-accent">
            <Download size={13} /> 一键导入全部
          </Button>
        </Card>
      )}

      {/* Model list */}
      <Card className="section-card p-5">
        {models.length === 0 && importableProviders.length === 0 ? (
          <div className="text-sm text-center py-12" style={{ color: "var(--yunque-text-muted)" }}>
            暂无模型，请先在「提供商」页面配置 LLM 后再导入
          </div>
        ) : models.length > 0 ? (
          <div className="space-y-2 stagger-children">
            {models.map((m) => (
              <div key={m.id} className="flex items-center justify-between p-4 rounded-lg transition-colors hover-lift" style={{ background: "rgba(255,255,255,0.02)" }}>
                <div className="flex items-center gap-3 min-w-0">
                  <div className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
                    {m.type === "chat" ? <Cpu size={14} style={{ color: "var(--yunque-accent)" }} /> : <Sparkles size={14} style={{ color: "var(--yunque-warning)" }} />}
                  </div>
                  <div className="min-w-0">
                    <div className="text-sm font-medium flex items-center gap-2" style={{ color: "var(--yunque-text)" }}>
                      <span className="truncate">{m.name || m.model_id}</span>
                      {m.supports_reasoning && <Chip size="sm" style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)", fontSize: 10 }}>reasoning</Chip>}
                    </div>
                    <div className="text-xs flex items-center gap-2" style={{ color: "var(--yunque-text-muted)" }}>
                      <span className="font-mono">{m.model_id}</span>
                      <span>&middot;</span>
                      <span>{clientLabels[m.client_type] || m.client_type}</span>
                      {m.type === "embedding" && m.dimensions && (<><span>&middot;</span><span>{m.dimensions}d</span></>)}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)", fontSize: 10 }}>{m.type}</Chip>
                  <Tooltip delay={0}>
                    <Button isIconOnly variant="ghost" size="sm" onPress={() => removeModel(m.id)} style={{ color: "var(--yunque-text-muted)" }}>
                      <Trash2 size={14} />
                    </Button>
                    <Tooltip.Content>删除</Tooltip.Content>
                  </Tooltip>
                </div>
              </div>
            ))}
          </div>
        ) : null}
      </Card>
    </div>
  );
}
