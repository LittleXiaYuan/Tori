"use client";

import { useState, useRef, useCallback, useEffect } from "react";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  GraduationCap,
  Upload,
  FileText,
  CheckCircle2,
  Download,
  Loader2,
  AlertCircle,
  ChevronRight,
  BookOpen,
  Search,
  PenLine,
  Sparkles,
  FileDown,
  Trash2,
} from "lucide-react";

/* ─── Types ─── */
interface TemplateSection {
  level: number;
  title: string;
  type: string;
}
interface CoverField {
  label: string;
  key: string;
}
interface TemplateResult {
  path: string;
  sections: number;
  fields: number;
  template: {
    cover_fields: CoverField[];
    sections: TemplateSection[];
    has_toc: boolean;
  };
}
interface PaperStatus {
  request?: { title: string };
  phase: string;
  detail: string;
  done: boolean;
  error?: string;
  result?: {
    output_path: string;
    stats: {
      total_words: number;
      section_count: number;
      reference_count: number;
      duration: number;
    };
  };
}

const STEPS = [
  { key: "upload", label: "上传模板", icon: Upload },
  { key: "info", label: "填写信息", icon: PenLine },
  { key: "generate", label: "生成论文", icon: Sparkles },
  { key: "done", label: "下载成品", icon: FileDown },
];

const PHASES = [
  { key: "parse_template", label: "解析模板", icon: FileText },
  { key: "research", label: "搜索文献", icon: Search },
  { key: "outline", label: "生成大纲", icon: BookOpen },
  { key: "writing", label: "撰写正文", icon: PenLine },
  { key: "optimize", label: "质量优化", icon: Sparkles },
  { key: "build", label: "生成文档", icon: FileDown },
];

const BASE = typeof window !== "undefined"
  ? (localStorage.getItem("yunque_api_base") || window.location.origin)
  : "";

function getApiKey(): string {
  if (typeof window === "undefined") return "";
  return localStorage.getItem("yunque_api_key") || "";
}

export default function PaperPage() {
  const [step, setStep] = useState(0);

  // Step 1: Template
  const [templateFile, setTemplateFile] = useState<File | null>(null);
  const [templateResult, setTemplateResult] = useState<TemplateResult | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);

  // Step 2: Info
  const [title, setTitle] = useState("");
  const [coverInfo, setCoverInfo] = useState<Record<string, string>>({});

  // Step 3: Generation
  const [status, setStatus] = useState<PaperStatus | null>(null);
  const [polling, setPolling] = useState(false);

  const inputStyle = {
    background: "var(--bg-main)",
    borderColor: "var(--border)",
    color: "var(--text-main)",
  };

  /* ─── Step 1: Upload ─── */
  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    const file = e.dataTransfer.files[0];
    if (file?.name.endsWith(".docx")) {
      setTemplateFile(file);
      uploadTemplate(file);
    }
  }, []);

  const handleFileSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setTemplateFile(file);
      uploadTemplate(file);
    }
  }, []);

  const uploadTemplate = async (file: File) => {
    setUploading(true);
    setUploadError("");
    try {
      const form = new FormData();
      form.append("template", file);
      const key = getApiKey();
      const res = await fetch(`${BASE}/v1/ext/paper/upload-template`, {
        method: "POST",
        headers: { ...(key ? { "X-API-Key": key } : {}) },
        body: form,
      });
      if (!res.ok) throw new Error(`上传失败: ${res.status}`);
      const data: TemplateResult = await res.json();
      setTemplateResult(data);
    } catch (e: unknown) {
      setUploadError(e instanceof Error ? e.message : String(e));
    } finally {
      setUploading(false);
    }
  };

  const skipTemplate = () => {
    setTemplateResult(null);
    setStep(1);
  };

  /* ─── Step 2 → 3: Start generation ─── */
  const startGeneration = async () => {
    setStep(2);
    setPolling(true);
    const key = getApiKey();
    try {
      await fetch(`${BASE}/v1/ext/paper/generate`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(key ? { "X-API-Key": key } : {}),
        },
        body: JSON.stringify({
          title,
          template_path: templateResult?.path || "",
          cover_info: coverInfo,
          reference_count: 20,
          language: "zh",
        }),
      });
    } catch { /* polling will catch errors */ }
  };

  /* ─── Polling ─── */
  useEffect(() => {
    if (!polling) return;
    const iv = setInterval(async () => {
      try {
        const key = getApiKey();
        const res = await fetch(`${BASE}/v1/ext/paper/status`, {
          headers: { ...(key ? { "X-API-Key": key } : {}) },
        });
        const data: PaperStatus = await res.json();
        setStatus(data);
        if (data.done) {
          setPolling(false);
          setStep(3);
        }
      } catch { /* retry */ }
    }, 2000);
    return () => clearInterval(iv);
  }, [polling]);

  /* ─── Download ─── */
  const handleDownload = () => {
    const key = getApiKey();
    const a = document.createElement("a");
    a.href = `${BASE}/v1/ext/paper/download${key ? `?key=${key}` : ""}`;
    a.download = "";
    a.click();
  };

  /* ─── Reset ─── */
  const handleReset = () => {
    setStep(0);
    setTemplateFile(null);
    setTemplateResult(null);
    setTitle("");
    setCoverInfo({});
    setStatus(null);
    setPolling(false);
    setUploadError("");
  };

  /* ─── Phase progress helper ─── */
  const currentPhaseIdx = status
    ? PHASES.findIndex((p) => p.key === status.phase)
    : -1;

  return (
    <div className="space-y-6 max-w-4xl mx-auto">
      {/* Header */}
      <BlurFade delay={0.05}>
        <div className="flex items-center gap-3 mb-2">
          <GraduationCap size={24} style={{ color: "var(--accent)" }} />
          <h1 className="text-2xl font-bold">论文写作助手</h1>
        </div>
        <p className="text-sm" style={{ color: "var(--text-muted)" }}>
          上传模板 → 填写信息 → AI 自动撰写 → 下载完整毕业论文 DOCX
        </p>
      </BlurFade>

      {/* Step Indicator */}
      <BlurFade delay={0.1}>
        <div className="flex items-center gap-2 px-2">
          {STEPS.map((s, i) => {
            const Icon = s.icon;
            const isActive = i === step;
            const isDone = i < step;
            const color = isDone ? "#22c55e" : isActive ? "#8b5cf6" : "var(--text-muted)";
            return (
              <div key={s.key} className="flex items-center gap-2 flex-1">
                <div className="flex items-center gap-2">
                  <div
                    className="w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold transition-all"
                    style={{
                      background: isDone ? "#22c55e20" : isActive ? "#8b5cf620" : "var(--bg-card)",
                      border: `2px solid ${color}`,
                      color,
                    }}
                  >
                    {isDone ? <CheckCircle2 size={14} /> : <Icon size={14} />}
                  </div>
                  <span className="text-xs font-medium whitespace-nowrap" style={{ color }}>
                    {s.label}
                  </span>
                </div>
                {i < STEPS.length - 1 && (
                  <div className="flex-1 h-px mx-2" style={{ background: isDone ? "#22c55e" : "var(--border)" }} />
                )}
              </div>
            );
          })}
        </div>
      </BlurFade>

      {/* Step Content */}
      <BlurFade delay={0.15}>
        <div
          className="rounded-xl p-6 border space-y-5"
          style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
        >
          {/* ── Step 0: Upload Template ── */}
          {step === 0 && (
            <>
              <div
                onDrop={handleDrop}
                onDragOver={(e) => e.preventDefault()}
                onClick={() => fileRef.current?.click()}
                className="border-2 border-dashed rounded-xl p-10 text-center cursor-pointer transition-all hover:border-[#8b5cf6]"
                style={{ borderColor: templateFile ? "#22c55e" : "var(--border)" }}
              >
                <input
                  ref={fileRef}
                  type="file"
                  accept=".docx"
                  className="hidden"
                  onChange={handleFileSelect}
                />
                {uploading ? (
                  <Loader2 size={32} className="mx-auto mb-3 animate-spin" style={{ color: "#8b5cf6" }} />
                ) : templateFile ? (
                  <CheckCircle2 size={32} className="mx-auto mb-3" style={{ color: "#22c55e" }} />
                ) : (
                  <Upload size={32} className="mx-auto mb-3" style={{ color: "var(--text-muted)" }} />
                )}
                <p className="text-sm font-medium" style={{ color: templateFile ? "#22c55e" : "var(--text-main)" }}>
                  {templateFile ? templateFile.name : "拖拽或点击上传 .docx 模板"}
                </p>
                <p className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>
                  没有模板？点击下方跳过，使用通用毕业论文模板
                </p>
              </div>

              {uploadError && (
                <div className="flex items-center gap-2 text-sm" style={{ color: "#ef4444" }}>
                  <AlertCircle size={14} /> {uploadError}
                </div>
              )}

              {/* Template preview */}
              {templateResult && (
                <div className="space-y-3">
                  <div className="text-xs font-medium" style={{ color: "var(--text-muted)" }}>
                    检测到 {templateResult.template.sections.length} 个章节，
                    {templateResult.template.cover_fields.length} 个封面字段
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    {templateResult.template.sections.slice(0, 10).map((s, i) => (
                      <div
                        key={i}
                        className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs"
                        style={{ background: "var(--bg-main)", color: "var(--text-main)" }}
                      >
                        <span style={{ color: "#8b5cf6" }}>
                          {s.type === "chapter" ? "📖" : s.type === "abstract" ? "📝" : s.type === "references" ? "📚" : "📄"}
                        </span>
                        {"  ".repeat(s.level - 1)}{s.title}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <div className="flex gap-3">
                <button
                  onClick={() => templateResult && setStep(1)}
                  disabled={!templateResult}
                  className="px-5 py-2.5 rounded-lg text-sm font-medium flex items-center gap-2 transition"
                  style={{
                    background: templateResult ? "#8b5cf6" : "var(--bg-main)",
                    color: templateResult ? "#fff" : "var(--text-muted)",
                    opacity: templateResult ? 1 : 0.5,
                  }}
                >
                  下一步 <ChevronRight size={14} />
                </button>
                <button
                  onClick={skipTemplate}
                  className="px-5 py-2.5 rounded-lg text-sm font-medium transition"
                  style={{ background: "var(--bg-main)", color: "var(--text-muted)", border: "1px solid var(--border)" }}
                >
                  跳过，使用默认模板
                </button>
              </div>
            </>
          )}

          {/* ── Step 1: Fill Info ── */}
          {step === 1 && (
            <>
              <div>
                <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
                  论文题目 *
                </label>
                <input
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder="例如：基于 Spring Boot 的在线商城系统设计与实现"
                  className="w-full rounded-lg px-3 py-2.5 text-sm border"
                  style={inputStyle}
                />
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {[
                  { key: "author", label: "作者姓名", ph: "张三" },
                  { key: "student_id", label: "学号", ph: "2024010001" },
                  { key: "department", label: "院系", ph: "计算机科学与技术学院" },
                  { key: "major", label: "专业", ph: "软件工程" },
                  { key: "class", label: "班级", ph: "软件2401" },
                  { key: "advisor", label: "指导教师", ph: "李教授" },
                  { key: "college", label: "学校名称", ph: "XX 大学" },
                  { key: "date", label: "日期", ph: "2026年03月" },
                ].map((f) => (
                  <div key={f.key}>
                    <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
                      {f.label}
                    </label>
                    <input
                      value={coverInfo[f.key] || ""}
                      onChange={(e) => setCoverInfo({ ...coverInfo, [f.key]: e.target.value })}
                      placeholder={f.ph}
                      className="w-full rounded-lg px-3 py-2 text-sm border"
                      style={inputStyle}
                    />
                  </div>
                ))}
              </div>

              <div className="flex gap-3">
                <button
                  onClick={() => setStep(0)}
                  className="px-5 py-2.5 rounded-lg text-sm font-medium transition"
                  style={{ background: "var(--bg-main)", color: "var(--text-muted)", border: "1px solid var(--border)" }}
                >
                  上一步
                </button>
                <button
                  onClick={startGeneration}
                  disabled={!title.trim()}
                  className="px-5 py-2.5 rounded-lg text-sm font-medium flex items-center gap-2 transition"
                  style={{
                    background: title.trim() ? "#8b5cf6" : "var(--bg-main)",
                    color: title.trim() ? "#fff" : "var(--text-muted)",
                    opacity: title.trim() ? 1 : 0.5,
                  }}
                >
                  <Sparkles size={14} /> 开始生成
                </button>
              </div>
            </>
          )}

          {/* ── Step 2: Generating ── */}
          {step === 2 && (
            <div className="space-y-4">
              <div className="flex items-center gap-3 mb-2">
                <Loader2 size={20} className="animate-spin" style={{ color: "#8b5cf6" }} />
                <span className="text-sm font-medium">
                  正在生成论文：{title}
                </span>
              </div>

              <div className="space-y-2">
                {PHASES.map((phase, i) => {
                  const Icon = phase.icon;
                  const isDone = i < currentPhaseIdx;
                  const isActive = i === currentPhaseIdx;
                  const color = isDone ? "#22c55e" : isActive ? "#8b5cf6" : "var(--text-muted)";
                  return (
                    <div
                      key={phase.key}
                      className="flex items-center gap-3 px-4 py-2.5 rounded-lg transition-all"
                      style={{
                        background: isActive ? "#8b5cf610" : "transparent",
                        borderLeft: `3px solid ${color}`,
                      }}
                    >
                      {isDone ? (
                        <CheckCircle2 size={16} style={{ color: "#22c55e" }} />
                      ) : isActive ? (
                        <Loader2 size={16} className="animate-spin" style={{ color: "#8b5cf6" }} />
                      ) : (
                        <Icon size={16} style={{ color: "var(--text-muted)" }} />
                      )}
                      <span className="text-sm" style={{ color }}>
                        {phase.label}
                      </span>
                      {isActive && status?.detail && (
                        <span className="text-xs ml-auto" style={{ color: "var(--text-muted)" }}>
                          {status.detail}
                        </span>
                      )}
                    </div>
                  );
                })}
              </div>

              {status?.error && (
                <div className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm"
                  style={{ background: "#ef444415", color: "#ef4444" }}>
                  <AlertCircle size={14} /> {status.error}
                </div>
              )}
            </div>
          )}

          {/* ── Step 3: Done ── */}
          {step === 3 && (
            <div className="space-y-5">
              <div className="flex items-center gap-3">
                <CheckCircle2 size={28} style={{ color: "#22c55e" }} />
                <div>
                  <h2 className="text-lg font-bold">论文生成完成！</h2>
                  <p className="text-xs" style={{ color: "var(--text-muted)" }}>
                    《{title}》已生成完毕
                  </p>
                </div>
              </div>

              {status?.result && (
                <div className="grid grid-cols-3 gap-3">
                  {[
                    { label: "总字数", value: status.result.stats.total_words.toLocaleString(), color: "#8b5cf6" },
                    { label: "章节数", value: String(status.result.stats.section_count), color: "#3b82f6" },
                    { label: "参考文献", value: `${status.result.stats.reference_count} 篇`, color: "#22c55e" },
                  ].map((s) => (
                    <div
                      key={s.label}
                      className="rounded-xl p-4 text-center border"
                      style={{ background: `${s.color}08`, borderColor: `${s.color}30` }}
                    >
                      <div className="text-2xl font-bold" style={{ color: s.color }}>
                        {s.value}
                      </div>
                      <div className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>
                        {s.label}
                      </div>
                    </div>
                  ))}
                </div>
              )}

              <div className="flex gap-3">
                <button
                  onClick={handleDownload}
                  className="px-6 py-3 rounded-lg text-sm font-medium flex items-center gap-2 transition"
                  style={{ background: "#22c55e", color: "#fff" }}
                >
                  <Download size={16} /> 下载 DOCX
                </button>
                <button
                  onClick={handleReset}
                  className="px-5 py-3 rounded-lg text-sm font-medium flex items-center gap-2 transition"
                  style={{ background: "var(--bg-main)", color: "var(--text-muted)", border: "1px solid var(--border)" }}
                >
                  <Trash2 size={14} /> 重新开始
                </button>
              </div>
            </div>
          )}
        </div>
      </BlurFade>
    </div>
  );
}
