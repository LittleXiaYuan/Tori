"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { api, type QQAnalysis } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  MessageSquareText,
  Upload,
  Trash2,
  Users,
  Clock,
  Hash,
  Smile,
  Tag,
  Send,
  User,
  Loader2,
  RefreshCw,
  ChevronDown,
  ChevronRight,
} from "lucide-react";

/* ── Helpers ── */

function relTime(ts?: string): string {
  if (!ts) return "";
  const d = Date.now() - new Date(ts).getTime();
  if (d < 60000) return `${Math.floor(d / 1000)}s ago`;
  if (d < 3600000) return `${Math.floor(d / 60000)}m ago`;
  return `${Math.floor(d / 3600000)}h ago`;
}

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
    reader.onload = () => {
      setContent(reader.result as string);
    };
    reader.readAsText(file);
  };

  const submit = async () => {
    if (!content.trim()) return;
    setLoading(true);
    setError("");
    try {
      await api.qqUpload(content, filename || "chat.txt");
      setContent("");
      setFilename("");
      onUploaded();
    } catch (err: any) {
      setError(err?.message || "上传失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      className="rounded-xl p-5 border mb-6"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
    >
      <div className="flex items-center gap-2 mb-3">
        <Upload size={16} style={{ color: "var(--accent)" }} />
        <span className="font-semibold text-sm">导入QQ聊天记录</span>
      </div>
      <p className="text-xs mb-3" style={{ color: "var(--text-muted)" }}>
        支持QQ聊天记录导出的 .txt 文件。格式：每条消息以日期时间+昵称开头。
      </p>

      <div className="flex gap-2 mb-3">
        <button
          onClick={() => fileRef.current?.click()}
          className="px-3 py-1.5 rounded-lg text-xs border"
          style={{ borderColor: "var(--border)" }}
        >
          {filename || "选择文件"}
        </button>
        <input ref={fileRef} type="file" accept=".txt,.text" className="hidden" onChange={handleFile} />
        <span className="text-xs self-center" style={{ color: "var(--text-muted)" }}>
          或直接粘贴内容 ↓
        </span>
      </div>

      <textarea
        className="w-full mb-3 px-3 py-2 rounded-lg text-xs border bg-transparent resize-none font-mono"
        style={{ borderColor: "var(--border)" }}
        placeholder="粘贴QQ聊天记录文本..."
        rows={5}
        value={content}
        onChange={(e) => setContent(e.target.value)}
      />

      {error && <div className="text-xs text-red-400 mb-2">{error}</div>}

      <button
        onClick={submit}
        disabled={loading || !content.trim()}
        className="px-4 py-1.5 rounded-lg text-sm font-medium disabled:opacity-50"
        style={{ background: "var(--accent)", color: "#fff" }}
      >
        {loading ? "解析中…" : "上传并分析"}
      </button>
    </div>
  );
}

/* ── Analysis Card ── */

function AnalysisCard({
  analysis,
  onDelete,
  onSelectRoleplay,
}: {
  analysis: QQAnalysis;
  onDelete: () => void;
  onSelectRoleplay: (persona: string) => void;
}) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div
      className="rounded-xl border mb-4 overflow-hidden"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
    >
      {/* Header */}
      <div
        className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:opacity-80 transition-opacity"
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
        <MessageSquareText size={16} style={{ color: "var(--accent)" }} />
        <div className="flex-1 min-w-0">
          <div className="font-medium text-sm">{analysis.file_name}</div>
          <div className="flex items-center gap-3 text-xs" style={{ color: "var(--text-muted)" }}>
            <span><Hash size={10} className="inline" /> {analysis.total_messages} 条</span>
            <span><Users size={10} className="inline" /> {analysis.participants.length} 人</span>
            <span><Clock size={10} className="inline" /> {analysis.time_range}</span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <span
            className="text-xs px-2 py-0.5 rounded-full"
            style={{ background: "var(--accent)20", color: "var(--accent)" }}
          >
            {analysis.sentiment}
          </span>
          <span className="text-xs" style={{ color: "var(--text-muted)" }}>
            {relTime(analysis.analyzed_at)}
          </span>
          <button
            onClick={(e) => { e.stopPropagation(); onDelete(); }}
            className="p-1 rounded-lg hover:bg-white/10"
            title="删除"
          >
            <Trash2 size={14} className="text-gray-400" />
          </button>
        </div>
      </div>

      {/* Expanded details */}
      {expanded && (
        <div className="px-4 py-3 border-t space-y-4" style={{ borderColor: "var(--border)" }}>
          {/* Summary */}
          <div>
            <div className="text-xs font-medium mb-1" style={{ color: "var(--text-muted)" }}>📝 对话总结</div>
            <p className="text-sm">{analysis.summary}</p>
          </div>

          {/* Topics */}
          {analysis.top_topics?.length > 0 && (
            <div>
              <div className="text-xs font-medium mb-1" style={{ color: "var(--text-muted)" }}>
                <Tag size={10} className="inline" /> 热门话题
              </div>
              <div className="flex flex-wrap gap-1">
                {analysis.top_topics.map((t) => (
                  <span
                    key={t}
                    className="text-xs px-2 py-0.5 rounded-full"
                    style={{ background: "var(--accent)15", color: "var(--accent)" }}
                  >
                    {t}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* Persona Profiles */}
          {analysis.persona_profiles && Object.keys(analysis.persona_profiles).length > 0 && (
            <div>
              <div className="text-xs font-medium mb-2" style={{ color: "var(--text-muted)" }}>
                <Users size={10} className="inline" /> 角色分析 — 点击开始角色扮演
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                {Object.entries(analysis.persona_profiles).map(([name, profile]) => (
                  <button
                    key={name}
                    onClick={() => onSelectRoleplay(name)}
                    className="text-left p-3 rounded-lg border hover:border-[var(--accent)] transition-colors"
                    style={{ borderColor: "var(--border)" }}
                  >
                    <div className="text-sm font-medium flex items-center gap-1">
                      <User size={12} /> {name}
                    </div>
                    <div className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>
                      {profile}
                    </div>
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/* ── Roleplay Chat Panel ── */

function RoleplayPanel({
  analysisId,
  persona,
  onClose,
}: {
  analysisId: string;
  persona: string;
  onClose: () => void;
}) {
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

    try {
      const res = await api.qqRoleplay(analysisId, persona, userMsg);
      setMessages((prev) => [...prev, { role: "persona", text: res.reply }]);
    } catch {
      setMessages((prev) => [...prev, { role: "persona", text: "（回复失败）" }]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    chatRef.current?.scrollTo(0, chatRef.current.scrollHeight);
  }, [messages]);

  return (
    <div
      className="rounded-xl border mb-6 overflow-hidden"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
    >
      <div className="flex items-center justify-between px-4 py-3 border-b" style={{ borderColor: "var(--border)" }}>
        <div className="flex items-center gap-2">
          <Smile size={16} style={{ color: "var(--accent)" }} />
          <span className="font-semibold text-sm">角色扮演 — {persona}</span>
        </div>
        <button onClick={onClose} className="text-xs px-2 py-1 rounded hover:bg-white/10" style={{ color: "var(--text-muted)" }}>
          关闭
        </button>
      </div>

      <div ref={chatRef} className="px-4 py-3 h-64 overflow-y-auto space-y-3">
        {messages.length === 0 && (
          <p className="text-xs text-center" style={{ color: "var(--text-muted)" }}>
            发送消息开始与"{persona}"对话
          </p>
        )}
        {messages.map((msg, i) => (
          <div key={i} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
            <div
              className="max-w-[75%] px-3 py-2 rounded-xl text-sm"
              style={{
                background: msg.role === "user" ? "var(--accent)" : "var(--bg-main)",
                color: msg.role === "user" ? "#fff" : "var(--text)",
              }}
            >
              {msg.text}
            </div>
          </div>
        ))}
        {loading && (
          <div className="flex justify-start">
            <div className="px-3 py-2 rounded-xl" style={{ background: "var(--bg-main)" }}>
              <Loader2 size={14} className="animate-spin" />
            </div>
          </div>
        )}
      </div>

      <div className="flex gap-2 px-4 py-3 border-t" style={{ borderColor: "var(--border)" }}>
        <input
          className="flex-1 px-3 py-2 rounded-lg text-sm border bg-transparent"
          style={{ borderColor: "var(--border)" }}
          placeholder={`对${persona}说...`}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && send()}
        />
        <button
          onClick={send}
          disabled={loading || !input.trim()}
          className="p-2 rounded-lg disabled:opacity-50"
          style={{ background: "var(--accent)", color: "#fff" }}
        >
          <Send size={16} />
        </button>
      </div>
    </div>
  );
}

/* ── Page ── */

export default function QQChatPage() {
  const [analyses, setAnalyses] = useState<QQAnalysis[]>([]);
  const [loading, setLoading] = useState(true);
  const [roleplay, setRoleplay] = useState<{ analysisId: string; persona: string } | null>(null);

  const refresh = useCallback(async () => {
    try {
      const res = await api.qqAnalyses();
      setAnalyses(res.analyses || []);
    } catch {
      /* ignore */
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const handleDelete = async (id: string) => {
    await api.qqDelete(id);
    refresh();
  };

  return (
    <div className="max-w-4xl mx-auto px-4 py-8">
      <BlurFade delay={0.05}>
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <MessageSquareText size={24} style={{ color: "var(--accent)" }} />
            <h1 className="text-2xl font-bold">QQ聊天分析</h1>
          </div>
          <button
            onClick={refresh}
            className="p-2 rounded-lg hover:bg-white/10 transition-colors"
            title="刷新"
          >
            <RefreshCw size={16} />
          </button>
        </div>
      </BlurFade>

      <BlurFade delay={0.1}>
        <UploadPanel onUploaded={refresh} />
      </BlurFade>

      {roleplay && (
        <BlurFade delay={0.1}>
          <RoleplayPanel
            analysisId={roleplay.analysisId}
            persona={roleplay.persona}
            onClose={() => setRoleplay(null)}
          />
        </BlurFade>
      )}

      <BlurFade delay={0.15}>
        {loading ? (
          <div className="text-center py-12" style={{ color: "var(--text-muted)" }}>
            <Loader2 size={24} className="animate-spin mx-auto mb-2" />
            加载中…
          </div>
        ) : analyses.length === 0 ? (
          <div className="text-center py-12" style={{ color: "var(--text-muted)" }}>
            <MessageSquareText size={48} className="mx-auto mb-3 opacity-30" />
            <p>暂无分析记录</p>
            <p className="text-xs mt-1">上传QQ聊天记录开始分析</p>
          </div>
        ) : (
          analyses.map((a) => (
            <AnalysisCard
              key={a.id}
              analysis={a}
              onDelete={() => handleDelete(a.id)}
              onSelectRoleplay={(persona) =>
                setRoleplay({ analysisId: a.id, persona })
              }
            />
          ))
        )}
      </BlurFade>
    </div>
  );
}
