"use client";

import { useState, useMemo } from "react";
import { Card, Button, Spinner, Chip, Tooltip, TextField, Input, Label, TextArea, Switch } from "@heroui/react";
import { api, type PluginMeta } from "@/lib/api";
import { Puzzle, Plus, Trash2, Save, Code2, FileCode, Search, Filter, Zap, Box, Terminal, ChevronDown, ChevronUp } from "lucide-react";
import { showToast } from "@/components/toast-provider";
import PageHeader from "@/components/page-header";
import { useApiData } from "@/lib/use-api-data";

const TEMPLATES = [
  { name: "Python Script", lang: "python", icon: "🐍", code: '#!/usr/bin/env python3\n"""Plugin description"""\n\ndef run(args):\n    return {"result": "ok"}\n' },
  { name: "Node.js Script", lang: "nodejs", icon: "⬢", code: '// Plugin description\nmodule.exports = {\n  run(args) {\n    return { result: "ok" };\n  }\n};\n' },
  { name: "Shell Script", lang: "shell", icon: "🖥", code: '#!/bin/bash\n# Plugin description\necho "ok"\n' },
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
  const [form, setForm] = useState({ name: "", description: "", language: "python", code: TEMPLATES[0].code, enabled: true });
  const [searchQuery, setSearchQuery] = useState("");
  const [filterSource, setFilterSource] = useState<"all" | "builtin" | "script">("all");

  const togglePlugin = async (name: string, enabled: boolean) => {
    await api.togglePlugin(name, enabled);
    refresh();
  };

  const deletePlugin = async (name: string) => {
    await api.deletePlugin(name);
    showToast("已删除插件", "success");
    refresh();
  };

  const applyTemplate = (t: typeof TEMPLATES[number]) => {
    setForm({ ...form, language: t.lang, code: t.code });
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
          <Button size="sm" onPress={() => setShowCreate(!showCreate)} className="btn-accent">
            {showCreate ? <ChevronUp size={14} /> : <Plus size={14} />} {showCreate ? "收起" : "创建插件"}
          </Button>
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
              {f === "all" ? "全部" : f === "builtin" ? "内置" : "脚本"}
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
          <div>
            <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text-muted)" }}>选择模板</div>
            <div className="flex gap-2 flex-wrap">
              {TEMPLATES.map((t) => (
                <button
                  key={t.name}
                  onClick={() => applyTemplate(t)}
                  className="filter-pill filter-pill-subtle"
                  data-active={form.language === t.lang}
                  style={form.language === t.lang ? { border: "1px solid var(--yunque-accent)" } : { border: "1px solid var(--yunque-border)" }}
                >
                  <span>{t.icon}</span> {t.name}
                </button>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <TextField value={form.name} onChange={(v) => setForm({ ...form, name: v })}>
              <Label>插件名称</Label>
              <Input placeholder="my-plugin" />
            </TextField>
            <TextField value={form.description} onChange={(v) => setForm({ ...form, description: v })}>
              <Label>描述</Label>
              <Input placeholder="功能描述" />
            </TextField>
          </div>
          <div>
            <TextField value={form.code} onChange={(v) => setForm({ ...form, code: v })}>
              <Label>代码</Label>
              <TextArea
                className="font-mono"
                style={{ background: "rgba(0,0,0,0.3)", minHeight: 200 }}
              />
            </TextField>
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" size="sm" onPress={() => setShowCreate(false)}>取消</Button>
            <Button size="sm" onPress={async () => {
              if (!form.name) return;
              try {
                await api.createPlugin({ name: form.name, description: form.description, language: form.language, main_file: "main." + (form.language === "nodejs" ? "js" : form.language === "shell" ? "sh" : "py") });
                const pluginName = form.name;
                const fileName = "main." + (form.language === "nodejs" ? "js" : form.language === "shell" ? "sh" : "py");
                await api.savePluginFile(pluginName, fileName, form.code);
                setForm({ name: "", description: "", language: "python", code: TEMPLATES[0].code, enabled: true });
                setShowCreate(false);
                showToast("插件创建成功", "success");
                refresh();
              } catch (e) { showToast(e instanceof Error ? e.message : "创建失败", "error"); }
            }} className="btn-accent">
              <Save size={14} /> 保存
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
                          {p.source === "builtin" ? "内置" : "脚本"}
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
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
