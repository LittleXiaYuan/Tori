"use client";

import { useState, useRef, useCallback, useEffect } from "react";
import { Card, Button, Spinner, Chip, ProgressBar, TextField, Input, Label } from "@heroui/react";
import {
  GraduationCap, Upload, FileText, CheckCircle2, Download,
  Loader2, AlertCircle, ChevronRight, BookOpen, Search,
  PenLine, Sparkles, FileDown, Trash2,
} from "lucide-react";
import { getAuthHeaders, getApiKey } from "@/lib/api";
import { isSafeApiBase } from "@/lib/safe-url";

interface TemplateResult {
  path: string; sections: number; fields: number;
  template: { cover_fields: { label: string; key: string }[]; sections: { level: number; title: string; type: string }[]; has_toc: boolean };
}
interface PaperStatus {
  request?: { title: string }; phase: string; detail: string; done: boolean; error?: string;
  result?: { output_path: string; stats: { total_words: number; section_count: number; reference_count: number; duration: number } };
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

// Resolve the API base at module load. We deliberately read localStorage only
// if the stored value parses as a plain http(s) origin — otherwise we fall
// back to same-origin. This prevents a user who was tricked into pasting a
// malicious string into browser devtools from redirecting every future fetch
// (with the current Bearer token) to an attacker-controlled host.
function resolveApiBase(): string {
  if (typeof window === "undefined") return "";
  const stored = localStorage.getItem("yunque_api_base");
  if (stored && isSafeApiBase(stored)) return stored;
  return window.location.origin;
}

const BASE = resolveApiBase();

export default function PaperPage() {
  const [step, setStep] = useState(0);
  const [templateFile, setTemplateFile] = useState<File | null>(null);
  const [templateResult, setTemplateResult] = useState<TemplateResult | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);
  const [title, setTitle] = useState("");
  const [coverInfo, setCoverInfo] = useState<Record<string, string>>({});
  const [status, setStatus] = useState<PaperStatus | null>(null);
  const [polling, setPolling] = useState(false);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    const file = e.dataTransfer.files[0];
    if (file?.name.endsWith(".docx")) { setTemplateFile(file); uploadTemplate(file); }
  }, []);

  const handleFileSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) { setTemplateFile(file); uploadTemplate(file); }
  }, []);

  const uploadTemplate = async (file: File) => {
    setUploading(true); setUploadError("");
    try {
      const form = new FormData(); form.append("template", file);
      const res = await fetch(`${BASE}/v1/ext/paper/upload-template`, { method: "POST", headers: { ...getAuthHeaders() }, body: form });
      if (!res.ok) throw new Error(`上传失败: ${res.status}`);
      setTemplateResult(await res.json());
    } catch (e: unknown) { setUploadError(e instanceof Error ? e.message : String(e)); }
    finally { setUploading(false); }
  };

  const startGeneration = async () => {
    setStep(2); setPolling(true);
    try {
      await fetch(`${BASE}/v1/ext/paper/generate`, {
        method: "POST", headers: { "Content-Type": "application/json", ...getAuthHeaders() },
        body: JSON.stringify({ title, template_path: templateResult?.path || "", cover_info: coverInfo, reference_count: 20, language: "zh" }),
      });
    } catch { /* polling will catch errors */ }
  };

  useEffect(() => {
    if (!polling) return;
    const iv = setInterval(async () => {
      try {
        const res = await fetch(`${BASE}/v1/ext/paper/status`, { headers: { ...getAuthHeaders() } });
        const data: PaperStatus = await res.json();
        setStatus(data);
        if (data.done) { setPolling(false); setStep(3); }
      } catch { /* retry */ }
    }, 2000);
    return () => clearInterval(iv);
  }, [polling]);

  const handleDownload = () => {
    const key = getApiKey();
    const a = document.createElement("a");
    a.href = `${BASE}/v1/ext/paper/download${key ? `?key=${key}` : ""}`;
    a.download = ""; a.click();
  };

  const handleReset = () => { setStep(0); setTemplateFile(null); setTemplateResult(null); setTitle(""); setCoverInfo({}); setStatus(null); setPolling(false); setUploadError(""); };

  const currentPhaseIdx = status ? PHASES.findIndex((p) => p.key === status.phase) : -1;

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <div className="flex items-center gap-3">
        <GraduationCap size={20} style={{ color: "var(--yunque-accent)" }} />
        <div>
          <h1 className="page-title">论文写作助手</h1>
          <p className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>上传模板 → 填写信息 → AI 自动撰写 → 下载完整毕业论文 DOCX</p>
        </div>
      </div>

      {/* Step Indicator */}
      <div className="flex items-center gap-2 px-2">
        {STEPS.map((s, i) => {
          const Icon = s.icon;
          const isActive = i === step;
          const isDone = i < step;
          const color = isDone ? "#22c55e" : isActive ? "#8b5cf6" : "var(--yunque-text-muted)";
          return (
            <div key={s.key} className="flex items-center gap-2 flex-1">
              <div className="flex items-center gap-2">
                <div className="w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold"
                  style={{ background: isDone ? "#22c55e20" : isActive ? "#8b5cf620" : "var(--yunque-card)", border: `2px solid ${color}`, color }}>
                  {isDone ? <CheckCircle2 size={14} /> : <Icon size={14} />}
                </div>
                <span className="text-xs font-medium whitespace-nowrap" style={{ color }}>{s.label}</span>
              </div>
              {i < STEPS.length - 1 && <div className="flex-1 h-px mx-2" style={{ background: isDone ? "#22c55e" : "var(--yunque-border)" }} />}
            </div>
          );
        })}
      </div>

      <Card className="section-card p-6">
        {/* Step 0: Upload */}
        {step === 0 && (
          <div className="space-y-5">
            <div onDrop={handleDrop} onDragOver={(e) => e.preventDefault()} onClick={() => fileRef.current?.click()}
              className="border-2 border-dashed rounded-xl p-10 text-center cursor-pointer transition-all hover:border-[#8b5cf6]"
              style={{ borderColor: templateFile ? "#22c55e" : "var(--yunque-border)" }}>
              <input ref={fileRef} type="file" accept=".docx" className="hidden" onChange={handleFileSelect} />
              {uploading ? <Loader2 size={32} className="mx-auto mb-3 animate-spin" style={{ color: "#8b5cf6" }} />
                : templateFile ? <CheckCircle2 size={32} className="mx-auto mb-3" style={{ color: "#22c55e" }} />
                : <Upload size={32} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />}
              <p className="text-sm font-medium" style={{ color: templateFile ? "#22c55e" : "var(--yunque-text)" }}>
                {templateFile ? templateFile.name : "拖拽或点击上传 .docx 模板"}
              </p>
              <p className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>没有模板？点击下方跳过，使用通用毕业论文模板</p>
            </div>
            {uploadError && <div className="flex items-center gap-2 text-sm text-red-400"><AlertCircle size={14} /> {uploadError}</div>}
            {templateResult && (
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                检测到 {templateResult.template.sections.length} 个章节，{templateResult.template.cover_fields.length} 个封面字段
              </div>
            )}
            <div className="flex gap-3">
              <Button isDisabled={!templateResult} onPress={() => templateResult && setStep(1)}
                style={{ background: templateResult ? "#8b5cf6" : undefined, color: templateResult ? "#fff" : undefined }}>
                下一步 <ChevronRight size={14} />
              </Button>
              <Button variant="outline" onPress={() => { setTemplateResult(null); setStep(1); }}>跳过，使用默认模板</Button>
            </div>
          </div>
        )}

        {/* Step 1: Fill Info */}
        {step === 1 && (
          <div className="space-y-5">
            <div>
              <TextField value={title} onChange={(v) => setTitle(v)}>
                <Label>论文题目 *</Label>
                <Input placeholder="例如：基于 Spring Boot 的在线商城系统设计与实现" />
              </TextField>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {[
                { key: "author", label: "作者姓名", ph: "张三" }, { key: "student_id", label: "学号", ph: "2024010001" },
                { key: "department", label: "院系", ph: "计算机科学与技术学院" }, { key: "major", label: "专业", ph: "软件工程" },
                { key: "class", label: "班级", ph: "软件2401" }, { key: "advisor", label: "指导教师", ph: "李教授" },
                { key: "college", label: "学校名称", ph: "XX 大学" }, { key: "date", label: "日期", ph: "2026年03月" },
              ].map((f) => (
                <div key={f.key}>
                  <TextField value={coverInfo[f.key] || ""} onChange={(v) => setCoverInfo({ ...coverInfo, [f.key]: v })}>
                    <Label>{f.label}</Label>
                    <Input placeholder={f.ph} />
                  </TextField>
                </div>
              ))}
            </div>
            <div className="flex gap-3">
              <Button variant="outline" onPress={() => setStep(0)}>上一步</Button>
              <Button isDisabled={!title.trim()} onPress={startGeneration}
                style={{ background: title.trim() ? "#8b5cf6" : undefined, color: title.trim() ? "#fff" : undefined }}>
                <Sparkles size={14} /> 开始生成
              </Button>
            </div>
          </div>
        )}

        {/* Step 2: Generating */}
        {step === 2 && (
          <div className="space-y-4">
            <div className="flex items-center gap-3 mb-2">
              <Loader2 size={20} className="animate-spin" style={{ color: "#8b5cf6" }} />
              <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>正在生成论文：{title}</span>
            </div>
            <div className="space-y-2">
              {PHASES.map((phase, i) => {
                const Icon = phase.icon;
                const isDone = i < currentPhaseIdx;
                const isActive = i === currentPhaseIdx;
                const color = isDone ? "#22c55e" : isActive ? "#8b5cf6" : "var(--yunque-text-muted)";
                return (
                  <div key={phase.key} className="flex items-center gap-3 px-4 py-2.5 rounded-lg"
                    style={{ background: isActive ? "#8b5cf610" : "transparent", borderLeft: `3px solid ${color}` }}>
                    {isDone ? <CheckCircle2 size={16} style={{ color: "#22c55e" }} />
                      : isActive ? <Loader2 size={16} className="animate-spin" style={{ color: "#8b5cf6" }} />
                      : <Icon size={16} style={{ color: "var(--yunque-text-muted)" }} />}
                    <span className="text-sm" style={{ color }}>{phase.label}</span>
                    {isActive && status?.detail && <span className="text-xs ml-auto" style={{ color: "var(--yunque-text-muted)" }}>{status.detail}</span>}
                  </div>
                );
              })}
            </div>
            {status?.error && (
              <div className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm" style={{ background: "#ef444415", color: "#ef4444" }}>
                <AlertCircle size={14} /> {status.error}
              </div>
            )}
          </div>
        )}

        {/* Step 3: Done */}
        {step === 3 && (
          <div className="space-y-5">
            <div className="flex items-center gap-3">
              <CheckCircle2 size={28} style={{ color: "#22c55e" }} />
              <div>
                <h2 className="text-lg font-bold" style={{ color: "var(--yunque-text)" }}>论文生成完成！</h2>
                <p className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>《{title}》已生成完毕</p>
              </div>
            </div>
            {status?.result && (
              <div className="grid grid-cols-3 gap-3">
                {[
                  { label: "总字数", value: status.result.stats.total_words.toLocaleString(), color: "#8b5cf6" },
                  { label: "章节数", value: String(status.result.stats.section_count), color: "#3b82f6" },
                  { label: "参考文献", value: `${status.result.stats.reference_count} 篇`, color: "#22c55e" },
                ].map((s) => (
                  <Card key={s.label} className="p-4 text-center" style={{ background: `${s.color}08`, borderColor: `${s.color}30` }}>
                    <div className="kpi-value" style={{ color: s.color }}>{s.value}</div>
                    <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>{s.label}</div>
                  </Card>
                ))}
              </div>
            )}
            <div className="flex gap-3">
              <Button onPress={handleDownload} style={{ background: "#22c55e", color: "#fff" }}><Download size={16} /> 下载 DOCX</Button>
              <Button variant="outline" onPress={handleReset}><Trash2 size={14} /> 重新开始</Button>
            </div>
          </div>
        )}
      </Card>
    </div>
  );
}
