"use client";

import { useEffect, useState } from "react";
import { api, type ModelInfo } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { AnimatedList } from "@/components/ui/animated-list";
import { NumberTicker } from "@/components/ui/number-ticker";
import { Cpu, Plus, Trash2, Sparkles, Eye, Type } from "lucide-react";

const clientLabels: Record<string, string> = {
  openai: "OpenAI",
  anthropic: "Anthropic",
  google: "Google",
  ollama: "Ollama",
};

const modalityIcons: Record<string, React.ElementType> = {
  text: Type,
  image: Eye,
};

export default function ModelsPage() {
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAdd, setShowAdd] = useState(false);
  const [form, setForm] = useState({
    id: "", model_id: "", name: "", type: "chat" as string,
    client_type: "openai", base_url: "", supports_reasoning: false, dimensions: 0,
  });

  const load = async () => {
    try {
      const res = await api.getModels();
      setModels(res.models || []);
    } catch { /* offline */ }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, []);

  const addModel = async () => {
    if (!form.id || !form.model_id) return;
    await api.addModel({
      ...form,
      dimensions: form.type === "embedding" ? form.dimensions : undefined,
    });
    setForm({ id: "", model_id: "", name: "", type: "chat", client_type: "openai", base_url: "", supports_reasoning: false, dimensions: 0 });
    setShowAdd(false);
    load();
  };

  const removeModel = async (id: string) => {
    await api.deleteModel(id);
    load();
  };

  const chatModels = models.filter((m) => m.type === "chat");
  const embeddingModels = models.filter((m) => m.type === "embedding");

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin" style={{ borderColor: "var(--text-muted)", borderTopColor: "transparent" }} />
      </div>
    );
  }

  return (
    <div className="max-w-4xl">
      <BlurFade delay={0}>
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-3">
            <Cpu size={20} />
            <h1 className="text-xl font-semibold tracking-tight">Models</h1>
          </div>
          <button
            onClick={() => setShowAdd(!showAdd)}
            className="flex items-center gap-2 px-4 py-2 rounded-full text-xs font-medium transition-all cursor-pointer"
            style={{ background: "var(--text)", color: "var(--bg)" }}
          >
            <Plus size={12} />
            Add Model
          </button>
        </div>
      </BlurFade>

      <BlurFade delay={0.05}>
        <div className="grid grid-cols-3 gap-4 mb-6">
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--text-muted)" }}>Total</div>
            <div className="text-3xl font-bold"><NumberTicker value={models.length} /></div>
          </div>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--text-muted)" }}>Chat</div>
            <div className="text-3xl font-bold"><NumberTicker value={chatModels.length} /></div>
          </div>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--text-muted)" }}>Embedding</div>
            <div className="text-3xl font-bold"><NumberTicker value={embeddingModels.length} /></div>
          </div>
        </div>
      </BlurFade>

      {showAdd && (
        <BlurFade delay={0}>
          <div className="rounded-xl border p-5 mb-6 space-y-3" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="grid grid-cols-2 gap-3">
              <input value={form.id} onChange={(e) => setForm({ ...form, id: e.target.value })}
                placeholder="ID (unique)" className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
              <input value={form.model_id} onChange={(e) => setForm({ ...form, model_id: e.target.value })}
                placeholder="Model ID (e.g. gpt-4o)" className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
            </div>
            <div className="grid grid-cols-3 gap-3">
              <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="Display name" className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
              <select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}
                className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none cursor-pointer" style={{ borderColor: "var(--border)" }}>
                <option value="chat">Chat</option>
                <option value="embedding">Embedding</option>
              </select>
              <select value={form.client_type} onChange={(e) => setForm({ ...form, client_type: e.target.value })}
                className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none cursor-pointer" style={{ borderColor: "var(--border)" }}>
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="google">Google</option>
                <option value="ollama">Ollama</option>
              </select>
            </div>
            <input value={form.base_url} onChange={(e) => setForm({ ...form, base_url: e.target.value })}
              placeholder="Base URL (optional)" className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
            {form.type === "embedding" && (
              <input type="number" value={form.dimensions || ""} onChange={(e) => setForm({ ...form, dimensions: parseInt(e.target.value) || 0 })}
                placeholder="Dimensions (e.g. 1536)" className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
            )}
            <div className="flex items-center justify-between">
              <label className="flex items-center gap-2 text-xs cursor-pointer" style={{ color: "var(--text-muted)" }}>
                <input type="checkbox" checked={form.supports_reasoning} onChange={(e) => setForm({ ...form, supports_reasoning: e.target.checked })} className="cursor-pointer" />
                Supports reasoning
              </label>
              <div className="flex gap-2">
                <button onClick={() => setShowAdd(false)} className="px-3 py-1.5 text-xs rounded-full cursor-pointer" style={{ color: "var(--text-muted)" }}>Cancel</button>
                <button onClick={addModel} className="px-4 py-1.5 text-xs rounded-full font-medium cursor-pointer" style={{ background: "var(--text)", color: "var(--bg)" }}>Add</button>
              </div>
            </div>
          </div>
        </BlurFade>
      )}

      <BlurFade delay={0.1}>
        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          {models.length === 0 ? (
            <div className="text-sm text-center py-12" style={{ color: "var(--text-muted)" }}>No models registered</div>
          ) : (
            <AnimatedList>
              {models.map((m) => (
                <div key={m.id} className="flex items-center justify-between p-4 rounded-lg transition-colors" style={{ background: "var(--bg-hover)" }}>
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0" style={{ background: "var(--bg-card)", border: "1px solid var(--border)" }}>
                      {m.type === "chat" ? <Cpu size={14} /> : <Sparkles size={14} />}
                    </div>
                    <div className="min-w-0">
                      <div className="text-sm font-medium flex items-center gap-2">
                        <span className="truncate">{m.name || m.model_id}</span>
                        {m.supports_reasoning && (
                          <span className="text-[10px] px-1.5 py-0.5 rounded-full" style={{ background: "var(--bg-card)", color: "var(--text-muted)", border: "1px solid var(--border)" }}>
                            reasoning
                          </span>
                        )}
                      </div>
                      <div className="text-xs flex items-center gap-2" style={{ color: "var(--text-muted)" }}>
                        <span className="font-mono">{m.model_id}</span>
                        <span>·</span>
                        <span>{clientLabels[m.client_type] || m.client_type}</span>
                        {m.type === "embedding" && m.dimensions && (
                          <><span>·</span><span>{m.dimensions}d</span></>
                        )}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    {m.input_modalities && m.input_modalities.length > 0 && (
                      <div className="flex items-center gap-1">
                        {m.input_modalities.map((mod) => {
                          const ModIcon = modalityIcons[mod] || Type;
                          return <ModIcon key={mod} size={12} style={{ color: "var(--text-muted)" }} />;
                        })}
                      </div>
                    )}
                    <span className="text-[10px] px-2 py-0.5 rounded-full" style={{ background: "var(--bg-card)", color: "var(--text-muted)", border: "1px solid var(--border)" }}>
                      {m.type}
                    </span>
                    <button onClick={() => removeModel(m.id)} className="p-1.5 rounded-lg transition-colors cursor-pointer" style={{ color: "var(--text-muted)" }}>
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>
              ))}
            </AnimatedList>
          )}
        </div>
      </BlurFade>
    </div>
  );
}
