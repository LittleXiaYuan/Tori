"use client";

import { useState, useEffect } from "react";
import { Card, Button, Spinner, Chip, TextField, Input, Label, TextArea, ToggleButton, ToggleButtonGroup } from "@heroui/react";
import { api, type DocTemplate } from "@/lib/api";
import { FileDown, FileText, Table2, Code2, Presentation, Download, Sparkles, LayoutGrid, Edit3, Check } from "lucide-react";
import { showToast } from "@/components/toast-provider";
import PageHeader from "@/components/page-header";

const formats = [
  { key: "docx", label: "Word", icon: FileText, color: "#4285f4" },
  { key: "xlsx", label: "Excel", icon: Table2, color: "#34a853" },
  { key: "html", label: "HTML", icon: Code2, color: "#ea4335" },
  { key: "pptx", label: "PPT", icon: Presentation, color: "#fbbc04" },
];

type ViewMode = "templates" | "editor";

export default function DocgenPage() {
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [format, setFormat] = useState("docx");
  const [generating, setGenerating] = useState(false);
  const [result, setResult] = useState<{ path?: string; result?: string } | null>(null);
  const [templates, setTemplates] = useState<DocTemplate[]>([]);
  const [loadingTpl, setLoadingTpl] = useState(true);
  const [view, setView] = useState<ViewMode>("templates");
  const [selectedTpl, setSelectedTpl] = useState<DocTemplate | null>(null);
  const [aiPrompt, setAiPrompt] = useState("");
  const [aiGenerating, setAiGenerating] = useState(false);

  useEffect(() => {
    api.docgenTemplates().then((r) => setTemplates(r.templates || [])).catch(() => {}).finally(() => setLoadingTpl(false));
  }, []);

  const generate = async () => {
    if (!content.trim()) return;
    setGenerating(true);
    setResult(null);
    try {
      const res = await api.docgenExport({ title: title || "Untitled", content, format });
      setResult(res);
    } catch (e) { showToast(e instanceof Error ? e.message : "生成失败", "error"); }
    setGenerating(false);
  };

  const selectTemplate = (tpl: DocTemplate) => {
    setSelectedTpl(tpl);
    setTitle(tpl.name);
    setContent(tpl.content);
    setFormat(tpl.format);
    setView("editor");
  };

  const aiAssist = async () => {
    if (!aiPrompt.trim()) return;
    setAiGenerating(true);
    try {
      const prompt = selectedTpl
        ? `请根据以下要求填写文档模板。模板名称：${selectedTpl.name}\n用户要求：${aiPrompt}\n\n模板内容：\n${selectedTpl.content}\n\n请将所有 {{xxx}} 占位符替换为合适的内容，保持 Markdown 格式，直接输出完整文档内容，不要加代码块包裹。`
        : `请根据以下要求生成文档内容：${aiPrompt}\n\n请使用 Markdown 格式，包含适当的标题、段落、表格等。直接输出文档内容，不要加代码块包裹。`;
      const res = await api.chat([{ role: "user", content: prompt }]);
      if (res.reply) {
        setContent(res.reply);
        showToast("AI 已生成内容", "success");
      }
    } catch (e) { showToast(e instanceof Error ? e.message : "AI 生成失败", "error"); }
    setAiGenerating(false);
  };

  const categories = [...new Set(templates.map((t) => t.category))];
  const formatTpls = templates.filter((t) => !format || t.format === format);

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader
        icon={<FileDown size={20} />}
        title="文档生成"
        actions={
          <ToggleButtonGroup selectionMode="single" disallowEmptySelection
            selectedKeys={new Set([view])}
            onSelectionChange={(keys) => { const k = [...keys][0]; if (k) setView(k as ViewMode); }}>
            <ToggleButton id="templates"><LayoutGrid size={13} /> 模板库</ToggleButton>
            <ToggleButton id="editor"><ToggleButtonGroup.Separator /><Edit3 size={13} /> 编辑器</ToggleButton>
          </ToggleButtonGroup>
        }
      />

      {/* Format pills */}
      <ToggleButtonGroup selectionMode="single" disallowEmptySelection
        selectedKeys={new Set([format])}
        onSelectionChange={(keys) => { const k = [...keys][0]; if (k) setFormat(k as string); }}>
        {formats.map((f, i) => {
          const Icon = f.icon;
          return (
            <ToggleButton key={f.key} id={f.key}>
              {i > 0 && <ToggleButtonGroup.Separator />}
              <Icon size={13} /> {f.label}
            </ToggleButton>
          );
        })}
      </ToggleButtonGroup>

      {/* Templates View */}
      {view === "templates" && (
        <div className="space-y-4">
          {loadingTpl ? <div className="flex items-center justify-center py-12"><Spinner size="sm" /></div> : (
            <>
              {categories.map((cat) => {
                const catTpls = formatTpls.filter((t) => t.category === cat);
                if (catTpls.length === 0) return null;
                return (
                  <div key={cat}>
                    <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--yunque-text-muted)" }}>{cat}</div>
                    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-2">
                      {catTpls.map((tpl) => (
                        <div key={tpl.id} className="section-card hover-lift transition-all duration-200 cursor-pointer rounded-lg" onClick={() => selectTemplate(tpl)}>
                          <div className="py-3 px-3 space-y-1.5">
                            <div className="text-2xl">{tpl.icon}</div>
                            <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{tpl.name}</div>
                            <div className="text-[11px] line-clamp-2" style={{ color: "var(--yunque-text-muted)" }}>{tpl.description}</div>
                            <Chip size="sm" style={{ background: (formats.find((f) => f.key === tpl.format)?.color || "#888") + "15", color: formats.find((f) => f.key === tpl.format)?.color || "#888", fontSize: "var(--text-2xs)" }}>
                              {tpl.format.toUpperCase()}
                            </Chip>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                );
              })}
              {formatTpls.length === 0 && (
                <div className="text-center py-12" style={{ color: "var(--yunque-text-muted)" }}>
                  <FileText size={36} className="mx-auto mb-3 opacity-20" />
                  <div className="text-sm">暂无 {format.toUpperCase()} 格式的模板</div>
                </div>
              )}
              <Button variant="outline" className="w-full" onPress={() => { setTitle(""); setContent(""); setView("editor"); setSelectedTpl(null); }}
                style={{ border: "2px dashed var(--yunque-border)", color: "var(--yunque-text-muted)", padding: "var(--sp-4)" }}>
                + 空白文档
              </Button>
            </>
          )}
        </div>
      )}

      {/* Editor View */}
      {view === "editor" && (
        <div className="grid grid-cols-1 lg:grid-cols-5 gap-4">
          <div className="lg:col-span-2 space-y-3">
            {/* AI Assistant */}
            <Card className="section-card p-4 space-y-3">
              <div className="flex items-center gap-2 text-xs font-medium" style={{ color: "var(--yunque-accent)" }}>
                <Sparkles size={13} /> AI 智能填充
              </div>
              <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                {selectedTpl ? `基于「${selectedTpl.name}」模板，描述你的需求` : "描述文档需求，AI 帮你生成内容"}
              </div>
              <TextField value={aiPrompt} onChange={setAiPrompt}>
                <Label className="sr-only">AI 描述</Label>
                <TextArea rows={3}
                  placeholder={selectedTpl ? "例如：昨天下午的产品评审会，参会人有张三李四..." : "例如：写一份关于电商平台的技术方案..."} />
              </TextField>
              <Button size="sm" isPending={aiGenerating} onPress={aiAssist} isDisabled={!aiPrompt.trim()} className="btn-accent w-full">
                <Sparkles size={12} /> {aiGenerating ? "生成中..." : "AI 生成内容"}
              </Button>
            </Card>
            {result && (
              <Card className="section-card p-4 animate-scale-in">
                <div className="flex items-center gap-2 text-xs font-medium" style={{ color: "var(--yunque-success)" }}>
                  <Check size={13} /> 生成成功
                </div>
                <div className="text-[11px] mt-2" style={{ color: "var(--yunque-text-secondary)" }}>{result.result}</div>
                {result.path && <div className="text-[11px] mt-1 font-mono" style={{ color: "var(--yunque-text-muted)" }}>路径: {result.path}</div>}
              </Card>
            )}
            {selectedTpl && (
              <Card className="section-card p-4">
                <div className="flex items-center gap-2 mb-2">
                  <span className="text-xl">{selectedTpl.icon}</span>
                  <div>
                    <div className="text-sm font-medium">{selectedTpl.name}</div>
                    <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{selectedTpl.description}</div>
                  </div>
                </div>
                <Button size="sm" variant="ghost" onPress={() => { setSelectedTpl(null); setView("templates"); }}>← 返回模板库</Button>
              </Card>
            )}
          </div>
          <div className="lg:col-span-3">
            <Card className="section-card p-4 space-y-3 h-full">
              <TextField value={title} onChange={setTitle}>
                <Label>文档标题</Label>
                <Input placeholder="输入标题..." />
              </TextField>
              <TextField value={content} onChange={setContent}>
                <Label>内容 (Markdown)</Label>
                <TextArea rows={18} placeholder="输入文档内容或使用 Markdown..." style={{ fontFamily: "monospace", fontSize: "12px" }} />
              </TextField>
              <div className="flex items-center justify-between">
                <div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{content.length} 字符 · {format.toUpperCase()}</div>
                <Button isPending={generating} onPress={generate} isDisabled={!content.trim()} className="btn-accent">
                  <Download size={14} /> 生成文档
                </Button>
              </div>
            </Card>
          </div>
        </div>
      )}
    </div>
  );
}
