"use client";

import { useState, useReducer, useRef, useCallback, useEffect, useMemo } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Avatar, Spinner, Tooltip, Chip, Dropdown, Label, Popover } from "@heroui/react";
import {
  Send, Plus, MessageCircle, Zap, BookOpen, ScanFace, Package,
  Brain, Gauge, Mic, StopCircle, Pencil, RotateCcw, Copy, Undo2,
  Sparkles, Check, Search, Library, ChevronDown, Cpu,
  Paperclip, ImageIcon, Trash2, Volume2, Pin, Archive,
  PanelRightOpen, PanelRightClose, VolumeX, ArchiveRestore, Edit3, Heart,
  PinOff, MoreHorizontal, Monitor, AlertTriangle, Plug,
  ArrowRight, Blocks, Maximize2, Minimize2,
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
import { ConversationSidebar } from "@/components/chat/conversation-sidebar";
import { BrowserResumeBanner } from "@/components/chat/browser-resume-banner";
import { ChatEmptyState } from "@/components/chat/chat-empty-state";
import { ChatMessageList } from "@/components/chat/chat-message-list";
import { chatReducer, chatInit } from "@/lib/chat-state";
import { convReducer, convInit } from "@/lib/conversation-state";
import { useChatInit } from "@/lib/use-chat-init";
import { useChatMedia, type PendingFile } from "@/lib/use-chat-media";
import { useChatRecording } from "@/lib/use-chat-recording";
import { useShortcuts } from "@/lib/use-shortcuts";

export default function ChatPage() {
  const router = useRouter();
  const [chat, chatD] = useReducer(chatReducer, chatInit);
  const [conv, convD] = useReducer(convReducer, convInit);
  const {
    currentModel, currentModelId, availableModels,
    setupNeeded, presets, activePreset,
    airiAvailable, heroSkills,
    setCurrentModel, setCurrentModelId,
    handleSwitchPreset,
  } = useChatInit();

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
  const [suggestedTab, setSuggestedTab] = useState<"terminal" | "browser" | "editor" | "thinking" | undefined>(undefined);
  const [computerWidth, setComputerWidth] = useState(340);
  const resizingRef = useRef(false);

  useEffect(() => {
    if (showComputer) setShowSidebar(false);
  }, [showComputer]);

  const [browserTraceEvents, setBrowserTraceEvents] = useState<AgentEvent[]>([]);
  const [resumePromptForBrowser, setResumePromptForBrowser] = useState<string | null>(null);
  const [browserResumePending, setBrowserResumePending] = useState(false);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const inputShellRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  const { ttsPlaying, isRecording, playTTS, startRecording, stopRecording } =
    useChatRecording(chatD, inputRef);

  const getCurrentInput = useCallback(() => chat.input, [chat.input]);
  const {
    pendingFiles, setPendingFiles, isDragging, fileInputRef,
    processFile, handleFileUpload, handleDrop, handleDragOver, handleDragLeave,
  } = useChatMedia(chatD, getCurrentInput);

  const loadConversations = useCallback(async () => {
    try {
      const data = await api.conversations(conv.showArchived);
      convD({ type: "SET_LIST", list: data.sessions || [] });
    } catch { /* offline */ }
  }, [conv.showArchived]);

  useEffect(() => { loadConversations(); }, [loadConversations]);

  const restoredRef = useRef(false);
  useEffect(() => {
    if (restoredRef.current) return;
    restoredRef.current = true;
    if (conv.activeId && conv.activeId !== "default") {
      switchConversation(conv.activeId);
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

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
        const userMsg: Message = { role: "user", content: text, id: newId(), timestamp: Date.now() };
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
        const userMsg: Message = { role: "user", content: text, id: newId(), timestamp: Date.now() };
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

      const userMsg: Message = { role: "user", content: text, id: newId(), timestamp: Date.now() };
      const asstMsg: Message = { role: "assistant", content: "", id: newId(), timestamp: Date.now(), traceEvents: [] };
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
    const userMsg: Message = { role: "user", content: text, id: newId(), timestamp: Date.now(), ...(mediaPreviews.length > 0 ? { images: mediaPreviews } : {}) };
    const asstMsg: Message = { role: "assistant", content: "", id: newId(), timestamp: Date.now(), traceEvents: [] };
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
      if (!resp.ok || !resp.body) throw new Error(`请求失败 (${resp.status}${resp.statusText ? " " + resp.statusText : ""})`);

      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      let currentEvent = "";
      let streamFinished = false;
      const IDLE_TIMEOUT = 60_000;

      while (!streamFinished) {
        let timerId: ReturnType<typeof setTimeout> | null = null;
        let timedOut = false;
        const timeout = new Promise<{ done: true; value: undefined }>((resolve) => {
          timerId = setTimeout(() => { timedOut = true; resolve({ done: true, value: undefined }); }, IDLE_TIMEOUT);
        });
        const { done, value } = await Promise.race([reader.read(), timeout]);
        if (timerId) clearTimeout(timerId);
        if (timedOut) {
          chatD({ type: "ERROR_LAST", error: "响应超时（60s 无数据），模型可能暂时不可用。点击下方「重新发送」按钮重试。" });
          try { await reader.cancel(); } catch { /* ignore */ }
          break;
        }
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


  const shortcutHandlers = useMemo(() => ({
    new_chat: () => newConversation(),
    search: () => document.dispatchEvent(new CustomEvent("yunque:open-command-palette")),
    stop: () => abortRef.current?.abort(),
    focus_input: () => inputRef.current?.focus(),
    toggle_sidebar: () => setShowSidebar((v) => !v),
    toggle_computer: () => setShowComputer((v) => !v),
    zen_mode: () => {
      setShowSidebar(false);
      window.dispatchEvent(new CustomEvent("yunque:zen-toggle"));
    },
    screenshot_analyze: () => sendMessage("/screenshot Take a screenshot and analyze the current page."),
    copy_last: () => {
      const last = chat.messages.filter((m) => m.role === "assistant").pop();
      if (last?.content) navigator.clipboard.writeText(last.content);
    },
  }), [newConversation, sendMessage, chat.messages]);

  useShortcuts(shortcutHandlers);

  const searchParams = useSearchParams();
  const qParamHandled = useRef(false);
  useEffect(() => {
    if (qParamHandled.current) return;
    const q = searchParams.get("q");
    if (q) {
      qParamHandled.current = true;
      chatD({ type: "SET_INPUT", value: q });
      setTimeout(() => sendMessage(q), 300);
      window.history.replaceState(null, "", "/chat");
    }
  }, [searchParams, sendMessage]);

  useEffect(() => {
    const handler = (e: Event) => {
      const detail = (e as CustomEvent<string>).detail;
      if (detail) sendMessage(detail);
    };
    document.addEventListener("yunque:quick-send", handler);
    return () => document.removeEventListener("yunque:quick-send", handler);
  }, [sendMessage]);

  const thinkingOptions = [
    { key: "none" as const, label: "快速", icon: <Zap size={12} /> },
    { key: "auto" as const, label: "自动", icon: <Gauge size={12} /> },
    { key: "deep" as const, label: "深度", icon: <Brain size={12} /> },
  ] as const;

  return (
    <div className="flex h-screen overflow-hidden" style={{ background: "transparent" }}>
      <div
        className="chat-sidebar-wrap"
        style={{
          width: showSidebar ? "var(--conv-rail-w, 272px)" : "0px",
          minWidth: showSidebar ? "var(--conv-rail-w, 272px)" : "0px",
          opacity: showSidebar ? 1 : 0,
          overflow: "hidden",
          transition: "width 0.25s cubic-bezier(.22,1,.36,1), min-width 0.25s cubic-bezier(.22,1,.36,1), opacity 0.2s ease",
        }}
      >
        <ConversationSidebar
          conv={conv}
          dispatch={convD}
          conversations={filteredConversations}
          onNew={newConversation}
          onSwitch={switchConversation}
          onManage={manageConversation}
          onDelete={deleteConversation}
        />
      </div>

      {/* Main Chat Area */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Top Bar */}
        <header
          className="flex items-center justify-between shrink-0 px-4 py-2.5 xl:px-5"
          style={{
            borderBottom: chat.messages.length > 0 ? "1px solid var(--glass-edge, var(--yunque-border))" : "none",
            background: chat.messages.length > 0 ? "var(--glass-sidebar, var(--yunque-sidebar))" : "transparent",
            backdropFilter: chat.messages.length > 0 ? "blur(var(--yunque-glass-blur)) saturate(var(--yunque-glass-saturate))" : "none",
            WebkitBackdropFilter: chat.messages.length > 0 ? "blur(var(--yunque-glass-blur)) saturate(var(--yunque-glass-saturate))" : "none",
          }}
        >
          <div className="flex items-center gap-3">
            <button
              onClick={() => setShowSidebar(!showSidebar)}
              className="p-1.5 rounded-lg transition-colors"
              style={{ color: "var(--yunque-text-muted)" }}
              aria-label={showSidebar ? "隐藏对话列表" : "显示对话列表"}
            >
              <MessageCircle size={16} />
            </button>

            
          </div>

          <div className="flex items-center gap-1.5">
            {chatMode !== "chat" && presets.length > 0 && (
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

            {chatMode !== "chat" && (
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
            )}

            <Tooltip delay={0}>
              <Button
                isIconOnly variant="ghost" size="sm" className="chat-tool-btn h-8 w-8 rounded-full"
                onPress={() => shortcutHandlers.zen_mode()}
                style={{ color: "var(--yunque-text-muted)" }}
              >
                <Maximize2 size={14} />
              </Button>
              <Tooltip.Content>禅模式 (Ctrl+\)</Tooltip.Content>
            </Tooltip>

          </div>
        </header>

        {resumePromptForBrowser && (
          <BrowserResumeBanner
            prompt={resumePromptForBrowser}
            bridgeConnected={Boolean(bridgeState?.connected)}
            resumePending={browserResumePending}
            chatLoading={chat.loading}
            onResume={() => continueBlockedBrowserTask()}
            onRefresh={() => {
              syncBridgeState();
              api.browserExtStatus()
                .then((status) => {
                  setBridgeNotice({
                    tone: status.connected ? "success" : "info",
                    text: status.connected
                      ? "Browser connector is ready."
                      : "Browser connector is still offline.",
                  });
                })
                .catch(() =>
                  setBridgeNotice({
                    tone: "error",
                    text: "Unable to refresh browser connector status.",
                  }),
                );
            }}
          />
        )}

        {/* Chat Messages */}
        <div ref={scrollRef} className="flex-1 overflow-y-auto chat-scroll-area px-5 py-4 xl:px-6">
          {chat.messages.length === 0 ? (
            <ChatEmptyState setupNeeded={setupNeeded} heroSkills={heroSkills} chatD={chatD} inputRef={inputRef} onSend={sendMessage} />
          ) : (
            <ChatMessageList
              messages={chat.messages}
              streaming={chat.streaming}
              chatMode={chatMode}
              currentModel={currentModel}
              copiedIdx={copiedIdx}
              ttsPlaying={ttsPlaying}
              bridgeState={bridgeState}
              resumePromptForBrowser={resumePromptForBrowser}
              onCopy={handleCopy}
              onPlayTTS={playTTS}
              onEdit={editMessage}
              onRollback={rollbackToMessage}
              onRetry={retryMessage}
              onAction={handleAction}
              onSlashSelect={handleSlashSelect}
              onSend={sendMessage}
              onBrowserRefresh={() => {
                syncBridgeState();
                api.browserExtStatus()
                  .then((status) => setBridgeNotice({ tone: status.connected ? "success" : "info", text: status.connected ? "Browser connector is ready." : "Browser connector is still offline." }))
                  .catch(() => setBridgeNotice({ tone: "error", text: "Unable to refresh browser connector status." }));
              }}
              onBrowserContinue={(prompt) => {
                setResumePromptForBrowser(prompt);
                continueBlockedBrowserTask(prompt);
              }}
            />
          )}
        </div>

        {/* Input Area */}
        <div className="px-5 py-2 shrink-0 xl:px-6" style={{ borderTop: chat.messages.length > 0 ? "1px solid var(--yunque-border)" : "none" }}
          onDrop={handleDrop} onDragOver={handleDragOver} onDragLeave={handleDragLeave}>
          <div className="mx-auto" style={{ maxWidth: "min(900px, 70%)" }}>
            <div
              ref={inputShellRef}
              className="chat-input-wrap chat-composer rounded-[24px] overflow-visible transition-all"
              data-busy={chat.loading ? "true" : "false"}
              style={{
                background: "linear-gradient(180deg, rgba(255,255,255,0.06), rgba(255,255,255,0.02)), var(--glass-card, var(--yunque-card))",
                border: isDragging ? "1px dashed var(--yunque-accent)" : "1px solid var(--glass-edge, var(--yunque-border))",
                boxShadow: isDragging
                  ? "0 0 0 1px rgba(59,130,246,0.22), 0 14px 36px rgba(15,23,42,0.32)"
                  : "0 10px 28px rgba(0,0,0,0.18), inset 0 1px 0 rgba(255,255,255,0.05)",
                backdropFilter: "blur(var(--yunque-glass-blur)) saturate(var(--yunque-glass-saturate))",
                WebkitBackdropFilter: "blur(var(--yunque-glass-blur)) saturate(var(--yunque-glass-saturate))",
              }}
            >
              {/* Frosted glass top bar — hidden in chat mode for simplicity */}
              {chat.messages.length > 0 && chatMode !== "chat" && (
                <div
                  className="flex items-center justify-between gap-3 rounded-t-[24px] px-4 py-1.5"
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
              )}

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
                        >×</button>
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
                style={{ color: "var(--yunque-text)", minHeight: 36, maxHeight: 160, lineHeight: 1.65 }}
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
                  thinkingLevel={thinkingLevel}
                  onThinkingChange={(lvl) => {
                    setThinkingLevel(lvl);
                    setThinkingEnabled(lvl === "deep" ? true : lvl === "none" ? false : null);
                  }}
                />
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

      {/* Computer Panel — Telegram-style side panel within the window */}
      {showComputer && (
        <>
          <div
            className="w-[3px] shrink-0 cursor-col-resize hover:bg-blue-500/30 active:bg-blue-500/50 transition-colors"
            style={{ background: "var(--yunque-border)" }}
            onMouseDown={(e) => {
              e.preventDefault();
              resizingRef.current = true;
              const startX = e.clientX;
              const startW = computerWidth;
              const onMove = (ev: MouseEvent) => {
                if (!resizingRef.current) return;
                const maxW = Math.min(600, Math.floor(window.innerWidth * 0.4));
                setComputerWidth(Math.max(260, Math.min(maxW, startW + (startX - ev.clientX))));
              };
              const onUp = () => { resizingRef.current = false; document.removeEventListener("mousemove", onMove); document.removeEventListener("mouseup", onUp); };
              document.addEventListener("mousemove", onMove);
              document.addEventListener("mouseup", onUp);
            }}
          />
          <div
            className="flex flex-col h-full shrink-0 overflow-hidden animate-slide-in-right"
            style={{
              width: computerWidth,
              background: "var(--yunque-sidebar)",
              borderLeft: "1px solid var(--yunque-border)",
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
