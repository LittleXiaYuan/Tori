"use client";

import { useState, useReducer, useRef, useCallback, useEffect, useMemo } from "react";
import { Button, Avatar, Spinner, Tooltip, Chip, Dropdown, Label } from "@heroui/react";
import {
  Send, Plus, MessageCircle, Zap, BookOpen, ScanFace, Package,
  Brain, Gauge, Mic, StopCircle, Pencil, RotateCcw, Copy,
  Sparkles, Check, Search, Library, ChevronDown, Cpu,
  Paperclip, ImageIcon, Trash2, Volume2, Pin, Archive,
  PanelRightOpen, PanelRightClose, VolumeX, ArchiveRestore, Edit3, Heart,
  PinOff, MoreHorizontal, Monitor, AlertTriangle, Plug,
} from "lucide-react";
import { api, type ConversationInfo, type EmotionResult, type StickerSuggestion, type PresetInfo } from "@/lib/api";
import MarkdownRenderer from "@/components/markdown-renderer";
import { ExecutionTrace, type AgentEvent } from "@/components/execution-trace";
import { ComputerPanel } from "@/components/computer-panel";
import { ConnectorPopover } from "@/components/connector-popover";
import { BrowserSessionCard, type BrowserActionArtifactSummary, type BrowserBridgeState, type BrowserSessionNotice } from "@/components/browser-session-card";
import { SlashCommandMenu } from "@/components/slash-command-menu";
import { EmotionBadge, StickerView, SkillTags, AgentActions, type AgentAction } from "@/components/chat-extras";
import { showToast } from "@/components/toast-provider";
import { useBrowserBridge } from "@/lib/use-browser-bridge";
import { browserActionLabel } from "@/lib/browser-action-labels";

interface Suggestion {
  type: "followup" | "save_skill";
  label: string;
  icon?: string;
}

interface Message {
  role: "user" | "assistant";
  content: string;
  id: string;
  emotion?: EmotionResult;
  sticker?: StickerSuggestion;
  stickers?: Record<string, StickerSuggestion>;
  skills_used?: string[];
  actions?: AgentAction[];
  traceEvents?: AgentEvent[];
  suggestions?: Suggestion[];
  images?: string[];
  reasoning?: string;
  browserSummary?: BrowserActionArtifactSummary;
}

let msgId = 0;
function newId() { return `msg-${++msgId}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`; }

function makeBrowserTraceEvent(summary: string, detail?: unknown, kind: "tool_start" | "tool_result" | "reflect" = "tool_result"): AgentEvent {
  const now = new Date().toISOString();
  return {
    id: `browser-trace-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    trace_id: "browser-bridge",
    ts: now,
    domain: "planner",
    type: kind,
    summary,
    detail,
    meta: {
      skill: "browser_runtime",
    },
  };
}

/** Map technical error messages to user-friendly Chinese text */
function friendlyError(msg: string): string {
  const m = (msg || "").toLowerCase();
  if (m.includes("no provider") || m.includes("provider not")) return "当前还没有可用的模型提供商，请先到设置中添加 API Key。";
  if (m.includes("planner_error") || m.includes("planner error")) return "Agent 规划阶段失败，请重试或切换模型。";
  if (m.includes("context deadline") || m.includes("timeout")) return "请求超时了，请稍后再试。";
  if (m.includes("rate limit") || m.includes("429")) return "当前请求过于频繁，模型提供商正在限流，请稍后重试。";
  if (m.includes("401") || m.includes("unauthorized") || m.includes("invalid api key")) return "API Key 可能无效或已过期，请检查提供商设置。";
  if (m.includes("502") || m.includes("503") || m.includes("bad gateway")) return "上游模型服务暂时不可用，请稍后再试。";
  if (m.includes("request failed")) return "请求执行失败，请检查网络或服务状态后重试。";
  return msg;
}

function collectGeneratedFiles(traceEvents?: AgentEvent[]) {
  const files: Array<{ path: string; name: string; size?: number }> = [];
  for (const evt of traceEvents || []) {
    const detail = evt.detail as Record<string, unknown> | undefined;
    if (detail && Array.isArray(detail.files)) {
      for (const file of detail.files as Array<{ path: string; name: string; size?: number }>) {
        if (!files.some((entry) => entry.path === file.path)) files.push(file);
      }
    }
  }
  return files;
}

function summarizeAssistantWork(message: Message) {
  const traceEvents = message.traceEvents || [];
  const toolEvents = traceEvents.filter((event) => event.type === "tool_start" || event.type === "tool_result");
  const skills = [...new Set(toolEvents.map((event) => event.meta?.skill).filter(Boolean))];
  const files = collectGeneratedFiles(traceEvents);
  const warnings = traceEvents.filter((event) => {
    const summary = (event.summary || "").toLowerCase();
    return event.type === "plan" && (
      summary.includes("warning") ||
      summary.includes("risk") ||
      summary.includes("blocked") ||
      summary.includes("needs review")
    );
  });

  return {
    toolCount: toolEvents.length,
    skillCount: skills.length,
    primarySkill: skills[skills.length - 1] || "",
    fileCount: files.length,
    warningCount: warnings.length,
  };
}

function getSlashState(input: string) {
  const trimmed = input.trimStart();
  if (!trimmed.startsWith("/")) return { visible: false, query: "" };
  const firstLine = trimmed.split("\n")[0];
  const commandPart = firstLine.slice(1);
  if (commandPart.includes(" ")) return { visible: false, query: "" };
  return { visible: true, query: commandPart };
}

function mapBrowserSummary(summary: any): BrowserActionArtifactSummary {
  return {
    action: summary?.skill,
    url: summary?.url,
    title: summary?.title,
    elementCount: summary?.element_count,
    tabId: summary?.tab_id,
    hasScreenshot: summary?.has_screenshot,
    textLength: summary?.text_length,
    preview: summary?.preview,
    suggestedCommand: summary?.next_command,
    suggestedLabel: summary?.next_label,
    updatedAt: Date.now(),
  };
}

function parseSlashBrowserCommand(input: string) {
  const trimmed = input.trim();
  if (!trimmed.startsWith("/")) return null;
  const [cmdRaw, ...restParts] = trimmed.split(/\s+/);
  const cmd = cmdRaw.toLowerCase();
  const args = restParts.join(" ").trim();
  const browserCommands: Record<string, { summary: string }> = {
    "/navigate": { summary: args ? `???????${args}` : "????????" },
    "/screenshot": { summary: "????????" },
    "/content": { summary: "??????????" },
    "/mark": { summary: "??????????" },
    "/unmark": { summary: "????????" },
    "/scroll": { summary: args ? `???????${args}` : "??????" },
    "/click": { summary: args ? `???????${args}` : "????????" },
    "/type": { summary: args ? `???????${args.slice(0, 32)}` : "??????" },
  };
  if (!browserCommands[cmd]) return null;
  return { command: cmd, args, ...browserCommands[cmd] };
}

// 閳光偓閳光偓 Chat state reducer 閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓

interface ChatState {
  messages: Message[];
  input: string;
  loading: boolean;
  streaming: boolean;
  liveTraceEvents: AgentEvent[];
}

type ChatAction =
  | { type: "SET_INPUT"; value: string }
  | { type: "SET_MESSAGES"; messages: Message[] }
  | { type: "ADD_PAIR"; userMsg: Message; asstMsg: Message }
  | { type: "APPEND_LAST"; delta: string }
  | { type: "UPDATE_LAST"; updates: Partial<Message> }
  | { type: "APPEND_LAST_TRACE"; event: AgentEvent }
  | { type: "APPEND_LAST_REASONING"; delta: string }
  | { type: "ERROR_LAST"; error: string }
  | { type: "REMOVE_MSG"; id: string }
  | { type: "START_SEND" }
  | { type: "FINISH_SEND" }
  | { type: "ADD_LIVE_TRACE"; event: AgentEvent }
  | { type: "CLEAR_LIVE_TRACE" };

const chatInit: ChatState = { messages: [], input: "", loading: false, streaming: false, liveTraceEvents: [] };

function chatReducer(state: ChatState, action: ChatAction): ChatState {
  switch (action.type) {
    case "SET_INPUT":
      return { ...state, input: action.value };
    case "SET_MESSAGES":
      return { ...state, messages: action.messages };
    case "ADD_PAIR":
      return { ...state, messages: [...state.messages, action.userMsg, action.asstMsg] };
    case "APPEND_LAST": {
      const msgs = [...state.messages];
      if (msgs.length === 0) return state;
      const last = msgs[msgs.length - 1];
      msgs[msgs.length - 1] = { ...last, content: last.content + action.delta };
      return { ...state, messages: msgs };
    }
    case "UPDATE_LAST": {
      const msgs = [...state.messages];
      if (msgs.length === 0) return state;
      msgs[msgs.length - 1] = { ...msgs[msgs.length - 1], ...action.updates };
      return { ...state, messages: msgs };
    }
    case "APPEND_LAST_TRACE": {
      const msgs = [...state.messages];
      if (msgs.length === 0) return state;
      const last = { ...msgs[msgs.length - 1] };
      last.traceEvents = [...(last.traceEvents || []), action.event];
      msgs[msgs.length - 1] = last;
      const liveTraceEvents = [...state.liveTraceEvents.slice(-50), action.event];
      return { ...state, messages: msgs, liveTraceEvents };
    }
    case "APPEND_LAST_REASONING": {
      const msgs = [...state.messages];
      if (msgs.length === 0) return state;
      const last = { ...msgs[msgs.length - 1] };
      last.reasoning = (last.reasoning || "") + action.delta;
      msgs[msgs.length - 1] = last;
      return { ...state, messages: msgs };
    }
    case "ERROR_LAST": {
      const msgs = [...state.messages];
      if (msgs.length === 0) return state;
      const last = msgs[msgs.length - 1];
      msgs[msgs.length - 1] = { ...last, content: last.content + `\n\n[FAIL] ${action.error}` };
      return { ...state, messages: msgs };
    }
    case "REMOVE_MSG":
      return { ...state, messages: state.messages.filter((m) => m.id !== action.id) };
    case "START_SEND":
      return { ...state, input: "", loading: true, streaming: true, liveTraceEvents: [] };
    case "FINISH_SEND":
      return { ...state, loading: false, streaming: false };
    case "ADD_LIVE_TRACE":
      return { ...state, liveTraceEvents: [...state.liveTraceEvents.slice(-50), action.event] };
    case "CLEAR_LIVE_TRACE":
      return { ...state, liveTraceEvents: [] };
    default:
      return state;
  }
}

// 閳光偓閳光偓 Conversation state reducer 閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓閳光偓

interface ConvState {
  list: ConversationInfo[];
  activeId: string;
  showArchived: boolean;
  searchQuery: string;
  renameId: string | null;
  renameText: string;
}

type ConvAction =
  | { type: "SET_LIST"; list: ConversationInfo[] }
  | { type: "UPDATE_ONE"; id: string; data: Partial<ConversationInfo> }
  | { type: "REMOVE"; id: string }
  | { type: "SET_ACTIVE"; id: string }
  | { type: "SET_ARCHIVED"; show: boolean }
  | { type: "SET_SEARCH"; query: string }
  | { type: "START_RENAME"; id: string; text: string }
  | { type: "SET_RENAME_TEXT"; text: string }
  | { type: "CANCEL_RENAME" };

const convInit: ConvState = { list: [], activeId: "default", showArchived: false, searchQuery: "", renameId: null, renameText: "" };

function convReducer(state: ConvState, action: ConvAction): ConvState {
  switch (action.type) {
    case "SET_LIST":
      return { ...state, list: action.list };
    case "UPDATE_ONE":
      return { ...state, list: state.list.map((c) => c.id === action.id ? { ...c, ...action.data } : c), renameId: null };
    case "REMOVE":
      return { ...state, list: state.list.filter((c) => c.id !== action.id) };
    case "SET_ACTIVE":
      return { ...state, activeId: action.id };
    case "SET_ARCHIVED":
      return { ...state, showArchived: action.show };
    case "SET_SEARCH":
      return { ...state, searchQuery: action.query };
    case "START_RENAME":
      return { ...state, renameId: action.id, renameText: action.text };
    case "SET_RENAME_TEXT":
      return { ...state, renameText: action.text };
    case "CANCEL_RENAME":
      return { ...state, renameId: null, renameText: "" };
    default:
      return state;
  }
}

export default function ChatPage() {
  const [chat, chatD] = useReducer(chatReducer, chatInit);
  const [conv, convD] = useReducer(convReducer, convInit);
  const [thinkingLevel, setThinkingLevel] = useState<"none" | "auto" | "deep">("auto");
  const [copiedIdx, setCopiedIdx] = useState<string | null>(null);
  const [showSidebar, setShowSidebar] = useState(true);
  const [showComputer, setShowComputer] = useState(false);
  const [showConnectors, setShowConnectors] = useState(false);
  const [showSlashMenu, setShowSlashMenu] = useState(false);
  const [slashQuery, setSlashQuery] = useState("");
  const [thinkingEnabled, setThinkingEnabled] = useState<boolean | null>(null);
  const [suggestedTab, setSuggestedTab] = useState<"terminal" | "browser" | "editor" | "thinking" | undefined>(undefined);
  const [computerWidth, setComputerWidth] = useState(420);
  const resizingRef = useRef(false);
  const [currentModel, setCurrentModel] = useState("");
  const [availableModels, setAvailableModels] = useState<Array<{ id: string; model: string; display_name?: string; enabled: boolean }>>([]);
  const [ttsPlaying, setTtsPlaying] = useState<string | null>(null);
  const [isRecording, setIsRecording] = useState(false);
  const [presets, setPresets] = useState<PresetInfo[]>([]);
  const [activePreset, setActivePreset] = useState("");
  const [setupNeeded, setSetupNeeded] = useState(false);
  const [browserTraceEvents, setBrowserTraceEvents] = useState<AgentEvent[]>([]);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const inputShellRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);

  // Load providers for model selector
  useEffect(() => {
    api.providerList().then((data) => {
      const providers = data.providers || [];
      setAvailableModels(providers.filter(p => p.type === "chat").map(p => ({ id: p.id, model: p.model, display_name: p.display_name, enabled: p.enabled })));
      const primary = providers.find(p => p.enabled);
      if (primary) {
        setCurrentModel(primary.model || primary.display_name || primary.id);
      }
    }).catch(() => {});
  }, []);

  // Load conversations from API
  const loadConversations = useCallback(async () => {
    try {
      const data = await api.conversations(conv.showArchived);
      convD({ type: "SET_LIST", list: data.sessions || [] });
    } catch { /* offline */ }
  }, [conv.showArchived]);

  useEffect(() => { loadConversations(); }, [loadConversations]);

  // Check if LLM setup is needed
  useEffect(() => {
    api.checkSetup().then((chk) => setSetupNeeded(chk.setup_needed)).catch(() => {});
  }, []);

  // Load presets
  useEffect(() => {
    api.getPresets().then((data) => {
      setPresets(data.presets || []);
      setActivePreset(data.active || "");
    }).catch(() => {});
  }, []);

  const showComputerRef = useRef(showComputer);
  showComputerRef.current = showComputer;

  const pushBrowserTrace = useCallback((event: AgentEvent) => {
    setBrowserTraceEvents((prev) => [...prev.slice(-7), event]);
    chatD({ type: "ADD_LIVE_TRACE", event });
    if (!showComputerRef.current) setShowComputer(true);
    setSuggestedTab("browser");
  }, []);

  const {
    bridgeState,
    bridgeActionPending,
    bridgeNotice,
    lastArtifact,
    sendBridgeAction,
    syncBridgeState,
    setBridgeNotice,
    setLastArtifact,
  } = useBrowserBridge({
    onActionStart: (type, extra) => {
      pushBrowserTrace(makeBrowserTraceEvent(`??????????${browserActionLabel(type)}`, { action: type, stage: "start", ...extra }, "tool_start"));
    },
    onActionSuccess: (action, result, successText) => {
      pushBrowserTrace(makeBrowserTraceEvent(successText, { action, result }, action === "bridge/takeover" ? "reflect" : "tool_result"));
    },
    onActionError: (action, payload, message) => {
      pushBrowserTrace(makeBrowserTraceEvent(action ? `??????????${browserActionLabel(action)}` : "???????", { action, payload, message }, "tool_result"));
      showToast(message, "error");
    },
  });

  // SSE for trace events 閳?use fetch-based SSE to avoid leaking credentials in URL
  useEffect(() => {
    let cancelled = false;
    const token = typeof window !== "undefined" ? localStorage.getItem("yunque_token") || "" : "";
    const key = typeof window !== "undefined" ? localStorage.getItem("yunque_api_key") || "" : "";
    const headers: Record<string, string> = {};
    if (token) headers["Authorization"] = `Bearer ${token}`;
    else if (key) headers["X-API-Key"] = key;

    (async () => {
      try {
        const res = await fetch(`/v1/events/stream`, { headers });
        if (!res.ok || !res.body) return;
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buf = "";
        while (!cancelled) {
          const { done, value } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          const lines = buf.split("\n");
          buf = lines.pop() || "";
          for (const line of lines) {
            if (line.startsWith("data: ")) {
              try {
                const evt: AgentEvent = JSON.parse(line.slice(6));
                chatD({ type: "ADD_LIVE_TRACE", event: evt });
                // Auto-open computer panel on tool activity
                const evtType = (evt.type || "").toLowerCase();
                if (!showComputerRef.current && (evtType === "tool_start" || evtType === "tool_result" || evtType === "thinking")) {
                  setShowComputer(true);
                }
              } catch { /* ignore parse */ }
            }
          }
        }
      } catch { /* SSE connection failed */ }
    })();
    return () => { cancelled = true; };
  }, []);

  // Load messages when switching conversations
  const switchConversation = useCallback(async (convId: string) => {
    convD({ type: "SET_ACTIVE", id: convId });
    chatD({ type: "CLEAR_LIVE_TRACE" });
    try {
      const data = await api.conversationMessages(convId);
      chatD({ type: "SET_MESSAGES", messages: (data.messages || []).map((m: { role: string; content: string }) => ({
        role: m.role as "user" | "assistant",
        content: m.content,
        id: newId(),
      })) });
    } catch { chatD({ type: "SET_MESSAGES", messages: [] }); }
  }, [pushBrowserTrace]);

  useEffect(() => {
    const el = inputRef.current;
    if (!el) return;
    el.style.height = "0px";
    el.style.height = `${Math.min(el.scrollHeight, 180)}px`;
  }, [chat.input]);

  // Manage conversation (rename/pin/archive)
  const manageConversation = useCallback(async (convId: string, opts: { name?: string; pinned?: boolean; archive?: boolean }) => {
    try {
      const result = await api.manageConversation(convId, opts);
      if (result.session) {
        convD({ type: "UPDATE_ONE", id: convId, data: result.session });
      } else {
        convD({ type: "CANCEL_RENAME" });
      }
    } catch (e) { showToast(e instanceof Error ? e.message : "Failed to update conversation.", "error"); }
  }, []);

  // TTS playback
  const playTTS = useCallback(async (mId: string, text: string) => {
    if (ttsPlaying === mId) {
      audioRef.current?.pause();
      setTtsPlaying(null);
      return;
    }
    try {
      setTtsPlaying(mId);
      const buf = await api.tts(text);
      const blob = new Blob([buf], { type: "audio/mp3" });
      const url = URL.createObjectURL(blob);
      if (audioRef.current) audioRef.current.pause();
      const audio = new Audio(url);
      audioRef.current = audio;
      audio.onended = () => { setTtsPlaying(null); URL.revokeObjectURL(url); };
      audio.onerror = () => { setTtsPlaying(null); URL.revokeObjectURL(url); };
      audio.play();
    } catch (e) { setTtsPlaying(null); showToast(e instanceof Error ? e.message : "Text-to-speech playback failed.", "error"); }
  }, [ttsPlaying]);

  // STT recording
  const speechRecRef = useRef<any>(null);

  const startRecording = useCallback(async () => {
    const SR = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
    if (SR) {
      const rec = new SR();
      rec.lang = "zh-CN";
      rec.interimResults = true;
      rec.continuous = true;
      let finalText = "";
      rec.onresult = (e: any) => {
        let interim = "";
        for (let i = e.resultIndex; i < e.results.length; i++) {
          if (e.results[i].isFinal) finalText += e.results[i][0].transcript;
          else interim += e.results[i][0].transcript;
        }
        chatD({ type: "SET_INPUT", value: finalText + interim });
      };
      rec.onerror = () => { setIsRecording(false); };
      rec.onend = () => { setIsRecording(false); inputRef.current?.focus(); };
      rec.start();
      speechRecRef.current = rec;
      setIsRecording(true);
      return;
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const recorder = new MediaRecorder(stream, { mimeType: "audio/webm" });
      const chunks: Blob[] = [];
      recorder.ondataavailable = (e) => { if (e.data.size > 0) chunks.push(e.data); };
      recorder.onstop = async () => {
        stream.getTracks().forEach(t => t.stop());
        const blob = new Blob(chunks, { type: "audio/webm" });
        setIsRecording(false);
        try {
          const result = await api.stt(blob);
          if (result.text) { chatD({ type: "SET_INPUT", value: chat.input + result.text }); inputRef.current?.focus(); }
        } catch { showToast("Speech transcription failed.", "error"); }
      };
      recorder.start();
      mediaRecorderRef.current = recorder;
      setIsRecording(true);
    } catch { showToast("Microphone access failed.", "error"); }
  }, []);

  const stopRecording = useCallback(() => {
    if (speechRecRef.current) { speechRecRef.current.stop(); speechRecRef.current = null; return; }
    mediaRecorderRef.current?.stop();
    mediaRecorderRef.current = null;
  }, []);

  // Switch preset
  const handleSwitchPreset = useCallback(async (presetId: string) => {
    try {
      await api.switchPreset(presetId);
      setActivePreset(presetId);
    } catch (e) { showToast(e instanceof Error ? e.message : "Failed to switch preset.", "error"); }
  }, []);

  // Delete conversation
  const deleteConversation = useCallback(async (convId: string) => {
    try {
      await api.deleteConversation(convId);
      convD({ type: "REMOVE", id: convId });
      if (conv.activeId === convId) { convD({ type: "SET_ACTIVE", id: "default" }); chatD({ type: "SET_MESSAGES", messages: [] }); }
      showToast("对话已删除。", "success");
    } catch (e) { showToast(e instanceof Error ? e.message : "删除对话失败。", "error"); }
  }, [conv.activeId]);

  const fileInputRef = useRef<HTMLInputElement>(null);

  const [pendingFiles, setPendingFiles] = useState<Array<{ name: string; size: number; preview?: string; base64?: string; type: "image" | "video" | "text" | "binary" }>>([]);
  const [isDragging, setIsDragging] = useState(false);

  const TEXT_EXTS = new Set(["txt","md","csv","json","yaml","yml","toml","xml","html","css","js","ts","tsx","jsx","py","go","rs","rb","java","c","cpp","h","sh","bash","sql","ini","cfg","env","log","gitignore","dockerfile"]);
  const isTextFile = (name: string) => {
    const ext = name.split(".").pop()?.toLowerCase() || "";
    return TEXT_EXTS.has(ext);
  };

  const processFile = useCallback((file: File) => {
    const isImage = file.type.startsWith("image/");
    const isVideo = file.type.startsWith("video/");
    const isText = isTextFile(file.name) || file.type.startsWith("text/");

    if (isImage || isVideo) {
      const previewUrl = URL.createObjectURL(file);
      const reader = new FileReader();
      reader.onload = () => {
        const base64 = reader.result as string;
        setPendingFiles(prev => [...prev, { name: file.name, size: file.size, preview: previewUrl, base64, type: isImage ? "image" : "video" }]);
      };
      reader.readAsDataURL(file);
    } else {
      setPendingFiles(prev => [...prev, { name: file.name, size: file.size, type: isText ? "text" : "binary" }]);
      api.uploadFile(file).then(res => {
        chatD({ type: "SET_INPUT", value: chat.input + (chat.input ? "\n" : "") + `Uploaded file: ${res.path}` });
      }).catch(() => {
        if (isText) {
          const reader = new FileReader();
          reader.onload = () => {
            const text = reader.result as string;
            chatD({ type: "SET_INPUT", value: chat.input + (chat.input ? "\n" : "") + `[File: ${file.name}]\n${text.slice(0, 4000)}` });
          };
          reader.readAsText(file);
        } else {
          const sizeStr = file.size > 1024 * 1024 ? `${(file.size / 1024 / 1024).toFixed(1)} MB` : `${(file.size / 1024).toFixed(1)} KB`;
          chatD({ type: "SET_INPUT", value: chat.input + (chat.input ? "\n" : "") + `[File: ${file.name} (${sizeStr})]` });
        }
      });
    }
  }, [chat.input]);

  const handleFileUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) processFile(file);
    e.target.value = "";
  }, [processFile]);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const files = Array.from(e.dataTransfer.files);
    files.forEach(processFile);
  }, [processFile]);

  const handleDragOver = useCallback((e: React.DragEvent) => { e.preventDefault(); setIsDragging(true); }, []);
  const handleDragLeave = useCallback((e: React.DragEvent) => { e.preventDefault(); setIsDragging(false); }, []);

  const retryMessage = useCallback((msgId: string) => {
    const idx = chat.messages.findIndex((m) => m.id === msgId);
    if (idx < 0) return;
    let userText = "";
    for (let i = idx; i >= 0; i--) {
      if (chat.messages[i].role === "user") { userText = chat.messages[i].content; break; }
    }
    if (!userText) return;
    chatD({ type: "REMOVE_MSG", id: msgId });
    sendMessage(userText);
  }, [chat.messages]);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [chat.messages]);

  const sendMessage = useCallback(async (overrideText?: string) => {
    const text = (overrideText || chat.input).trim();
    if (!text || chat.loading) return;

    const slashBrowserCommand = parseSlashBrowserCommand(text);
    if (slashBrowserCommand) {
      setSuggestedTab("browser");
      setShowComputer(true);
      setBridgeNotice({ tone: "info", text: `???????????${browserActionLabel(slashBrowserCommand.command)}?` });
      pushBrowserTrace(makeBrowserTraceEvent(
        `Slash ??????${browserActionLabel(slashBrowserCommand.command)}`,
        { command: slashBrowserCommand.command, args: slashBrowserCommand.args, summary: slashBrowserCommand.summary },
        "tool_start",
      ));
    }

    const mediaPreviews = pendingFiles.filter(f => (f.type === "image" || f.type === "video") && f.base64).map(f => f.base64!);
    const userMsg: Message = { role: "user", content: text, id: newId(), ...(mediaPreviews.length > 0 ? { images: mediaPreviews } : {}) };
    const asstMsg: Message = { role: "assistant", content: "", id: newId(), traceEvents: [] };
    chatD({ type: "START_SEND" });
    chatD({ type: "ADD_PAIR", userMsg, asstMsg });
    pendingFiles.forEach(f => { if (f.preview) URL.revokeObjectURL(f.preview); });
    setPendingFiles([]);
    // Computer panel auto-opens when tool events are received, not on every message

    const abort = new AbortController();
    abortRef.current = abort;

    try {
      const mediaFiles = pendingFiles.filter(f => (f.type === "image" || f.type === "video") && f.base64);
      const historyMsgs: Array<{ role: string; content: string | Array<{ type: string; text?: string; image_url?: { url: string }; video_url?: { url: string } }> }> =
        chat.messages.filter(m => m.role === "user" || m.role === "assistant")
          .slice(-20)
          .map(m => ({ role: m.role, content: m.content as string }));
      if (mediaFiles.length > 0) {
        const mediaParts = mediaFiles.map(f => {
          if (f.type === "video") return { type: "video_url" as const, video_url: { url: f.base64! } };
          return { type: "image_url" as const, image_url: { url: f.base64! } };
        });
        historyMsgs.push({
          role: "user",
          content: [{ type: "text", text }, ...mediaParts],
        });
      } else {
        historyMsgs.push({ role: "user", content: text });
      }
      const token = typeof window !== "undefined" ? localStorage.getItem("yunque_token") || "" : "";
      const apiKey = typeof window !== "undefined" ? localStorage.getItem("yunque_api_key") || "" : "";
      const authHeaders: Record<string, string> = token
        ? { Authorization: `Bearer ${token}` }
        : apiKey ? { "X-API-Key": apiKey } : {};
      const resp = await fetch("/v1/chat/agentic", {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders },
        body: JSON.stringify({ messages: historyMsgs, session_id: conv.activeId, ...(thinkingEnabled !== null ? { thinking: thinkingEnabled } : {}) }),
        signal: abort.signal,
      });
      if (!resp.ok || !resp.body) throw new Error("request failed");

      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      let currentEvent = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done || abort.signal.aborted) break;
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split("\n");
        buf = lines.pop() || "";

        for (const line of lines) {
          if (line.startsWith("event: ")) {
            currentEvent = line.slice(7).trim();
          } else if (line.startsWith("data: ")) {
            const raw = line.slice(6);
            if (raw === "[DONE]") break;

            if (currentEvent === "error") {
              try {
                const err = JSON.parse(raw);
                chatD({ type: "ERROR_LAST", error: friendlyError(err.message || raw) });
              } catch { /* ignore */ }
              continue;
            }

            if (currentEvent === "done") {
              try {
                const doneData = JSON.parse(raw);
                const updates: Partial<Message> = {};
                if (doneData.emotion) updates.emotion = doneData.emotion;
                if (doneData.sticker_suggestion) updates.sticker = doneData.sticker_suggestion;
                if (doneData.sticker_suggestions) updates.stickers = doneData.sticker_suggestions;
                if (doneData.skills_used) updates.skills_used = doneData.skills_used;
                if (doneData.actions?.length > 0) updates.actions = doneData.actions;
                if (doneData.suggestions?.length > 0) updates.suggestions = doneData.suggestions;
                if (doneData.reasoning_content) updates.reasoning = doneData.reasoning_content;
                if (doneData.browser_summary) {
                  updates.browserSummary = mapBrowserSummary(doneData.browser_summary);
                }
                chatD({ type: "UPDATE_LAST", updates });
                if (doneData.browser_summary) {
                  setLastArtifact(mapBrowserSummary(doneData.browser_summary));
                }
                if (slashBrowserCommand) {
                  syncBridgeState();
                  const used = Array.isArray(doneData.skills_used) ? doneData.skills_used : [];
                  const commandSummary = used.length > 0 ? `??????????${used.join(", ")}` : `??????????${browserActionLabel(slashBrowserCommand.command)}`;
                  pushBrowserTrace(makeBrowserTraceEvent(commandSummary, { command: slashBrowserCommand.command, done: doneData }, "tool_result"));
                }
              } catch { /* ignore */ }
              continue;
            }

            if (currentEvent === "actions") {
              try {
                const actions = JSON.parse(raw);
                chatD({ type: "UPDATE_LAST", updates: { actions: Array.isArray(actions) ? actions : actions.actions || [] } });
              } catch { /* ignore */ }
              continue;
            }

            if (currentEvent === "thinking") {
              try {
                const thinking = JSON.parse(raw);
                if (thinking.content) {
                  chatD({ type: "UPDATE_LAST", updates: { reasoning: thinking.content } });
                }
              } catch { /* ignore */ }
              continue;
            }

            try {
              const parsed = JSON.parse(raw);
              if (parsed.content || parsed.type === "delta") {
                chatD({ type: "APPEND_LAST", delta: parsed.content || "" });
              } else if (parsed.id && parsed.domain) {
                // Stream thinking delta: planner.thinking with detail.stream_type
                if (parsed.domain === "planner" && parsed.type === "thinking" && parsed.detail?.stream_type === "thinking_delta") {
                  chatD({ type: "APPEND_LAST_REASONING", delta: parsed.message || "" });
                } else if (parsed.domain === "planner" && parsed.type === "thinking" && parsed.detail?.stream_type === "reasoning_batch") {
                  chatD({ type: "APPEND_LAST_REASONING", delta: (parsed.summary || "") + "\n" });
                } else {
                  chatD({ type: "APPEND_LAST_TRACE", event: parsed as AgentEvent });
                  // Auto-open computer panel and switch tab on tool/thinking events
                  if (parsed.type === "tool_start" || parsed.type === "tool_end" || parsed.type === "thinking") {
                    if (!showComputer) setShowComputer(true);
                    const domain = parsed.domain || "";
                    const skillName = parsed.detail?.skill || parsed.summary || "";
                    if (domain === "browser" || /browser|screenshot|navigate/.test(skillName)) {
                      setSuggestedTab("browser");
                    } else if (/file_write|file_read|editor/.test(skillName)) {
                      setSuggestedTab("editor");
                    } else if (parsed.type === "thinking") {
                      setSuggestedTab("thinking");
                    } else if (/shell|command|terminal/.test(skillName)) {
                      setSuggestedTab("terminal");
                    }
                  }
                }
              }
            } catch {
              if (raw.trim()) chatD({ type: "APPEND_LAST", delta: raw });
            }
          } else if (line.trim() === "") {
            currentEvent = "";
          }
        }
      }
    } catch (e: unknown) {
      if ((e as Error).name === "AbortError") {
        chatD({ type: "APPEND_LAST", delta: "\n\n[Generation stopped]" });
      } else {
        chatD({ type: "ERROR_LAST", error: friendlyError((e as Error).message) });
      }
    } finally {
      chatD({ type: "FINISH_SEND" });
      abortRef.current = null;
      loadConversations();
    }
  }, [chat.input, chat.loading, chat.messages, thinkingLevel, conv.activeId, loadConversations, pushBrowserTrace, setBridgeNotice, setLastArtifact, syncBridgeState]);

  const stopGeneration = () => abortRef.current?.abort();

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (showSlashMenu) return; // slash menu handles its own keys
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); sendMessage(); }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value;
    chatD({ type: "SET_INPUT", value: val });
    const slashState = getSlashState(val);
    setShowSlashMenu(slashState.visible);
    setSlashQuery(slashState.query);
  };

  const handleSlashSelect = (commandText: string) => {
    chatD({ type: "SET_INPUT", value: commandText });
    setShowSlashMenu(false);
    setSlashQuery("");
    inputRef.current?.focus();
  };

  const handleCopy = (id: string, content: string) => {
    navigator.clipboard.writeText(content);
    setCopiedIdx(id);
    setTimeout(() => setCopiedIdx(null), 2000);
  };

  const handleAction = useCallback((action: AgentAction) => {
    if (action.action === "send_message" || action.action === "chat") {
      sendMessage(action.label);
    } else if (action.action === "create_task") {
      sendMessage(`Create a task for: ${action.label}`);
    } else {
      sendMessage(action.label);
    }
  }, [sendMessage]);

  // Filter & sort conversations
  const filteredConversations = useMemo(() => {
    let list = conv.list;
    if (conv.searchQuery) {
      const q = conv.searchQuery.toLowerCase();
      list = list.filter((c) => (c.name || c.id).toLowerCase().includes(q) || (c.summary || "").toLowerCase().includes(q));
    }
    return [...list].sort((a, b) => {
      if (a.pinned && !b.pinned) return -1;
      if (!a.pinned && b.pinned) return 1;
      return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
    });
  }, [conv.list, conv.searchQuery]);


  const thinkingOptions = [
    { key: "none" as const, label: "快速", icon: <Zap size={12} /> },
    { key: "auto" as const, label: "自动", icon: <Gauge size={12} /> },
    { key: "deep" as const, label: "深度", icon: <Brain size={12} /> },
  ] as const;

  return (
    <div className="flex h-screen overflow-hidden" style={{ background: "var(--yunque-bg)" }}>
      {/* Conversation Sidebar */}
      {showSidebar && (
        <div className="w-64 flex flex-col h-full shrink-0 animate-slide-in-left" style={{ background: "var(--yunque-sidebar)", borderRight: "1px solid var(--yunque-border)" }}>
          {/* Sidebar Header */}
          <div className="p-3 space-y-3">
            <div className="rounded-[24px] border px-3 py-3" style={{ background: "linear-gradient(180deg, rgba(59,130,246,0.12), rgba(59,130,246,0.04))", borderColor: "rgba(59,130,246,0.18)" }}>
              <div className="flex items-center gap-2">
                <div className="flex h-9 w-9 items-center justify-center rounded-2xl" style={{ background: "rgba(255,255,255,0.08)", color: "#dbeafe" }}>
                  <Sparkles size={16} />
                </div>
                <div className="min-w-0">
                  <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>会话工作台</div>
                  <div className="text-[11px]" style={{ color: "rgba(219,234,254,0.78)" }}>在一个工作区里对话、浏览、推理并执行任务。</div>
                </div>
              </div>
              <div className="mt-3 flex items-center gap-2 text-[11px]">
                <span className="rounded-full px-2.5 py-1" style={{ background: "rgba(255,255,255,0.08)", color: "var(--yunque-text-secondary)" }}>{filteredConversations.length} 个可见</span>
                <span className="rounded-full px-2.5 py-1" style={{ background: "rgba(255,255,255,0.08)", color: "var(--yunque-text-secondary)" }}>{conv.showArchived ? "归档视图" : "活跃线程"}</span>
              </div>
            </div>
            <Button
              className="w-full justify-start gap-2 rounded-lg text-sm btn-accent"
              size="sm"
              onPress={() => { convD({ type: "SET_ACTIVE", id: "new-" + Date.now() }); chatD({ type: "SET_MESSAGES", messages: [] }); }}
            >
              <Plus size={14} /> 新对话
            </Button>
            <div
              className="flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-xs"
              style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-muted)" }}
            >
              <Search size={12} />
              <input
                placeholder="搜索对话…"
                value={conv.searchQuery}
                onChange={(e) => convD({ type: "SET_SEARCH", query: e.target.value })}
                className="bg-transparent outline-none text-xs flex-1"
                style={{ color: "var(--yunque-text)" }}
              />
            </div>
          </div>

          {/* Archive toggle */}
          <div className="px-3 pb-2 flex gap-1">
            <button
              onClick={() => convD({ type: "SET_ARCHIVED", show: false })}
              className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-[11px] transition-colors flex-1 justify-center"
              style={{
                color: !conv.showArchived ? "var(--yunque-accent)" : "var(--yunque-text-muted)",
                background: !conv.showArchived ? "rgba(0,111,238,0.1)" : "rgba(255,255,255,0.03)",
              }}
            >
              <MessageCircle size={13} /> 活跃
            </button>
            <button
              onClick={() => convD({ type: "SET_ARCHIVED", show: true })}
              className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-[11px] transition-colors flex-1 justify-center"
              style={{
                color: conv.showArchived ? "var(--yunque-accent)" : "var(--yunque-text-muted)",
                background: conv.showArchived ? "rgba(0,111,238,0.1)" : "rgba(255,255,255,0.03)",
              }}
            >
              <Archive size={13} /> 归档
            </button>
          </div>

          {/* Conversation List */}
          <div className="flex-1 overflow-y-auto px-2 custom-scrollbar">
            <div className="text-[10px] font-semibold uppercase tracking-widest px-2 py-2" style={{ color: "var(--yunque-text-muted)" }}>
              {conv.showArchived ? "归档对话" : "最近对话"} ({filteredConversations.length})
            </div>
            <div className="space-y-0.5">
              {filteredConversations.map((c) => (
                <div
                  key={c.id}
                  onClick={() => { if (conv.renameId !== c.id) switchConversation(c.id); }}
                  className="conv-item w-full text-left px-3 py-3 rounded-[20px] group relative"
                  data-active={conv.activeId === c.id || undefined}
                  style={{ color: conv.activeId === c.id ? "var(--yunque-accent)" : "var(--yunque-text-secondary)" }}
                >
                  {conv.activeId === c.id && (
                    <div className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-4 rounded-r-full" style={{ background: "var(--yunque-accent)" }} />
                  )}
                  {c.pinned && (
                    <Pin size={10} className="absolute right-2 top-2" style={{ color: "var(--yunque-accent)", opacity: 0.6 }} />
                  )}
                  {conv.renameId === c.id ? (
                    <input
                      autoFocus
                      value={conv.renameText}
                      onChange={(e) => convD({ type: "SET_RENAME_TEXT", text: e.target.value })}
                      onBlur={() => { if (conv.renameText.trim()) manageConversation(c.id, { name: conv.renameText.trim() }); convD({ type: "CANCEL_RENAME" }); }}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") { if (conv.renameText.trim()) manageConversation(c.id, { name: conv.renameText.trim() }); convD({ type: "CANCEL_RENAME" }); }
                        if (e.key === "Escape") convD({ type: "CANCEL_RENAME" });
                      }}
                      className="text-xs font-medium bg-transparent outline-none w-full px-1 py-0.5 rounded"
                      style={{ color: "var(--yunque-text)", background: "rgba(255,255,255,0.08)", border: "1px solid var(--yunque-accent)" }}
                      onClick={(e) => e.stopPropagation()}
                    />
                  ) : (
                    <div className="text-xs font-medium truncate pr-4">{c.name || c.id}</div>
                  )}
                  <div className="text-[11px] truncate mt-1" style={{ color: "var(--yunque-text-muted)" }}>{c.summary || "暂无摘要"}</div>
                  <div className="flex items-center justify-between mt-1">
                    <div className="flex items-center gap-2">
                      <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{new Date(c.updated_at).toLocaleDateString()}</span>
                      {c.pinned && (
                        <span className="rounded-full px-2 py-0.5 text-[10px]" style={{ background: "rgba(59,130,246,0.1)", color: "var(--yunque-accent)" }}>
                          置顶
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                      <Button isIconOnly aria-label="重命名对话" variant="ghost" size="sm"
                        onPress={() => convD({ type: "START_RENAME", id: c.id, text: c.name || c.id })}
                      >
                        <Edit3 size={11} style={{ color: "var(--yunque-text-muted)" }} />
                      </Button>
                      <Button isIconOnly aria-label="置顶对话" variant="ghost" size="sm"
                        onPress={() => manageConversation(c.id, { pinned: !c.pinned })}
                      >
                        {c.pinned ? <PinOff size={11} style={{ color: "var(--yunque-accent)" }} /> : <Pin size={11} style={{ color: "var(--yunque-text-muted)" }} />}
                      </Button>
                      <Button isIconOnly aria-label={conv.showArchived ? "恢复对话" : "归档对话"} variant="ghost" size="sm"
                        onPress={() => manageConversation(c.id, { archive: !conv.showArchived })}
                      >
                        {conv.showArchived ? <ArchiveRestore size={11} style={{ color: "var(--yunque-text-muted)" }} /> : <Archive size={11} style={{ color: "var(--yunque-text-muted)" }} />}
                      </Button>
                      <Button isIconOnly aria-label="Delete conversation" variant="ghost" size="sm"
                        onPress={() => deleteConversation(c.id)}
                      >
                        <Trash2 size={11} style={{ color: "#ef4444" }} />
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
              {filteredConversations.length === 0 && (
                <div className="text-center py-8 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  {conv.searchQuery ? "没有匹配的对话。" : conv.showArchived ? "暂时没有归档对话。" : "还没有对话，开始新建一个吧。"}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Main Chat Area */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Top Bar */}
        <header className="flex items-center justify-between px-5 py-3 shrink-0" style={{ borderBottom: "1px solid var(--yunque-border)", background: "linear-gradient(180deg, rgba(255,255,255,0.02), rgba(255,255,255,0))" }}>
          <div className="flex items-center gap-3">
            <button
              onClick={() => setShowSidebar(!showSidebar)}
              className="p-1.5 rounded-lg transition-colors"
              style={{ color: "var(--yunque-text-muted)" }}
            >
              <MessageCircle size={16} />
            </button>

            <div className="hidden md:flex items-center gap-2">
              <div className="inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-[11px]" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-secondary)" }}>
                <Sparkles size={12} />
                <span>工作台模式</span>
              </div>
            </div>

            {/* Model Selector */}
            {currentModel && (
              availableModels.length > 1 ? (
                <Dropdown>
                  <Button variant="ghost" size="sm" className="gap-1 text-xs font-mono rounded-lg"
                    style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-secondary)" }}>
                    <Cpu size={12} />
                    {currentModel.length > 25 ? `${currentModel.slice(0, 25)}…` : currentModel}
                    <ChevronDown size={10} style={{ color: "var(--yunque-text-muted)" }} />
                  </Button>
                  <Dropdown.Popover className="min-w-[200px]">
                    <Dropdown.Menu onAction={(key) => {
                      const target = availableModels.find(m => m.id === key);
                      if (target) {
                        setCurrentModel(target.model || target.display_name || target.id);
                        api.providerSessionOverride(target.id).catch(() => {});
                      }
                    }}>
                      {availableModels.filter(m => m.enabled).map(m => (
                        <Dropdown.Item key={m.id} id={m.id} textValue={m.model || m.id}>
                          <Label>{m.display_name || m.model || m.id}</Label>
                        </Dropdown.Item>
                      ))}
                    </Dropdown.Menu>
                  </Dropdown.Popover>
                </Dropdown>
              ) : (
                <Chip size="sm" variant="soft" className="text-xs font-mono">
                  {currentModel}
                </Chip>
              )
            )}
          </div>

          <div className="flex items-center gap-2">
            {/* Preset selector */}
            {presets.length > 0 && (
              <Dropdown>
                <Button
                  variant="ghost"
                  size="sm"
                  className="gap-1.5 text-xs font-medium rounded-lg"
                  style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-secondary)" }}
                >
                  <Heart size={12} />
                  {presets.find(p => p.id === activePreset)?.name || "Preset"}
                  <ChevronDown size={12} style={{ color: "var(--yunque-text-muted)" }} />
                </Button>
                <Dropdown.Popover className="min-w-[160px]">
                  <Dropdown.Menu onAction={(key) => handleSwitchPreset(key as string)}>
                    {presets.map((p) => (
                      <Dropdown.Item key={p.id} id={p.id} textValue={p.name}>
                        <Label>{p.name}{p.id === activePreset ? "（当前）" : ""}</Label>
                      </Dropdown.Item>
                    ))}
                  </Dropdown.Menu>
                </Dropdown.Popover>
              </Dropdown>
            )}

            {chat.streaming && (
              <Chip className="animate-pulse-dot" size="sm" style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)" }}>
                <Sparkles size={11} className="mr-1" /> 流式生成中
              </Chip>
            )}

            {/* Computer panel toggle */}
            <Tooltip delay={0}>
              <Button
                isIconOnly variant="ghost" size="sm"
                onPress={() => setShowComputer(!showComputer)}
                style={{ color: showComputer ? "var(--yunque-accent)" : "var(--yunque-text-muted)" }}
              >
                <Monitor size={15} />
              </Button>
              <Tooltip.Content>{showComputer ? "隐藏计算机面板" : "显示计算机面板"}</Tooltip.Content>
            </Tooltip>

            {/* Thinking level pills */}
            <div className="flex items-center gap-0.5 p-0.5 rounded-lg" style={{ background: "rgba(255,255,255,0.04)" }}>
              {thinkingOptions.map(({ key, label, icon }) => (
                <button
                  key={key}
                  onClick={() => {
                    setThinkingLevel(key);
                    setThinkingEnabled(key === "deep" ? true : key === "none" ? false : null);
                  }}
                  className="flex items-center gap-1 px-2 py-1 rounded-md text-[11px] font-medium transition-all"
                  style={{
                    background: thinkingLevel === key ? "var(--yunque-accent)" : "transparent",
                    color: thinkingLevel === key ? "#fff" : "var(--yunque-text-muted)",
                  }}
                >
                  {icon} {label}
                </button>
              ))}
            </div>
          </div>
        </header>

        {/* Chat Messages */}
        <div ref={scrollRef} className="flex-1 overflow-y-auto px-6 py-4 custom-scrollbar">
          {chat.messages.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full gap-6 animate-fade-in-up">
              {setupNeeded && (
                <div className="w-full max-w-md p-4 rounded-xl border-l-4" style={{ background: "rgba(245,158,11,0.06)", borderColor: "var(--yunque-border)", borderLeftColor: "#f59e0b" }}>
                  <div className="flex items-center gap-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                    <AlertTriangle size={16} style={{ color: "#f59e0b" }} /> 先完成模型配置
                  </div>
                  <p className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                    请先在设置中添加模型提供商 API Key，再开始第一轮对话。
                  </p>
                  <a href="/settings" className="inline-flex items-center gap-1 text-xs mt-2 font-medium" style={{ color: "#f59e0b" }}>前往设置 →</a>
                </div>
              )}
              <div className="w-12 h-12 rounded-2xl flex items-center justify-center chat-hero-icon" style={{ background: "rgba(0,111,238,0.1)" }}>
                <Sparkles size={24} style={{ color: "var(--yunque-accent)" }} />
              </div>
              <div className="text-center space-y-1.5 max-w-xl">
                <h1 className="text-[28px] font-bold tracking-tight" style={{ color: "var(--yunque-text)" }}>从这里开始一轮真正可执行的工作</h1>
                <p className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>发起研究、浏览网页、调用连接器、生成代码，或把需求沉淀成任务。</p>
              </div>

              <div className="w-full max-w-lg grid grid-cols-2 gap-2 mt-2">
                {[
                  { icon: <BookOpen size={14} />, label: "总结文档 / 需求", desc: "贴入文档、需求或笔记，让 Agent 先帮你提炼重点。" },
                  { icon: <Search size={14} />, label: "研究一个主题", desc: "发起研究流程，整理来源、结论和下一步建议。" },
                  { icon: <Brain size={14} />, label: "规划多步骤任务", desc: "先拆解步骤，再执行，减少长任务中的混乱。" },
                  { icon: <Zap size={14} />, label: "编写或修复代码", desc: "结合代码上下文、工具与连接器完成开发任务。" },
                ].map(({ icon, label, desc }) => (
                  <button
                    key={label}
                    onClick={() => { chatD({ type: "SET_INPUT", value: label }); inputRef.current?.focus(); }}
                    className="flex items-start gap-3 p-3 rounded-xl text-left transition-all duration-200 hover-lift"
                    style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}
                  >
                    <span className="mt-0.5 shrink-0" style={{ color: "var(--yunque-accent)" }}>{icon}</span>
                    <div className="min-w-0">
                      <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{label}</div>
                      <div className="text-[11px] mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>{desc}</div>
                    </div>
                  </button>
                ))}
              </div>
            </div>
          ) : (
            <div className="max-w-3xl mx-auto space-y-5">
              {chat.messages.map((msg, idx) => (
                <div key={msg.id} className={`group flex gap-3 animate-fade-in-up ${msg.role === "user" ? "justify-end" : ""}`}>
                  {msg.role === "assistant" && (
                    <Avatar size="sm" className="shrink-0 mt-1" style={{ background: "var(--yunque-accent)" }}>
                      <Avatar.Fallback className="text-white text-xs font-bold">Y</Avatar.Fallback>
                    </Avatar>
                  )}
                  <div className={`max-w-[78%] ${msg.role === "user" ? "flex flex-col items-end" : ""}`}>
                    {/* Step summary (compact, replaces full trace in chat) */}
                    {msg.role === "assistant" && msg.traceEvents && msg.traceEvents.length > 0 && (() => {
                      const isLive = chat.streaming && msg.id === chat.messages[chat.messages.length - 1]?.id;
                      const toolEvents = msg.traceEvents.filter(e => e.type === "tool_start" || e.type === "tool_result");
                      const warnEvents = msg.traceEvents.filter((e) => {
                        const summary = (e.summary || "").toLowerCase();
                        return e.type === "plan" && (
                          summary.includes("warning") ||
                          summary.includes("risk") ||
                          summary.includes("blocked") ||
                          summary.includes("needs review")
                        );
                      });
                      return (
                        <>
                          <div className="mb-2 rounded-[18px] border px-3 py-3" style={{ background: "rgba(255,255,255,0.025)", borderColor: "rgba(255,255,255,0.06)" }}>
                            <div className="mb-2 flex flex-wrap items-center gap-2">
                              <span className="rounded-full px-2.5 py-1 text-[10px]" style={{ background: "rgba(59,130,246,0.12)", color: "#93c5fd" }}>
                                {isLive ? "运行中" : "已完成"}
                              </span>
                              {(() => {
                                const summary = summarizeAssistantWork(msg);
                                return (
                                  <>
                                    {summary.primarySkill && (
                                      <span className="rounded-full px-2.5 py-1 text-[10px]" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-secondary)" }}>
                                        {summary.primarySkill}
                                      </span>
                                    )}
                                    {summary.toolCount > 0 && (
                                      <span className="rounded-full px-2.5 py-1 text-[10px]" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-secondary)" }}>
                                        {summary.toolCount} tool events
                                      </span>
                                    )}
                                    {summary.fileCount > 0 && (
                                      <span className="rounded-full px-2.5 py-1 text-[10px]" style={{ background: "rgba(34,197,94,0.1)", color: "#4ade80" }}>
                                        {summary.fileCount} files
                                      </span>
                                    )}
                                  </>
                                );
                              })()}
                            </div>
                            <div className="text-xs leading-6" style={{ color: "var(--yunque-text-muted)" }}>
                              {isLive ? "The agent is still executing this request. Watch the live trace and computer panel for progress." : "This response includes structured execution steps, generated changes, and follow-up actions."}
                            </div>
                          </div>
                          {warnEvents.length > 0 && (
                            <div className="mb-2 rounded-lg px-3 py-1.5 text-[11px]"
                              style={{ background: "rgba(245,158,11,0.08)", color: "#f59e0b", border: "1px solid rgba(245,158,11,0.15)" }}>
                              {warnEvents.map((w, wi) => <div key={wi}>{w.summary}</div>)}
                            </div>
                          )}
                          {toolEvents.length > 0 && (() => {
                            const uniqueSkills = [...new Set(toolEvents.map(e => e.meta?.skill).filter(Boolean))];
                            const lastSkill = toolEvents[toolEvents.length - 1]?.meta?.skill || "";
                            return (
                              <div className="mb-2 flex items-center gap-2 text-[11px] rounded-lg px-3 py-1.5"
                                style={{ background: "rgba(59,130,246,0.06)", color: "var(--yunque-text-muted)" }}>
                                {isLive && <span className="w-1.5 h-1.5 rounded-full animate-pulse" style={{ background: "var(--yunque-accent)" }} />}
                                <span>
                                  {isLive ? `Working with ${lastSkill || "tools"}…` : `Used ${uniqueSkills.length} tools`}
                                  {uniqueSkills.length > 0 && !isLive && (
                                    <span style={{ color: "var(--yunque-text-muted)", marginLeft: 4 }}>
                                      ({uniqueSkills.slice(0, 3).join(", ")}{uniqueSkills.length > 3 ? "…" : ""})
                                    </span>
                                  )}
                                </span>
                              </div>
                            );
                          })()}
                        </>
                      );
                    })()}
                    <div
                      className={`px-4 py-3 rounded-[22px] text-sm leading-relaxed whitespace-pre-wrap ${msg.role === "assistant" ? "assistant-message-shell" : ""}`}
                      style={{
                        background: msg.role === "user"
                          ? "linear-gradient(180deg, rgba(59,130,246,0.96), rgba(37,99,235,0.92))"
                          : "linear-gradient(180deg, rgba(255,255,255,0.035), rgba(255,255,255,0.015)), var(--yunque-card)",
                        color: msg.role === "user" ? "#fff" : "var(--yunque-text)",
                        border: msg.role === "assistant" ? "1px solid rgba(255,255,255,0.06)" : "1px solid rgba(59,130,246,0.16)",
                        borderBottomRightRadius: msg.role === "user" ? "6px" : undefined,
                        borderBottomLeftRadius: msg.role === "assistant" ? "6px" : undefined,
                        boxShadow: msg.role === "assistant" ? "0 14px 34px rgba(0,0,0,0.18)" : "0 14px 28px rgba(37,99,235,0.18)",
                      }}
                    >
                      {msg.role === "user" && msg.images && msg.images.length > 0 && (
                        <div className="flex gap-2 flex-wrap mb-2">
                          {msg.images.map((src, i) => (
                            <img key={i} src={src} alt="" className="max-w-[200px] max-h-[200px] rounded-lg object-cover cursor-pointer hover:opacity-90 transition-opacity"
                              onClick={() => window.open(src, "_blank")} />
                          ))}
                        </div>
                      )}
                      {msg.role === "assistant" && msg.reasoning && (
                        <details className="mb-2" open={chat.streaming && idx === chat.messages.length - 1} style={{ fontSize: "var(--text-sm)" }}>
                          <summary style={{ cursor: "pointer", color: "var(--yunque-text-muted)", fontStyle: "italic", display: "flex", alignItems: "center", gap: 4 }}>
                            <span style={{ fontSize: "var(--text-xs)", background: "rgba(245,158,11,0.12)", color: "#f59e0b", padding: "1px 6px", borderRadius: 4 }}>
                              {chat.streaming && idx === chat.messages.length - 1 ? "推理中…" : "推理过程"}
                            </span>
                            <span style={{ color: "var(--yunque-text-muted)", fontSize: "var(--text-xs)" }}>（{msg.reasoning.length} 字符）</span>
                          </summary>
                          <div style={{ marginTop: 6, padding: "8px 12px", borderRadius: 8, background: "rgba(245,158,11,0.04)", border: "1px solid rgba(245,158,11,0.12)", whiteSpace: "pre-wrap", color: "var(--yunque-text-secondary)", fontSize: "var(--text-xs)", maxHeight: 300, overflow: "auto" }}>
                            {msg.reasoning}
                          </div>
                        </details>
                      )}
                      {msg.content ? (
                        msg.role === "assistant" ? (
                          <MarkdownRenderer content={msg.content} />
                        ) : (
                          msg.content.replace(/\[(Uploaded file|File):\s*[^\]]+\]\s*/g, "").trim() || (msg.images?.length ? null : msg.content)
                        )
                      ) : (
                        !msg.images?.length && (
                          <div className="flex items-center gap-2">
                            <Spinner size="sm" color="current" /> Thinking…
                          </div>
                        )
                      )}
                    </div>
                    {/* Emotion badge + Sticker */}
                    {msg.role === "assistant" && (msg.emotion || msg.sticker || msg.stickers) && (
                      <div className="flex items-center gap-2 mt-1.5 flex-wrap">
                        {msg.emotion && <EmotionBadge emotion={msg.emotion} />}
                        {msg.sticker && <StickerView sticker={msg.sticker} />}
                        {msg.stickers && Object.values(msg.stickers).map((s, i) => <StickerView key={i} sticker={s} />)}
                      </div>
                    )}
                    {/* Skill tags */}
                    {msg.role === "assistant" && msg.skills_used && msg.skills_used.length > 0 && (
                      <SkillTags skills={msg.skills_used} />
                    )}
                    {/* Agent action buttons */}
                    {msg.role === "assistant" && msg.actions && msg.actions.length > 0 && (
                      <div className="mt-3 rounded-[20px] border p-3" style={{ background: "rgba(255,255,255,0.02)", borderColor: "rgba(255,255,255,0.06)" }}>
                        <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>
                          Suggested actions
                        </div>
                        <AgentActions actions={msg.actions} onAction={handleAction} />
                      </div>
                    )}
                    {msg.role === "assistant" && msg.browserSummary && (
                      <div className="mt-3 rounded-[20px] border p-3" style={{ background: "rgba(255,255,255,0.02)", borderColor: "rgba(255,255,255,0.06)" }}>
                        <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>
                          Browser artifact
                        </div>
                        <div className="flex flex-wrap items-center gap-2 text-[11px]" style={{ color: "var(--yunque-text-secondary)" }}>
                          {msg.browserSummary.action && (
                            <span className="rounded-full px-2.5 py-1" style={{ background: "rgba(59,130,246,0.12)", color: "#93c5fd" }}>
                              {browserActionLabel(msg.browserSummary.action)}
                            </span>
                          )}
                          {typeof msg.browserSummary.elementCount === "number" && (
                            <span className="rounded-full px-2.5 py-1" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
                              {msg.browserSummary.elementCount} elements
                            </span>
                          )}
                          {msg.browserSummary.hasScreenshot && (
                            <span className="rounded-full px-2.5 py-1" style={{ background: "rgba(34,197,94,0.1)", color: "#86efac" }}>
                              screenshot ready
                            </span>
                          )}
                          {typeof msg.browserSummary.textLength === "number" && msg.browserSummary.textLength > 0 && (
                            <span className="rounded-full px-2.5 py-1" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
                              {msg.browserSummary.textLength} chars
                            </span>
                          )}
                        </div>
                        {(msg.browserSummary.title || msg.browserSummary.url) && (
                          <div className="mt-2 min-w-0">
                            {msg.browserSummary.title && (
                              <div className="truncate text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                                {msg.browserSummary.title}
                              </div>
                            )}
                            {msg.browserSummary.url && (
                              <div className="mt-1 truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                                {msg.browserSummary.url}
                              </div>
                            )}
                          </div>
                        )}
                        {msg.browserSummary.preview && (
                          <div className="mt-2 rounded-2xl px-3 py-2 text-xs leading-6" style={{ background: "rgba(15,23,42,0.35)", color: "var(--yunque-text-secondary)" }}>
                            {msg.browserSummary.preview}
                          </div>
                        )}
                        {(msg.browserSummary.suggestedCommand || msg.browserSummary.url) && (
                          <div className="mt-3 flex flex-wrap items-center gap-2">
                            {msg.browserSummary.suggestedCommand && (
                              <Button
                                size="sm"
                                variant="ghost"
                                className="rounded-full px-3"
                                onPress={() => handleSlashSelect(msg.browserSummary?.suggestedCommand || "/")}
                              >
                                {msg.browserSummary.suggestedLabel || "Use next command"}
                              </Button>
                            )}
                            {msg.browserSummary.url && (
                              <Button
                                size="sm"
                                variant="ghost"
                                className="rounded-full px-3"
                                onPress={() => window.open(msg.browserSummary?.url, "_blank", "noopener,noreferrer")}
                              >
                                Open page
                              </Button>
                            )}
                          </div>
                        )}
                      </div>
                    )}
                    {/* Generated file downloads */}
                    {msg.role === "assistant" && msg.traceEvents && (() => {
                      const files = collectGeneratedFiles(msg.traceEvents);
                      if (files.length === 0) return null;
                      return (
                        <div className="mt-3 rounded-[20px] border p-3" style={{ background: "rgba(255,255,255,0.02)", borderColor: "rgba(255,255,255,0.06)" }}>
                          <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>
                            Generated files
                          </div>
                          <div className="space-y-2">
                          {files.map((f, i) => {
                            const ext = (f.name || f.path).split(".").pop()?.toLowerCase() || "";
                            const isDoc = ["pdf", "docx", "xlsx", "pptx", "doc", "xls", "ppt"].includes(ext);
                            return (
                              <a key={i} href={`/api/files/download?path=${encodeURIComponent(f.path)}`} download={f.name || f.path}
                                className="flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-medium transition-all hover:scale-[1.01]"
                                style={{
                                  background: isDoc ? "rgba(59,130,246,0.12)" : "rgba(255,255,255,0.06)",
                                  border: "1px solid rgba(59,130,246,0.2)",
                                  color: "#93c5fd",
                                }}>
                                <div className="w-10 h-10 rounded-lg flex items-center justify-center shrink-0"
                                  style={{ background: isDoc ? "rgba(59,130,246,0.2)" : "rgba(255,255,255,0.08)" }}>
                                  <Paperclip size={18} />
                                </div>
                                <div className="flex-1 min-w-0">
                                  <div className="truncate font-semibold">{f.name || f.path.split("/").pop() || f.path}</div>
                                  <div className="text-[11px] mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>
                                    {ext.toUpperCase()} {f.size != null && f.size > 0 ? `  ${f.size > 1024 * 1024 ? `${(f.size / 1024 / 1024).toFixed(1)} MB` : `${(f.size / 1024).toFixed(1)} KB`}` : ""}
                                  </div>
                                </div>
                                <div className="w-8 h-8 rounded-full flex items-center justify-center shrink-0"
                                  style={{ background: "rgba(59,130,246,0.15)" }}>
                                  <span style={{ color: "#60a5fa", fontSize: 16 }}>↗</span>
                                </div>
                              </a>
                            );
                          })}
                          </div>
                        </div>
                      );
                    })()}
                    {/* Follow-up suggestions */}
                    {msg.role === "assistant" && msg.suggestions && msg.suggestions.length > 0 && !chat.streaming && (
                      <div className="mt-3 rounded-[20px] border p-3" style={{ background: "rgba(255,255,255,0.02)", borderColor: "rgba(255,255,255,0.06)" }}>
                        <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>
                          Next moves
                        </div>
                        <div className="flex flex-wrap gap-2">
                        {msg.suggestions.map((s, i) => (
                          <button key={i} onClick={() => {
                            if (s.type === "save_skill") {
                              sendMessage("Turn this workflow into a reusable skill and save it for later.");
                            } else {
                              sendMessage(s.label);
                            }
                          }}
                            className="px-3 py-1.5 rounded-full text-xs font-medium transition-all hover:scale-105 cursor-pointer"
                            style={{
                              background: s.type === "save_skill" ? "rgba(139,92,246,0.12)" : "rgba(59,130,246,0.08)",
                              border: `1px solid ${s.type === "save_skill" ? "rgba(139,92,246,0.3)" : "rgba(59,130,246,0.15)"}`,
                              color: s.type === "save_skill" ? "#a78bfa" : "#93c5fd",
                            }}>
                            {s.type === "save_skill" ? "Save " : "→ "}{s.label}
                          </button>
                        ))}
                        </div>
                      </div>
                    )}
                    {msg.role === "assistant" && msg.traceEvents && msg.traceEvents.length > 0 && (
                      <div className="mt-3">
                        <ExecutionTrace events={msg.traceEvents} isLive={chat.streaming && idx === chat.messages.length - 1} />
                      </div>
                    )}
                    {msg.content && (
                      <div className="hidden group-hover:flex gap-0.5 mt-1 animate-fade-in" style={{ justifyContent: msg.role === "user" ? "flex-end" : "flex-start" }}>
                        {msg.role === "user" && (
                          <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm"><Pencil size={11} /></Button><Tooltip.Content>编辑</Tooltip.Content></Tooltip>
                        )}
                        {msg.role === "assistant" && (
                          <>
                            <Tooltip delay={0}>
                              <Button isIconOnly variant="ghost" size="sm" onPress={() => handleCopy(msg.id, msg.content)}>
                                {copiedIdx === msg.id ? <Check size={11} className="text-green-400" /> : <Copy size={11} />}
                              </Button>
                              <Tooltip.Content>{copiedIdx === msg.id ? "已复制" : "复制"}</Tooltip.Content>
                            </Tooltip>
                            <Tooltip delay={0}>
                              <Button isIconOnly variant="ghost" size="sm" onPress={() => playTTS(msg.id, msg.content)}>
                                {ttsPlaying === msg.id ? <VolumeX size={11} style={{ color: "var(--yunque-accent)" }} /> : <Volume2 size={11} />}
                              </Button>
                              <Tooltip.Content>{ttsPlaying === msg.id ? "停止播放" : "播放语音"}</Tooltip.Content>
                            </Tooltip>
                          </>
                        )}
                        <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={() => retryMessage(msg.id)}><RotateCcw size={11} /></Button><Tooltip.Content>重新发送</Tooltip.Content></Tooltip>
                      </div>
                    )}
                  </div>
                  {msg.role === "user" && (
                    <Avatar size="sm" className="shrink-0 mt-1" style={{ background: "#374151" }}>
                      <Avatar.Fallback className="text-white text-xs">U</Avatar.Fallback>
                    </Avatar>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Input Area */}
        <div className="px-6 py-3 shrink-0" style={{ borderTop: chat.messages.length > 0 ? "1px solid var(--yunque-border)" : "none" }}
          onDrop={handleDrop} onDragOver={handleDragOver} onDragLeave={handleDragLeave}>
          <div className="max-w-3xl mx-auto">
            <div
              ref={inputShellRef}
              className="chat-input-wrap chat-composer rounded-[26px] overflow-visible transition-all"
              style={{
                background: "linear-gradient(180deg, rgba(255,255,255,0.028), rgba(255,255,255,0.01)), var(--yunque-card)",
                border: isDragging ? "1px dashed var(--yunque-accent)" : "1px solid var(--yunque-border)",
                boxShadow: isDragging
                  ? "0 0 0 1px rgba(59,130,246,0.22), 0 14px 36px rgba(15,23,42,0.32)"
                  : "0 14px 42px rgba(0,0,0,0.22), inset 0 1px 0 rgba(255,255,255,0.03)",
              }}
            >
              <div className="flex items-center justify-between gap-3 border-b px-4 py-2.5" style={{ borderColor: "rgba(255,255,255,0.05)" }}>
                <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                  Tori 负责执行，云雀负责工作台与上下文组织。
                </div>
                <div className="hidden items-center gap-2 md:flex">
                  <Button size="sm" variant="ghost" className="rounded-full px-3" onPress={() => setShowConnectors(true)}>
                    <Plug size={13} />
                    连接器
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="rounded-full px-3"
                    onPress={() => { chatD({ type: "SET_INPUT", value: "/" }); setShowSlashMenu(true); setSlashQuery(""); inputRef.current?.focus(); }}
                  >
                    <Sparkles size={13} />
                    命令
                  </Button>
                </div>
              </div>

              <div className="px-4 pt-3">
                <BrowserSessionCard
                  state={bridgeState}
                  pendingAction={bridgeActionPending}
                  notice={bridgeNotice}
                  artifact={lastArtifact}
                  traceEvents={browserTraceEvents}
                  onAction={sendBridgeAction}
                  onOpenBrowserPage={() => window.open("/browser", "_blank", "noopener,noreferrer")}
                  onSuggestCommand={handleSlashSelect}
                />
              </div>

              {pendingFiles.length > 0 && (
                <div className="flex gap-2 px-5 pt-4 flex-wrap">
                  {pendingFiles.map((f, i) => (
                    <div key={i} className="relative group/file flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-xs"
                      style={{ background: "rgba(255,255,255,0.04)", border: "1px solid var(--yunque-border)" }}>
                      {f.type === "image" && f.preview ? (
                        <img src={f.preview} alt={f.name} className="w-8 h-8 rounded object-cover" />
                      ) : f.type === "video" && f.preview ? (
                        <video src={f.preview} className="w-8 h-8 rounded object-cover" muted />
                      ) : (
                        <Paperclip size={12} style={{ color: "var(--yunque-text-muted)" }} />
                      )}
                      <span className="truncate max-w-[120px]" style={{ color: "var(--yunque-text-secondary)" }}>{f.name}</span>
                      <button
                        onClick={() => { if (f.preview) URL.revokeObjectURL(f.preview); setPendingFiles(prev => prev.filter((_, j) => j !== i)); }}
                        className="ml-1 w-4 h-4 rounded-full flex items-center justify-center text-[10px] opacity-0 group-hover/file:opacity-100 transition-opacity shrink-0"
                        style={{ background: "rgba(239,68,68,0.9)", color: "#fff" }}
                      >?</button>
                    </div>
                  ))}
                </div>
              )}

              <div className="relative px-4 pt-3">
                <SlashCommandMenu
                  query={slashQuery}
                  visible={showSlashMenu}
                  onSelect={handleSlashSelect}
                  onClose={() => setShowSlashMenu(false)}
                  anchorRef={inputShellRef}
                />
                {showSlashMenu && (
                  <div className="pointer-events-none absolute left-5 top-0 rounded-full px-2.5 py-1 text-[10px]" style={{ background: "rgba(59,130,246,0.1)", color: "var(--yunque-accent)" }}>
                    命令模式
                  </div>
                )}
              </div>

              <textarea
                ref={inputRef}
                value={chat.input}
                onChange={handleInputChange}
                onKeyDown={handleKeyDown}
                placeholder="输入消息，或输入 / 使用命令、连接器与内置技能…"
                rows={1}
                className="chat-composer-textarea w-full resize-none px-5 pt-3 pb-2 text-[15px] outline-none bg-transparent"
                style={{ color: "var(--yunque-text)", minHeight: 72, maxHeight: 180, lineHeight: 1.7 }}
                disabled={chat.loading}
              />

              <div className="flex flex-wrap items-center justify-between gap-3 px-4 pb-4 pt-2">
                <div className="flex items-center gap-1">
                  <input type="file" ref={fileInputRef} className="hidden" onChange={handleFileUpload} />
                  <Tooltip delay={0}>
                    <Button isIconOnly variant="ghost" size="sm" onPress={() => fileInputRef.current?.click()}><Paperclip size={14} /></Button>
                    <Tooltip.Content>添加文件</Tooltip.Content>
                  </Tooltip>
                  <Tooltip delay={0}>
                    <Button isIconOnly variant="ghost" size="sm" onPress={() => { if (fileInputRef.current) { fileInputRef.current.accept = "image/*"; fileInputRef.current.click(); } }}><ImageIcon size={14} /></Button>
                    <Tooltip.Content>添加图片</Tooltip.Content>
                  </Tooltip>
                  <Tooltip delay={0}>
                    <Button
                      isIconOnly
                      variant="ghost"
                      size="sm"
                      onPress={isRecording ? stopRecording : startRecording}
                      style={isRecording ? { color: "#ef4444" } : {}}
                    >
                      {isRecording ? <StopCircle size={14} className="animate-pulse" /> : <Mic size={14} />}
                    </Button>
                    <Tooltip.Content>{isRecording ? "停止录音" : "语音输入"}</Tooltip.Content>
                  </Tooltip>
                  <div className="relative">
                    <Tooltip delay={0}>
                      <Button isIconOnly variant="ghost" size="sm" onPress={() => setShowConnectors(!showConnectors)}>
                        <Plug size={14} />
                      </Button>
                      <Tooltip.Content>连接器</Tooltip.Content>
                    </Tooltip>
                    <ConnectorPopover
                      open={showConnectors}
                      onClose={() => setShowConnectors(false)}
                    />
                  </div>
                </div>
                <div className="flex items-center gap-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                  <span className="hidden md:inline">Enter 发送</span>
                  <span className="hidden md:inline">·</span>
                  <span className="hidden md:inline">Shift + Enter 换行</span>
                  <span className="hidden md:inline">·</span>
                  <span>/ 打开命令</span>
                </div>
                {chat.loading ? (
                  <Button
                    isIconOnly aria-label="停止生成" size="sm" className="rounded-2xl"
                    style={{ background: "rgba(239,68,68,0.12)", color: "#ef4444" }}
                    onPress={stopGeneration}
                  >
                    <StopCircle size={14} />
                  </Button>
                ) : (
                  <Button
                    isIconOnly aria-label="发送" size="sm"
                    className={`h-11 w-11 rounded-2xl ${chat.input.trim() ? "chat-send-active" : ""}`}
                    isDisabled={!chat.input.trim()}
                    style={{
                      background: chat.input.trim() ? "var(--yunque-accent)" : "rgba(255,255,255,0.06)",
                      color: chat.input.trim() ? "#fff" : "var(--yunque-text-muted)",
                    }}
                    onPress={() => sendMessage()}
                  >
                    <Send size={14} />
                  </Button>
                )}
              </div>
              {!chat.loading && (
                <div className="border-t px-5 py-3" style={{ borderColor: "rgba(255,255,255,0.05)" }}>
                  <div className="flex flex-wrap items-center gap-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                    <span className="rounded-full px-2.5 py-1" style={{ background: "rgba(255,255,255,0.04)" }}>
                      {chat.messages.length === 0 ? "等待新的任务" : `当前线程 ${chat.messages.length} 条消息`}
                    </span>
                    <span className="rounded-full px-2.5 py-1" style={{ background: "rgba(255,255,255,0.04)" }}>
                      {showComputer ? "计算机面板已展开" : "计算机面板已隐藏"}
                    </span>
                    {pendingFiles.length > 0 && (
                      <span className="rounded-full px-2.5 py-1" style={{ background: "rgba(34,197,94,0.1)", color: "#4ade80" }}>
                        {pendingFiles.length} 个附件待发送
                      </span>
                    )}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Computer Panel (right side) */}
      {showComputer && (
        <>
          <div
            className="w-1 shrink-0 cursor-col-resize hover:bg-blue-500/30 active:bg-blue-500/50 transition-colors"
            style={{ background: "var(--yunque-border)" }}
            onMouseDown={(e) => {
              e.preventDefault();
              resizingRef.current = true;
              const startX = e.clientX;
              const startW = computerWidth;
              const onMove = (ev: MouseEvent) => {
                if (!resizingRef.current) return;
                const delta = startX - ev.clientX;
                setComputerWidth(Math.max(300, Math.min(800, startW + delta)));
              };
              const onUp = () => { resizingRef.current = false; document.removeEventListener("mousemove", onMove); document.removeEventListener("mouseup", onUp); };
              document.addEventListener("mousemove", onMove);
              document.addEventListener("mouseup", onUp);
            }}
          />
          <div className="shrink-0 flex flex-col h-full animate-slide-in-right"
            style={{ width: computerWidth, background: "var(--yunque-sidebar)" }}>
            <ComputerPanel traceEvents={chat.liveTraceEvents} isLive onClose={() => setShowComputer(false)} suggestedTab={suggestedTab} />
          </div>
        </>
      )}
    </div>
  );
}
