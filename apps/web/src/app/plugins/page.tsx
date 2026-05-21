"use client";

import { useState, useMemo } from "react";
import { Card, Button, Spinner, Chip, Tooltip, TextField, Input, Label, Switch } from "@heroui/react";
import { api, type PluginMeta } from "@/lib/api";
import { Puzzle, Plus, Trash2, Save, FileCode, Search, Zap, Box, Terminal, ChevronUp, RefreshCw, FolderOpen, File, ExternalLink } from "lucide-react";
import { showToast } from "@/components/toast-provider";
import PageHeader from "@/components/page-header";
import { useApiData } from "@/lib/use-api-data";

type TemplateItem = {
  name: string;
  lang: string;
  template: string;
  icon: string;
  desc: string;
  category: "basic" | "scene";
};

const TEMPLATES: TemplateItem[] = [
  { name: "Python 基础", lang: "python", template: "custom", icon: "🐍", desc: "Python 空白模板，适合自由发挥", category: "basic" },
  { name: "Node.js 基础", lang: "node", template: "custom", icon: "⬢", desc: "Node.js 空白模板", category: "basic" },
  { name: "Shell 脚本", lang: "shell", template: "custom", icon: "🖥", desc: "Shell 空白模板", category: "basic" },
  { name: "API 调用", lang: "python", template: "api_call", icon: "🌐", desc: "REST API 请求/响应处理", category: "scene" },
  { name: "数据分析", lang: "python", template: "data_analysis", icon: "📊", desc: "CSV/Excel/JSON 数据分析 (pandas)", category: "scene" },
  { name: "Excel 处理", lang: "python", template: "excel", icon: "📗", desc: "Excel 读写与修改 (openpyxl)", category: "scene" },
  { name: "Word 文档", lang: "python", template: "word_doc", icon: "📝", desc: "Word 文档生成与解析 (python-docx)", category: "scene" },
  { name: "Node 工具", lang: "node", template: "node_tool", icon: "🔧", desc: "HTTP 请求 + 网页解析 (axios)", category: "scene" },
];

const LANG_COLORS: Record<string, string> = {
  python: "#3572A5",
  nodejs: "#f1e05a",
  shell: "#89e051",
};

export default function PluginsPage() {
  const { data: plugins, loading, refresh } = useApiData(
    async () => { const res = await api.getPlugins(); return res.plugins || []; },
    [] as PluginMeta[],
  );
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState({ name: "", description: "", language: "python", template: "custom", enabled: true });
  const [searchQuery, setSearchQuery] = useState("");
  const [filterSource, setFilterSource] = useState<"all" | "builtin" | "script">("all");
  const [expandedPlugin, setExpandedPlugin] = useState<string | null>(null);
  const [pluginFiles, setPluginFiles] = useState<{ name: string; content: string; size: number }[]>([]);
  const [editingFile, setEditingFile] = useState<string | null>(null);
  const [editContent, setEditContent] = useState("");
  const [reloading, setReloading] = useState(false);

  const reloadPlugins = async () => {
    setReloading(true);
    try {
      const res = await api.reloadPlugins();
      showToast(`Plugins reloaded (${res.skills} skills)`, "success");
      refresh();
    } catch (e) { showToast(e instanceof Error ? e.message : "Reload failed", "error"); }
    setReloading(false);
  };

  const loadPluginFiles = async (name: string) => {
    if (expandedPlugin === name) { setExpandedPlugin(null); return; }
    try {
      const res = await api.getPluginFiles(name);
      setPluginFiles(res.files || []);
      setExpandedPlugin(name);
      setEditingFile(null);
    } catch { setPluginFiles([]); setExpandedPlugin(name); }
  };

  const saveFile = async (pluginName: string, fileName: string) => {
    try {
      await api.savePluginFile(pluginName, fileName, editContent);
      showToast("Saved. Click Reload to apply.", "success");
      setEditingFile(null);
      loadPluginFiles(pluginName);
    } catch (e) { showToast(e instanceof Error ? e.message : "Save failed", "error"); }
  };

  const togglePlugin = async (name: string, enabled: boolean) => {
    await api.togglePlugin(name, enabled);
    refresh();
  };

  const deletePlugin = async (name: string) => {
    await api.deletePlugin(name);
    showToast("已删除插件", "success");
    refresh();
  };

  const applyTemplate = (t: TemplateItem) => {
    setForm({ ...form, language: t.lang, template: t.template });
  };

  const filteredPlugins = useMemo(() => {
    let list = plugins;
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      list = list.filter((p) => p.name.toLowerCase().includes(q) || (p.description || "").toLowerCase().includes(q));
    }
    if (filterSource !== "all") {
      list = list.filter((p) => p.source === filterSource);
    }
    return list;
  }, [plugins, searchQuery, filterSource]);

  const stats = useMemo(() => ({
    total: plugins.length,
    enabled: plugins.filter((p) => p.enabled).length,
    builtin: plugins.filter((p) => p.source === "builtin").length,
    script: plugins.filter((p) => p.source === "script").length,
  }), [plugins]);

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader
        icon={<Puzzle size={20} />}
        title="插件管理"
        actions={
          <div className="flex items-center gap-2">
            <Tooltip delay={0}>
              <Button size="sm" variant="ghost" onPress={async () => { try { await api.openPluginFolder(); } catch {} }}>
                <ExternalLink size={14} /> 打开目录
              </Button>
              <Tooltip.Content>在资源管理器中打开插件目录</Tooltip.Content>
            </Tooltip>
            <Tooltip delay={0}>
              <Button size="sm" variant="outline" isDisabled={reloading} onPress={reloadPlugins}>
                <RefreshCw size={14} className={reloading ? "animate-spin" : ""} /> 刷新
              </Button>
              <Tooltip.Content>重新扫描插件目录并加载变更</Tooltip.Content>
            </Tooltip>
            <Button size="sm" onPress={() => setShowCreate(!showCreate)} className="btn-accent">
              {showCreate ? <ChevronUp size={14} /> : <Plus size={14} />} {showCreate ? "收起" : "新建插件"}
            </Button>
          </div>
        }
      />

      {/* Stats bar */}
      <div className="flex items-center gap-3 flex-wrap">
        <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
          <Box size={12} style={{ color: "var(--yunque-accent)" }} /> <span style={{ color: "var(--yunque-text-muted)" }}>共</span> <span className="font-semibold" style={{ color: "var(--yunque-text)" }}>{stats.total}</span>
        </div>
        <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
          <Zap size={12} style={{ color: "#22c55e" }} /> <span style={{ color: "var(--yunque-text-muted)" }}>已启用</span> <span className="font-semibold" style={{ color: "#22c55e" }}>{stats.enabled}</span>
        </div>
        {stats.builtin > 0 && (
          <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
            <Puzzle size={12} style={{ color: "var(--yunque-text-muted)" }} /> <span style={{ color: "var(--yunque-text-muted)" }}>内置</span> <span className="font-semibold" style={{ color: "var(--yunque-text)" }}>{stats.builtin}</span>
          </div>
        )}
        {stats.script > 0 && (
          <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
            <Terminal size={12} style={{ color: "var(--yunque-text-muted)" }} /> <span style={{ color: "var(--yunque-text-muted)" }}>脚本</span> <span className="font-semibold" style={{ color: "var(--yunque-text)" }}>{stats.script}</span>
          </div>
        )}
      </div>

      {/* Search & Filter */}
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-2 px-3 py-2 rounded-lg flex-1" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
          <Search size={14} style={{ color: "var(--yunque-text-muted)" }} />
          <input
            placeholder="搜索插件名称或描述..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="bg-transparent outline-none text-sm flex-1"
            style={{ color: "var(--yunque-text)" }}
          />
        </div>
        <div className="flex items-center gap-1 p-1 rounded-lg" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
          {(["all", "builtin", "script"] as const).map((f) => (
            <button
              key={f}
              onClick={() => setFilterSource(f)}
              className="filter-pill"
              data-active={filterSource === f}
            >
              {f === "all" ? "全部" : f === "builtin" ? "内置" : "本地"}
            </button>
          ))}
        </div>
      </div>

      {/* Create form */}
      {showCreate && (
        <Card className="section-card p-5 space-y-4 animate-scale-in">
          <div className="flex items-center gap-2 mb-1">
            <Plus size={14} style={{ color: "var(--yunque-accent)" }} />
            <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>创建新插件</span>
          </div>

          {/* Template selection */}
          <div className="space-y-3">
            <div>
              <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text-muted)" }}>基础模板</div>
              <div className="flex gap-2 flex-wrap">
                {TEMPLATES.filter(t => t.category === "basic").map((t) => {
                  const active = form.language === t.lang && form.template === t.template;
                  return (
                    <button
                      key={t.name}
                      onClick={() => applyTemplate(t)}
                      className="flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium transition-all"
                      style={{
                        border: active ? "1px solid var(--yunque-accent)" : "1px solid var(--yunque-border)",
                        background: active ? "rgba(0,111,238,0.08)" : "var(--yunque-bg)",
                        color: active ? "var(--yunque-accent)" : "var(--yunque-text)",
                      }}
                    >
                      <span className="text-base">{t.icon}</span>
                      <div className="text-left">
                        <div>{t.name}</div>
                        <div className="text-[10px] opacity-60 font-normal">{t.desc}</div>
                      </div>
                    </button>
                  );
                })}
              </div>
            </div>
            <div>
              <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text-muted)" }}>场景模板 <span className="opacity-50">— 自带完整示例代码</span></div>
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-2">
                {TEMPLATES.filter(t => t.category === "scene").map((t) => {
                  const active = form.language === t.lang && form.template === t.template;
                  return (
                    <button
                      key={t.name}
                      onClick={() => applyTemplate(t)}
                      className="flex items-center gap-2 px-3 py-2.5 rounded-lg text-xs font-medium transition-all text-left"
                      style={{
                        border: active ? "1px solid var(--yunque-accent)" : "1px solid var(--yunque-border)",
                        background: active ? "rgba(0,111,238,0.08)" : "var(--yunque-bg)",
                        color: active ? "var(--yunque-accent)" : "var(--yunque-text)",
                      }}
                    >
                      <span className="text-base">{t.icon}</span>
                      <div>
                        <div>{t.name}</div>
                        <div className="text-[10px] opacity-50 font-normal">{t.desc}</div>
                      </div>
                    </button>
                  );
                })}
              </div>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <TextField value={form.name} onChange={(v: string) => setForm({ ...form, name: v })}>
              <Label>插件名称</Label>
              <Input placeholder="my-plugin" />
            </TextField>
            <TextField value={form.description} onChange={(v: string) => setForm({ ...form, description: v })}>
              <Label>描述</Label>
              <Input placeholder="功能描述" />
            </TextField>
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" size="sm" onPress={() => setShowCreate(false)}>取消</Button>
            <Button size="sm" onPress={async () => {
              if (!form.name) return;
              try {
                await api.createPlugin({
                  name: form.name,
                  description: form.description,
                  language: form.language,
                  template: form.template,
                });
                try { await api.openPluginFolder(form.name); } catch {}
                setForm({ name: "", description: "", language: "python", template: "custom", enabled: true });
                setShowCreate(false);
                showToast("插件已创建，资源管理器已打开", "success");
                refresh();
              } catch (e) { showToast(e instanceof Error ? e.message : "创建失败", "error"); }
            }} className="btn-accent">
              <Plus size={14} /> 创建
            </Button>
          </div>
        </Card>
      )}

      {/* Plugin grid */}
      {filteredPlugins.length === 0 ? (
        <Card className="section-card p-12 text-center">
          <FileCode size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
          <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>
            {searchQuery || filterSource !== "all" ? "没有匹配的插件" : "暂无插件"}
          </div>
          {!searchQuery && filterSource === "all" && (
            <Button size="sm" className="btn-accent mt-3" onPress={() => setShowCreate(true)}>
              <Plus size={14} /> 创建第一个插件
            </Button>
          )}
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3 stagger-children">
          {filteredPlugins.map((p) => (
            <Card key={p.name} className="section-card hover-lift overflow-hidden">
              {/* Top accent bar */}
              <div className="h-[3px]" style={{ background: p.enabled ? (LANG_COLORS[p.language || "python"] || "var(--yunque-accent)") : "var(--yunque-border)" }} />
              <div className="p-4">
                <div className="flex items-start justify-between gap-2">
                  <div className="flex items-start gap-3 min-w-0 flex-1">
                    <div className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0 mt-0.5"
                      style={{ background: p.enabled ? "rgba(0,111,238,0.1)" : "rgba(255,255,255,0.03)" }}>
                      <Puzzle size={18} style={{ color: p.enabled ? "var(--yunque-accent)" : "var(--yunque-text-muted)" }} />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-semibold truncate" style={{ color: "var(--yunque-text)" }}>{p.name}</span>
                        <Chip size="sm" style={{
                          background: `${LANG_COLORS[p.language || "python"] || "#666"}22`,
                          color: LANG_COLORS[p.language || "python"] || "var(--yunque-text-muted)",
                          fontSize: 10,
                          fontWeight: 600,
                        }}>
                          {p.language || "python"}
                        </Chip>
                        <Chip size="sm" style={{
                          background: p.source === "builtin" ? "rgba(168, 85, 247, 0.1)" : "rgba(255,255,255,0.05)",
                          color: p.source === "builtin" ? "#a855f7" : "var(--yunque-text-muted)",
                          fontSize: 10,
                        }}>
                          {p.source === "builtin" ? "内置" : "本地"}
                        </Chip>
                      </div>
                      <div className="text-xs mt-1 line-clamp-2" style={{ color: "var(--yunque-text-muted)" }}>
                        {p.description || "无描述"}
                      </div>
                    </div>
                  </div>
                </div>

                {/* Bottom row: skills + actions */}
                <div className="flex items-center justify-between mt-3 pt-3" style={{ borderTop: "1px solid var(--yunque-border)" }}>
                  <div className="flex items-center gap-2">
                    {p.skill_count > 0 && (
                      <span className="text-[11px] flex items-center gap-1" style={{ color: "var(--yunque-text-muted)" }}>
                        <Zap size={11} /> {p.skill_count} 技能
                      </span>
                    )}
                    <span className="flex items-center gap-1">
                      <span className="w-1.5 h-1.5 rounded-full" style={{ background: p.enabled ? "#22c55e" : "#666" }} />
                      <span className="text-[11px]" style={{ color: p.enabled ? "#22c55e" : "var(--yunque-text-muted)" }}>
                        {p.enabled ? "运行中" : "已禁用"}
                      </span>
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5">
                    {p.source !== "builtin" && (
                      <>
                        <Tooltip delay={0}>
                          <Button isIconOnly variant="ghost" size="sm" onPress={async () => { try { await api.openPluginFolder(p.name); } catch {} }}>
                            <ExternalLink size={13} style={{ color: "var(--yunque-text-muted)" }} />
                          </Button>
                          <Tooltip.Content>打开目录</Tooltip.Content>
                        </Tooltip>
                        <Tooltip delay={0}>
                          <Button isIconOnly variant="ghost" size="sm" onPress={() => loadPluginFiles(p.name)}>
                            <FolderOpen size={13} style={{ color: expandedPlugin === p.name ? "var(--yunque-accent)" : "var(--yunque-text-muted)" }} />
                          </Button>
                          <Tooltip.Content>浏览文件</Tooltip.Content>
                        </Tooltip>
                      </>
                    )}
                    <Switch isSelected={p.enabled} onChange={(v: boolean) => togglePlugin(p.name, v)} size="sm">
                      <Switch.Control><Switch.Thumb /></Switch.Control>
                    </Switch>
                    {p.source !== "builtin" && (
                      <Tooltip delay={0}>
                        <Button isIconOnly variant="ghost" size="sm" onPress={() => deletePlugin(p.name)}
                          style={{ color: "var(--yunque-danger)" }}>
                          <Trash2 size={13} />
                        </Button>
                        <Tooltip.Content>删除</Tooltip.Content>
                      </Tooltip>
                    )}
                  </div>
                </div>

                {/* File browser panel */}
                {expandedPlugin === p.name && p.source !== "builtin" && (
                  <div className="mt-3 pt-3 space-y-2" style={{ borderTop: "1px solid var(--yunque-border)" }}>
                    <div className="flex items-center gap-1.5 mb-2">
                      <FolderOpen size={12} style={{ color: "var(--yunque-accent)" }} />
                      <span className="text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>插件文件</span>
                    </div>
                    {pluginFiles.length === 0 ? (
                      <div className="text-xs py-2 text-center" style={{ color: "var(--yunque-text-muted)" }}>无文件</div>
                    ) : pluginFiles.map((f) => (
                      <div key={f.name}>
                        <div className="flex items-center justify-between gap-2 px-2 py-1.5 rounded-md hover:opacity-80 cursor-pointer"
                          style={{ background: editingFile === f.name ? "rgba(0,111,238,0.08)" : "rgba(255,255,255,0.02)" }}
                          onClick={() => {
                            if (editingFile === f.name) { setEditingFile(null); } else { setEditingFile(f.name); setEditContent(f.content); }
                          }}>
                          <div className="flex items-center gap-2 min-w-0">
                            <File size={12} style={{ color: "var(--yunque-text-muted)" }} />
                            <span className="text-xs font-mono truncate" style={{ color: "var(--yunque-text)" }}>{f.name}</span>
                          </div>
                          <span className="text-[10px] shrink-0" style={{ color: "var(--yunque-text-muted)" }}>{f.size}B</span>
                        </div>
                        {editingFile === f.name && (
                          <div className="mt-1.5 space-y-2">
                            <textarea
                              value={editContent}
                              onChange={(e) => setEditContent(e.target.value)}
                              className="w-full font-mono text-xs p-2.5 rounded-lg resize-y min-h-[120px] outline-none"
                              style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)", border: "1px solid var(--yunque-border)" }}
                            />
                            <div className="flex justify-end gap-2">
                              <Button variant="ghost" size="sm" onPress={() => setEditingFile(null)}>取消</Button>
                              <Button size="sm" className="btn-accent" onPress={() => saveFile(p.name, f.name)}>
                                <Save size={12} /> 保存
                              </Button>
                            </div>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
