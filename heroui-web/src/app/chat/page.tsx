"use client";

import { useState, useReducer, useRef, useCallback, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import { Button, Avatar, Spinner, Tooltip, Chip, Dropdown, Label, Popover } from "@heroui/react";
import {
  Send, Plus, MessageCircle, Zap, BookOpen, ScanFace, Package,
  Brain, Gauge, Mic, StopCircle, Pencil, RotateCcw, Copy, Undo2,
  Sparkles, Check, Search, Library, ChevronDown, Cpu,
  Paperclip, ImageIcon, Trash2, Volume2, Pin, Archive,
  PanelRightOpen, PanelRightClose, VolumeX, ArchiveRestore, Edit3, Heart,
  PinOff, MoreHorizontal, Monitor, AlertTriangle, Plug,
  ArrowRight, Blocks,
} from "lucide-react";
import { api, type ConversationInfo, type EmotionResult, type StickerSuggestion, type PresetInfo, type SkillInfo } from "@/lib/api";
import type { SkillSuggestion as SkillGrowthSuggestion } from "@/lib/api-types";
import MarkdownRenderer from "@/components/markdown-renderer";
import { ExecutionTrace, type AgentEvent } from "@/components/execution-trace";
import { ComputerPanel } from "@/components/computer-panel";
import { TaskProgressPanel } from "@/components/task-progress-panel";
import { ConnectorPopover } from "@/components/connector-popover";
import { BrowserSessionCard, type BrowserActionArtifactSummary, type BrowserBridgeState, type BrowserSessionNotice } from "@/components/browser-session-card";
import { BrowserConnectCard, type BrowserRequirement } from "@/components/browser-connect-card";
import { SkillGrowthPanel } from "@/components/skill-growth-panel";
import { SlashCommandMenu } from "@/components/slash-command-menu";
import { EmotionBadge, StickerView, SkillTags, AgentActions, type AgentAction } from "@/components/chat-extras";
import { showToast } from "@/components/toast-provider";
import { ModelSelectorPopup, type ModelOption } from "@/components/model-selector-popup";
import { useBrowserBridge } from "@/lib/use-browser-bridge";
import { openExternal } from "@/lib/safe-url";
import { browserActionLabel, browserActionPhase } from "@/lib/browser-action-labels";
import type { Suggestion, SandboxInfo, Message } from "@/lib/chat-types";
import {
  newId,
  browserTraceSummary,
  makeBrowserTraceEvent,
  friendlyError,
  collectGeneratedFiles,
  summarizeAssistantWork,
} from "@/lib/chat-utils";
import {
  getSlashState,
  getActiveSlashCommand,
  mapBrowserSummary,
  parseSlashBrowserCommand,
  buildSlashBrowserAction,
  summarizeSlashBrowserResult,
  formatSlashBrowserResponse,
} from "@/lib/slash-commands";
import { ThinkingTimer } from "@/components/chat/thinking-timer";
import { chatReducer, chatInit } from "@/lib/chat-state";
import { convReducer, convInit } from "@/lib/conversation-state";

export default function ChatPage() {
  const router = useRouter();
  const [chat, chatD] = useReducer(chatReducer, chatInit);
  const [conv, convD] = useReducer(convReducer, convInit);
  const [thinkingLevel, setThinkingLevel] = useState<"none" | "auto" | "deep">("auto");
  const [copiedIdx, setCopiedIdx] = useState<string | null>(null);
  const [showSidebar, setShowSidebar] = useState(() => {
    if (typeof window === "undefined") return true;
    return window.innerWidth >= 1024;
  });
  const [showComputer, setShowComputer] = useState(false);
  const [showConnectors, setShowConnectors] = useState(false);
  const [showSlashMenu, setShowSlashMenu] = useState(false);
  const [slashQuery, setSlashQuery] = useState("");
  const [activeSlashCommand, setActiveSlashCommand] = useState<string | null>(null);
  const [thinkingEnabled, setThinkingEnabled] = useState<boolean | null>(null);
  const [chatMode, setChatMode] = useState<"agent" | "fast" | "chat">("agent");
  const [airiMode, setAiriMode] = useState(false);
  const [airiAvailable, setAiriAvailable] = useState(false);
  const [suggestedTab, setSuggestedTab] = useState<"terminal" | "browser" | "editor" | "thinking" | undefined>(undefined);
  const [computerWidth, setComputerWidth] = useState(420);
  const resizingRef = useRef(false);
  const [isNarrowViewport, setIsNarrowViewport] = useState(false);

  useEffect(() => {
    const mq = window.matchMedia("(max-width: 1280px)");
    setIsNarrowViewport(mq.matches);
    const handler = (e: MediaQueryListEvent) => setIsNarrowViewport(e.matches);
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, []);

  // Narrow viewports can't comfortably fit sidebar + chat + computer panel at
  // once. When the computer panel opens on a ≤1280px screen, auto-collapse the
  // conversation sidebar so the chat area keeps breathing room.
  useEffect(() => {
    if (isNarrowViewport && showComputer) setShowSidebar(false);
  }, [isNarrowViewport, showComputer]);

  const [currentModel, setCurrentModel] = useState("");
  const [currentModelId, setCurrentModelId] = useState("");
  const [availableModels, setAvailableModels] = useState<ModelOption[]>([]);
  const [ttsPlaying, setTtsPlaying] = useState<string | null>(null);
  const [isRecording, setIsRecording] = useState(false);
  const [presets, setPresets] = useState<PresetInfo[]>([]);
  const [activePreset, setActivePreset] = useState("");
  const [setupNeeded, setSetupNeeded] = useState(false);
  const [browserTraceEvents, setBrowserTraceEvents] = useState<AgentEvent[]>([]);
  const [resumePromptForBrowser, setResumePromptForBrowser] = useState<string | null>(null);
  const [browserResumePending, setBrowserResumePending] = useState(false);
  const [heroSkills, setHeroSkills] = useState<SkillInfo[]>([]);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const inputShellRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);

  useEffect(() => {
    api.skills().then((res) => setHeroSkills((res.skills || []).slice(0, 4))).catch(() => {});
  }, []);

  useEffect(() => {
    const t = typeof window !== "undefined" ? localStorage.getItem("yunque_token") || "" : "";
    const k = typeof window !== "undefined" ? localStorage.getItem("yunque_api_key") || "" : "";
    const ah: Record<string, string> = t ? { Authorization: `Bearer ${t}` } : k ? { "X-API-Key": k } : {};
    fetch("/v1/plugins/ui", { headers: ah }).then(r => r.json()).then((data: any) => {
      const tabs = data?.tabs || data || [];
      if (Array.isArray(tabs) && tabs.some((t: any) => t.key === "airi")) {
        fetch("/v1/ext/airi/status", { headers: ah }).then(r => r.json()).then(() => {
          setAiriAvailable(true);
        }).catch(() => {});
      }
    }).catch(() => {});
  }, []);

  // Load providers for model selector
  useEffect(() => {
    api.providerList().then((data) => {
      const providers = data.providers || [];
      setAvailableModels(providers.filter(p => p.type === "chat").map(p => ({
        id: p.id, model: p.model, display_name: p.display_name, enabled: p.enabled,
        type: p.id.split("-")[0] || p.id,
        tier: p.tier, capabilities: p.capabilities,
      })));
      const primary = providers.find(p => p.enabled);
      if (primary) {
        setCurrentModel(primary.model || primary.display_name || primary.id);
        setCurrentModelId(primary.id);
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

  // Restore active conversation messages on mount
  const restoredRef = useRef(false);
  useEffect(() => {
    if (restoredRef.current) return;
    restoredRef.current = true;
    if (conv.activeId && conv.activeId !== "default") {
      switchConversation(conv.activeId);
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    api.checkSetup().then((chk) => {
      setSetupNeeded(chk.setup_needed);
    }).catch(() => {});
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
      pushBrowserTrace(makeBrowserTraceEvent(browserTraceSummary(type, "start"), { action: type, stage: "start", ...extra }, "tool_start"));
    },
    onActionSuccess: (action, result, successText) => {
      pushBrowserTrace(makeBrowserTraceEvent(action === "bridge/takeover" ? browserTraceSummary(action, "handoff") : browserTraceSummary(action, "success"), { action, result, successText }, action === "bridge/takeover" ? "reflect" : "tool_result"));
    },
    onActionError: (action, payload, message) => {
      pushBrowserTrace(makeBrowserTraceEvent(action ? browserTraceSummary(action, "error") : "Browser action failed", { action, payload, message }, "tool_result"));
      showToast(message, "error");
    },
  });

  useEffect(() => {
    if (!bridgeState?.connected || !resumePromptForBrowser) return;
    setBridgeNotice({
      tone: "success",
      text: "Browser connector is ready. You can continue the blocked task now.",
    });
  }, [bridgeState?.connected, resumePromptForBrowser, setBridgeNotice]);

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
                const evtType = (evt.type || "").toLowerCase();
                if (!showComputerRef.current && (evtType === "tool_start" || evtType === "tool_result" || evtType === "thinking" || evtType === "handoff_start")) {
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
      const sandboxMarkerRe = /\n?<!-- yunque:sandbox:(.*?) -->/;
      chatD({ type: "SET_MESSAGES", messages: (data.messages || []).map((m: { role: string; content: string }) => {
        let content = m.content;
        let sandbox: SandboxInfo | undefined;
        const match = content.match(sandboxMarkerRe);
        if (match) {
          try { sandbox = JSON.parse(match[1]) as SandboxInfo; } catch { /* skip */ }
          content = content.replace(sandboxMarkerRe, "");
        }
        return { role: m.role as "user" | "assistant", content, id: newId(), sandbox };
      }) });
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

  const newConversation = useCallback(() => {
    convD({ type: "SET_ACTIVE", id: "new-" + Date.now() });
    chatD({ type: "SET_MESSAGES", messages: [] });
    chatD({ type: "CLEAR_LIVE_TRACE" });
  }, []);

  // Delete conversation
  const deleteConversation = useCallback(async (convId: string) => {
    try {
      await api.deleteConversation(convId);
      convD({ type: "REMOVE", id: convId });
      if (conv.activeId === convId) { convD({ type: "SET_ACTIVE", id: "default" }); chatD({ type: "SET_MESSAGES", messages: [] }); chatD({ type: "CLEAR_LIVE_TRACE" }); }
      showToast("对话已删除。", "success");
    } catch (e) { showToast(e instanceof Error ? e.message : "删除对话失败。", "error"); }
  }, [conv.activeId]);

  const fileInputRef = useRef<HTMLInputElement>(null);

  type PendingFile = {
    id: string;
    name: string;
    size: number;
    preview?: string;
    base64?: string;
    type: "image" | "video" | "text" | "binary";
    status?: "ready" | "uploading" | "parsed" | "error";
    note?: string;
  };

  const [pendingFiles, setPendingFiles] = useState<PendingFile[]>([]);
  const [isDragging, setIsDragging] = useState(false);

  const TEXT_EXTS = new Set(["txt","md","csv","json","yaml","yml","toml","xml","html","css","js","ts","tsx","jsx","py","go","rs","rb","java","c","cpp","h","sh","bash","sql","ini","cfg","env","log","gitignore","dockerfile"]);
  const isTextFile = (name: string) => {
    const ext = name.split(".").pop()?.toLowerCase() || "";
    return TEXT_EXTS.has(ext);
  };

  const processFile = useCallback((file: File) => {
    const fileId = `${file.name}-${file.size}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const isImage = file.type.startsWith("image/");
    const isVideo = file.type.startsWith("video/");
    const isText = isTextFile(file.name) || file.type.startsWith("text/");

    if (isImage || isVideo) {
      const previewUrl = URL.createObjectURL(file);
      const reader = new FileReader();
      reader.onload = () => {
        const base64 = reader.result as string;
        setPendingFiles(prev => [...prev, { id: fileId, name: file.name, size: file.size, preview: previewUrl, base64, type: isImage ? "image" : "video", status: "ready", note: isImage ? "Image ready" : "Video ready" }]);
      };
      reader.readAsDataURL(file);
    } else {
      setPendingFiles(prev => [...prev, { id: fileId, name: file.name, size: file.size, type: isText ? "text" : "binary", status: "uploading", note: "Uploading to workspace..." }]);
      api.uploadFile(file).then(res => {
        const parsePreview = typeof res.parse?.preview === "string" ? res.parse.preview.trim() : "";
        const uploadLine = parsePreview
          ? [`[Parsed document: ${file.name}]`, `Workspace path: ${res.path}`, "", parsePreview].join("\n")
          : `Uploaded file: ${res.path}`;
        chatD({ type: "SET_INPUT", value: chat.input + (chat.input ? "\n" : "") + uploadLine });
        setPendingFiles(prev => prev.map(item => item.id === fileId ? {
          ...item,
          status: res.parse?.parser === "mineru" ? "parsed" : "ready",
          note: res.parse?.parser === "mineru" ? "Parsed by MinerU" : `Saved to ${res.path}`,
        } : item));
        if (res.parse?.parser === "mineru") {
          showToast(`Parsed ${file.name} with MinerU.`, "success");
        }
      }).catch(() => {
        setPendingFiles(prev => prev.map(item => item.id === fileId ? { ...item, status: "error", note: "Upload failed, using local fallback" } : item));
        if (isText) {
          const reader = new FileReader();
          reader.onload = () => {
            const text = reader.result as string;
            chatD({ type: "SET_INPUT", value: chat.input + (chat.input ? "\n" : "") + `[File: ${file.name}]
${text.slice(0, 4000)}` });
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

  const editMessage = useCallback((msgId: string) => {
    const msg = chat.messages.find((m) => m.id === msgId);
    if (!msg || msg.role !== "user") return;
    chatD({ type: "SET_INPUT", value: msg.content });
    const msgIdx = chat.messages.findIndex((m) => m.id === msgId);
    const toRemove = chat.messages.slice(msgIdx).map((m) => m.id);
    toRemove.forEach((id) => chatD({ type: "REMOVE_MSG", id }));
    inputRef.current?.focus();
  }, [chat.messages]);

  const rollbackToMessage = useCallback((msgId: string) => {
    const msgIdx = chat.messages.findIndex((m) => m.id === msgId);
    if (msgIdx < 0) return;
    const toRemove = chat.messages.slice(msgIdx + 1).map((m) => m.id);
    toRemove.forEach((id) => chatD({ type: "REMOVE_MSG", id }));
    showToast("已回滚到该消息", "success");
  }, [chat.messages]);

  const pendingRetryRef = useRef<string | null>(null);
  const retryMessage = useCallback((msgId: string) => {
    const idx = chat.messages.findIndex((m) => m.id === msgId);
    if (idx < 0) return;
    let userText = "";
    for (let i = idx; i >= 0; i--) {
      if (chat.messages[i].role === "user") { userText = chat.messages[i].content; break; }
    }
    if (!userText) return;
    const toRemove = chat.messages.slice(idx).map((m) => m.id);
    toRemove.forEach((id) => chatD({ type: "REMOVE_MSG", id }));
    pendingRetryRef.current = userText;
  }, [chat.messages]);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [chat.messages]);

  const sendMessage = useCallback(async (
    overrideText?: string,
    cherryOpts?: {
      attachments?: Array<{ name: string; mime: string; dataB64: string }>;
    },
  ) => {
    const text = (overrideText || chat.input).trim();
    if (!text || chat.loading) return;
    if (setupNeeded) {
      showToast("请先在设置中配置模型提供商 API Key", "warning");
      router.push("/settings/providers");
      return;
    }
    const slashBrowserCommand = parseSlashBrowserCommand(text);
    if (slashBrowserCommand) {
      setSuggestedTab("browser");
      setShowComputer(true);
      const extStatus = await api.browserExtStatus().catch(() => ({ connected: false }));
      if (!extStatus.connected) {
        setShowConnectors(true);
        setResumePromptForBrowser(text);
        setBridgeNotice({ tone: "warning", text: "Browser extension not connected. Opened install guide for you." });
        pushBrowserTrace(makeBrowserTraceEvent(
          "Browser extension required",
          { command: slashBrowserCommand.command, args: slashBrowserCommand.args, summary: slashBrowserCommand.summary },
          "reflect",
        ));
        const userMsg: Message = { role: "user", content: text, id: newId() };
        const asstMsg: Message = {
          role: "assistant",
          content: [
            "The browser extension is not connected yet.",
            "I opened the browser install guide for you. Connect **Yunque Browser Connector**, then run this command again.",
            "",
            "Open the workspace here: [/browser](/browser)",
          ].join("\n"),
          id: newId(),
          browserRequirement: {
            required: true,
            reason: "browser_connector_required",
            message: "This command needs the live Yunque Browser Connector before it can operate your real browser tab.",
            install_path: "/browser",
            settings_path: "/browser",
          },
          traceEvents: [makeBrowserTraceEvent("Opened browser install guide", { source: "chat-slash", command: slashBrowserCommand.command }, "reflect")],
        };
        chatD({ type: "SET_INPUT", value: "" });
        chatD({ type: "ADD_PAIR", userMsg, asstMsg });
        setActiveSlashCommand(null);
        setShowSlashMenu(false);
        if (typeof window !== "undefined") {
          window.setTimeout(() => window.open("/browser", "_blank", "noopener,noreferrer"), 80);
        }
        return;
      }

      const builtAction = buildSlashBrowserAction(slashBrowserCommand);
      if ("error" in builtAction) {
        const errorMessage = builtAction.error || "Browser command needs clarification.";
        const userMsg: Message = { role: "user", content: text, id: newId() };
        const asstMsg: Message = {
          role: "assistant",
          content: errorMessage,
          id: newId(),
          traceEvents: [makeBrowserTraceEvent("Browser command needs clarification", { command: slashBrowserCommand.command, args: slashBrowserCommand.args }, "reflect")],
        };
        chatD({ type: "SET_INPUT", value: "" });
        chatD({ type: "ADD_PAIR", userMsg, asstMsg });
        setActiveSlashCommand(null);
        setShowSlashMenu(false);
        return;
      }

      const userMsg: Message = { role: "user", content: text, id: newId() };
      const asstMsg: Message = { role: "assistant", content: "", id: newId(), traceEvents: [] };
      setActiveSlashCommand(null);
      setShowSlashMenu(false);
      chatD({ type: "START_SEND" });
      chatD({ type: "ADD_PAIR", userMsg, asstMsg });
      pushBrowserTrace(makeBrowserTraceEvent(
        browserTraceSummary(slashBrowserCommand.command, "start"),
        { command: slashBrowserCommand.command, args: slashBrowserCommand.args, action: builtAction.action },
        "tool_start",
      ));

      try {
        const result = await api.browserExtAction(builtAction.action);
        if (!result?.ok) {
          throw new Error(result?.error || "Browser action failed.");
        }
        const artifact = summarizeSlashBrowserResult(String(builtAction.action.type), result);
        const content = formatSlashBrowserResponse(slashBrowserCommand, artifact, result);
        chatD({ type: "UPDATE_LAST", updates: { content, browserSummary: artifact } });
        setResumePromptForBrowser(null);
        setLastArtifact(artifact);
        setBridgeNotice({ tone: "success", text: browserTraceSummary(slashBrowserCommand.command, "success") });
        pushBrowserTrace(makeBrowserTraceEvent(
          browserTraceSummary(slashBrowserCommand.command, "success"),
          { command: slashBrowserCommand.command, args: slashBrowserCommand.args, result },
          "tool_result",
        ));
        syncBridgeState();
      } catch (e: unknown) {
        const message = friendlyError((e as Error).message || "Browser action failed.");
        chatD({ type: "ERROR_LAST", error: message });
        setBridgeNotice({ tone: "error", text: message });
        pushBrowserTrace(makeBrowserTraceEvent(
          browserTraceSummary(slashBrowserCommand.command, "error"),
          { command: slashBrowserCommand.command, args: slashBrowserCommand.args, error: message },
          "reflect",
        ));
      } finally {
        chatD({ type: "FINISH_SEND" });
      }
      return;
    }

    const mediaPreviews = pendingFiles.filter(f => (f.type === "image" || f.type === "video") && f.base64).map(f => f.base64!);
    const userMsg: Message = { role: "user", content: text, id: newId(), ...(mediaPreviews.length > 0 ? { images: mediaPreviews } : {}) };
    const asstMsg: Message = { role: "assistant", content: "", id: newId(), traceEvents: [] };
    setActiveSlashCommand(null);
    setShowSlashMenu(false);
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
      // Cherry overlays persist their toggles in localStorage; in simple
      // mode we honour those here so the user's drawer picks flow into the
      // same /v1/chat/agentic request without extra plumbing. Attachments
      // come via `cherryOpts` because they are strictly in-memory.
      const cherryWebSearch = (() => {
        if (typeof window === "undefined") return false;
        return localStorage.getItem("yunque_web_search_enabled") === "1";
      })();
      const cherryToolIds = (() => {
        if (typeof window === "undefined") return undefined;
        try {
          const raw = localStorage.getItem("yunque_tools_selected");
          if (!raw) return undefined;
          const parsed = JSON.parse(raw) as unknown;
          if (Array.isArray(parsed) && parsed.every((x) => typeof x === "string")) {
            return parsed.length > 0 ? (parsed as string[]) : undefined;
          }
        } catch { /* corrupted storage; ignore */ }
        return undefined;
      })();

      const bodyObj: Record<string, unknown> = {
        messages: historyMsgs,
        session_id: conv.activeId,
        ...(thinkingEnabled !== null ? { thinking: thinkingEnabled } : {}),
        ...(chatMode !== "agent" ? { mode: chatMode } : {}),
        ...(airiMode ? { airi_mode: true } : {}),
      };
      if (cherryWebSearch) bodyObj.web_search = true;
      if (cherryToolIds) bodyObj.tool_ids = cherryToolIds;
      if (cherryOpts?.attachments && cherryOpts.attachments.length > 0) {
        bodyObj.attachments = cherryOpts.attachments.map((a) => ({
          name: a.name,
          mime: a.mime,
          data_b64: a.dataB64,
        }));
      }

      const resp = await fetch("/v1/chat/agentic", {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders },
        body: JSON.stringify(bodyObj),
        signal: abort.signal,
      });
      if (!resp.ok || !resp.body) throw new Error("request failed");

      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      let currentEvent = "";
      let streamFinished = false;
      const IDLE_TIMEOUT = 180_000;

      while (!streamFinished) {
        let timerId: ReturnType<typeof setTimeout> | null = null;
        const timeout = new Promise<{ done: true; value: undefined }>((resolve) => {
          timerId = setTimeout(() => resolve({ done: true, value: undefined }), IDLE_TIMEOUT);
        });
        const { done, value } = await Promise.race([reader.read(), timeout]);
        if (timerId) clearTimeout(timerId);
        if (done || abort.signal.aborted) {
          if (!abort.signal.aborted) {
            try { await reader.cancel(); } catch { /* stream already closed */ }
          }
          break;
        }
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split("\n");
        buf = lines.pop() || "";

        for (const line of lines) {
          if (line.startsWith("event: ")) {
            currentEvent = line.slice(7).trim();
          } else if (line.startsWith("data: ")) {
            const raw = line.slice(6);
            if (raw === "[DONE]") { streamFinished = true; break; }

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
                if (doneData.context_layers?.length > 0) updates.contextLayers = doneData.context_layers;
                if (doneData.reasoning_content) updates.reasoning = doneData.reasoning_content;
                if (doneData.browser_summary) {
                  updates.browserSummary = mapBrowserSummary(doneData.browser_summary);
                  setResumePromptForBrowser(null);
                }
                if (doneData.browser_requirement) {
                  setResumePromptForBrowser(text);
                  updates.browserRequirement = doneData.browser_requirement;
                }
                if (doneData.sandbox && doneData.sandbox.sandbox_id) {
                  updates.sandbox = doneData.sandbox as SandboxInfo;
                }
                if (doneData.airi_synced) {
                  updates.airiSynced = true;
                  const actMatch = (doneData.reply || "").match(/<\|ACT\s+(\{[^|]*\})\s*\|>/);
                  if (actMatch) {
                    try {
                      const act = JSON.parse(actMatch[1]);
                      updates.airiEmotion = act?.emotion?.name || "neutral";
                    } catch { updates.airiEmotion = "neutral"; }
                  }
                  const cleaned = (doneData.reply || "").replace(/<\|ACT\s+\{[^|]*\}\s*\|>\n?/g, "").trim();
                  if (cleaned) updates.content = cleaned;
                }
                chatD({ type: "UPDATE_LAST", updates });
                if (doneData.browser_summary) {
                  setLastArtifact(mapBrowserSummary(doneData.browser_summary));
                }
                if (doneData.browser_requirement) {
                  setShowComputer(true);
                  setSuggestedTab("browser");
                }
              }               catch { /* ignore */ }
              streamFinished = true;
              break;
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
      if (conv.activeId) {
        setTimeout(async () => {
          try {
            const res = await api.skillSuggestions(conv.activeId);
            if (res.suggestions?.length > 0) {
              const skillSugs = res.suggestions.map((s) => ({
                type: "save_skill" as const,
                label: `${s.name} · ${s.description} · ${s.confidence}/10`,
              }));
              chatD({ type: "UPDATE_LAST", updates: { suggestions: skillSugs, skillSuggestions: res.suggestions } });
            }
          } catch { /* ignore */ }
        }, 3000);
      }
    }
  }, [chat.input, chat.loading, chat.messages, thinkingLevel, conv.activeId, loadConversations, pushBrowserTrace, setBridgeNotice, setLastArtifact, syncBridgeState]);

  useEffect(() => {
    if (pendingRetryRef.current && !chat.loading) {
      const text = pendingRetryRef.current;
      pendingRetryRef.current = null;
      sendMessage(text);
    }
  }, [chat.messages, chat.loading, sendMessage]);

  const continueBlockedBrowserTask = useCallback(async (promptOverride?: string | null) => {
    const nextPrompt = promptOverride || resumePromptForBrowser;
    if (!nextPrompt || browserResumePending) return;
    setBrowserResumePending(true);
    try {
      await sendMessage(nextPrompt);
      setResumePromptForBrowser(null);
      setBridgeNotice({ tone: "success", text: "Resumed the browser task." });
    } finally {
      setBrowserResumePending(false);
    }
  }, [browserResumePending, resumePromptForBrowser, sendMessage, setBridgeNotice]);

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
    setActiveSlashCommand(getActiveSlashCommand(val));
  };

  const handleSlashSelect = (commandText: string) => {
    chatD({ type: "SET_INPUT", value: commandText });
    setShowSlashMenu(false);
    setSlashQuery("");
    setActiveSlashCommand(getActiveSlashCommand(commandText));
    requestAnimationFrame(() => {
      inputRef.current?.focus();
      const len = commandText.length;
      inputRef.current?.setSelectionRange(len, len);
    });
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
      {showSidebar && (
        <div
          className="flex flex-col h-full animate-slide-in-left w-[228px] xl:w-[244px] shrink-0"
          style={{ background: "var(--yunque-sidebar)", borderRight: "1px solid var(--yunque-border)", transition: "width 0.2s ease" }}
        >
          {/* Sidebar Header */}
          <div className="p-2.5 space-y-2">
            <div className="flex items-center justify-between px-1 pt-0.5">
              <span className="text-xs font-semibold" style={{ color: "var(--yunque-text-muted)" }}>
                {conv.showArchived ? "归档" : "对话"} · {filteredConversations.length}
              </span>
            </div>
            <Button
              className="w-full justify-start gap-2 rounded-[14px] text-[13px] btn-accent"
              size="sm"
              onPress={newConversation}
            >
              <Plus size={14} /> 新对话
            </Button>
            <div
              className="flex items-center gap-2 rounded-[14px] px-2.5 py-1.5 text-[11px]"
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
          <div className="px-2.5 pb-2 flex gap-1">
            <button
              onClick={() => convD({ type: "SET_ARCHIVED", show: false })}
              className="flex items-center gap-1.5 rounded-[12px] px-2 py-1.5 text-[10px] transition-colors flex-1 justify-center"
              style={{
                color: !conv.showArchived ? "var(--yunque-accent)" : "var(--yunque-text-muted)",
                background: !conv.showArchived ? "rgba(0,111,238,0.1)" : "rgba(255,255,255,0.03)",
              }}
            >
              <MessageCircle size={13} /> 活跃
            </button>
            <button
              onClick={() => convD({ type: "SET_ARCHIVED", show: true })}
              className="flex items-center gap-1.5 rounded-[12px] px-2 py-1.5 text-[10px] transition-colors flex-1 justify-center"
              style={{
                color: conv.showArchived ? "var(--yunque-accent)" : "var(--yunque-text-muted)",
                background: conv.showArchived ? "rgba(0,111,238,0.1)" : "rgba(255,255,255,0.03)",
              }}
            >
              <Archive size={13} /> 归档
            </button>
          </div>

          {/* Conversation List */}
          <div className="flex-1 overflow-y-auto px-2 pb-2" style={{ overscrollBehavior: "contain", WebkitOverflowScrolling: "touch" }}>
            <div className="px-2 py-2 text-[10px] font-semibold uppercase tracking-[0.22em]" style={{ color: "var(--yunque-text-muted)" }}>
              {conv.showArchived ? "归档对话" : "最近对话"} ({filteredConversations.length})
            </div>
            <div className="chat-thread-list space-y-1">
              {filteredConversations.map((c) => (
                <div
                  key={c.id}
                  onClick={() => { if (conv.renameId !== c.id) switchConversation(c.id); }}
                  className="conv-item chat-thread-item w-full text-left px-3 py-2.5 rounded-[16px] group relative"
                  data-active={conv.activeId === c.id || undefined}
                  style={{ color: conv.activeId === c.id ? "var(--yunque-accent)" : "var(--yunque-text-secondary)" }}
                >
                  <div className="chat-thread-indicator" aria-hidden="true" />
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
                    <div className="text-[12px] font-medium truncate pr-4">{c.name || c.id}</div>
                  )}
                  <div className="mt-0.5 truncate text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{c.summary || "暂无摘要"}</div>
                  <div className="mt-1.5 flex items-center justify-between">
                    <div className="flex items-center gap-1.5">
                      <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{new Date(c.updated_at).toLocaleDateString([], { month: "numeric", day: "numeric" })}</span>
                      {c.pinned && (
                        <span className="rounded-full px-2 py-0.5 text-[10px]" style={{ background: "rgba(59,130,246,0.1)", color: "var(--yunque-accent)" }}>
                          置顶
                        </span>
                      )}
                    </div>
                    <div className="chat-thread-actions flex items-center gap-0.5">
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
        <header
          className="flex items-center justify-between shrink-0 px-4 py-3 xl:px-5"
          style={{ borderBottom: "1px solid var(--yunque-border)", background: "var(--yunque-sidebar)" }}
        >
          <div className="flex items-center gap-3">
            <button
              onClick={() => setShowSidebar(!showSidebar)}
              className="p-1.5 rounded-lg transition-colors"
              style={{ color: "var(--yunque-text-muted)" }}
            >
              <MessageCircle size={16} />
            </button>

            {/* Model Selector + Mode Switcher */}
            <ModelSelectorPopup
              models={availableModels}
              currentModelId={currentModelId}
              currentModelLabel={currentModel || "选择模型"}
              onSelect={(m) => {
                setCurrentModel(m.model || m.display_name || m.id);
                setCurrentModelId(m.id);
                api.providerSessionOverride(m.id, conv.activeId || "default").catch(() => {});
              }}
              chatMode={chatMode}
              onModeChange={(mode) => {
                setChatMode(mode);
                if (mode === "chat" && airiAvailable) setAiriMode(true);
                else setAiriMode(false);
              }}
              airiAvailable={airiAvailable}
            />
          </div>

          <div className="flex items-center gap-1.5">
            {presets.length > 0 && (
              <Dropdown>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 gap-1.5 rounded-full px-2.5 text-[11px] font-medium"
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
              <Chip className="animate-pulse-dot hidden lg:inline-flex" size="sm" style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)" }}>
                <Sparkles size={11} className="mr-1" /> 流式生成中
              </Chip>
            )}

            {/* Computer panel toggle */}
            <Tooltip delay={0}>
              <Button
                isIconOnly variant="ghost" size="sm" className="chat-tool-btn h-8 w-8 rounded-full"
                onPress={() => setShowComputer(!showComputer)}
                style={{ color: showComputer ? "var(--yunque-accent)" : "var(--yunque-text-muted)" }}
              >
                <Monitor size={15} />
              </Button>
              <Tooltip.Content>{showComputer ? "隐藏计算机面板" : "显示计算机面板"}</Tooltip.Content>
            </Tooltip>

            {(
              <div className="flex items-center gap-0.5 rounded-full p-[3px]" style={{ background: "rgba(255,255,255,0.035)", border: "1px solid rgba(255,255,255,0.05)" }}>
                {thinkingOptions.map(({ key, label, icon }) => (
                  <button
                    key={key}
                    onClick={() => {
                      setThinkingLevel(key);
                      setThinkingEnabled(key === "deep" ? true : key === "none" ? false : null);
                    }}
                    className="flex items-center gap-1 rounded-full px-2.5 py-1 text-[10px] font-medium transition-all"
                    style={{
                      background: thinkingLevel === key ? "var(--yunque-accent)" : "transparent",
                      color: thinkingLevel === key ? "#fff" : "var(--yunque-text-muted)",
                    }}
                  >
                    {icon} {label}
                  </button>
                ))}
              </div>
            )}

          </div>
        </header>

        {resumePromptForBrowser && (
          <div className="px-4 pt-3 xl:px-5 shrink-0">
            <div
              className="rounded-[18px] border px-4 py-3"
              style={{
                background: bridgeState?.connected
                  ? "linear-gradient(180deg, rgba(34,197,94,0.1), rgba(34,197,94,0.03))"
                  : "linear-gradient(180deg, rgba(245,158,11,0.12), rgba(245,158,11,0.03))",
                borderColor: bridgeState?.connected ? "rgba(34,197,94,0.18)" : "rgba(245,158,11,0.18)",
              }}
            >
              <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                    <Plug size={15} style={{ color: bridgeState?.connected ? "#86efac" : "#fbbf24" }} />
                    {bridgeState?.connected ? "Browser task ready to resume" : "Browser connector blocked this task"}
                  </div>
                  <div className="mt-1 text-xs leading-6" style={{ color: "var(--yunque-text-muted)" }}>
                    {bridgeState?.connected
                      ? "The browser runtime is connected again. Continue the blocked task with one click."
                      : "This task needs the live browser connector. Connect it first, then resume from where the flow paused."}
                  </div>
                  <div className="mt-2 truncate rounded-xl px-2.5 py-2 text-[11px]" style={{ background: "rgba(15,23,42,0.3)", color: "var(--yunque-text-secondary)" }}>
                    {resumePromptForBrowser}
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    size="sm"
                    className="rounded-full px-3"
                    variant={bridgeState?.connected ? "primary" : "ghost"}
                    isDisabled={!bridgeState?.connected || browserResumePending || chat.loading}
                    isPending={browserResumePending}
                    onPress={() => continueBlockedBrowserTask()}
                  >
                    Continue task
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="rounded-full px-3"
                    onPress={() => window.open("/browser", "_blank", "noopener,noreferrer")}
                  >
                    Open browser setup
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="rounded-full px-3"
                    onPress={() => {
                      syncBridgeState();
                      api.browserExtStatus()
                        .then((status) => {
                          setBridgeNotice({
                            tone: status.connected ? "success" : "info",
                            text: status.connected ? "Browser connector is ready." : "Browser connector is still offline.",
                          });
                        })
                        .catch(() => setBridgeNotice({ tone: "error", text: "Unable to refresh browser connector status." }));
                    }}
                  >
                    Refresh status
                  </Button>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Chat Messages */}
        <div ref={scrollRef} className="flex-1 overflow-y-auto chat-scroll-area px-5 py-4 xl:px-6">
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
                  <a href="/settings/providers" className="inline-flex items-center gap-1 text-xs mt-2 font-medium" style={{ color: "#f59e0b" }}>前往配置提供商 →</a>
                </div>
              )}
              <div className="w-12 h-12 rounded-2xl flex items-center justify-center chat-hero-icon" style={{ background: "rgba(0,111,238,0.1)" }}>
                <Sparkles size={24} style={{ color: "var(--yunque-accent)" }} />
              </div>
              <div className="max-w-lg text-center space-y-1.5">
                <h1 className="text-[28px] font-bold tracking-tight" style={{ color: "var(--yunque-text)" }}>从这里开始一轮真正可执行的工作</h1>
                <p className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>发起研究、浏览网页、调用连接器、生成代码，或把需求沉淀成任务。</p>
              </div>

              <div className="mt-1 grid w-full max-w-[520px] grid-cols-2 gap-2">
                {(() => {
                  const fixedCards = [
                    { icon: <BookOpen size={14} />, label: "总结文档 / 需求", desc: "贴入文档、需求或笔记，让 Agent 先帮你提炼重点。" },
                    { icon: <Search size={14} />, label: "研究一个主题", desc: "发起研究流程，整理来源、结论和下一步建议。" },
                  ];
                  const fallbackCards = [
                    { icon: <Brain size={14} />, label: "规划多步骤任务", desc: "先拆解步骤，再执行，减少长任务中的混乱。" },
                    { icon: <Zap size={14} />, label: "编写或修复代码", desc: "结合代码上下文、工具与连接器完成开发任务。" },
                  ];
                  const dynamicCards = heroSkills.slice(0, 2).map((sk) => ({
                    icon: <Package size={14} />,
                    label: sk.name,
                    desc: sk.description || "已安装技能，点击直接使用",
                  }));
                  const cards = [...fixedCards, ...(dynamicCards.length >= 2 ? dynamicCards : fallbackCards)];
                  return cards.map(({ icon, label, desc }) => (
                    <button
                      key={label}
                      onClick={() => { chatD({ type: "SET_INPUT", value: label }); inputRef.current?.focus(); }}
                      className="flex items-start gap-2.5 rounded-[16px] p-2.5 text-left transition-all duration-200 hover-lift"
                      style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}
                    >
                      <span className="mt-0.5 shrink-0" style={{ color: "var(--yunque-accent)" }}>{icon}</span>
                      <div className="min-w-0">
                        <div className="text-[13px] font-medium" style={{ color: "var(--yunque-text)" }}>{label}</div>
                        <div className="mt-0.5 text-[10px] leading-5" style={{ color: "var(--yunque-text-muted)" }}>{desc}</div>
                      </div>
                    </button>
                  ));
                })()}
              </div>

              <div className="mt-2 flex w-full max-w-[520px] items-center justify-center gap-3">
                <a href="/skills" className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-colors hover:bg-white/5"
                  style={{ color: "var(--yunque-text-secondary)", border: "1px solid var(--yunque-border)" }}>
                  <Package size={12} /> 浏览技能库 <ArrowRight size={10} />
                </a>
                <a href="/workflows" className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-colors hover:bg-white/5"
                  style={{ color: "var(--yunque-text-secondary)", border: "1px solid var(--yunque-border)" }}>
                  <Blocks size={12} /> 浏览工作流 <ArrowRight size={10} />
                </a>
              </div>
            </div>
          ) : (
            <div className="mx-auto max-w-3xl space-y-5">
              {chat.messages.map((msg, idx) => (
                <div key={msg.id} className={`group chat-message-row flex gap-2.5 ${msg.role === "user" ? "justify-end" : ""}`}>
                  {msg.role === "assistant" && (
                    <Avatar size="sm" className="chat-message-avatar shrink-0 mt-1" style={{ background: "var(--yunque-accent)" }}>
                      <Avatar.Fallback className="text-white text-xs font-bold">Y</Avatar.Fallback>
                    </Avatar>
                  )}
                  <div className={`chat-message-stack max-w-[74%] xl:max-w-[72%] ${msg.role === "user" ? "flex flex-col items-end" : ""}`}>
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
                          <div className="chat-inline-panel mb-1.5 rounded-xl border px-2 py-2" style={{ background: "rgba(255,255,255,0.025)", borderColor: "rgba(255,255,255,0.06)" }}>
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
                              <div className="chat-inline-panel mb-1.5 flex items-center gap-2 rounded-xl px-2 py-1 text-[10px]"
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
                      className={`chat-message-card px-3.5 py-2.5 rounded-[18px] text-[14px] leading-7 whitespace-pre-wrap ${msg.role === "assistant" ? "assistant-message-shell chat-message-card--assistant" : "chat-message-card--user"}`}
                      style={{
                        background: msg.role === "user"
                          ? "linear-gradient(180deg, rgba(59,130,246,0.9), rgba(37,99,235,0.86))"
                          : "linear-gradient(180deg, rgba(255,255,255,0.022), rgba(255,255,255,0.008)), var(--yunque-card)",
                        color: msg.role === "user" ? "#fff" : "var(--yunque-text)",
                        border: msg.role === "assistant" ? "1px solid rgba(255,255,255,0.05)" : "1px solid rgba(59,130,246,0.12)",
                        borderBottomRightRadius: msg.role === "user" ? "8px" : undefined,
                        borderBottomLeftRadius: msg.role === "assistant" ? "8px" : undefined,
                        boxShadow: msg.role === "assistant" ? "0 8px 22px rgba(0,0,0,0.14)" : "0 8px 20px rgba(37,99,235,0.14)",
                      }}
                    >
                      {msg.role === "user" && msg.images && msg.images.length > 0 && (
                        <div className="flex gap-2 flex-wrap mb-2">
                          {msg.images.map((src, i) => (
                            <img key={i} src={src} alt="" className="max-w-[200px] max-h-[200px] rounded-lg object-cover cursor-pointer hover:opacity-90 transition-opacity"
                              onClick={() => openExternal(src)} />
                          ))}
                        </div>
                      )}
                      {msg.role === "assistant" && msg.reasoning && (
                        <details className="mb-2" open={false} style={{ fontSize: "var(--text-sm)" }}>
                          <summary style={{ cursor: "pointer", color: "var(--yunque-text-muted)", fontStyle: "italic", display: "flex", alignItems: "center", gap: 4 }}>
                            <span style={{ fontSize: "var(--text-xs)", background: "rgba(245,158,11,0.12)", color: "#f59e0b", padding: "1px 6px", borderRadius: 4 }}>
                              {chat.streaming && idx === chat.messages.length - 1 ? "推理中…" : "已深度思考"}
                            </span>
                            <ThinkingTimer
                              startMs={msg.reasoningStartMs}
                              endMs={msg.reasoningEndMs}
                              isStreaming={chat.streaming && idx === chat.messages.length - 1}
                            />
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
                          <div className="flex items-center gap-1.5">
                            <Spinner size="sm" color="current" /> Thinking…
                          </div>
                        )
                      )}
                    </div>
                    {/* Emotion badge + Sticker + Airi */}
                    {msg.role === "assistant" && (msg.emotion || msg.sticker || msg.stickers || msg.airiSynced) && (
                      <div className="flex items-center gap-2 mt-1.5 flex-wrap">
                        {msg.emotion && <EmotionBadge emotion={msg.emotion} />}
                        {msg.sticker && <StickerView sticker={msg.sticker} />}
                        {msg.stickers && Object.values(msg.stickers).map((s, i) => <StickerView key={i} sticker={s} />)}
                        {msg.airiSynced && (
                          <span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium" style={{ background: "linear-gradient(135deg, rgba(236,72,153,0.15), rgba(139,92,246,0.15))", color: "#d946ef", border: "1px solid rgba(217,70,239,0.2)" }}>
                            <Heart size={10} fill="#d946ef" /> Airi {msg.airiEmotion && msg.airiEmotion !== "neutral" ? `· ${msg.airiEmotion}` : ""}
                          </span>
                        )}
                      </div>
                    )}
                    {/* Skill tags */}
                    {msg.role === "assistant" && msg.skills_used && msg.skills_used.length > 0 && (
                      <SkillTags skills={msg.skills_used} />
                    )}
                    {msg.role === "assistant" && msg.contextLayers && msg.contextLayers.length > 0 && (
                      <div className="mt-1.5 flex flex-wrap gap-1">
                        {msg.contextLayers.includes("memory") && (
                          <span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px]"
                            style={{ background: "rgba(139,92,246,0.1)", color: "#a78bfa" }}>
                            <Brain size={9} /> 参考了记忆
                          </span>
                        )}
                        {(msg.contextLayers.includes("graph") || msg.contextLayers.includes("code")) && (
                          <span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px]"
                            style={{ background: "rgba(6,182,212,0.1)", color: "#22d3ee" }}>
                            <Library size={9} /> 引用了知识
                          </span>
                        )}
                        {msg.contextLayers.includes("emotion") && (
                          <span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px]"
                            style={{ background: "rgba(236,72,153,0.1)", color: "#f472b6" }}>
                            <Heart size={9} /> 情绪感知
                          </span>
                        )}
                        {msg.contextLayers.includes("strategy") && (
                          <span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px]"
                            style={{ background: "rgba(245,158,11,0.1)", color: "#fbbf24" }}>
                            <Sparkles size={9} /> 运用了经验
                          </span>
                        )}
                      </div>
                    )}
                    {/* Agent action buttons */}
                    {msg.role === "assistant" && msg.actions && msg.actions.length > 0 && (
                      <div className="chat-inline-panel mt-2 rounded-xl border p-2" style={{ background: "rgba(255,255,255,0.02)", borderColor: "rgba(255,255,255,0.06)" }}>
                        <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>
                          Suggested actions
                        </div>
                        <AgentActions actions={msg.actions} onAction={handleAction} />
                      </div>
                    )}
                    {msg.role === "assistant" && msg.browserSummary && (
                      <div className="chat-inline-panel mt-2 rounded-xl border p-2" style={{ background: "rgba(255,255,255,0.02)", borderColor: "rgba(255,255,255,0.06)" }}>
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
                                onPress={() => openExternal(msg.browserSummary?.url)}
                              >
                                Open page
                              </Button>
                            )}
                          </div>
                        )}
                      </div>
                    )}
                    {/* E2B Desktop Sandbox */}
                    {msg.role === "assistant" && msg.sandbox && (
                      <div className="chat-inline-panel mt-2 rounded-xl border p-3" style={{ background: "linear-gradient(135deg, rgba(34,197,94,0.06), rgba(59,130,246,0.06))", borderColor: "rgba(34,197,94,0.2)" }}>
                        <div className="flex items-center gap-2 mb-2">
                          <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: "rgba(34,197,94,0.15)" }}>
                            <Monitor size={16} style={{ color: "#22c55e" }} />
                          </div>
                          <div>
                            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                              E2B Desktop
                            </div>
                            <div className="text-[11px] font-mono" style={{ color: "var(--yunque-text-muted)" }}>
                              {msg.sandbox.sandbox_id}
                            </div>
                          </div>
                          <Chip size="sm" style={{ marginLeft: "auto", background: "rgba(34,197,94,0.12)", color: "#22c55e", fontSize: "10px" }}>LIVE</Chip>
                        </div>
                        {msg.sandbox.stream_url && (
                          <Button
                            size="sm"
                            className="w-full mt-1"
                            onPress={() => openExternal(msg.sandbox?.stream_url)}
                            style={{ background: "rgba(34,197,94,0.15)", color: "#22c55e", border: "1px solid rgba(34,197,94,0.25)" }}
                          >
                            <Monitor size={14} className="mr-2" /> Open Desktop
                          </Button>
                        )}
                      </div>
                    )}
                    {/* Generated file downloads */}
                    {msg.role === "assistant" && msg.traceEvents && (() => {
                      const files = collectGeneratedFiles(msg.traceEvents);
                      if (files.length === 0) return null;
                      return (
                        <div className="chat-inline-panel mt-2 rounded-xl border p-2" style={{ background: "rgba(255,255,255,0.02)", borderColor: "rgba(255,255,255,0.06)" }}>
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
                    {/* Follow-up suggestions (collapsed by default) */}
                    {msg.role === "assistant" && msg.suggestions && msg.suggestions.length > 0 && !chat.streaming && (
                      <details className="mt-3">
                        <summary className="cursor-pointer text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>
                          Next moves
                        </summary>
                        <div className="chat-inline-panel mt-2 rounded-xl border p-2" style={{ background: "rgba(255,255,255,0.02)", borderColor: "rgba(255,255,255,0.06)" }}>
                          <div className="flex flex-wrap gap-2">
                          {msg.suggestions.map((s, i) => (
                            <button key={i} onClick={() => {
                              if (s.type === "save_skill") {
                                sendMessage("Turn this workflow into a reusable skill and save it for later.");
                              } else {
                                sendMessage(s.label);
                              }
                            }}
                              className="chat-followup-chip px-3 py-1.5 rounded-full text-xs font-medium cursor-pointer"
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
                      </details>
                    )}
                    {msg.role === "assistant" && msg.browserRequirement?.required && (
                      <BrowserConnectCard
                        requirement={msg.browserRequirement}
                        connected={Boolean(bridgeState?.connected)}
                        onOpenSetup={() => window.open(msg.browserRequirement?.install_path || "/browser", "_blank", "noopener,noreferrer")}
                        onRefresh={() => {
                          syncBridgeState();
                          api.browserExtStatus()
                            .then((status) => {
                              setBridgeNotice({
                                tone: status.connected ? "success" : "info",
                                text: status.connected ? "Browser connector is ready." : "Browser connector is still offline.",
                              });
                            })
                            .catch(() => {
                              setBridgeNotice({ tone: "error", text: "Unable to refresh browser connector status." });
                            });
                        }}
                        onContinue={bridgeState?.connected ? () => {
                          const previousUserPrompt = chat.messages[idx - 1]?.role === "user"
                            ? chat.messages[idx - 1]?.content
                            : resumePromptForBrowser;
                          if (!previousUserPrompt) return;
                          setResumePromptForBrowser(previousUserPrompt);
                          continueBlockedBrowserTask(previousUserPrompt);
                        } : undefined}
                        continueLabel="Continue blocked task"
                      />
                    )}
                    {msg.role === "assistant" && msg.skillSuggestions && msg.skillSuggestions.length > 0 && (
                      <details className="mt-3">
                        <summary className="cursor-pointer text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>
                          Skill growth proposal
                        </summary>
                        <div className="mt-2">
                          <SkillGrowthPanel
                            suggestions={msg.skillSuggestions}
                            onSave={(suggestion) => {
                              sendMessage(`Turn this into a reusable skill.\n\nName: ${suggestion.name}\nDescription: ${suggestion.description}\nTrigger: ${suggestion.trigger}`);
                            }}
                          />
                        </div>
                      </details>
                    )}
                    {msg.role === "assistant" && msg.traceEvents && msg.traceEvents.length > 0 && (
                      <details className="mt-3">
                        <summary className="cursor-pointer text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>Execution trace</summary>
                        <div className="mt-2">
                          <ExecutionTrace events={msg.traceEvents} isLive={chat.streaming && idx === chat.messages.length - 1} />
                        </div>
                      </details>
                    )}
                    {msg.content && (
                      <div className="chat-message-tools flex gap-0.5 mt-1" style={{ justifyContent: msg.role === "user" ? "flex-end" : "flex-start" }}>
                        {msg.role === "user" && (
                          <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={() => editMessage(msg.id)}><Pencil size={11} /></Button><Tooltip.Content>编辑</Tooltip.Content></Tooltip>
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
                            <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={() => rollbackToMessage(msg.id)}><Undo2 size={11} /></Button><Tooltip.Content>回滚到此</Tooltip.Content></Tooltip>
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
        <div className="px-5 py-3 shrink-0 xl:px-6" style={{ borderTop: chat.messages.length > 0 ? "1px solid var(--yunque-border)" : "none" }}
          onDrop={handleDrop} onDragOver={handleDragOver} onDragLeave={handleDragLeave}>
          <div className="max-w-3xl mx-auto">
            <div
              ref={inputShellRef}
              className="chat-input-wrap chat-composer rounded-[24px] overflow-visible transition-all"
              data-busy={chat.loading ? "true" : "false"}
              style={{
                background: "linear-gradient(180deg, rgba(255,255,255,0.024), rgba(255,255,255,0.008)), var(--yunque-card)",
                border: isDragging ? "1px dashed var(--yunque-accent)" : "1px solid var(--yunque-border)",
                boxShadow: isDragging
                  ? "0 0 0 1px rgba(59,130,246,0.22), 0 14px 36px rgba(15,23,42,0.32)"
                  : "0 10px 28px rgba(0,0,0,0.18), inset 0 1px 0 rgba(255,255,255,0.03)",
              }}
            >
              {/* Frosted glass top bar */}
              <div
                className="flex items-center justify-between gap-3 rounded-t-[24px] px-4 py-2"
                style={{
                  background: "rgba(255,255,255,0.03)",
                  backdropFilter: "blur(16px) saturate(1.6)",
                  WebkitBackdropFilter: "blur(16px) saturate(1.6)",
                  borderBottom: "1px solid rgba(255,255,255,0.06)",
                }}
              >
                <div className="text-[11px] truncate" style={{ color: "var(--yunque-text-muted)" }}>
                  {bridgeState?.connected
                    ? <span className="flex items-center gap-1.5"><Monitor size={11} /><span className="w-1.5 h-1.5 rounded-full bg-blue-400 inline-block" /> 浏览器已连接</span>
                    : "Yunque Agent"}
                </div>
                <div className="flex items-center gap-1.5">
                  <Button size="sm" variant="ghost" className="chat-tool-btn h-7 rounded-full px-2 text-[10px]" data-active={showConnectors ? "true" : undefined} onPress={() => setShowConnectors(true)}>
                    <Plug size={11} /> 连接器
                  </Button>
                  <Button size="sm" variant="ghost" className="chat-tool-btn h-7 rounded-full px-2 text-[10px]"
                    data-active={showSlashMenu || activeSlashCommand ? "true" : undefined}
                    onPress={() => { chatD({ type: "SET_INPUT", value: "/" }); setShowSlashMenu(true); setSlashQuery(""); setActiveSlashCommand(null); inputRef.current?.focus(); }}>
                    <Sparkles size={11} /> 命令
                  </Button>
                </div>
              </div>

              {pendingFiles.length > 0 && (
                <div className="flex gap-2 px-5 pt-4 flex-wrap">
                  {pendingFiles.map((f) => {
                    const statusColor = f.status === "parsed"
                      ? "#4ade80"
                      : f.status === "uploading"
                        ? "#60a5fa"
                        : f.status === "error"
                          ? "#f87171"
                          : "#94a3b8";
                    return (
                      <div key={f.id} className="relative group/file flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-xs"
                        style={{ background: "rgba(255,255,255,0.04)", border: "1px solid var(--yunque-border)" }}>
                        {f.type === "image" && f.preview ? (
                          <img src={f.preview} alt={f.name} className="w-8 h-8 rounded object-cover" />
                        ) : f.type === "video" && f.preview ? (
                          <video src={f.preview} className="w-8 h-8 rounded object-cover" muted />
                        ) : (
                          <Paperclip size={12} style={{ color: "var(--yunque-text-muted)" }} />
                        )}
                        <div className="min-w-0">
                          <div className="truncate max-w-[140px]" style={{ color: "var(--yunque-text-secondary)" }}>{f.name}</div>
                          {f.note && (
                            <div className="flex items-center gap-1 text-[10px]" style={{ color: statusColor }}>
                              <span className="inline-block h-1.5 w-1.5 rounded-full" style={{ background: statusColor }} />
                              <span className="truncate max-w-[160px]">{f.note}</span>
                            </div>
                          )}
                        </div>
                        <button
                          onClick={() => { if (f.preview) URL.revokeObjectURL(f.preview); setPendingFiles(prev => prev.filter((item) => item.id !== f.id)); }}
                          className="ml-1 w-4 h-4 rounded-full flex items-center justify-center text-[10px] opacity-0 group-hover/file:opacity-100 transition-opacity shrink-0"
                          style={{ background: "rgba(239,68,68,0.9)", color: "#fff" }}
                        >?</button>
                      </div>
                    );
                  })}
                </div>
              )}

              <div className="relative px-4 pt-2.5">
                <SlashCommandMenu
                  query={slashQuery}
                  visible={showSlashMenu}
                  onSelect={handleSlashSelect}
                  onClose={() => setShowSlashMenu(false)}
                  anchorRef={inputShellRef}
                />
                {(showSlashMenu || activeSlashCommand) && (
                  <div className="slash-trigger-pill pointer-events-none absolute left-4 top-0 flex items-center gap-1.5 rounded-full px-2 py-1 text-[10px]" style={{ background: "rgba(59,130,246,0.1)", color: "var(--yunque-accent)", boxShadow: "0 8px 24px rgba(59,130,246,0.12)" }}>
                    <span>{showSlashMenu ? "Command menu" : "Slash command"}</span>
                    {activeSlashCommand && (
                      <span className="rounded-full px-2 py-0.5" style={{ background: "rgba(255,255,255,0.12)", color: "var(--yunque-text)" }}>
                        /{activeSlashCommand}
                      </span>
                    )}
                  </div>
                )}
              </div>

              <textarea
                ref={inputRef}
                value={chat.input}
                onChange={handleInputChange}
                onKeyDown={handleKeyDown}
                placeholder="输入消息，/ 打开命令…"
                rows={1}
                className="chat-composer-textarea w-full resize-none bg-transparent px-4 pt-2.5 pb-1.5 text-[14px] outline-none"
                style={{ color: "var(--yunque-text)", minHeight: 42, maxHeight: 160, lineHeight: 1.65 }}
                disabled={chat.loading}
              />

              <div className="flex flex-wrap items-center justify-between gap-3 px-4 pb-3.5 pt-2">
                <div className="flex items-center gap-1.5">
                  <input type="file" ref={fileInputRef} className="hidden" onChange={handleFileUpload} />
                  <Tooltip delay={0}>
                    <Button isIconOnly variant="ghost" size="sm" className="chat-tool-btn compact-tool" onPress={() => fileInputRef.current?.click()}><Paperclip size={14} /></Button>
                    <Tooltip.Content>添加文件</Tooltip.Content>
                  </Tooltip>
                  <Tooltip delay={0}>
                    <Button isIconOnly variant="ghost" size="sm" className="chat-tool-btn compact-tool" onPress={() => { if (fileInputRef.current) { fileInputRef.current.accept = "image/*"; fileInputRef.current.click(); } }}><ImageIcon size={14} /></Button>
                    <Tooltip.Content>添加图片</Tooltip.Content>
                  </Tooltip>
                  <Tooltip delay={0}>
                    <Button
                      isIconOnly
                      variant="ghost"
                      size="sm"
                      className="chat-tool-btn compact-tool"
                      onPress={isRecording ? stopRecording : startRecording}
                      style={isRecording ? { color: "#ef4444" } : {}}
                    >
                      {isRecording ? <StopCircle size={14} className="animate-pulse" /> : <Mic size={14} />}
                    </Button>
                    <Tooltip.Content>{isRecording ? "停止录音" : "语音输入"}</Tooltip.Content>
                  </Tooltip>
                  <div className="relative">
                    <Tooltip delay={0}>
                      <Button isIconOnly variant="ghost" size="sm" className="chat-tool-btn compact-tool" data-active={showConnectors ? "true" : undefined} onPress={() => setShowConnectors(!showConnectors)}>
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
                <div className="hidden items-center gap-2 text-[10px] md:flex" style={{ color: "var(--yunque-text-muted)" }}>
                  <span>Enter 发送</span>
                  <span>·</span>
                  <span>⇧↵ 换行</span>
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
                    className={`chat-send-btn h-10 w-10 rounded-[18px] ${chat.input.trim() ? "chat-send-active" : ""}`}
                    data-active={chat.input.trim() ? "true" : "false"}
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
              {!chat.loading && pendingFiles.length > 0 && (
                <div className="border-t px-4 py-1.5" style={{ borderColor: "rgba(255,255,255,0.05)" }}>
                  <span className="text-[10px]" style={{ color: "#4ade80" }}>
                    {pendingFiles.length} 个附件待发送
                  </span>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Computer Panel — side-by-side on wide screens, fixed overlay on ≤1280px */}
      {showComputer && (
        <>
          {isNarrowViewport && (
            <div
              className="computer-panel-backdrop"
              onClick={() => setShowComputer(false)}
              aria-hidden="true"
            />
          )}
          {!isNarrowViewport && (
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
                  const maxW = Math.min(800, Math.floor(window.innerWidth * 0.4));
                  setComputerWidth(Math.max(280, Math.min(maxW, startW + delta)));
                };
                const onUp = () => { resizingRef.current = false; document.removeEventListener("mousemove", onMove); document.removeEventListener("mouseup", onUp); };
                document.addEventListener("mousemove", onMove);
                document.addEventListener("mouseup", onUp);
              }}
            />
          )}
          <div
            className={`flex flex-col h-full animate-slide-in-right overflow-hidden ${isNarrowViewport ? "computer-panel-overlay" : "shrink-0"}`}
            style={{
              width: isNarrowViewport
                ? Math.min(480, Math.floor(typeof window !== "undefined" ? window.innerWidth * 0.85 : 360))
                : Math.min(computerWidth, Math.floor(typeof window !== "undefined" ? window.innerWidth * 0.4 : 420)),
              background: "var(--yunque-sidebar)",
            }}
          >
            <div className="shrink-0 p-3">
              <TaskProgressPanel events={chat.liveTraceEvents} isLive={chat.streaming} />
            </div>
            <ComputerPanel className="min-h-0 flex-1" traceEvents={chat.liveTraceEvents} isLive onClose={() => setShowComputer(false)} suggestedTab={suggestedTab} />
          </div>
        </>
      )}
    </div>
  );
}
