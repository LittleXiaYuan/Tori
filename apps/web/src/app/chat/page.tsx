"use client";

import { useState, useReducer, useRef, useCallback, useEffect, useMemo } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Tooltip, Dropdown, Header, Label } from "@heroui/react";
import {
  Plus, MessageCircle, Zap,
  Brain, Gauge, Sparkles, Heart,
  Monitor, Maximize2, MoreHorizontal,
} from "lucide-react";
import { api, type ConversationInfo, type NotifyChannel } from "@/lib/api";
import { createBrowserIntentPackClient } from "@/lib/browser-intent-pack-client";
import type { AgentEvent } from "@/components/execution-trace";
import { ComputerPanel } from "@/components/computer-panel";
import { TaskProgressPanel } from "@/components/task-progress-panel";
import type { AgentAction } from "@/components/chat-extras";
import { showToast } from "@/components/toast-provider";
import { useBrowserBridge } from "@/lib/use-browser-bridge";
import type { ChatSharePayload, SandboxInfo, Message } from "@/lib/chat-types";
import {
  newId,
  browserTraceSummary,
  makeBrowserTraceEvent,
  friendlyError,
  chatHttpErrorMessage,
  collectGeneratedFiles,
  summarizeAssistantWork,
} from "@/lib/chat-utils";
import {
  getSlashState,
  getActiveSlashCommand,
  mapBrowserSummary,
} from "@/lib/slash-commands";
import {
  runSlashBrowserCommand,
  runSocialPublish,
  type ChatBrowserActionContext,
} from "@/lib/chat-browser-actions";
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
import { useChatStream } from "@/lib/use-chat-stream";
import { useI18n } from "@/lib/i18n";
import { ChatInputArea } from "@/components/chat/chat-input-area";
import { TaskResourceMeter, type ResourceSnapshot } from "@/components/chat/task-resource-meter";
import { ChatStreamTimeoutError, parseAgenticChatStream } from "@/lib/chat-sse";
import { buildHiddenContextAttachments } from "@/lib/chat-attachments";
import { workspacePathsFromProjects } from "@/lib/chat-workspace";
import { PlannerRecoveryShelf } from "@/components/chat/planner-recovery-shelf";
import { formatErrorMessage } from "@/lib/error-utils";
import { providerModelLabel } from "@/lib/provider-ui";
import { ModelSelectorPopup, type ModelOption } from "@/components/model-selector-popup";

const browserIntentClient = createBrowserIntentPackClient();

function conversationTitle(c: ConversationInfo | undefined, fallback: string): string {
  const name = (c?.name || "").trim();
  if (name && name !== c?.id && !name.startsWith("new-")) return name;
  const summary = (c?.summary || "").trim();
  if (summary) {
    const runes = [...summary];
    return runes.length > 30 ? runes.slice(0, 30).join("") + "…" : summary;
  }
  return fallback;
}

export default function ChatPage() {
  const { t } = useI18n();
  const router = useRouter();
  const [chat, chatD] = useReducer(chatReducer, chatInit);
  const [conv, convD] = useReducer(convReducer, convInit);
  const {
    currentModel, currentModelId, availableModels,
    setupNeeded, presets, activePreset,
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
  const [suggestedTab, setSuggestedTab] = useState<"terminal" | "browser" | "editor" | "thinking" | undefined>(undefined);
  const [resourceSnapshot, setResourceSnapshot] = useState<ResourceSnapshot | null>(null);
  const [prevResourceSnapshot, setPrevResourceSnapshot] = useState<ResourceSnapshot | null>(null);
  const [workspacePaths, setWorkspacePaths] = useState<string[]>([]);
  const [plannerRecoveryRefreshSignal, setPlannerRecoveryRefreshSignal] = useState<number | undefined>(undefined);
  const sendStartRef = useRef<number>(0);
  const refreshPlannerRecovery = useCallback(() => setPlannerRecoveryRefreshSignal(Date.now()), []);

  const [browserTraceEvents, setBrowserTraceEvents] = useState<AgentEvent[]>([]);
  const [resumePromptForBrowser, setResumePromptForBrowser] = useState<string | null>(null);
  const [browserResumePending, setBrowserResumePending] = useState(false);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const inputShellRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const computerPanelRef = useRef<HTMLDivElement>(null);

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
    } catch (e) {
      console.warn("[chat] loadConversations failed:", e);
    }
  }, [conv.showArchived]);

  const handleModelSelect = useCallback(async (m: ModelOption) => {
    setCurrentModel(providerModelLabel(m));
    setCurrentModelId(m.id);
    try {
      await api.setExecProvider(m.id);
      if (conv.activeId) {
        await api.providerSessionOverride(m.id, conv.activeId).catch(() => {});
      }
    } catch (e) {
      showToast(formatErrorMessage(e, t("chat.toast.modelSwitchFailed")), "error");
    }
  }, [conv.activeId, setCurrentModel, setCurrentModelId, t]);

  useEffect(() => { loadConversations(); }, [loadConversations]);

  useEffect(() => {
    if (!showComputer) return;
    computerPanelRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setShowComputer(false);
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [showComputer]);

  useEffect(() => {
    let cancelled = false;
    api.listProjects()
      .then((res) => {
        if (!cancelled) setWorkspacePaths(workspacePathsFromProjects(res.projects || []));
      })
      .catch((e) => {
        console.warn("[chat] load workspace projects failed:", e);
      });
    return () => { cancelled = true; };
  }, []);

  const restoredRef = useRef(false);
  useEffect(() => {
    if (restoredRef.current) return;
    restoredRef.current = true;
    if (conv.activeId && conv.activeId !== "default") {
      switchConversation(conv.activeId);
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const pushBrowserTrace = useCallback((event: AgentEvent) => {
    setBrowserTraceEvents((prev) => [...prev.slice(-7), event]);
    chatD({ type: "ADD_LIVE_TRACE", event });
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
      text: t("chat.bridge.readyResume"),
    });
  }, [bridgeState?.connected, resumePromptForBrowser, setBridgeNotice]);

  useChatStream({
    onTraceEvent: useCallback((evt: AgentEvent) => {
      chatD({ type: "ADD_LIVE_TRACE", event: evt });
    }, []),
    onShouldOpenComputer: useCallback(() => {
      setSuggestedTab("thinking");
    }, []),
  });

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
    } catch (e) { showToast(formatErrorMessage(e, t("chat.toast.updateFailed")), "error"); }
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
      showToast(t("chat.toast.deleted"), "success");
    } catch (e) { showToast(formatErrorMessage(e, t("chat.toast.deleteFailed")), "error"); }
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
    showToast(t("chat.toast.rolledBack"), "success");
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
    const hasPendingFileContext = pendingFiles.some(f => f.parsedText || f.workspacePath || f.base64);
    if ((!text && !hasPendingFileContext) || chat.loading) return;
    if (pendingFiles.some(f => f.status === "uploading")) {
      showToast(t("chat.toast.attachWait"), "warning");
      return;
    }
    const displayText = text || t("chat.defaultAttachPrompt");
    if (setupNeeded) {
      showToast(t("chat.toast.setupKey"), "warning");
      router.push("/setup");
      return;
    }
    // Browser-intent flows (slash `/browser …` and social-publish) live in
    // chat-browser-actions.ts to keep this function focused on the agentic
    // stream. Each returns true when it fully handled the message.
    const browserActionCtx: ChatBrowserActionContext = {
      browserIntentClient,
      chatD,
      pushBrowserTrace,
      syncBridgeState,
      setBridgeNotice,
      setLastArtifact,
      setSuggestedTab,
      setShowComputer,
      setShowConnectors,
      setResumePromptForBrowser,
      setActiveSlashCommand,
      setShowSlashMenu,
    };
    if (await runSlashBrowserCommand(browserActionCtx, displayText, text)) return;
    if (await runSocialPublish(browserActionCtx, displayText, text)) return;

    const mediaPreviews = pendingFiles.filter(f => (f.type === "image" || f.type === "video") && f.base64).map(f => f.base64!);
    const attachedFiles = pendingFiles
      .filter(f => f.workspacePath || f.parsedText)
      .map(f => ({ name: f.name, path: f.workspacePath || f.name, size: f.size }));
    const userMsg: Message = {
      role: "user",
      content: displayText,
      id: newId(),
      timestamp: Date.now(),
      ...(mediaPreviews.length > 0 ? { images: mediaPreviews } : {}),
      ...(attachedFiles.length > 0 ? { files: attachedFiles } : {}),
    };
    const asstMsg: Message = { role: "assistant", content: "", id: newId(), timestamp: Date.now(), traceEvents: [] };
    setActiveSlashCommand(null);
    setShowSlashMenu(false);
    chatD({ type: "START_SEND" });
    chatD({ type: "ADD_PAIR", userMsg, asstMsg });
    pendingFiles.forEach(f => { if (f.preview) URL.revokeObjectURL(f.preview); });
    setPendingFiles([]);
    sendStartRef.current = Date.now();
    if (resourceSnapshot) setPrevResourceSnapshot(resourceSnapshot);
    setResourceSnapshot({ tokensIn: 0, tokensOut: 0, costUsd: 0, startMs: Date.now() });

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
          content: [{ type: "text", text: displayText }, ...mediaParts],
        });
      } else {
        historyMsgs.push({ role: "user", content: displayText });
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
      };
      if (cherryWebSearch) bodyObj.web_search = true;
      if (cherryToolIds) bodyObj.tool_ids = cherryToolIds;
      if (workspacePaths.length > 0) bodyObj.workspace_paths = workspacePaths;
      const contextAttachments = buildHiddenContextAttachments(pendingFiles);
      const allAttachments = [...(cherryOpts?.attachments || []), ...contextAttachments];
      if (allAttachments.length > 0) {
        bodyObj.attachments = allAttachments.map((a) => ({
          name: a.name,
          mime: a.mime,
          data_b64: a.dataB64,
        }));
      }

      const INITIAL_RESPONSE_TIMEOUT = 240_000;
      let initialResponseTimedOut = false;
      const initialResponseTimer = window.setTimeout(() => {
        initialResponseTimedOut = true;
        abort.abort();
      }, INITIAL_RESPONSE_TIMEOUT);

      let resp: Response;
      try {
        resp = await fetch("/v1/chat/agentic", {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders },
        body: JSON.stringify(bodyObj),
        signal: abort.signal,
        });
      } catch (e) {
        if (initialResponseTimedOut) {
          throw new ChatStreamTimeoutError(INITIAL_RESPONSE_TIMEOUT);
        }
        throw e;
      } finally {
        window.clearTimeout(initialResponseTimer);
      }
      if (!resp.ok) throw new Error(await chatHttpErrorMessage(resp));
      if (!resp.body) throw new Error(t("chat.error.streamInterrupted"));

      // Subagents (esp. file_exec generating whole PPT/Word docs) run a single
      // long LLM turn that emits no intermediate SSE events. The backend handoff
      // cap is 240s, so the idle window must exceed it or the stream is killed
      // mid-generation right before the deliverable arrives.
      const IDLE_TIMEOUT = 420_000;

      for await (const item of parseAgenticChatStream(resp.body, { idleTimeoutMs: IDLE_TIMEOUT })) {
        if (abort.signal.aborted) break;
        if (item.kind === "error") {
          chatD({ type: "ERROR_LAST", error: friendlyError(item.message) });
          refreshPlannerRecovery();
          continue;
        }
        if (item.kind === "done") {
          const doneData = item.data as Record<string, any>;
          const updates: Partial<Message> = {};
          if (doneData.emotion) updates.emotion = doneData.emotion;
          if (doneData.sticker_suggestion) updates.sticker = doneData.sticker_suggestion;
          if (doneData.sticker_suggestions) updates.stickers = doneData.sticker_suggestions;
          if (doneData.skills_used) updates.skills_used = doneData.skills_used;
          if (doneData.actions?.length > 0) updates.actions = doneData.actions;
          if (doneData.suggestions?.length > 0) updates.suggestions = doneData.suggestions;
          if (doneData.context_layers?.length > 0) updates.contextLayers = doneData.context_layers;
          if (doneData.reasoning_content) updates.reasoning = doneData.reasoning_content;
          const doneModel = typeof doneData.model === "string" ? doneData.model : "";
          const doneProviderId = typeof doneData.provider_id === "string" ? doneData.provider_id : "";
          if (doneModel) updates.model = doneModel;
          if (doneProviderId) updates.providerId = doneProviderId;
          if (doneData.browser_summary) {
            updates.browserSummary = mapBrowserSummary(doneData.browser_summary);
            setResumePromptForBrowser(null);
          }
          if (doneData.browser_requirement) {
            setResumePromptForBrowser(displayText);
            updates.browserRequirement = doneData.browser_requirement;
          }
          if (doneData.sandbox && doneData.sandbox.sandbox_id) {
            updates.sandbox = doneData.sandbox as SandboxInfo;
          }
          // Reconcile the live-streamed body with the authoritative final reply.
          // During true token streaming the raw answer (including any trailing
          // NEXT-move markers) is shown live; on done we settle to the clean
          // reply so those markers render as suggestion chips, not inline text.
          if (updates.content === undefined && typeof doneData.reply === "string" && doneData.reply.trim()) {
            updates.content = doneData.reply;
          }
          chatD({ type: "UPDATE_LAST", updates });
          if (doneData.browser_summary) {
            setLastArtifact(mapBrowserSummary(doneData.browser_summary));
          }
          if (doneData.browser_requirement) {
            setSuggestedTab("browser");
          }
          const usage = doneData.usage as { prompt_tokens?: number; completion_tokens?: number } | undefined;
          setResourceSnapshot((prev) => ({
            tokensIn: usage?.prompt_tokens ?? prev?.tokensIn ?? 0,
            tokensOut: usage?.completion_tokens ?? prev?.tokensOut ?? 0,
            costUsd: (doneData.cost_usd as number) ?? prev?.costUsd ?? 0,
            startMs: prev?.startMs ?? sendStartRef.current,
            endMs: Date.now(),
          }));
          break;
        }
        if (item.kind === "actions") {
          if (item.actions.length > 0) {
            chatD({ type: "UPDATE_LAST", updates: { actions: item.actions as AgentAction[] } });
          }
          continue;
        }
        if (item.kind === "thinking") {
          if (item.content) chatD({ type: "UPDATE_LAST", updates: { reasoning: item.content } });
          continue;
        }
        if (item.kind === "delta") {
          chatD({ type: "APPEND_LAST", delta: item.content });
          continue;
        }
        if (item.kind === "agent_event") {
          const parsed = item.event as unknown as AgentEvent;
          const detail = parsed.detail as { stream_type?: string; skill?: string } | undefined;
          if (parsed.domain === "planner" && parsed.type === "thinking" && detail?.stream_type === "thinking_delta") {
            chatD({ type: "APPEND_LAST_REASONING", delta: friendlyError((item.event.message as string) || "") });
          } else if (parsed.domain === "planner" && parsed.type === "thinking" && detail?.stream_type === "reasoning_batch") {
            chatD({ type: "APPEND_LAST_REASONING", delta: friendlyError(parsed.summary || "") + "\n" });
          } else {
            chatD({ type: "APPEND_LAST_TRACE", event: parsed });
            if (parsed.type === "tool_start" || parsed.type === "tool_end" || parsed.type === "thinking") {
              const domain = parsed.domain || "";
              const skillName = detail?.skill || parsed.summary || "";
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
          continue;
        }
        if (item.kind === "raw") {
          chatD({ type: "APPEND_LAST", delta: friendlyError(item.data) });
        }
      }
    } catch (e: unknown) {
      if ((e as Error).name === "AbortError") {
        chatD({ type: "APPEND_LAST", delta: `\n\n${t("chat.aborted")}` });
      } else if (e instanceof ChatStreamTimeoutError) {
        chatD({ type: "ERROR_LAST", error: friendlyError(e.message) });
        refreshPlannerRecovery();
      } else {
        chatD({ type: "ERROR_LAST", error: friendlyError((e as Error).message) });
        refreshPlannerRecovery();
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
  }, [chat.input, chat.loading, chat.messages, thinkingEnabled, thinkingLevel, conv.activeId, loadConversations, pushBrowserTrace, setBridgeNotice, setLastArtifact, syncBridgeState]);

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
      setBridgeNotice({ tone: "success", text: t("chat.bridge.resumed") });
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

  const handleShare = useCallback(async (messageId: string, channel: NotifyChannel, payload: ChatSharePayload) => {
    const activeId = conv.activeId || "default";
    const taskUrl = typeof window !== "undefined"
      ? `${window.location.origin}/chat${activeId && activeId !== "default" ? `?session=${encodeURIComponent(activeId)}` : ""}`
      : "";
    try {
      const result = await api.notifyShare({
        channel_id: channel.id,
        title: payload.title,
        message: payload.message,
        files: payload.files,
        session_id: activeId,
        task_id: activeId,
        url: taskUrl,
      });
      chatD({
        type: "ADD_SHARE_RECEIPT",
        messageId,
        receipt: {
          id: `share-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
          status: "sent",
          channelId: result.channel?.id || channel.id,
          channelName: result.channel?.name || channel.name,
          channelType: result.channel?.type || channel.type,
          targetTitle: payload.title,
          sentAt: result.sent_at ? new Date(result.sent_at).getTime() : Date.now(),
          shareCode: result.share?.code,
        },
      });
      showToast(result.share?.code ? `已同步到 ${result.channel?.name || channel.name}，协作码 ${result.share.code}` : `已同步到 ${result.channel?.name || channel.name}`, "success");
    } catch (e) {
      const error = formatErrorMessage(e, t("chat.toast.syncFailed"));
      chatD({
        type: "ADD_SHARE_RECEIPT",
        messageId,
        receipt: {
          id: `share-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
          status: "failed",
          channelId: channel.id,
          channelName: channel.name,
          channelType: channel.type,
          targetTitle: payload.title,
          sentAt: Date.now(),
          error,
        },
      });
      showToast(error, "error");
    }
  }, [conv.activeId]);

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

  const activeConversation = useMemo(
    () => conv.list.find((c) => c.id === conv.activeId),
    [conv.list, conv.activeId],
  );


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
    // Quick-send arrives through two channels:
    //   1. In-app SelectionPopup (`app-shell.tsx`) still dispatches a DOM
    //      CustomEvent — that path is purely renderer-side so DOM events
    //      are the natural fit.
    //   2. The Tauri shell (floating panel) used to inject a `document
    //      .dispatchEvent(...)` via `webview.eval()`, but that forced us
    //      to keep `'unsafe-eval'` in CSP. It now uses the Tauri event
    //      bus instead; listen for both so either channel works.
    const handler = (e: Event) => {
      const detail = (e as CustomEvent<string>).detail;
      if (detail) sendMessage(detail);
    };
    document.addEventListener("yunque:quick-send", handler);

    let unlistenTauri: (() => void) | undefined;
    void import("@tauri-apps/api/event")
      .then(({ listen }) => listen<string>("yunque:quick-send", (e) => {
        if (typeof e.payload === "string" && e.payload) sendMessage(e.payload);
      }))
      .then((un) => {
        unlistenTauri = un;
      })
      .catch((err) => {
        console.warn("[chat] tauri listen yunque:quick-send failed", err);
      });

    return () => {
      document.removeEventListener("yunque:quick-send", handler);
      unlistenTauri?.();
    };
  }, [sendMessage]);

  useEffect(() => {
    const session = searchParams.get("session");
    if (!session || session === conv.activeId) return;
    switchConversation(session);
  }, [searchParams, conv.activeId, switchConversation]);

  const thinkingOptions = [
    { key: "none" as const, label: t("model.think.none"), icon: <Zap size={12} /> },
    { key: "auto" as const, label: t("model.think.auto"), icon: <Gauge size={12} /> },
    { key: "deep" as const, label: t("model.think.deep"), icon: <Brain size={12} /> },
  ] as const;

  // The composer is rendered in one of two places depending on whether a
  // conversation has started: centered on the empty screen (Claude.ai-style)
  // or pinned to the bottom once messages exist. Build it once so both
  // branches share the exact same props.
  const composer = (
    <ChatInputArea
      input={chat.input}
      loading={chat.loading}
      streaming={chat.streaming}
      hasMessages={chat.messages.length > 0}
      chatMode={chatMode}
      isDragging={isDragging}
      pendingFiles={pendingFiles}
      showSlashMenu={showSlashMenu}
      slashQuery={slashQuery}
      activeSlashCommand={activeSlashCommand}
      showConnectors={showConnectors}
      bridgeConnected={Boolean(bridgeState?.connected)}
      availableModels={availableModels}
      currentModel={currentModel || t("composer.selectModel")}
      currentModelId={currentModelId}
      thinkingLevel={thinkingLevel}
      isRecording={isRecording}
      inputRef={inputRef}
      fileInputRef={fileInputRef}
      inputShellRef={inputShellRef}
      onInputChange={handleInputChange}
      onKeyDown={handleKeyDown}
      onSlashSelect={handleSlashSelect}
      onSlashClose={() => setShowSlashMenu(false)}
      onFileUpload={handleFileUpload}
      onDrop={handleDrop}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onSend={() => sendMessage()}
      onStop={stopGeneration}
      onRemoveFile={(id, preview) => { if (preview) URL.revokeObjectURL(preview); setPendingFiles(prev => prev.filter((item) => item.id !== id)); }}
      onConnectorsToggle={setShowConnectors}
      onModelSelect={handleModelSelect}
      onModeChange={setChatMode}
      onThinkingChange={(lvl) => {
        setThinkingLevel(lvl);
        setThinkingEnabled(lvl === "deep" ? true : lvl === "none" ? false : null);
      }}
      onStartRecording={startRecording}
      onStopRecording={stopRecording}
      onOpenImagePicker={() => { if (fileInputRef.current) { fileInputRef.current.accept = "image/*"; fileInputRef.current.click(); } }}
    />
  );

  return (
    <div className="flex h-screen overflow-hidden" style={{ background: "transparent" }}>
      <div
        className="chat-sidebar-wrap"
        data-open={showSidebar ? "true" : "false"}
        style={{
          width: showSidebar ? "var(--conv-rail-w, 248px)" : "0px",
          minWidth: showSidebar ? "var(--conv-rail-w, 248px)" : "0px",
          flexShrink: 0,
          opacity: showSidebar ? 1 : 0,
          pointerEvents: showSidebar ? "auto" : "none",
          overflow: "hidden",
          borderRight: showSidebar ? "1px solid var(--glass-edge, var(--yunque-border))" : "none",
          transition: showSidebar
            ? "width 0.25s cubic-bezier(.22,1,.36,1), min-width 0.25s cubic-bezier(.22,1,.36,1), opacity 0.2s ease, border-color 0.2s ease"
            : "width 0.25s cubic-bezier(.22,1,.36,1), min-width 0.25s cubic-bezier(.22,1,.36,1), opacity 0.15s ease, border-color 0.15s ease",
        }}
      >
        <ConversationSidebar
          conv={conv}
          dispatch={convD}
          conversations={filteredConversations}
          chatMode={chatMode}
          onModeChange={(mode) => {
            setChatMode(mode);
          }}
          onNew={newConversation}
          onSwitch={switchConversation}
          onManage={manageConversation}
          onDelete={deleteConversation}
        />
      </div>

      {/* Main Chat Area */}
      <section
        className="flex-1 flex flex-col min-w-0"
        aria-labelledby="chat-current-title"
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
      >
        {/* Top Bar — Gemini-style: empty, just a sidebar toggle and a more menu */}
        <header
          className="chat-topbar flex items-center justify-between shrink-0 px-3 py-2 xl:px-4"
          style={{
            borderBottom: "1px solid transparent",
            background: "transparent",
          }}
        >
          <div className="chat-topbar__left flex items-center gap-2 min-w-0">
            <button
              onClick={() => setShowSidebar(!showSidebar)}
              className="chat-topbar__icon-btn p-1.5 rounded-lg transition-colors"
              style={{ color: "var(--yunque-text-muted)" }}
              aria-label={showSidebar ? t("convo.hideList") : t("convo.showList")}
            >
              <MessageCircle size={16} />
            </button>
          </div>

          <div className="flex items-center gap-1.5">
            <Dropdown>
              <Tooltip delay={0}>
              <Button
                isIconOnly
                variant="ghost"
                size="sm"
                className="chat-tool-btn h-8 w-8 rounded-full"
                aria-label={t("chat.more")}
                style={{ color: "var(--yunque-text-muted)" }}
              >
                <MoreHorizontal size={16} />
              </Button>
              <Tooltip.Content>{t("chat.more")}</Tooltip.Content>
              </Tooltip>
              <Dropdown.Popover className="min-w-[220px]">
                <Dropdown.Menu
                  onAction={(key) => {
                    const action = String(key);
                    if (action === "tasks") router.push("/missions");
                    if (action === "computer") setShowComputer((v) => !v);
                    if (action === "zen") shortcutHandlers.zen_mode();
                    if (action === "new") newConversation();
                    if (action.startsWith("preset:")) handleSwitchPreset(action.slice("preset:".length));
                  }}
                >
                  <Dropdown.Item id="new" textValue={t("convo.new")}>
                    <Label className="flex items-center gap-2"><Plus size={14} />{t("convo.new")}</Label>
                  </Dropdown.Item>
                  <Dropdown.Item id="tasks" textValue={t("chat.toTasks")}>
                    <Label className="flex items-center gap-2"><Zap size={14} />{t("chat.toTasks")}</Label>
                  </Dropdown.Item>
                  {chatMode !== "chat" && (
                    <Dropdown.Item id="computer" textValue={showComputer ? t("chat.computer.hide") : t("chat.computer.show")}>
                      <Label className="flex items-center gap-2"><Monitor size={14} />{showComputer ? t("chat.computer.hide") : t("chat.computer.show")}</Label>
                    </Dropdown.Item>
                  )}
                  <Dropdown.Item id="zen" textValue={t("chat.zen")}>
                    <Label className="flex items-center gap-2"><Maximize2 size={14} />{t("chat.zen")}</Label>
                  </Dropdown.Item>
                  {chatMode !== "chat" && presets.length > 0 && (
                    <Dropdown.Section>
                      <Header>{t("chat.presets")}</Header>
                      {presets.map((p) => (
                        <Dropdown.Item key={`preset:${p.id}`} id={`preset:${p.id}`} textValue={p.name}>
                          <Label className="flex items-center gap-2">
                            <Heart size={14} />
                            {p.name}{p.id === activePreset ? t("chat.presetCurrent") : ""}
                          </Label>
                        </Dropdown.Item>
                      ))}
                    </Dropdown.Section>
                  )}
                </Dropdown.Menu>
              </Dropdown.Popover>
            </Dropdown>
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
              browserIntentClient.extensionStatus()
                .then((status) => {
                  setBridgeNotice({
                    tone: status.connected ? "success" : "info",
                    text: status.connected
                      ? t("chat.bridge.ready")
                      : t("chat.bridge.offline"),
                  });
                })
                .catch(() =>
                  setBridgeNotice({
                    tone: "error",
                    text: t("chat.bridge.refreshFailed"),
                  }),
                );
            }}
          />
        )}

        {chatMode !== "chat" && (
          <PlannerRecoveryShelf onSend={sendMessage} disabled={chat.loading} refreshSignal={plannerRecoveryRefreshSignal} />
        )}

        {/* Chat Messages */}
        {chat.messages.length === 0 ? (
          <div className="flex-1 overflow-y-auto chat-scroll-area chat-scroll-area--empty px-5 py-4 xl:px-6">
            <ChatEmptyState setupNeeded={setupNeeded} chatD={chatD} inputRef={inputRef} composer={composer} />
          </div>
        ) : (
          <>
            <div ref={scrollRef} className="flex-1 overflow-y-auto chat-scroll-area px-5 py-4 xl:px-6">
              <div className="chat-content-column mx-auto w-full max-w-4xl">
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
                  onShare={handleShare}
                  onBrowserRefresh={() => {
                    syncBridgeState();
                    browserIntentClient.extensionStatus()
                      .then((status) => setBridgeNotice({ tone: status.connected ? "success" : "info", text: status.connected ? t("chat.bridge.ready") : t("chat.bridge.offline") }))
                      .catch(() => setBridgeNotice({ tone: "error", text: t("chat.bridge.refreshFailed") }));
                  }}
                  onBrowserContinue={(prompt) => {
                    setResumePromptForBrowser(prompt);
                    continueBlockedBrowserTask(prompt);
                  }}
                />
              </div>
            </div>
            {composer}
          </>
        )}
      </section>

      {/* Computer Panel — user-opened overlay, never steals chat width. */}
      {showComputer && (
        <div className="computer-panel-backdrop" onClick={() => setShowComputer(false)}>
          <div
            ref={computerPanelRef}
            role="dialog"
            aria-label={t("chat.computer.show")}
            tabIndex={-1}
            className="computer-panel-overlay flex flex-col overflow-hidden animate-slide-in-right"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="shrink-0 p-3 space-y-2">
              <TaskResourceMeter snapshot={resourceSnapshot} prevSnapshot={prevResourceSnapshot} isLive={chat.streaming} />
              <TaskProgressPanel events={chat.liveTraceEvents} isLive={chat.streaming} />
            </div>
            <ComputerPanel className="min-h-0 flex-1" traceEvents={chat.liveTraceEvents} isLive onClose={() => setShowComputer(false)} suggestedTab={suggestedTab} />
          </div>
        </div>
      )}
    </div>
  );
}
