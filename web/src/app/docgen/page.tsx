"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { useI18n } from "@/lib/i18n";
import {
  FileText,
  Table,
  Globe,
  Presentation,
  Download,
  CheckCircle2,
  AlertCircle,
} from "lucide-react";

const formats = [
  { id: "docx", icon: FileText, color: "#3b82f6", labelKey: "docgen.formatDocx", ext: ".docx" },
  { id: "xlsx", icon: Table, color: "#22c55e", labelKey: "docgen.formatXlsx", ext: ".xlsx" },
  { id: "html", icon: Globe, color: "#f59e0b", labelKey: "docgen.formatHtml", ext: ".html" },
  { id: "pptx", icon: Presentation, color: "#a78bfa", labelKey: "docgen.formatPptx", ext: ".pptx" },
];

const hintKeys: Record<string, string> = {
  docx: "docgen.contentHintDocx",
  xlsx: "docgen.contentHintXlsx",
  html: "docgen.contentHintHtml",
  pptx: "docgen.contentHintPptx",
};

const sampleContent: Record<string, string> = {
  docx: `# 项目报告

## 概述
这是一个示例文档，展示 Word 文档生成能力。

## 功能特点
- 多步规划引擎
- 插件/技能架构
- 五层记忆系统

## 总结
云雀 Agent 是一个可成长的 Agent Runtime。`,
  xlsx: `姓名,部门,职位,入职日期
张三,技术部,工程师,2024-01-15
李四,产品部,产品经理,2024-03-20
王五,设计部,UI设计师,2024-06-01`,
  html: `# 技术文档

## 简介
这是一份 **HTML 格式** 的技术文档。

## 功能列表
- 网络搜索
- 代码执行
- 文档生成
- *AI 图片生成*

\`\`\`
func main() {
    fmt.Println("Hello, Tori!")
}
\`\`\`

## 结语
感谢阅读。`,
  pptx: `# 云雀 Agent 介绍
可成长的 Agent Runtime
---
# 核心特性
多步规划引擎
插件/技能架构
五层记忆系统
混合 RAG 检索
---
# 架构概览
Gateway → Planner → Skills
Memory ← Pipeline ← LLM
---
# 感谢
欢迎使用云雀 Agent`,
};

export default function DocGenPage() {
  const { t } = useI18n();
  const [format, setFormat] = useState("docx");
  const [title, setTitle] = useState("");
  const [content, setContent] = useState(sampleContent.docx);
  const [path, setPath] = useState("");
  const [sheetName, setSheetName] = useState("");
  const [generating, setGenerating] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null);

  const handleFormatChange = (f: string) => {
    setFormat(f);
    setContent(sampleContent[f] || "");
    setResult(null);
  };

  const handleGenerate = async () => {
    if (!content.trim()) return;
    setGenerating(true);
    setResult(null);
    try {
      const res = await api.generateDocument({
        format,
        content,
        title: title || undefined,
        path: path || undefined,
        sheet_name: sheetName || undefined,
      });
      setResult({ ok: true, message: `${t("docgen.success")}: ${res.path}` });
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      setResult({ ok: false, message: msg });
    } finally {
      setGenerating(false);
    }
  };

  const inputStyle = {
    background: "var(--bg-main)",
    borderColor: "var(--border)",
    color: "var(--text-main)",
  };

  const selectedFormat = formats.find((f) => f.id === format)!;

  return (
    <div className="space-y-6 max-w-4xl mx-auto">
      {/* Header */}
      <BlurFade delay={0.05}>
        <div className="flex items-center gap-3 mb-2">
          <Download size={24} style={{ color: "var(--accent)" }} />
          <h1 className="text-2xl font-bold">{t("docgen.title")}</h1>
        </div>
      </BlurFade>

      {/* Format Selector */}
      <BlurFade delay={0.1}>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {formats.map((f) => {
            const Icon = f.icon;
            const active = format === f.id;
            return (
              <button
                key={f.id}
                onClick={() => handleFormatChange(f.id)}
                className="rounded-xl p-4 border text-center transition-all"
                style={{
                  background: active ? `${f.color}15` : "var(--bg-card)",
                  borderColor: active ? f.color : "var(--border)",
                  transform: active ? "scale(1.02)" : "scale(1)",
                }}
              >
                <Icon
                  size={28}
                  className="mx-auto mb-2"
                  style={{ color: active ? f.color : "var(--text-muted)" }}
                />
                <div
                  className="text-sm font-medium"
                  style={{ color: active ? f.color : "var(--text-main)" }}
                >
                  {t(f.labelKey)}
                </div>
                <div className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>
                  {f.ext}
                </div>
              </button>
            );
          })}
        </div>
      </BlurFade>

      {/* Form */}
      <BlurFade delay={0.15}>
        <div
          className="rounded-xl p-5 border space-y-4"
          style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
        >
          {/* Title + Path row */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
                {t("docgen.docTitle")}
              </label>
              <input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder={t("docgen.docTitle")}
                className="w-full rounded-lg px-3 py-2 text-sm border"
                style={inputStyle}
              />
            </div>
            <div>
              <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
                {t("docgen.path")}
              </label>
              <input
                value={path}
                onChange={(e) => setPath(e.target.value)}
                placeholder={`data/output/document${selectedFormat.ext}`}
                className="w-full rounded-lg px-3 py-2 text-sm border"
                style={inputStyle}
              />
            </div>
          </div>

          {/* Sheet name for xlsx */}
          {format === "xlsx" && (
            <div>
              <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
                Sheet Name
              </label>
              <input
                value={sheetName}
                onChange={(e) => setSheetName(e.target.value)}
                placeholder="Sheet1"
                className="w-full rounded-lg px-3 py-2 text-sm border"
                style={inputStyle}
              />
            </div>
          )}

          {/* Content */}
          <div>
            <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
              {t("docgen.content")}
            </label>
            <div
              className="text-xs mb-2 px-3 py-1.5 rounded-lg"
              style={{ background: `${selectedFormat.color}10`, color: selectedFormat.color }}
            >
              {t(hintKeys[format])}
            </div>
            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              rows={12}
              className="w-full rounded-lg px-3 py-2 text-sm border font-mono"
              style={{
                ...inputStyle,
                resize: "vertical",
                minHeight: "200px",
              }}
            />
          </div>

          {/* Generate button + result */}
          <div className="flex items-center gap-3">
            <button
              onClick={handleGenerate}
              disabled={generating || !content.trim()}
              className="px-5 py-2.5 rounded-lg text-sm font-medium transition flex items-center gap-2"
              style={{
                background: selectedFormat.color,
                color: "#fff",
                opacity: generating || !content.trim() ? 0.5 : 1,
              }}
            >
              {generating ? (
                <span>{t("docgen.generating")}</span>
              ) : (
                <>
                  <Download size={14} />
                  {t("docgen.generate")}
                </>
              )}
            </button>

            {result && (
              <div className="flex items-center gap-2 text-sm">
                {result.ok ? (
                  <CheckCircle2 size={16} style={{ color: "#22c55e" }} />
                ) : (
                  <AlertCircle size={16} style={{ color: "#ef4444" }} />
                )}
                <span style={{ color: result.ok ? "#22c55e" : "#ef4444" }}>
                  {result.message}
                </span>
              </div>
            )}
          </div>
        </div>
      </BlurFade>
    </div>
  );
}
