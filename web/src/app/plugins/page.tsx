"use client";

import { useEffect, useState, useCallback } from "react";
import { api, type PluginMeta, type PluginFile } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  Cog, Power, Plus, Trash2, Code2, Save, X,
  FileCode, ChevronDown, ChevronRight, Puzzle,
  FileText, PlusCircle, Package, Zap,
} from "lucide-react";

// --- Plugin templates ---
interface PluginTemplate {
  id: string;
  name: string;
  description: string;
  language: string;
  skillName: string;
  skillDesc: string;
  handler: string;
  icon: string;
}

const TEMPLATES: PluginTemplate[] = [
  { id: "custom", name: "自定义插件", description: "空白模板，自由编写", language: "python", skillName: "", skillDesc: "", handler: "handler.py", icon: "⚡" },
  { id: "word", name: "Word 文档处理", description: "使用 python-docx 操作 Word 文档", language: "python", skillName: "word_process", skillDesc: "创建/编辑 Word 文档", handler: "handler.py", icon: "📄" },
  { id: "excel", name: "Excel 表格处理", description: "使用 openpyxl 处理 Excel 文件", language: "python", skillName: "excel_process", skillDesc: "读写 Excel 表格数据", handler: "handler.py", icon: "📊" },
  { id: "api", name: "API 调用", description: "调用外部 REST API 获取数据", language: "python", skillName: "api_call", skillDesc: "调用外部 API 接口", handler: "handler.py", icon: "🌐" },
  { id: "data", name: "数据分析", description: "使用 pandas 进行数据处理与分析", language: "python", skillName: "data_analyze", skillDesc: "分析和处理结构化数据", handler: "handler.py", icon: "📈" },
  { id: "node_tool", name: "Node.js 工具", description: "使用 npm 生态构建工具插件", language: "node", skillName: "node_tool", skillDesc: "Node.js 工具调用", handler: "handler.js", icon: "🟢" },
];

const LANG_LABELS: Record<string, { label: string; color: string }> = {
  python: { label: "Python", color: "#3776AB" },
  node: { label: "Node.js", color: "#339933" },
  shell: { label: "Shell", color: "#89E051" },
};

export default function PluginsPage() {
  const [plugins, setPlugins] = useState<PluginMeta[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<{ plugin: string; files: PluginFile[]; active: number } | null>(null);
  const [saving, setSaving] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState("custom");
  const [expandedBuiltin, setExpandedBuiltin] = useState<string | null>(null);
  const [form, setForm] = useState({
    name: "", description: "", language: "python",
    skillName: "", skillDesc: "", handler: "handler.py",
  });

  const load = useCallback(async () => {
    try {
      const res = await api.getPlugins();
      setPlugins(res.plugins || []);
    } catch { /* offline */ }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { load(); }, [load]);

  const builtinPlugins = plugins.filter((p) => p.source === "builtin");
  const scriptPlugins = plugins.filter((p) => p.source === "script");

  const toggle = async (name: string, enabled: boolean) => {
    await api.togglePlugin(name, !enabled);
    setPlugins((prev) => prev.map((p) => p.name === name ? { ...p, enabled: !enabled } : p));
  };

  const remove = async (name: string) => {
    if (!confirm(`确定删除插件「${name}」？此操作不可撤销。`)) return;
    await api.deletePlugin(name);
    load();
  };

  const selectTemplate = (tpl: PluginTemplate) => {
    setSelectedTemplate(tpl.id);
    if (tpl.id !== "custom") {
      setForm({
        name: "", description: tpl.description,
        language: tpl.language, skillName: tpl.skillName,
        skillDesc: tpl.skillDesc, handler: tpl.handler,
      });
    } else {
      setForm({ name: "", description: "", language: "python", skillName: "", skillDesc: "", handler: "handler.py" });
    }
  };

  const create = async () => {
    if (!form.name) return;
    setCreating(true);
    try {
      const manifest: Record<string, unknown> = {
        name: form.name,
        description: form.description,
        language: form.language,
        template: selectedTemplate,
        system_prompt: form.skillName ? `你可以使用 '${form.skillName}' 工具。` : "",
        skills: form.skillName ? [{
          name: form.skillName,
          description: form.skillDesc || form.skillName,
          handler: form.handler,
          parameters: { input: { type: "string", description: "输入数据" } },
        }] : [],
      };
      await api.createPlugin(manifest);
      setForm({ name: "", description: "", language: "python", skillName: "", skillDesc: "", handler: "handler.py" });
      setShowCreate(false);
      setSelectedTemplate("custom");
      load();
    } catch (e) {
      alert(`创建失败: ${e}`);
    } finally {
      setCreating(false);
    }
  };

  const openEditor = async (name: string) => {
    try {
      const res = await api.getPluginFiles(name);
      setEditing({ plugin: name, files: res.files || [], active: 0 });
    } catch {
      setEditing({ plugin: name, files: [], active: 0 });
    }
  };

  const saveFile = async () => {
    if (!editing) return;
    setSaving(true);
    const file = editing.files[editing.active];
    try {
      await api.savePluginFile(editing.plugin, file.name, file.content);
    } finally {
      setSaving(false);
    }
  };

  const addNewFile = () => {
    if (!editing) return;
    const name = prompt("文件名 (如 utils.py):");
    if (!name) return;
    setEditing({
      ...editing,
      files: [...editing.files, { name, content: "", size: 0 }],
      active: editing.files.length,
    });
  };

  const updateFileContent = (content: string) => {
    if (!editing) return;
    setEditing({
      ...editing,
      files: editing.files.map((f, i) => i === editing.active ? { ...f, content } : f),
    });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin" style={{ borderColor: "var(--text-muted)", borderTopColor: "transparent" }} />
      </div>
    );
  }

  // ===== Editor view =====
  if (editing) {
    const file = editing.files[editing.active];
    return (
      <div className="max-w-5xl">
        <BlurFade delay={0}>
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <button onClick={() => setEditing(null)} className="p-2 rounded-lg cursor-pointer" style={{ color: "var(--text-muted)" }}>
                <X size={16} />
              </button>
              <Code2 size={18} />
              <h1 className="text-lg font-semibold tracking-tight">{editing.plugin}</h1>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={addNewFile}
                className="flex items-center gap-1.5 px-3 py-2 rounded-full text-xs font-medium cursor-pointer transition-all"
                style={{ border: "1px solid var(--border)", color: "var(--text-muted)" }}
              >
                <PlusCircle size={12} />
                新建文件
              </button>
              <button
                onClick={saveFile}
                disabled={saving || !file}
                className="flex items-center gap-2 px-4 py-2 rounded-full text-xs font-medium cursor-pointer transition-all"
                style={{ background: "var(--text)", color: "var(--bg)" }}
              >
                <Save size={12} />
                {saving ? "保存中..." : "保存"}
              </button>
            </div>
          </div>
        </BlurFade>

        <div className="flex gap-3">
          {/* File tabs */}
          <div className="w-48 shrink-0 rounded-xl border p-2 space-y-1" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            {editing.files.map((f, i) => (
              <button
                key={f.name}
                onClick={() => setEditing({ ...editing, active: i })}
                className="w-full flex items-center gap-2 px-3 py-2 rounded-lg text-xs text-left cursor-pointer transition-colors"
                style={{
                  background: i === editing.active ? "var(--bg-hover)" : "transparent",
                  color: i === editing.active ? "var(--text)" : "var(--text-muted)",
                }}
              >
                <FileCode size={12} />
                <span className="truncate font-mono">{f.name}</span>
              </button>
            ))}
            {editing.files.length === 0 && (
              <div className="text-xs px-3 py-4 text-center" style={{ color: "var(--text-muted)" }}>
                暂无文件
              </div>
            )}
          </div>

          {/* Code editor */}
          <div className="flex-1 rounded-xl border overflow-hidden" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            {file ? (
              <textarea
                value={file.content}
                onChange={(e) => updateFileContent(e.target.value)}
                className="w-full h-[70vh] bg-transparent p-4 text-sm font-mono resize-none focus:outline-none"
                style={{ tabSize: 2 }}
                spellCheck={false}
              />
            ) : (
              <div className="flex flex-col items-center justify-center h-64 gap-2" style={{ color: "var(--text-muted)" }}>
                <FileText size={32} style={{ opacity: 0.3 }} />
                <div className="text-sm">点击「新建文件」开始编写</div>
              </div>
            )}
          </div>
        </div>
      </div>
    );
  }

  // ===== Plugin list view =====
  return (
    <div className="max-w-4xl">
      <BlurFade delay={0}>
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-3">
            <Puzzle size={20} />
            <h1 className="text-xl font-semibold tracking-tight">插件</h1>
          </div>
          <button
            onClick={() => setShowCreate(!showCreate)}
            className="flex items-center gap-2 px-4 py-2 rounded-full text-xs font-medium transition-all cursor-pointer"
            style={{ background: "var(--text)", color: "var(--bg)" }}
          >
            <Plus size={12} />
            新建插件
          </button>
        </div>
      </BlurFade>

      {/* Create form */}
      {showCreate && (
        <BlurFade delay={0}>
          <div className="rounded-xl border p-5 mb-6 space-y-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>选择模板</div>

            {/* Template grid */}
            <div className="grid grid-cols-3 gap-2">
              {TEMPLATES.map((tpl) => (
                <button
                  key={tpl.id}
                  onClick={() => selectTemplate(tpl)}
                  className="flex items-center gap-2.5 px-3 py-2.5 rounded-lg text-left cursor-pointer transition-all"
                  style={{
                    border: `1.5px solid ${selectedTemplate === tpl.id ? "var(--text)" : "var(--border)"}`,
                    background: selectedTemplate === tpl.id ? "var(--bg-hover)" : "transparent",
                  }}
                >
                  <span className="text-lg">{tpl.icon}</span>
                  <div className="min-w-0">
                    <div className="text-xs font-medium truncate">{tpl.name}</div>
                    <div className="text-[10px] truncate" style={{ color: "var(--text-muted)" }}>{tpl.description}</div>
                  </div>
                </button>
              ))}
            </div>

            {/* Form fields */}
            <div className="grid grid-cols-2 gap-3">
              <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="插件名称 *" className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
              <input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })}
                placeholder="描述" className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div>
                <div className="text-[10px] mb-1" style={{ color: "var(--text-muted)" }}>语言</div>
                <select value={form.language} onChange={(e) => {
                  const lang = e.target.value;
                  const handler = lang === "node" ? "handler.js" : lang === "shell" ? "handler.sh" : "handler.py";
                  setForm({ ...form, language: lang, handler });
                }}
                  className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none cursor-pointer" style={{ borderColor: "var(--border)" }}>
                  <option value="python">Python</option>
                  <option value="node">Node.js</option>
                  <option value="shell">Shell</option>
                </select>
              </div>
              <div>
                <div className="text-[10px] mb-1" style={{ color: "var(--text-muted)" }}>Skill 名称 (可选)</div>
                <input value={form.skillName} onChange={(e) => setForm({ ...form, skillName: e.target.value })}
                  placeholder="如 word_process" className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none font-mono" style={{ borderColor: "var(--border)" }} />
              </div>
              <div>
                <div className="text-[10px] mb-1" style={{ color: "var(--text-muted)" }}>Skill 描述</div>
                <input value={form.skillDesc} onChange={(e) => setForm({ ...form, skillDesc: e.target.value })}
                  placeholder="这个 Skill 做什么" className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
              </div>
            </div>

            <div className="flex justify-end gap-2 pt-1">
              <button onClick={() => setShowCreate(false)} className="px-3 py-1.5 text-xs rounded-full cursor-pointer" style={{ color: "var(--text-muted)" }}>取消</button>
              <button onClick={create} disabled={!form.name || creating}
                className="px-4 py-1.5 text-xs rounded-full font-medium cursor-pointer transition-opacity"
                style={{ background: "var(--text)", color: "var(--bg)", opacity: form.name ? 1 : 0.5 }}>
                {creating ? "创建中..." : "创建插件"}
              </button>
            </div>
          </div>
        </BlurFade>
      )}

      {/* Built-in modules */}
      {builtinPlugins.length > 0 && (
        <BlurFade delay={0.03}>
          <div className="mb-6">
            <div className="flex items-center gap-2 mb-3">
              <Package size={14} style={{ color: "var(--text-muted)" }} />
              <span className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>内置模块</span>
            </div>
            <div className="rounded-xl border overflow-hidden" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              {builtinPlugins.map((p, idx) => (
                <div key={p.name} style={idx > 0 ? { borderTop: "1px solid var(--border)" } : {}}>
                  <div className="flex items-center justify-between p-4">
                    <div className="flex items-center gap-3 min-w-0 flex-1 cursor-pointer" onClick={() => setExpandedBuiltin(expandedBuiltin === p.name ? null : p.name)}>
                      {expandedBuiltin === p.name ? <ChevronDown size={14} style={{ color: "var(--text-muted)" }} /> : <ChevronRight size={14} style={{ color: "var(--text-muted)" }} />}
                      <div className="min-w-0">
                        <div className="text-sm font-medium flex items-center gap-2">
                          {p.name}
                          <span className="text-[10px] px-1.5 py-0.5 rounded-full" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>内置</span>
                        </div>
                        <div className="text-xs" style={{ color: "var(--text-muted)" }}>
                          {p.description || "无描述"} · {p.skill_count} 个技能
                        </div>
                      </div>
                    </div>
                    <button
                      onClick={() => toggle(p.name, p.enabled)}
                      className="p-2 rounded-lg transition-colors cursor-pointer"
                      style={{ color: p.enabled ? "var(--text)" : "var(--text-muted)" }}
                      title={p.enabled ? "禁用" : "启用"}
                    >
                      <Power size={14} />
                    </button>
                  </div>
                  {expandedBuiltin === p.name && (
                    <div className="px-4 pb-4 pt-0">
                      <div className="text-xs p-3 rounded-lg" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
                        内置 Go 模块，已编译进二进制。包含 {p.skill_count} 个技能，无法编辑源码。
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        </BlurFade>
      )}

      {/* User plugins */}
      <BlurFade delay={0.06}>
        <div>
          <div className="flex items-center gap-2 mb-3">
            <Zap size={14} style={{ color: "var(--text-muted)" }} />
            <span className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>用户插件</span>
          </div>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            {scriptPlugins.length === 0 ? (
              <div className="text-center py-10">
                <Puzzle size={36} className="mx-auto mb-3" style={{ color: "var(--text-muted)", opacity: 0.3 }} />
                <div className="text-sm mb-1" style={{ color: "var(--text-muted)" }}>还没有用户插件</div>
                <div className="text-xs mb-4" style={{ color: "var(--text-muted)", opacity: 0.6 }}>
                  插件让你用 Python / Node.js / Shell 编写代码扩展 Agent 能力
                </div>
                <button
                  onClick={() => setShowCreate(true)}
                  className="inline-flex items-center gap-1.5 px-4 py-2 rounded-full text-xs font-medium cursor-pointer transition-all"
                  style={{ background: "var(--text)", color: "var(--bg)" }}
                >
                  <Plus size={12} />
                  创建第一个插件
                </button>
              </div>
            ) : (
              <div className="space-y-2">
                {scriptPlugins.map((p) => {
                  const langInfo = LANG_LABELS[p.language || "python"];
                  return (
                    <div key={p.name} className="flex items-center justify-between p-3.5 rounded-lg transition-colors" style={{ background: "var(--bg-hover)" }}>
                      <div className="flex items-center gap-3 min-w-0 flex-1">
                        <div className="w-2 h-2 rounded-full shrink-0" style={{ background: p.enabled ? langInfo?.color || "var(--text)" : "var(--text-muted)" }} />
                        <div className="min-w-0">
                          <div className="text-sm font-medium flex items-center gap-2">
                            {p.name}
                            <span className="text-[10px] px-1.5 py-0.5 rounded font-mono" style={{ background: langInfo?.color + "20", color: langInfo?.color }}>
                              {langInfo?.label || p.language}
                            </span>
                          </div>
                          <div className="text-xs" style={{ color: "var(--text-muted)" }}>
                            {p.description || "无描述"} · {p.skill_count} 个技能
                          </div>
                        </div>
                      </div>
                      <div className="flex items-center gap-1 shrink-0">
                        <button
                          onClick={() => openEditor(p.name)}
                          className="p-2 rounded-lg transition-colors cursor-pointer"
                          style={{ color: "var(--text-muted)" }}
                          title="编辑代码"
                        >
                          <Code2 size={14} />
                        </button>
                        <button
                          onClick={() => toggle(p.name, p.enabled)}
                          className="p-2 rounded-lg transition-colors cursor-pointer"
                          style={{ color: p.enabled ? "var(--text)" : "var(--text-muted)" }}
                          title={p.enabled ? "禁用" : "启用"}
                        >
                          <Power size={14} />
                        </button>
                        <button
                          onClick={() => remove(p.name)}
                          className="p-2 rounded-lg transition-colors cursor-pointer"
                          style={{ color: "var(--text-muted)" }}
                          title="删除"
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </BlurFade>
    </div>
  );
}
