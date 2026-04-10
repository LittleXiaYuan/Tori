"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { api, ConversationInfo, EmotionResult, StickerSuggestion, PresetInfo } from "@/lib/api";
import MarkdownRenderer from "@/components/markdown-renderer";
import { ExecutionTrace, type AgentEvent } from "@/components/execution-trace";
import { ComputerPanel } from "@/components/computer-panel";
import {
  Send, Bot, User, Sparkles, RotateCcw, Trash2, ChevronDown,
  Plus, MessageSquare, Pin, PinOff, Archive, ArchiveRestore, Pencil, Check, X,
  Volume2, Square, Image as ImageIcon, Music, Film, FileText, Heart,
  Zap, Brain, Gauge, Mic, MicOff, Monitor, StopCircle, CornerUpLeft, Copy,
  Globe, Terminal, MessageCircle, BookOpen, ScanFace, Package,
} from "lucide-react";

interface ActionOption {
  label: string;
  value: string;
}

interface AgentActionItem {
  kind: string;
  payload: { question?: string; options?: ActionOption[] };
}

interface Message {
  role: "user" | "assistant";
  content: string;
  timestamp?: number;
  skillsUsed?: string[];
  steps?: number;
  emotion?: EmotionResult;
  stickerSuggestion?: StickerSuggestion;
  traceEvents?: AgentEvent[];
  actions?: AgentActionItem[];
}

// Emotion display mapping
const emotionBadges: Record<string, { label: string; emoji: string; color: string }> = {
  happy: { label: "开心", emoji: "😊", color: "#22c55e" },
  sad: { label: "悲伤", emoji: "😢", color: "#3b82f6" },
  angry: { label: "愤怒", emoji: "😠", color: "#ef4444" },
  fearful: { label: "焦虑", emoji: "😰", color: "#f59e0b" },
  disgusted: { label: "反感", emoji: "😒", color: "#8b5cf6" },
  surprised: { label: "惊讶", emoji: "😮", color: "#06b6d4" },
};

// RichContent: detects rich media patterns in text and renders them as visual components.
// Patterns: [贴图: packageId=X, stickerId=Y], [图片: alt], [语音 Xs], [视频 Xs], [文件: name]
function RichContent({ content }: { content: string }) {
  const mediaPattern = /\[贴图: packageId=(\d+), stickerId=(\d+)\]|\[图片(?:: ([^\]]*))?\]|\[语音 (\d+)s\]|\[视频 (\d+)s\]|\[文件: ([^\]]+)\]/g;

  const elements: React.ReactNode[] = [];
  let lastIdx = 0;
  let match: RegExpExecArray | null;
  let i = 0;

  while ((match = mediaPattern.exec(content)) !== null) {
    if (match.index > lastIdx) {
      elements.push(<MarkdownRenderer key={i++} content={content.slice(lastIdx, match.index)} />);
    }

    if (match[1] && match[2]) {
      // LINE sticker → render as image
      const url = `https://stickershop.line-scdn.net/stickershop/v1/sticker/${match[2]}/iPhone/sticker.png`;
      elements.push(
        <div key={i++} className="my-2">
          <img src={url} alt={`LINE Sticker ${match[2]}`} className="max-w-[120px] max-h-[120px] rounded" loading="lazy" />
        </div>
      );
    } else if (match[0].startsWith("[图片")) {
      elements.push(
        <span key={i++} className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
          <ImageIcon size={12} /> {match[3] || "图片"}
        </span>
      );
    } else if (match[4]) {
      elements.push(
        <span key={i++} className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
          <Music size={12} /> 语音 {match[4]}s
        </span>
      );
    } else if (match[5]) {
      elements.push(
        <span key={i++} className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
          <Film size={12} /> 视频 {match[5]}s
        </span>
      );
    } else if (match[6]) {
      elements.push(
        <span key={i++} className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
          <FileText size={12} /> {match[6]}
        </span>
      );
    }

    lastIdx = match.index + match[0].length;
  }

  if (lastIdx < content.length) {
    elements.push(<MarkdownRenderer key={i++} content={content.slice(lastIdx)} />);
  }

  return elements.length > 0 ? <>{elements}</> : <MarkdownRenderer content={content} />;
}

// Renders interactive action buttons (template fill / edit / reference choices)
function ActionButtons({ actions, onSelect }: { actions: AgentActionItem[]; onSelect: (value: string) => void }) {
  return (
    <div className="mt-3 space-y-2">
      {actions.map((action, ai) => {
        if (action.kind !== "ask" || !action.payload?.options) return null;
        return (
          <div key={ai} className="flex flex-wrap gap-2">
            {action.payload.options.map((opt, oi) => (
              <button
                key={oi}
                onClick={() => onSelect(opt.value)}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all hover:scale-[1.03] active:scale-95 border"
                style={{
                  background: oi === 0 ? "var(--accent)" : "var(--bg)",
                  color: oi === 0 ? "white" : "var(--text)",
                  borderColor: oi === 0 ? "transparent" : "var(--border)",
                  cursor: "pointer",
                  animation: `fade-slide-in 0.3s ease-out ${oi * 0.06}s both`,
                }}
              >
                {opt.label}
              </button>
            ))}
          </div>
        );
      })}
    </div>
  );
}

export default function ChatPage() {
  const [messages, setMessages] = useState<Message[]>([]);
  const messagesRef = useRef<Message[]>([]);
  // Keep ref in sync so sendMessage always has latest messages
  useEffect(() => { messagesRef.current = messages; }, [messages]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [streaming, setStreaming] = useState(false);
  const [thinkingLevel, setThinkingLevel] = useState<"auto" | "none" | "deep">("auto");
  const [sessionId, setSessionId] = useState(() => {
    if (typeof window !== "undefined") {
      return localStorage.getItem("yunque_session_id") || `s_${Date.now()}`;
    }
    return `s_${Date.now()}`;
  });
  const [conversations, setConversations] = useState<ConversationInfo[]>([]);
  const [showArchived, setShowArchived] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [showScrollBtn, setShowScrollBtn] = useState(false);
  const [showComputer, setShowComputer] = useState(false);
  const [liveTraceEvents, setLiveTraceEvents] = useState<AgentEvent[]>([]);
  const [editingMsgIdx, setEditingMsgIdx] = useState<number | null>(null);
  const [editingMsgText, setEditingMsgText] = useState("");
  const [playingIdx, setPlayingIdx] = useState<number | null>(null);
  const [recording, setRecording] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const audioChunksRef = useRef<Blob[]>([]);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const [presets, setPresets] = useState<PresetInfo[]>([]);
  const [activePreset, setActivePreset] = useState<string>("");

  const scrollToBottom = useCallback(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, []);

  useEffect(() => { scrollToBottom(); }, [messages, scrollToBottom]);

  // Load conversation list
  const loadConversations = useCallback(async () => {
    try {
      const data = await api.conversations(showArchived);
      setConversations(data.sessions || []);
    } catch { /* ignore */ }
  }, [showArchived]);

  useEffect(() => { loadConversations(); }, [loadConversations]);

  // Load Persona Presets
  useEffect(() => {
    (async () => {
      try {
        const data = await api.getPresets();
        setPresets(data.presets || []);
        setActivePreset(data.active || "");
      } catch { /* ignore */ }
    })();
  }, []);

  const handlePresetChange = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    const id = e.target.value;
    setActivePreset(id);
    try {
      await api.switchPreset(id);
    } catch { /* revert or ignore */ }
  };

  // Persist sessionId to localStorage
  useEffect(() => {
    if (typeof window !== "undefined") {
      localStorage.setItem("yunque_session_id", sessionId);
    }
  }, [sessionId]);

  // Restore messages for the current session on page load
  useEffect(() => {
    (async () => {
      try {
        const data = await api.conversationMessages(sessionId);
        const msgs: Message[] = (data.messages || []).map((m: { role: string; content: string }) => ({
          role: m.role as "user" | "assistant",
          content: m.content,
        }));
        if (msgs.length > 0) setMessages(msgs);
      } catch { /* new session, no history */ }
      // Check for quick message from home page
      if (typeof window !== "undefined") {
        const quickMsg = localStorage.getItem("yunque_quick_msg");
        if (quickMsg) {
          localStorage.removeItem("yunque_quick_msg");
          setTimeout(() => sendMessage(quickMsg), 300);
        }
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Auto-resize textarea
  useEffect(() => {
    const el = inputRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, 160) + "px";
  }, [input]);

  // Detect scroll position
  useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container) return;
    const onScroll = () => {
      const atBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 100;
      setShowScrollBtn(!atBottom && messages.length > 0);
    };
    container.addEventListener("scroll", onScroll);
    return () => container.removeEventListener("scroll", onScroll);
  }, [messages.length]);

  // Switch to a conversation
  const switchConversation = async (conv: ConversationInfo) => {
    setSessionId(conv.id);
    try {
      const data = await api.conversationMessages(conv.id);
      const msgs: Message[] = (data.messages || []).map((m: { role: string; content: string }) => ({
        role: m.role as "user" | "assistant",
        content: m.content,
      }));
      setMessages(msgs);
    } catch {
      setMessages([]);
    }
  };

  // New conversation
  const newConversation = () => {
    const id = `s_${Date.now()}`;
    setSessionId(id);
    setMessages([]);
    inputRef.current?.focus();
  };

  // Rename
  const startRename = (conv: ConversationInfo) => {
    setEditingId(conv.id);
    setEditName(conv.name || conv.id);
  };

  const confirmRename = async () => {
    if (!editingId) return;
    await api.manageConversation(editingId, { name: editName });
    setEditingId(null);
    loadConversations();
  };

  // Pin/Unpin
  const togglePin = async (conv: ConversationInfo) => {
    await api.manageConversation(conv.id, { pinned: !conv.pinned });
    loadConversations();
  };

  // Archive/Unarchive
  const toggleArchive = async (conv: ConversationInfo) => {
    await api.manageConversation(conv.id, { archive: !conv.archived_at });
    loadConversations();
  };

  // Delete
  const deleteConversation = async (conv: ConversationInfo) => {
    if (!window.confirm(`确定删除对话「${conv.name || conv.id}」？此操作不可撤销。`)) return;
    await api.deleteConversation(conv.id);
    if (sessionId === conv.id) newConversation();
    loadConversations();
  };

  const sendMessage = async (overrideText?: string) => {
    const text = (overrideText || input).trim();
    if (!text || loading) return;

    const userMsg: Message = { role: "user", content: text, timestamp: Date.now() };
    setMessages((prev) => [...prev, userMsg]);
    setInput("");
    setLoading(true);

    // Create AbortController for this request
    const abort = new AbortController();
    abortRef.current = abort;

    try {
      setStreaming(true);
      setLiveTraceEvents([]);
      const assistantMsg: Message = { role: "assistant", content: "", timestamp: Date.now() };
      setMessages((prev) => [...prev, assistantMsg]);

      let fullContent = "";
      let streamWorked = false;
      let streamEmotion: Message["emotion"] = undefined;
      let streamSticker: Message["stickerSuggestion"] = undefined;
      let streamActions: AgentActionItem[] | undefined = undefined;
      const traceEventsRef: AgentEvent[] = [];
      let streamingRafPending = false;

      try {
        for await (const chunk of api.chatStream(
          [...messagesRef.current.slice(0, -1), userMsg].map((m) => ({ role: m.role, content: m.content })),
          sessionId,
          thinkingLevel !== "auto" ? thinkingLevel : undefined,
          abort.signal
        )) {
          if (abort.signal.aborted) break;
          streamWorked = true;
          // Parse step/trace events from agentic SSE
          if (chunk.startsWith("{\"id\":\"evt-")) {
            try {
              const evt: AgentEvent = JSON.parse(chunk);
              traceEventsRef.push(evt);
              setLiveTraceEvents([...traceEventsRef]);
              setShowComputer(true);
              setMessages((prev) => {
                const updated = [...prev];
                updated[updated.length - 1] = {
                  ...updated[updated.length - 1],
                  traceEvents: [...traceEventsRef],
                };
                return updated;
              });
              continue;
            } catch { /* not a trace event */ }
          }
          // Parse emotion/sticker/actions markers from done event
          const emotionMatch = chunk.match(/<!--emotion:(.+?)-->/);
          const stickerMatch = chunk.match(/<!--sticker:(.+?)-->/);
          const stickersMatch = chunk.match(/<!--stickers:(.+?)-->/);
          const actionsMatch = chunk.match(/<!--actions:(.+?)-->/);
          if (emotionMatch) {
            try { streamEmotion = JSON.parse(emotionMatch[1]); } catch { /* skip */ }
            continue;
          }
          if (stickerMatch) {
            try { streamSticker = JSON.parse(stickerMatch[1]); } catch { /* skip */ }
            continue;
          }
          if (stickersMatch) {
            // Multi-platform: pick LINE first, then any available
            try {
              const multi = JSON.parse(stickersMatch[1]) as Record<string, StickerSuggestion>;
              streamSticker = multi.line || Object.values(multi)[0];
            } catch { /* skip */ }
            continue;
          }
          if (actionsMatch) {
            try { streamActions = JSON.parse(actionsMatch[1]); } catch { /* skip */ }
            continue;
          }
          fullContent += chunk;
          // Throttle UI updates to animation frames to avoid excessive re-renders
          if (!streamingRafPending) {
            streamingRafPending = true;
            requestAnimationFrame(() => {
              streamingRafPending = false;
              const contentSnapshot = fullContent;
              setMessages((prev) => {
                const updated = [...prev];
                updated[updated.length - 1] = {
                  role: "assistant",
                  content: contentSnapshot,
                  timestamp: Date.now(),
                  emotion: streamEmotion,
                  stickerSuggestion: streamSticker,
                  traceEvents: traceEventsRef,
                  actions: streamActions,
                };
                return updated;
              });
            });
          }
        }
        // Final update with all metadata
        if (streamWorked) {
          setMessages((prev) => {
            const updated = [...prev];
            updated[updated.length - 1] = {
              ...updated[updated.length - 1],
              emotion: streamEmotion,
              stickerSuggestion: streamSticker,
              actions: streamActions,
            };
            return updated;
          });
        }
      } catch {
        if (!streamWorked) {
          const result = await api.chat(
            [...messages, userMsg].map((m) => ({ role: m.role, content: m.content })),
            sessionId,
            thinkingLevel !== "auto" ? thinkingLevel : undefined
          );
          fullContent = result.reply;
          setMessages((prev) => {
            const updated = [...prev];
            updated[updated.length - 1] = {
              role: "assistant",
              content: fullContent,
              timestamp: Date.now(),
              skillsUsed: result.skills_used,
              steps: result.steps,
              emotion: result.emotion,
              stickerSuggestion: result.sticker_suggestion || (result.sticker_suggestions ? (result.sticker_suggestions.line || Object.values(result.sticker_suggestions)[0]) : undefined),
            };
            return updated;
          });
        }
      }
      // Refresh conversation list after first message
      loadConversations();
    } catch (e: unknown) {
      if (abort.signal.aborted) {
        // User stopped — keep partial content, mark as stopped
        setMessages((prev) => {
          const updated = [...prev];
          const last = updated[updated.length - 1];
          if (last?.role === "assistant") {
            updated[updated.length - 1] = {
              ...last,
              content: last.content + (last.content ? "\n\n" : "") + "⏹ *已停止生成*",
            };
          }
          return updated;
        });
      } else {
        const errMsg = e instanceof Error ? e.message : "Error";
        setMessages((prev) => [
          ...prev.slice(0, -1),
          { role: "assistant", content: `> **Error:** ${errMsg}`, timestamp: Date.now() },
        ]);
      }
    } finally {
      setLoading(false);
      setStreaming(false);
      abortRef.current = null;
      inputRef.current?.focus();
    }
  };

  /** Stop the current generation */
  const stopGeneration = () => {
    if (abortRef.current) {
      abortRef.current.abort();
    }
  };

  /** Resend: re-run from a specific user message index */
  const resendFrom = (userMsgIdx: number) => {
    const msg = messages[userMsgIdx];
    if (!msg || msg.role !== "user") return;
    // Roll back to just before that user message, then resend
    setMessages(messages.slice(0, userMsgIdx));
    setTimeout(() => sendMessage(msg.content), 50);
  };

  /** Rollback: discard all messages after a given index (checkpoint) */
  const rollbackTo = (idx: number) => {
    if (loading) return;
    setMessages(messages.slice(0, idx + 1));
  };

  /** Edit a user message in-place, then resend */
  const startEditMsg = (idx: number) => {
    if (messages[idx]?.role !== "user") return;
    setEditingMsgIdx(idx);
    setEditingMsgText(messages[idx].content);
  };
  const confirmEditMsg = () => {
    if (editingMsgIdx === null) return;
    const text = editingMsgText.trim();
    if (!text) { setEditingMsgIdx(null); return; }
    // Replace user message and discard everything after it, then resend
    setMessages(messages.slice(0, editingMsgIdx));
    setEditingMsgIdx(null);
    setEditingMsgText("");
    setTimeout(() => sendMessage(text), 50);
  };
  const cancelEditMsg = () => {
    setEditingMsgIdx(null);
    setEditingMsgText("");
  };

  /** Copy message content to clipboard */
  const copyMessage = (content: string) => {
    navigator.clipboard.writeText(content).catch(() => {});
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  const clearChat = () => { if (!loading) setMessages([]); };

  const retryLast = () => {
    if (messages.length < 2 || loading) return;
    // Find last user message index
    const lastUserIdx = messages.map((m, i) => ({ m, i })).reverse().find(x => x.m.role === "user")?.i;
    if (lastUserIdx === undefined) return;
    resendFrom(lastUserIdx);
  };

  // TTS playback
  const stopAudio = useCallback(() => {
    if (audioRef.current) {
      audioRef.current.pause();
      audioRef.current = null;
    }
    setPlayingIdx(null);
  }, []);

  const playText = async (text: string, idx: number) => {
    stopAudio();
    if (playingIdx === idx) return; // was playing this one → just stop
    setPlayingIdx(idx);
    try {
      const buf = await api.tts(text);
      const blob = new Blob([buf], { type: "audio/mpeg" });
      const url = URL.createObjectURL(blob);
      const audio = new Audio(url);
      audioRef.current = audio;
      audio.onended = () => { URL.revokeObjectURL(url); setPlayingIdx(null); };
      audio.onerror = () => { URL.revokeObjectURL(url); setPlayingIdx(null); };
      await audio.play();
    } catch {
      setPlayingIdx(null);
    }
  };

  // STT recording
  const startRecording = async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const recorder = new MediaRecorder(stream, { mimeType: "audio/webm" });
      audioChunksRef.current = [];
      recorder.ondataavailable = (e) => {
        if (e.data.size > 0) audioChunksRef.current.push(e.data);
      };
      recorder.onstop = async () => {
        stream.getTracks().forEach((t) => t.stop());
        const blob = new Blob(audioChunksRef.current, { type: "audio/webm" });
        if (blob.size === 0) return;
        try {
          const result = await api.stt(blob, "zh");
          if (result.text) {
            setInput((prev) => (prev ? prev + " " : "") + result.text);
            inputRef.current?.focus();
          }
        } catch (err) {
          console.error("STT failed:", err);
        }
        setRecording(false);
      };
      mediaRecorderRef.current = recorder;
      recorder.start();
      setRecording(true);
    } catch {
      console.error("Microphone access denied");
    }
  };

  const stopRecording = () => {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state === "recording") {
      mediaRecorderRef.current.stop();
    }
  };

  // Sort: pinned first, then by updated_at desc
  const sortedConversations = [...conversations].sort((a, b) => {
    if (a.pinned && !b.pinned) return -1;
    if (!a.pinned && b.pinned) return 1;
    return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
  });

  return (
    <div className="flex h-[calc(100vh-3rem)] gap-0 -mx-6">
      {/* Conversation sidebar */}
      <div
        className="w-64 shrink-0 flex flex-col border-r"
        style={{ borderColor: "var(--border)", background: "var(--bg)" }}
      >
        {/* New conversation button */}
        <div className="p-3">
          <button
            onClick={newConversation}
            className="w-full flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-colors border"
            style={{ borderColor: "var(--border)", color: "var(--text)" }}
          >
            <Plus size={14} /> 新对话
          </button>
        </div>

        {/* Conversation list */}
        <div className="flex-1 overflow-y-auto px-2 space-y-0.5">
          {sortedConversations.map((conv) => (
            <div
              key={conv.id}
              className="group flex items-center gap-1 px-2 py-2 rounded-lg cursor-pointer transition-colors text-xs"
              style={{
                background: conv.id === sessionId ? "var(--bg-hover)" : "transparent",
                color: conv.id === sessionId ? "var(--text)" : "var(--text-muted)",
              }}
              onClick={() => switchConversation(conv)}
            >
              {conv.pinned && <Pin size={10} className="shrink-0" style={{ color: "var(--accent)" }} />}
              <MessageSquare size={12} className="shrink-0" />

              {editingId === conv.id ? (
                <div className="flex items-center gap-1 flex-1 min-w-0">
                  <input
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    className="flex-1 bg-transparent border-b text-xs outline-none"
                    style={{ borderColor: "var(--accent)", color: "var(--text)" }}
                    onClick={(e) => e.stopPropagation()}
                    onKeyDown={(e) => { if (e.key === "Enter") confirmRename(); if (e.key === "Escape") setEditingId(null); }}
                    autoFocus
                  />
                  <button onClick={(e) => { e.stopPropagation(); confirmRename(); }}><Check size={10} /></button>
                  <button onClick={(e) => { e.stopPropagation(); setEditingId(null); }}><X size={10} /></button>
                </div>
              ) : (
                <span className="truncate flex-1">{conv.name || conv.id}</span>
              )}

              {/* Hover actions */}
              <div className="hidden group-hover:flex items-center gap-0.5 shrink-0">
                <button onClick={(e) => { e.stopPropagation(); startRename(conv); }} title="重命名">
                  <Pencil size={10} />
                </button>
                <button onClick={(e) => { e.stopPropagation(); togglePin(conv); }} title={conv.pinned ? "取消置顶" : "置顶"}>
                  {conv.pinned ? <PinOff size={10} /> : <Pin size={10} />}
                </button>
                <button onClick={(e) => { e.stopPropagation(); toggleArchive(conv); }} title={conv.archived_at ? "恢复" : "归档"}>
                  {conv.archived_at ? <ArchiveRestore size={10} /> : <Archive size={10} />}
                </button>
                <button onClick={(e) => { e.stopPropagation(); deleteConversation(conv); }} title="删除" style={{ color: "#ef4444" }}>
                  <Trash2 size={10} />
                </button>
              </div>
            </div>
          ))}
        </div>

        {/* Archive toggle */}
        <div className="p-2 border-t text-center" style={{ borderColor: "var(--border)" }}>
          <button
            onClick={() => setShowArchived(!showArchived)}
            className="text-xs transition-colors"
            style={{ color: "var(--text-muted)" }}
          >
            {showArchived ? "隐藏归档" : "显示归档"}
          </button>
        </div>
      </div>

      {/* Main chat area */}
      <div className="flex-1 flex flex-col px-6">
        {/* Header — Manus-style clean title */}
        <div className="flex items-center justify-between py-3">
          <div className="flex items-center gap-2">
            <h1 className="text-sm font-medium" style={{ color: "var(--text)" }}>云雀 Agent</h1>
            {presets.length > 0 && (
              <select
                value={activePreset}
                onChange={handlePresetChange}
                className="text-sm px-2 py-1 rounded border outline-none cursor-pointer transition-colors"
                style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }}
              >
                {presets.map(p => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
            )}
            {streaming && (
              <span
                className="flex items-center gap-1.5 text-xs px-2 py-1 rounded-full"
                style={{ background: "var(--bg-hover)", color: "var(--accent)", animation: "pulse-glow 2s ease-in-out infinite" }}
              >
                <Sparkles size={12} /> Streaming
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            {liveTraceEvents.length > 0 && (
              <button
                onClick={() => setShowComputer(!showComputer)}
                className="p-2 rounded-lg transition-colors"
                style={{ color: showComputer ? "var(--accent)" : "var(--text-muted)" }}
                title={showComputer ? "隐藏云雀的电脑" : "显示云雀的电脑"}
              >
                <Monitor size={16} />
              </button>
            )}
            {messages.length > 0 && (
              <>
                <button onClick={retryLast} className="p-2 rounded-lg transition-colors" style={{ color: "var(--text-muted)" }} title="Retry last message">
                  <RotateCcw size={16} />
                </button>
                <button onClick={clearChat} className="p-2 rounded-lg transition-colors" style={{ color: "var(--text-muted)" }} title="Clear chat">
                  <Trash2 size={16} />
                </button>
              </>
            )}
            <span className="text-xs" style={{ color: "var(--text-muted)" }}>
              {messages.filter((m) => m.role === "user").length} messages
            </span>
          </div>
        </div>

        {/* Messages */}
        <div ref={scrollContainerRef} className="flex-1 overflow-y-auto space-y-4 pb-4 relative">
          {messages.length === 0 && (
            <div className="flex flex-col items-center justify-center h-full gap-8 px-4">
              {/* Hero title — human, warm */}
              <div className="text-center">
                <h1 className="text-[28px] font-semibold tracking-tight" style={{ color: "var(--text)", lineHeight: 1.3 }}>
                  我能为你做什么？
                </h1>
              </div>

              {/* Input card — softer, no hard border */}
              <div
                className="w-full max-w-[640px] rounded-2xl"
                style={{
                  background: "var(--bg-card)",
                  boxShadow: "0 0 0 1px var(--border), var(--shadow-sm)",
                }}
              >
                <textarea
                  ref={inputRef}
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="分配一个任务或提问任何问题..."
                  rows={3}
                  className="w-full resize-none px-5 pt-5 pb-2 text-[14px] outline-none placeholder:text-[var(--text-muted)]"
                  style={{ background: "transparent", color: "var(--text)", minHeight: 80 }}
                  disabled={loading}
                />
                <div className="flex items-center justify-between px-4 pb-3 pt-1">
                  <div className="flex items-center gap-0.5">
                    <button className="p-2 rounded-lg transition-colors hover:bg-[var(--bg-hover)]" style={{ color: "var(--text-muted)" }} title="附件">
                      <Plus size={16} />
                    </button>
                    {([
                      { key: "none" as const, icon: <Zap size={14} />, tip: "快速", color: "#22c55e" },
                      { key: "auto" as const, icon: <Gauge size={14} />, tip: "自动", color: "var(--accent)" },
                      { key: "deep" as const, icon: <Brain size={14} />, tip: "深度", color: "#8b5cf6" },
                    ] as const).map(({ key, icon, tip, color }) => (
                      <button
                        key={key}
                        onClick={() => setThinkingLevel(key)}
                        className="p-2 rounded-lg transition-all"
                        style={{ color: thinkingLevel === key ? color : "var(--text-muted)", background: thinkingLevel === key ? color + "15" : "transparent" }}
                        title={tip}
                      >
                        {icon}
                      </button>
                    ))}
                    <button
                      onClick={() => recording ? stopRecording() : startRecording()}
                      className="p-2 rounded-lg transition-colors"
                      style={{ color: recording ? "#ef4444" : "var(--text-muted)", background: recording ? "#ef444415" : "transparent" }}
                      title={recording ? "停止录音" : "语音输入"}
                    >
                      {recording ? <MicOff size={16} className="animate-pulse" /> : <Mic size={16} />}
                    </button>
                  </div>
                    <button
                      onClick={() => sendMessage()}
                      disabled={!input.trim()}
                      className="w-9 h-9 rounded-xl flex items-center justify-center transition-all"
                      style={{
                        background: input.trim() ? "var(--accent)" : "transparent",
                        color: input.trim() ? "white" : "var(--text-muted)",
                        cursor: input.trim() ? "pointer" : "default",
                        opacity: input.trim() ? 1 : 0.3,
                      }}
                    >
                      <Send size={15} />
                    </button>
                </div>
              </div>

              {/* Quick actions — subtle, no obvious border */}
              <div className="flex flex-wrap gap-2.5 justify-center max-w-[640px]">
                {[
                  { icon: <MessageCircle size={13} />, label: "对话" },
                  { icon: <Zap size={13} />, label: "创建任务" },
                  { icon: <BookOpen size={13} />, label: "知识库" },
                  { icon: <ScanFace size={13} />, label: "人格" },
                  { icon: <Package size={13} />, label: "技能" },
                ].map(({ icon, label }) => (
                  <button
                    key={label}
                    onClick={() => { setInput(label); inputRef.current?.focus(); }}
                    className="flex items-center gap-2 px-4 py-2.5 rounded-xl text-[13px] transition-all duration-200 cursor-pointer"
                    style={{
                      background: "var(--bg-hover)",
                      color: "var(--text-secondary)",
                      border: "none",
                    }}
                    onMouseEnter={(e) => { e.currentTarget.style.background = "var(--bg-elevated)"; e.currentTarget.style.color = "var(--text)"; }}
                    onMouseLeave={(e) => { e.currentTarget.style.background = "var(--bg-hover)"; e.currentTarget.style.color = "var(--text-secondary)"; }}
                  >
                    {icon}
                    {label}
                  </button>
                ))}
              </div>
            </div>
          )}

          {messages.map((msg, i) => (
            <div key={i} className={`group flex gap-3 ${msg.role === "user" ? "justify-end" : ""}`}>
              {msg.role === "assistant" && (
                <div className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0 mt-1" style={{ background: "var(--accent)", color: "white" }}>
                  <Bot size={16} />
                </div>
              )}
              <div className="flex flex-col max-w-[75%]">
                {/* Message bubble */}
                <div
                  className="rounded-xl px-4 py-3 text-sm leading-relaxed"
                  style={{
                    background: msg.role === "user" ? "var(--accent)" : "var(--bg-card)",
                    border: msg.role === "assistant" ? "1px solid var(--border)" : "none",
                    color: msg.role === "user" ? "white" : "var(--text)",
                  }}
                >
                  {/* Inline edit mode for user messages */}
                  {msg.role === "user" && editingMsgIdx === i ? (
                    <div className="flex flex-col gap-2">
                      <textarea
                        value={editingMsgText}
                        onChange={(e) => setEditingMsgText(e.target.value)}
                        className="w-full bg-transparent text-white text-sm resize-none outline-none border-b border-white/30 pb-1"
                        rows={2}
                        autoFocus
                        onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); confirmEditMsg(); } if (e.key === "Escape") cancelEditMsg(); }}
                      />
                      <div className="flex gap-1.5 justify-end">
                        <button onClick={cancelEditMsg} className="text-xs px-2 py-1 rounded" style={{ background: "rgba(255,255,255,0.15)" }}>取消</button>
                        <button onClick={confirmEditMsg} className="text-xs px-2 py-1 rounded font-medium" style={{ background: "rgba(255,255,255,0.3)" }}>发送</button>
                      </div>
                    </div>
                  ) : (
                    <>
                      {msg.role === "assistant" && msg.traceEvents && msg.traceEvents.length > 0 && (
                        <ExecutionTrace events={msg.traceEvents} isLive={streaming && i === messages.length - 1} />
                      )}
                      {msg.content ? (
                        msg.role === "assistant" ? (
                          <RichContent content={msg.content} />
                        ) : (
                          <span className="whitespace-pre-wrap">{msg.content}</span>
                        )
                      ) : (
                        <div className="flex items-center gap-2 py-1">
                          <span className="inline-flex gap-1">
                            {[0, 1, 2].map((d) => (
                              <span
                                key={d}
                                className="w-2 h-2 rounded-full"
                                style={{
                                  background: "var(--accent)",
                                  animation: "bounce-dot 1.2s ease-in-out infinite",
                                  animationDelay: `${d * 0.15}s`,
                                }}
                              />
                            ))}
                          </span>
                        </div>
                      )}
                      {msg.role === "assistant" && msg.skillsUsed && msg.skillsUsed.length > 0 && (
                        <div className="mt-2 pt-2 border-t flex flex-wrap gap-1" style={{ borderColor: "var(--border)" }}>
                          {msg.skillsUsed.map((s) => (
                            <span key={s} className="text-xs px-2 py-0.5 rounded-full" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
                              {s}
                            </span>
                          ))}
                          {msg.steps && <span className="text-xs" style={{ color: "var(--text-muted)" }}>· {msg.steps} steps</span>}
                        </div>
                      )}
                      {msg.role === "assistant" && msg.emotion && emotionBadges[msg.emotion.emotion] && (
                        <div className="mt-1 flex items-center gap-1.5" style={{ animation: "fade-slide-in 0.4s ease-out" }}>
                          <span
                            className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full"
                            style={{ background: emotionBadges[msg.emotion.emotion].color + "18", color: emotionBadges[msg.emotion.emotion].color }}
                          >
                            <Heart size={10} />
                            {emotionBadges[msg.emotion.emotion].emoji} {emotionBadges[msg.emotion.emotion].label}
                            <span className="opacity-60">{Math.round(msg.emotion.confidence * 100)}%</span>
                          </span>
                          {msg.stickerSuggestion && (
                            msg.stickerSuggestion.sticker_id ? (
                              <img
                                src={`https://stickershop.line-scdn.net/stickershop/v1/sticker/${msg.stickerSuggestion.sticker_id}/iPhone/sticker.png`}
                                alt="sticker"
                                className="w-8 h-8"
                                style={{ animation: "fade-scale-in 0.5s ease-out 0.15s both" }}
                                loading="lazy"
                              />
                            ) : msg.stickerSuggestion.emoji ? (
                              <span className="text-lg" style={{ animation: "fade-scale-in 0.5s ease-out 0.15s both" }}>
                                {msg.stickerSuggestion.emoji}
                              </span>
                            ) : null
                          )}
                        </div>
                      )}
                      {msg.role === "assistant" && msg.content && (
                        <div className="mt-1 flex justify-end">
                          <button
                            onClick={() => playingIdx === i ? stopAudio() : playText(msg.content, i)}
                            className="p-1 rounded transition-colors"
                            style={{ color: playingIdx === i ? "var(--accent)" : "var(--text-muted)" }}
                            title={playingIdx === i ? "停止播放" : "朗读"}
                          >
                            {playingIdx === i ? <Square size={14} /> : <Volume2 size={14} />}
                          </button>
                        </div>
                      )}
                      {msg.role === "assistant" && msg.actions && msg.actions.length > 0 && (
                        <ActionButtons
                          actions={msg.actions}
                          onSelect={(value) => {
                            setMessages((prev) => prev.map((m) =>
                              m === msg ? { ...m, actions: undefined } : m
                            ));
                            sendMessage(value);
                          }}
                        />
                      )}
                    </>
                  )}
                </div>

                {/* Hover action bar — Claude/Manus style */}
                {!loading && editingMsgIdx !== i && (
                  <div
                    className="hidden group-hover:flex items-center gap-0.5 mt-1"
                    style={{ justifyContent: msg.role === "user" ? "flex-end" : "flex-start" }}
                  >
                    {msg.role === "user" && (
                      <>
                        <button onClick={() => startEditMsg(i)} className="p-1 rounded transition-colors" style={{ color: "var(--text-muted)" }} title="编辑并重发">
                          <Pencil size={12} />
                        </button>
                        <button onClick={() => resendFrom(i)} className="p-1 rounded transition-colors" style={{ color: "var(--text-muted)" }} title="重新发送">
                          <RotateCcw size={12} />
                        </button>
                      </>
                    )}
                    {msg.role === "assistant" && msg.content && (
                      <button onClick={() => copyMessage(msg.content)} className="p-1 rounded transition-colors" style={{ color: "var(--text-muted)" }} title="复制">
                        <Copy size={12} />
                      </button>
                    )}
                    {i > 0 && (
                      <button onClick={() => rollbackTo(i - 1)} className="p-1 rounded transition-colors" style={{ color: "var(--text-muted)" }} title="回滚到此之前">
                        <CornerUpLeft size={12} />
                      </button>
                    )}
                  </div>
                )}
              </div>
              {msg.role === "user" && editingMsgIdx !== i && (
                <div className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0 mt-1" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
                  <User size={16} />
                </div>
              )}
            </div>
          ))}
          <div ref={bottomRef} />

          {showScrollBtn && (
            <button onClick={scrollToBottom} className="fixed bottom-24 right-8 p-2 rounded-full shadow-lg z-10" style={{ background: "var(--accent)", color: "white" }}>
              <ChevronDown size={20} />
            </button>
          )}
        </div>

        {/* Input — card style */}
        {messages.length > 0 && (
          <div className="py-3">
            <div
              className="rounded-2xl"
              style={{ background: "var(--bg-card)", boxShadow: "0 0 0 1px var(--border), var(--shadow-sm)" }}
            >
              <textarea
                ref={inputRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="分配一个任务或提问任何问题..."
                rows={1}
                className="w-full resize-none px-5 pt-3 pb-1 text-[14px] outline-none placeholder:text-[var(--text-muted)]"
                style={{ background: "transparent", color: "var(--text)", maxHeight: "160px" }}
                disabled={loading}
              />
              <div className="flex items-center justify-between px-3 pb-2">
                <div className="flex items-center gap-0.5">
                  <button className="p-1.5 rounded-lg transition-colors" style={{ color: "var(--text-muted)" }} title="附件">
                    <Plus size={15} />
                  </button>
                  {([
                    { key: "none" as const, icon: <Zap size={13} />, tip: "快速", color: "#22c55e" },
                    { key: "auto" as const, icon: <Gauge size={13} />, tip: "自动", color: "var(--accent)" },
                    { key: "deep" as const, icon: <Brain size={13} />, tip: "深度", color: "#8b5cf6" },
                  ] as const).map(({ key, icon, tip, color }) => (
                    <button
                      key={key}
                      onClick={() => setThinkingLevel(key)}
                      className="p-1.5 rounded-lg transition-all"
                      style={{ color: thinkingLevel === key ? color : "var(--text-muted)", background: thinkingLevel === key ? color + "15" : "transparent" }}
                      title={tip}
                    >
                      {icon}
                    </button>
                  ))}
                  <button
                    onClick={() => recording ? stopRecording() : startRecording()}
                    className="p-1.5 rounded-lg transition-colors"
                    style={{ color: recording ? "#ef4444" : "var(--text-muted)", background: recording ? "#ef444415" : "transparent" }}
                    title={recording ? "停止录音" : "语音输入"}
                  >
                    {recording ? <MicOff size={14} className="animate-pulse" /> : <Mic size={14} />}
                  </button>
                </div>
                <div className="flex items-center gap-1.5">
                  {loading ? (
                    <button
                      onClick={stopGeneration}
                      className="w-8 h-8 rounded-full flex items-center justify-center transition-all"
                      style={{ background: "#ef444420", color: "#ef4444", cursor: "pointer" }}
                      title="停止生成"
                    >
                      <StopCircle size={14} />
                    </button>
                  ) : (
                    <button
                      onClick={() => sendMessage()}
                      disabled={!input.trim()}
                      className="w-8 h-8 rounded-full flex items-center justify-center transition-all"
                      style={{
                        background: input.trim() ? "var(--accent)" : "var(--bg-hover)",
                        color: input.trim() ? "white" : "var(--text-muted)",
                        cursor: input.trim() ? "pointer" : "default",
                        opacity: input.trim() ? 1 : 0.5,
                      }}
                    >
                      <Send size={14} />
                    </button>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Computer Panel — right column (toggleable) */}
      {showComputer && (
        <div
          className="w-[40%] shrink-0 flex flex-col border-l"
          style={{ borderColor: "var(--border)" }}
        >
          <ComputerPanel
            traceEvents={liveTraceEvents}
            isLive={streaming}
            className="h-full"
          />
        </div>
      )}
    </div>
  );
}
