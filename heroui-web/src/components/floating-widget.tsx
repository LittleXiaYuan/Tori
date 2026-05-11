"use client";

import { useState, useRef, useCallback, useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
import {
  MessageCircle, Send, X, Minimize2, Sparkles,
  Settings, Plus, Loader2, Wifi, WifiOff, CheckCircle2, AlertTriangle,
} from "lucide-react";
import { api, getAuthHeaders } from "@/lib/api";
import { formatErrorMessage } from "@/lib/error-utils";
import { COLOR_THEMES, RADIUS_OPTIONS, patchAndApply, type ThemeConfig } from "@/lib/theme-engine";

const POSITION_KEY = "yunque_widget_pos";

interface RecentMsg { role: string; content: string }

interface PageAssistantField {
  label?: string;
  name?: string;
  placeholder?: string;
  value?: string;
}

type PageAssistantAction =
  | { type: "navigate"; path?: string }
  | { type: "open_settings" }
  | { type: "open_command_palette" }
  | { type: "toggle_zen" }
  | { type: "set_theme"; updates?: Record<string, unknown>; mode?: string }
  | { type: "prefill_form"; fields?: PageAssistantField[] }
  | { type: "send_to_chat"; message?: string };

interface PageAssistantResult {
  reply: string;
  actions: PageAssistantAction[];
}

interface PendingActionRequest {
  reply: string;
  actions: PageAssistantAction[];
  fallbackMessage: string;
  context: PageContext;
}

interface PageContext {
  path: string;
  title: string;
  pageName: string;
  headings: string[];
  controls: string[];
}

interface ActionExecution {
  executed: number;
  notes: string[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function describePath(pathname: string | null) {
  const path = pathname || "/";
  if (path.startsWith("/settings/theme")) return "主题设置";
  if (path.startsWith("/settings/providers")) return "模型提供商设置";
  if (path.startsWith("/settings/connectors")) return "连接器设置";
  if (path.startsWith("/settings/notifications")) return "通知设置";
  if (path.startsWith("/settings")) return "设置";
  if (path.startsWith("/browser")) return "浏览器工作区";
  if (path.startsWith("/workspace")) return "Workspace";
  if (path.startsWith("/workers")) return "AI IDE 协作";
  if (path.startsWith("/inbox")) return "收件箱";
  if (path.startsWith("/dashboard")) return "概览";
  if (path.startsWith("/missions")) return "任务中心";
  if (path.startsWith("/plugins")) return "插件";
  return path;
}

function collectPageContext(pathname: string | null): PageContext {
  if (typeof document === "undefined") {
    return { path: pathname || "/", title: "", pageName: describePath(pathname), headings: [], controls: [] };
  }
  const headings = Array.from(document.querySelectorAll("h1,h2,[data-page-title]"))
    .map((el) => (el.textContent || "").trim())
    .filter(Boolean)
    .slice(0, 8);
  const controls = Array.from(document.querySelectorAll("button,a,input,textarea,select"))
    .map((el) => {
      const input = el as HTMLInputElement;
      return (el.textContent || input.placeholder || input.getAttribute("aria-label") || "").trim();
    })
    .filter(Boolean)
    .slice(0, 16);
  return {
    path: pathname || "/",
    title: document.title || "",
    pageName: describePath(pathname),
    headings,
    controls,
  };
}

function extractJsonObject(text: string) {
  const fenced = text.match(/```(?:json)?\s*([\s\S]*?)```/i);
  const raw = fenced?.[1] || text;
  const start = raw.indexOf("{");
  const end = raw.lastIndexOf("}");
  if (start < 0 || end <= start) return null;
  return raw.slice(start, end + 1);
}

function normalizeActions(value: unknown): PageAssistantAction[] {
  if (!Array.isArray(value)) return [];
  const actions: PageAssistantAction[] = [];
  for (const item of value.filter(isRecord)) {
    const type = typeof item.type === "string" ? item.type : "";
    if (type === "navigate") actions.push({ type, path: typeof item.path === "string" ? item.path : "" });
    if (type === "open_settings") actions.push({ type });
    if (type === "open_command_palette") actions.push({ type });
    if (type === "toggle_zen") actions.push({ type });
    if (type === "send_to_chat" && typeof item.message === "string" && item.message.trim()) actions.push({ type, message: item.message });
    if (type === "set_theme") actions.push({ type, mode: typeof item.mode === "string" ? item.mode : "", updates: isRecord(item.updates) ? item.updates : {} });
    if (type === "prefill_form" && Array.isArray(item.fields)) {
      const fields = item.fields.filter(isRecord).map((field) => ({
        label: typeof field.label === "string" ? field.label : "",
        name: typeof field.name === "string" ? field.name : "",
        placeholder: typeof field.placeholder === "string" ? field.placeholder : "",
        value: typeof field.value === "string" ? field.value : "",
      })).filter((field) => field.value && (field.label || field.name || field.placeholder));
      actions.push({ type, fields });
    }
  }
  return actions;
}

function parseAssistantResult(reply: string): PageAssistantResult {
  const json = extractJsonObject(reply);
  if (!json) return { reply, actions: [] };
  try {
    const parsed = JSON.parse(json) as unknown;
    if (!isRecord(parsed)) return { reply, actions: [] };
    return {
      reply: typeof parsed.reply === "string" ? parsed.reply : reply,
      actions: normalizeActions(parsed.actions),
    };
  } catch {
    return { reply, actions: [] };
  }
}

function sanitizeRoute(path: string | undefined) {
  if (!path || !path.startsWith("/") || path.startsWith("//") || path.startsWith("/api/")) return "";
  const allowed = [
    "/dashboard", "/chat", "/settings", "/browser", "/workspace", "/workers",
    "/inbox", "/missions", "/plugins", "/knowledge", "/memory", "/tools",
    "/providers", "/connectors", "/backup", "/trust", "/tenants", "/bots",
  ];
  return allowed.some((prefix) => path === prefix || path.startsWith(`${prefix}/`)) ? path : "";
}

function sanitizeThemeUpdates(action: PageAssistantAction): Partial<ThemeConfig> {
  const updates = action.type === "set_theme" && action.updates ? action.updates : {};
  const next: Partial<ThemeConfig> = {};
  const mode = action.type === "set_theme" ? action.mode || updates.presetTheme : "";
  if (mode === "light" || mode === "dark" || mode === "auto") next.presetTheme = mode;
  if (typeof updates.colorTheme === "string" && COLOR_THEMES.some((item) => item.id === updates.colorTheme)) next.colorTheme = updates.colorTheme;
  if (typeof updates.radius === "string" && RADIUS_OPTIONS.some((item) => item.id === updates.radius)) next.radius = updates.radius;
  for (const key of ["sidebarOpacity", "contentOpacity", "shadowOpacity"] as const) {
    const value = updates[key];
    if (typeof value === "number" && value >= 0 && value <= 100) next[key] = value;
  }
  return next;
}

function normText(text = "") {
  return text.toLowerCase().replace(/\s+/g, "");
}

function fieldIsSensitive(field: PageAssistantField, input?: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null) {
  const text = normText(`${field.label || ""} ${field.name || ""} ${field.placeholder || ""} ${input?.getAttribute("aria-label") || ""} ${input?.getAttribute("name") || ""} ${input?.getAttribute("placeholder") || ""}`);
  const type = input instanceof HTMLInputElement ? input.type : "";
  return type === "password"
    || text.includes("password")
    || text.includes("apikey")
    || text.includes("api密钥")
    || text.includes("密钥")
    || text.includes("token")
    || text.includes("secret")
    || text.includes("webhook")
    || text.includes("url");
}

function setNativeValue(input: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement, value: string) {
  const proto = input instanceof HTMLTextAreaElement
    ? window.HTMLTextAreaElement.prototype
    : input instanceof HTMLSelectElement
      ? window.HTMLSelectElement.prototype
      : window.HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, "value")?.set;
  setter?.call(input, value);
  input.dispatchEvent(new Event("input", { bubbles: true }));
  input.dispatchEvent(new Event("change", { bubbles: true }));
}

function inputDescriptor(input: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement) {
  const id = input.id;
  const label = id ? document.querySelector(`label[for="${CSS.escape(id)}"]`)?.textContent || "" : "";
  const parentLabel = input.closest("label")?.textContent || "";
  const nearbyLabel = input.closest("[role='group'],div")?.querySelector("label")?.textContent || "";
  return normText([
    label,
    parentLabel,
    nearbyLabel,
    input.getAttribute("aria-label") || "",
    input.getAttribute("name") || "",
    input.getAttribute("placeholder") || "",
  ].join(" "));
}

function findInputForField(field: PageAssistantField) {
  const target = normText(`${field.label || ""} ${field.name || ""} ${field.placeholder || ""}`);
  if (!target) return null;
  const inputs = Array.from(document.querySelectorAll<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>("input,textarea,select"))
    .filter((input) => !input.disabled && !(input instanceof HTMLInputElement || input instanceof HTMLTextAreaElement ? input.readOnly : false) && input.type !== "file" && input.type !== "hidden");
  return inputs.find((input) => {
    const desc = inputDescriptor(input);
    return desc.includes(target) || target.includes(desc) || [field.label, field.name, field.placeholder].some((part) => part && desc.includes(normText(part)));
  }) || null;
}

function prefillVisibleFields(fields: PageAssistantField[]): ActionExecution {
  const notes: string[] = [];
  let executed = 0;
  for (const field of fields) {
    const label = field.label || field.name || field.placeholder || "字段";
    if (!field.value) {
      notes.push(`跳过「${label}」：没有可填写内容`);
      continue;
    }
    if (fieldIsSensitive(field)) {
      notes.push(`跳过「${label}」：敏感字段需要你手动填写`);
      continue;
    }
    const input = findInputForField(field);
    if (!input) {
      notes.push(`未找到可见字段「${label}」`);
      continue;
    }
    if (fieldIsSensitive(field, input)) {
      notes.push(`跳过「${label}」：敏感字段需要你手动填写`);
      continue;
    }
    setNativeValue(input, field.value);
    input.focus();
    executed++;
  }
  return { executed, notes };
}

function actionRequiresConfirmation(action: PageAssistantAction) {
  return action.type === "set_theme" || action.type === "prefill_form";
}

function actionLabel(action: PageAssistantAction) {
  if (action.type === "navigate") return `打开 ${action.path || "页面"}`;
  if (action.type === "open_settings") return "打开设置面板";
  if (action.type === "open_command_palette") return "打开命令面板";
  if (action.type === "toggle_zen") return "切换专注模式";
  if (action.type === "send_to_chat") return "带当前页面上下文到 Chat";
  if (action.type === "prefill_form") {
    const labels = (action.fields || []).map((field) => field.label || field.name || field.placeholder).filter(Boolean);
    return labels.length > 0 ? `预填表单：${labels.join("、")}` : "预填当前页面表单";
  }
  if (action.type === "set_theme") {
    const updates = sanitizeThemeUpdates(action);
    const labels: string[] = [];
    if (updates.presetTheme === "light") labels.push("浅色主题");
    if (updates.presetTheme === "dark") labels.push("深色主题");
    if (updates.presetTheme === "auto") labels.push("跟随系统主题");
    if (updates.colorTheme) labels.push(COLOR_THEMES.find((item) => item.id === updates.colorTheme)?.name || "颜色主题");
    if (updates.radius) labels.push(`圆角：${RADIUS_OPTIONS.find((item) => item.id === updates.radius)?.name || updates.radius}`);
    if (typeof updates.sidebarOpacity === "number") labels.push(`侧边栏透明度：${updates.sidebarOpacity}%`);
    if (typeof updates.contentOpacity === "number") labels.push(`内容透明度：${updates.contentOpacity}%`);
    if (typeof updates.shadowOpacity === "number") labels.push(`阴影透明度：${updates.shadowOpacity}%`);
    return labels.length > 0 ? `修改主题：${labels.join("、")}` : "修改主题设置";
  }
  return "执行页面动作";
}

function inferLocalAction(text: string): PageAssistantResult | null {
  const q = text.toLowerCase();
  const searchMatch = text.match(/(?:搜索|查找|筛选)[：:\s]*(.+)$/);
  if (searchMatch?.[1]) {
    return { reply: "我准备帮你预填当前页面的搜索框。", actions: [{ type: "prefill_form", fields: [{ label: "搜索", placeholder: "搜索", value: searchMatch[1].trim() }] }] };
  }
  const modelMatch = text.match(/(?:模型名称|模型名|model)[：:\s]*(.+)$/i);
  if (modelMatch?.[1]) {
    return { reply: "我准备帮你预填模型名称，保存前请你再确认。", actions: [{ type: "prefill_form", fields: [{ label: "模型名称", placeholder: "model-name", value: modelMatch[1].trim() }] }] };
  }
  const baseURLMatch = text.match(/(?:base\s*url|baseURL|基础地址)[：:\s]*(https?:\/\/\S+)/i);
  if (baseURLMatch?.[1]) {
    return { reply: "Base URL 属于连接地址，我不会自动填写；你可以复制后手动确认。", actions: [] };
  }
  if ((q.includes("主题") || q.includes("外观")) && (q.includes("设置") || q.includes("打开") || q.includes("去"))) {
    return { reply: "已为你打开主题设置页。", actions: [{ type: "navigate", path: "/settings/theme" }] };
  }
  if ((q.includes("模型") || q.includes("provider") || q.includes("提供商") || q.includes("api key")) && (q.includes("设置") || q.includes("打开") || q.includes("去"))) {
    return { reply: "已为你打开模型提供商设置页。涉及密钥的内容需要你确认后手动填写。", actions: [{ type: "navigate", path: "/settings/providers" }] };
  }
  if ((q.includes("连接器") || q.includes("connector")) && (q.includes("设置") || q.includes("打开") || q.includes("去"))) {
    return { reply: "已为你打开连接器设置页。", actions: [{ type: "navigate", path: "/settings/connectors" }] };
  }
  if ((q.includes("通知") || q.includes("notification")) && (q.includes("设置") || q.includes("打开") || q.includes("去"))) {
    return { reply: "已为你打开通知设置页。", actions: [{ type: "navigate", path: "/settings/notifications" }] };
  }
  if ((q.includes("浏览器") || q.includes("云电脑")) && (q.includes("打开") || q.includes("去"))) {
    return { reply: "已为你打开浏览器工作区。", actions: [{ type: "navigate", path: "/browser" }] };
  }
  if ((q.includes("workspace") || q.includes("工作区") || q.includes("文件")) && (q.includes("打开") || q.includes("去"))) {
    return { reply: "已为你打开 Workspace。", actions: [{ type: "navigate", path: "/workspace" }] };
  }
  if ((q.includes("ai ide") || q.includes("worker") || q.includes("协作")) && (q.includes("打开") || q.includes("去"))) {
    return { reply: "已为你打开 AI IDE 协作页。", actions: [{ type: "navigate", path: "/workers" }] };
  }
  if (q.includes("插件") && (q.includes("打开") || q.includes("去"))) {
    return { reply: "已为你打开插件页。", actions: [{ type: "navigate", path: "/plugins" }] };
  }
  if (q.includes("深色") || q.includes("暗色") || q.includes("dark")) {
    return { reply: "我准备把界面切换为深色主题。", actions: [{ type: "set_theme", mode: "dark" }] };
  }
  if (q.includes("浅色") || q.includes("亮色") || q.includes("light")) {
    return { reply: "我准备把界面切换为浅色主题。", actions: [{ type: "set_theme", mode: "light" }] };
  }
  if ((q.includes("自动") || q.includes("跟随")) && q.includes("主题")) {
    return { reply: "我准备把主题改为跟随系统。", actions: [{ type: "set_theme", mode: "auto" }] };
  }
  if (q.includes("打开设置") || q.includes("去设置")) {
    return { reply: "已打开设置面板。", actions: [{ type: "open_settings" }] };
  }
  if (q.includes("命令面板") || q.includes("command palette")) {
    return { reply: "已打开命令面板。", actions: [{ type: "open_command_palette" }] };
  }
  if (q.includes("专注") || q.includes("禅") || q.includes("zen")) {
    return { reply: "已切换专注模式。", actions: [{ type: "toggle_zen" }] };
  }
  if (q.includes("回到 chat") || q.includes("去 chat") || q.includes("回到对话") || q.includes("去对话")) {
    return { reply: "已带着当前页面上下文回到 Chat。", actions: [{ type: "send_to_chat", message: text }] };
  }
  return null;
}

function buildAssistantPrompt(text: string, context: PageContext) {
  return [
    "你是云雀的页面助手，负责帮助用户理解当前页面，并在安全范围内触发页面动作。",
    "你只能返回 JSON，不要返回 Markdown。",
    "格式：{\"reply\":\"给用户看的简短中文回复\",\"actions\":[{\"type\":\"动作名\"}]}",
    "允许动作：",
    "navigate: {\"type\":\"navigate\",\"path\":\"/settings/theme\"}",
    "open_settings: {\"type\":\"open_settings\"}",
    "open_command_palette: {\"type\":\"open_command_palette\"}",
    "toggle_zen: {\"type\":\"toggle_zen\"}",
    "set_theme: {\"type\":\"set_theme\",\"mode\":\"light|dark|auto\"} 或 {\"type\":\"set_theme\",\"updates\":{\"colorTheme\":\"deep_sea\",\"radius\":\"large\",\"contentOpacity\":90}}",
    "prefill_form: {\"type\":\"prefill_form\",\"fields\":[{\"label\":\"模型名称\",\"value\":\"gpt-4o-mini\"}]}，只用于当前页面已经可见的非敏感输入框；不要填写 API Key、Token、密码、Secret、Webhook URL 或外部 URL。",
    "send_to_chat: {\"type\":\"send_to_chat\",\"message\":\"带到 Chat 的完整问题\"}",
    "不要执行删除、购买、发送通知、修改模型密钥、调用外部网络、提交表单、点击保存等高风险动作；这类需求只解释下一步或仅预填安全草稿。",
    `当前页面：${context.pageName}`,
    `路径：${context.path}`,
    `标题：${context.title}`,
    `页面标题：${context.headings.join(" / ") || "无"}`,
    `可见控件：${context.controls.join(" / ") || "无"}`,
    `用户请求：${text}`,
  ].join("\n");
}

export function FloatingWidget() {
  const router = useRouter();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const [input, setInput] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  const dragRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ x: number; y: number }>({ x: -1, y: -1 });
  const dragging = useRef(false);
  const dragMoved = useRef(false);
  const [status, setStatus] = useState<"idle" | "loading" | "connected" | "error">("idle");
  const [recentMsgs, setRecentMsgs] = useState<RecentMsg[]>([]);
  const [assistantReply, setAssistantReply] = useState("");
  const [assistantTone, setAssistantTone] = useState<"info" | "success" | "error">("info");
  const [pendingRequest, setPendingRequest] = useState<PendingActionRequest | null>(null);
  const statusTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;
    try {
      const stored = localStorage.getItem(POSITION_KEY);
      if (stored) setPos(JSON.parse(stored));
      else setPos({ x: window.innerWidth - 72, y: window.innerHeight - 72 });
    } catch {
      setPos({ x: window.innerWidth - 72, y: window.innerHeight - 72 });
    }
  }, []);

  useEffect(() => {
    const checkStatus = async () => {
      try {
        const res = await fetch("/v1/auth/status", { headers: getAuthHeaders() });
        if (res.ok) setStatus("connected");
        else setStatus("error");
      } catch { setStatus("error"); }
    };
    checkStatus();
    statusTimer.current = setInterval(checkStatus, 30_000);
    return () => { if (statusTimer.current) clearInterval(statusTimer.current); };
  }, []);

  useEffect(() => {
    if (!open) return;
    api.conversationMessages("default").then((data) => {
      const msgs = (data.messages || []).slice(-3).map((m: { role: string; content: string }) => ({
        role: m.role,
        content: m.content.length > 60 ? m.content.slice(0, 60) + "…" : m.content,
      }));
      setRecentMsgs(msgs);
    }).catch(() => setRecentMsgs([]));
  }, [open]);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    dragging.current = true;
    dragMoved.current = false;
    const startX = e.clientX;
    const startY = e.clientY;
    const startPos = { ...pos };
    const onMove = (ev: MouseEvent) => {
      if (!dragging.current) return;
      const dx = ev.clientX - startX;
      const dy = ev.clientY - startY;
      if (Math.abs(dx) > 3 || Math.abs(dy) > 3) dragMoved.current = true;
      const nx = Math.max(0, Math.min(window.innerWidth - 56, startPos.x + dx));
      const ny = Math.max(0, Math.min(window.innerHeight - 56, startPos.y + dy));
      setPos({ x: nx, y: ny });
    };
    const onUp = () => {
      dragging.current = false;
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
      setPos((p) => { localStorage.setItem(POSITION_KEY, JSON.stringify(p)); return p; });
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }, [pos]);

  const executeActions = useCallback((actions: PageAssistantAction[], fallbackMessage: string, context: PageContext): ActionExecution => {
    let executed = 0;
    let nextRoute = "";
    const notes: string[] = [];

    for (const action of actions) {
      if (action.type === "navigate") {
        const route = sanitizeRoute(action.path);
        if (route) {
          nextRoute = route;
          executed++;
        } else {
          notes.push(`跳过不安全或不支持的路径：${action.path || "空路径"}`);
        }
      } else if (action.type === "open_settings") {
        window.dispatchEvent(new CustomEvent("yunque:open-settings"));
        executed++;
      } else if (action.type === "open_command_palette") {
        document.dispatchEvent(new CustomEvent("yunque:open-command-palette"));
        executed++;
      } else if (action.type === "toggle_zen") {
        window.dispatchEvent(new CustomEvent("yunque:zen-toggle"));
        executed++;
      } else if (action.type === "set_theme") {
        const updates = sanitizeThemeUpdates(action);
        if (Object.keys(updates).length > 0) {
          patchAndApply(updates);
          executed++;
        } else {
          notes.push("没有可应用的主题设置");
        }
      } else if (action.type === "prefill_form") {
        const res = prefillVisibleFields(action.fields || []);
        executed += res.executed;
        notes.push(...res.notes);
      } else if (action.type === "send_to_chat") {
        const message = action.message || fallbackMessage;
        const prompt = [
          `来自页面助手：${context.pageName}`,
          `页面路径：${context.path}`,
          "",
          message,
        ].join("\n");
        nextRoute = `/chat?q=${encodeURIComponent(prompt)}`;
        executed++;
      }
    }

    if (nextRoute) router.push(nextRoute);
    return { executed, notes };
  }, [router]);

  const applyAssistantResult = useCallback((result: PageAssistantResult, text: string, context: PageContext, confirmed = false) => {
    const needsConfirmation = result.actions.some(actionRequiresConfirmation);
    if (needsConfirmation && !confirmed) {
      setPendingRequest({ reply: result.reply, actions: result.actions, fallbackMessage: text, context });
      setAssistantReply(result.reply || "我准备执行这些设置变更，请确认后继续。");
      setAssistantTone("info");
      setStatus("connected");
      return;
    }
    const execution = executeActions(result.actions, text, context);
    setPendingRequest(null);
    const noteText = execution.notes.length > 0 ? `\n${execution.notes.join("\n")}` : "";
    setAssistantReply(`${result.reply || (execution.executed > 0 ? "已处理当前页面请求。" : "我理解了你的问题，但没有需要直接执行的页面动作。")}${noteText}`);
    setAssistantTone(execution.executed > 0 ? "success" : "info");
    setStatus("connected");
  }, [executeActions]);

  const handleSend = useCallback(async () => {
    const text = input.trim();
    if (!text) return;
    const context = collectPageContext(pathname);
    setInput("");
    setStatus("loading");
    setAssistantReply("");
    setAssistantTone("info");
    setPendingRequest(null);

    try {
      const local = inferLocalAction(text);
      const result = local || parseAssistantResult((await api.chat([
        { role: "user", content: buildAssistantPrompt(text, context) },
      ], "page-assistant")).reply);
      applyAssistantResult(result, text, context);
    } catch (e) {
      setAssistantReply(`处理失败：${formatErrorMessage(e, "请稍后重试。")}`);
      setAssistantTone("error");
      setStatus("error");
    }
  }, [applyAssistantResult, input, pathname]);

  const confirmPendingRequest = useCallback(() => {
    if (!pendingRequest) return;
    applyAssistantResult(
      { reply: pendingRequest.reply || "已按你的确认执行。", actions: pendingRequest.actions },
      pendingRequest.fallbackMessage,
      pendingRequest.context,
      true,
    );
  }, [applyAssistantResult, pendingRequest]);

  if (pathname === "/login" || pathname === "/setup" || pos.x < 0) return null;

  const statusColor = status === "connected" ? "var(--yunque-success)"
    : status === "loading" ? "var(--yunque-warning)"
    : status === "error" ? "var(--yunque-danger)"
    : "var(--yunque-text-muted)";
  const replyTone = assistantTone === "success"
    ? { color: "var(--yunque-success)", bg: "rgba(34,197,94,0.1)", border: "rgba(34,197,94,0.2)" }
    : assistantTone === "error"
      ? { color: "var(--yunque-danger)", bg: "rgba(239,68,68,0.1)", border: "rgba(239,68,68,0.2)" }
      : { color: "var(--yunque-accent)", bg: "var(--yunque-accent-muted)", border: "var(--yunque-border)" };

  return (
    <div
      ref={dragRef}
      style={{ position: "fixed", left: pos.x, top: pos.y, zIndex: 9999 }}
    >
      {open && (
        <div
          className="absolute bottom-14 right-0 rounded-2xl overflow-hidden"
          style={{
            width: 340,
            background: "var(--yunque-elevated)",
            border: "1px solid var(--yunque-border)",
            boxShadow: "0 8px 32px rgba(0,0,0,0.24), 0 2px 8px rgba(0,0,0,0.12)",
            animation: "widget-open 0.2s ease-out",
          }}
        >
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-3" style={{ borderBottom: "1px solid var(--yunque-border)" }}>
            <div className="flex items-center gap-2">
              <Sparkles size={14} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>页面助手</span>
              <span className="flex items-center gap-1 text-[10px]" style={{ color: statusColor }}>
                {status === "connected" ? <Wifi size={10} /> : status === "loading" ? <Loader2 size={10} className="animate-spin" /> : <WifiOff size={10} />}
                {status === "connected" ? "在线" : status === "loading" ? "发送中" : status === "error" ? "离线" : ""}
              </span>
            </div>
            <button onClick={() => setOpen(false)} className="p-1 rounded-lg transition-colors hover:bg-white/10" style={{ color: "var(--yunque-text-muted)" }}>
              <Minimize2 size={14} />
            </button>
          </div>

          {/* Recent messages preview */}
          {recentMsgs.length > 0 && (
            <div className="px-3 pt-2 pb-1" style={{ maxHeight: 120, overflowY: "auto" }}>
              {recentMsgs.map((m, i) => (
                <div key={i} className="flex gap-2 py-1" style={{ fontSize: "var(--text-xs)" }}>
                  <span style={{ color: m.role === "user" ? "var(--yunque-accent)" : "var(--yunque-text-muted)", fontWeight: 600, flexShrink: 0 }}>
                    {m.role === "user" ? "你" : "AI"}
                  </span>
                  <span style={{ color: "var(--yunque-text-secondary)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {m.content}
                  </span>
                </div>
              ))}
            </div>
          )}

          {assistantReply && (
            <div className="px-3 pt-2">
              <div className="rounded-xl border px-3 py-2 text-xs leading-5" style={{ background: replyTone.bg, borderColor: replyTone.border, color: "var(--yunque-text)" }}>
                <div className="mb-1 flex items-center gap-1.5 font-semibold" style={{ color: replyTone.color }}>
                  {assistantTone === "error" ? <AlertTriangle size={12} /> : <CheckCircle2 size={12} />}
                  <span>{assistantTone === "success" ? "已执行" : assistantTone === "error" ? "需要处理" : "建议"}</span>
                </div>
                <div>{assistantReply}</div>
              </div>
            </div>
          )}

          {pendingRequest && (
            <div className="px-3 pt-2">
              <div className="rounded-xl border px-3 py-2" style={{ background: "rgba(251,191,36,0.08)", borderColor: "rgba(251,191,36,0.24)" }}>
                <div className="flex items-center gap-1.5 text-xs font-semibold" style={{ color: "var(--yunque-warning)" }}>
                  <AlertTriangle size={12} />
                  <span>请确认页面动作</span>
                </div>
                <div className="mt-2 space-y-1">
                  {pendingRequest.actions.map((action, index) => (
                    <div key={`${action.type}-${index}`} className="rounded-lg px-2 py-1 text-[11px]" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-secondary)" }}>
                      {actionLabel(action)}
                    </div>
                  ))}
                </div>
                <div className="mt-2 flex gap-2">
                  <button
                    onClick={confirmPendingRequest}
                    className="rounded-lg px-2.5 py-1.5 text-[11px] font-semibold"
                    style={{ background: "var(--neutral-strong-bg)", color: "var(--neutral-strong-fg)" }}
                  >
                    确认执行
                  </button>
                  <button
                    onClick={() => { setPendingRequest(null); setAssistantReply("已取消这次页面动作。"); setAssistantTone("info"); }}
                    className="rounded-lg px-2.5 py-1.5 text-[11px]"
                    style={{ color: "var(--yunque-text-secondary)", border: "1px solid var(--yunque-border)" }}
                  >
                    取消
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Input */}
          <div className="p-3">
            <div className="flex items-center gap-2 rounded-xl px-3 py-2.5" style={{ background: "var(--yunque-bg-muted)", border: "1px solid var(--yunque-border)", transition: "border-color 0.15s" }}>
              <input
                ref={inputRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") handleSend(); if (e.key === "Escape") setOpen(false); }}
                placeholder="问这个页面，或让它帮你设置…"
                className="flex-1 bg-transparent outline-none text-sm"
                style={{ color: "var(--yunque-text)" }}
                autoFocus
              />
              <button
                onClick={handleSend}
                disabled={!input.trim()}
                className="p-1.5 rounded-lg transition-all"
                style={{
                  background: input.trim() ? "var(--neutral-strong-bg)" : "transparent",
                  color: input.trim() ? "var(--neutral-strong-fg)" : "var(--yunque-text-muted)",
                  opacity: input.trim() ? 1 : 0.5,
                  transform: input.trim() ? "scale(1)" : "scale(0.9)",
                }}
              >
                <Send size={12} />
              </button>
            </div>

            {/* Quick commands */}
            <div className="mt-2 flex gap-1.5 flex-wrap">
              {["解释这个页面", "打开设置", "切换深色主题"].map((cmd) => (
                <button
                  key={cmd}
                  onClick={() => { setInput(cmd); inputRef.current?.focus(); }}
                  className="rounded-full px-2.5 py-1 text-[10px] font-medium transition-colors hover:opacity-80"
                  style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent)", border: "1px solid var(--yunque-border)" }}
                >
                  {cmd}
                </button>
              ))}
            </div>

            {/* Quick action buttons */}
            <div className="mt-2 flex gap-2" style={{ borderTop: "1px solid var(--yunque-border)", paddingTop: 8 }}>
              <button
                onClick={() => { const context = collectPageContext(pathname); router.push(`/chat?q=${encodeURIComponent(`来自页面助手：${context.pageName}\n页面路径：${context.path}\n\n请帮我理解并继续处理这个页面。`)}`); }}
                className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[11px] font-medium transition-colors hover:bg-white/8"
                style={{ color: "var(--yunque-text-secondary)" }}
              >
                <Plus size={11} /> 带到 Chat
              </button>
              <button
                onClick={() => { setOpen(false); router.push("/settings"); }}
                className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[11px] font-medium transition-colors hover:bg-white/8"
                style={{ color: "var(--yunque-text-secondary)" }}
              >
                <Settings size={11} /> 设置
              </button>
            </div>
          </div>
        </div>
      )}

      {/* FAB button */}
      <button
        onMouseDown={handleMouseDown}
        onClick={() => { if (!dragMoved.current) setOpen(!open); }}
        className="w-12 h-12 rounded-full flex items-center justify-center transition-all hover:scale-110 active:scale-95"
        style={{
          background: "var(--neutral-strong-bg)",
          color: "var(--neutral-strong-fg)",
          boxShadow: "0 4px 16px rgba(0,0,0,0.2), 0 1px 4px rgba(0,0,0,0.1)",
          position: "relative",
        }}
      >
        {open ? <X size={18} /> : status === "loading" ? <Loader2 size={18} className="animate-spin" /> : <MessageCircle size={18} />}
        {/* Status dot */}
        <span
          style={{
            position: "absolute", top: 2, right: 2,
            width: 8, height: 8, borderRadius: "50%",
            background: statusColor,
            border: "2px solid var(--neutral-strong-bg)",
          }}
        />
      </button>
    </div>
  );
}
