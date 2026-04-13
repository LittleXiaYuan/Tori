"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { api, type QQAnalysis } from "@/lib/api";
import { Card, Button, Spinner, Chip, TextField, Input, TextArea } from "@heroui/react";
import {
  MessageSquareText, Upload, Trash2, Users, Clock, Hash,
  Smile, Tag, Send, User, RefreshCw, ChevronDown, ChevronRight, Loader2,
} from "lucide-react";
import { relTime } from "@/lib/constants";

/* ── Upload Panel ── */
function UploadPanel({ onUploaded }: { onUploaded: () => void }) {
  const [content, setContent] = useState("");
  const [filename, setFilename] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);

  const handleFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setFilename(file.name);
    const reader = new FileReader();
    reader.onload = () => setContent(reader.result as string);
    reader.readAsText(file);
  };

  const submit = async () => {
    if (!content.trim()) return;
    setLoading(true); setError("");
    try { await api.qqUpload(content, filename || "chat.txt"); setContent(""); setFilename(""); onUploaded(); }
    catch (err: unknown) { setError(err instanceof Error ? err.message : "上传失败"); }
    finally { setLoading(false); }
  };

  return (
    <Card className="section-card p-5 mb-6">
      <div className="flex items-center gap-2 mb-3">
        <Upload size={16} style={{ color: "var(--yunque-accent)" }} />
        <span className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>导入QQ聊天记录</span>
      </div>
      <p className="text-xs mb-3" style={{ color: "var(--yunque-text-muted)" }}>支持QQ聊天记录导出的 .txt 文件。格式：每条消息以日期时间+昵称开头。</p>
      <div className="flex gap-2 mb-3">
        <Button size="sm" variant="outline" onPress={() => fileRef.current?.click()}>{filename || "选择文件"}</Button>
        <input ref={fileRef} type="file" accept=".txt,.text" className="hidden" onChange={handleFile} />
        <span className="text-xs self-center" style={{ color: "var(--yunque-text-muted)" }}>或直接粘贴内容。</span>
      </div>
      <TextField>
        <TextArea className="font-mono" placeholder="粘贴QQ聊天记录文本..." rows={5} value={content} onChange={(e) => setContent(e.target.value)} />
      </TextField>
      {error && <div className="text-xs text-red-400 mb-2">{error}</div>}
      <Button size="sm" isDisabled={loading || !content.trim()} onPress={submit}
        className="btn-accent">
        {loading ? "解析中…" : "上传并分析"}
      </Button>
    </Card>
  );
}

/* ── Analysis Card ── */
function AnalysisCard({ analysis, onDelete, onSelectRoleplay }: { analysis: QQAnalysis; onDelete: () => void; onSelectRoleplay: (persona: string) => void }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <Card className="section-card mb-4 overflow-hidden">
      <div className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:opacity-80 transition-opacity" onClick={() => setExpanded(!expanded)}>
        {expanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
        <MessageSquareText size={16} style={{ color: "var(--yunque-accent)" }} />
        <div className="flex-1 min-w-0">
          <div className="font-medium text-sm" style={{ color: "var(--yunque-text)" }}>{analysis.file_name}</div>
          <div className="flex items-center gap-3 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
            <span><Hash size={10} className="inline" /> {analysis.total_messages} 条</span>
            <span><Users size={10} className="inline" /> {analysis.participants.length} 人</span>
            <span><Clock size={10} className="inline" /> {analysis.time_range}</span>
          </div>
        </div>
        <Chip size="sm" style={{ background: "rgba(0,111,238,0.15)", color: "var(--yunque-accent)" }}>{analysis.sentiment}</Chip>
        <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{relTime(analysis.analyzed_at)}</span>
        <Button isIconOnly aria-label="删除" size="sm" variant="ghost" onPress={(e) => { onDelete(); }}><Trash2 size={14} className="text-gray-400" /></Button>
      </div>

      {expanded && (
        <div className="px-4 py-3 border-t space-y-4" style={{ borderColor: "var(--yunque-border)" }}>
          <div>
            <div className="text-xs font-medium mb-1" style={{ color: "var(--yunque-text-muted)" }}># 对话总结</div>
            <p className="text-sm" style={{ color: "var(--yunque-text)" }}>{analysis.summary}</p>
          </div>
          {analysis.top_topics?.length > 0 && (
            <div>
              <div className="text-xs font-medium mb-1" style={{ color: "var(--yunque-text-muted)" }}><Tag size={10} className="inline" /> 热门话题</div>
              <div className="flex flex-wrap gap-1">
                {analysis.top_topics.map((t) => <Chip key={t} size="sm" variant="soft" style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)" }}>{t}</Chip>)}
              </div>
            </div>
          )}
          {analysis.persona_profiles && Object.keys(analysis.persona_profiles).length > 0 && (
            <div>
              <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text-muted)" }}><Users size={10} className="inline" /> 角色分析 · 点击开始角色扮演</div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                {Object.entries(analysis.persona_profiles).map(([name, profile]) => (
                  <Button key={name} variant="ghost" onPress={() => onSelectRoleplay(name)}
                    className="text-left p-3 rounded-lg border h-auto"
                    style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg)" }}>
                    <div className="text-sm font-medium flex items-center gap-1" style={{ color: "var(--yunque-text)" }}><User size={12} /> {name}</div>
                    <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>{profile as string}</div>
                  </Button>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </Card>
  );
}

/* ── Roleplay Panel ── */
function RoleplayPanel({ analysisId, persona, onClose }: { analysisId: string; persona: string; onClose: () => void }) {
  const [messages, setMessages] = useState<Array<{ role: "user" | "persona"; text: string }>>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const chatRef = useRef<HTMLDivElement>(null);

  const send = async () => {
    if (!input.trim() || loading) return;
    const userMsg = input.trim();
    setInput("");
    setMessages((prev) => [...prev, { role: "user", text: userMsg }]);
    setLoading(true);
    try { const res = await api.qqRoleplay(analysisId, persona, userMsg); setMessages((prev) => [...prev, { role: "persona", text: res.reply }]); }
    catch { setMessages((prev) => [...prev, { role: "persona", text: "（回复失败）" }]); }
    finally { setLoading(false); }
  };

  useEffect(() => { chatRef.current?.scrollTo(0, chatRef.current.scrollHeight); }, [messages]);

  return (
    <Card className="section-card mb-6 overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b" style={{ borderColor: "var(--yunque-border)" }}>
        <div className="flex items-center gap-2">
          <Smile size={16} style={{ color: "var(--yunque-accent)" }} />
          <span className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>角色扮演 · {persona}</span>
        </div>
        <Button size="sm" variant="ghost" onPress={onClose}>关闭</Button>
      </div>
      <div ref={chatRef} className="px-4 py-3 h-64 overflow-y-auto space-y-3">
        {messages.length === 0 && <p className="text-xs text-center" style={{ color: "var(--yunque-text-muted)" }}>发送消息开始与"{persona}"对话</p>}
        {messages.map((msg, i) => (
          <div key={i} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
            <div className="max-w-[75%] px-3 py-2 rounded-xl text-sm"
              style={{ background: msg.role === "user" ? "var(--yunque-accent)" : "var(--yunque-bg)", color: msg.role === "user" ? "#fff" : "var(--yunque-text)" }}>
              {msg.text}
            </div>
          </div>
        ))}
        {loading && <div className="flex justify-start"><div className="px-3 py-2 rounded-xl" style={{ background: "var(--yunque-bg)" }}><Loader2 size={14} className="animate-spin" /></div></div>}
      </div>
      <div className="flex gap-2 px-4 py-3 border-t" style={{ borderColor: "var(--yunque-border)" }}>
        <Input className="flex-1"
          placeholder={`对 {persona} 说...`} value={input} onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && send()} />
        <Button isIconOnly aria-label="发送" size="sm" isDisabled={loading || !input.trim()} onPress={send}
          className="btn-accent"><Send size={14} /></Button>
      </div>
    </Card>
  );
}

/* ── Page ── */
export default function QQChatPage() {
  const [analyses, setAnalyses] = useState<QQAnalysis[]>([]);
  const [loading, setLoading] = useState(true);
  const [roleplay, setRoleplay] = useState<{ analysisId: string; persona: string } | null>(null);

  const refresh = useCallback(async () => {
    try { const res = await api.qqAnalyses(); setAnalyses(res.analyses || []); }
    catch { /* ignore */ } finally { setLoading(false); }
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const handleDelete = async (id: string) => { await api.qqDelete(id); refresh(); };

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <MessageSquareText size={20} style={{ color: "var(--yunque-accent)" }} />
          <h1 className="page-title">QQ聊天分析</h1>
        </div>
        <Button isIconOnly aria-label="刷新" variant="ghost" size="sm" onPress={refresh}><RefreshCw size={16} /></Button>
      </div>

      <UploadPanel onUploaded={refresh} />

      {roleplay && <RoleplayPanel analysisId={roleplay.analysisId} persona={roleplay.persona} onClose={() => setRoleplay(null)} />}

      {loading ? (
        <div className="flex items-center justify-center py-12"><Spinner size="lg" /></div>
      ) : analyses.length === 0 ? (
        <div className="text-center py-12" style={{ color: "var(--yunque-text-muted)" }}>
          <MessageSquareText size={48} className="mx-auto mb-3 opacity-30" />
          <p>暂无分析记录</p>
          <p className="text-xs mt-1">上传QQ聊天记录开始分析</p>
        </div>
      ) : (
        analyses.map((a) => (
          <AnalysisCard key={a.id} analysis={a} onDelete={() => handleDelete(a.id)}
            onSelectRoleplay={(persona) => setRoleplay({ analysisId: a.id, persona })} />
        ))
      )}
    </div>
  );
}
