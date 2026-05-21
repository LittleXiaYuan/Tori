export type ProviderModeType = "local" | "tori" | "hybrid";

export interface ProviderLike {
  id: string;
  display_name?: string;
  type: string;
  model: string;
  base_url: string;
  enabled: boolean;
  tier?: string;
  priority: number;
  capabilities?: string[];
  key_count: number;
  breaker_state: string;
}

export interface ProviderTestLike {
  status?: string;
  success?: boolean;
  latency_ms?: number;
  model?: string;
  error?: string;
}

export interface NormalizedProviderTestResult {
  status: "ok" | "error";
  latency_ms: number;
  model?: string;
  error?: string;
}

export const modeConfig: Record<ProviderModeType, { label: string; desc: string; color: string }> = {
  local: { label: "自带 Key", desc: "直连你自己的模型服务", color: "#3b82f6" },
  tori: { label: "Tori 中转", desc: "统一代理与账号绑定", color: "#8b5cf6" },
  hybrid: { label: "智能混合", desc: "直连优先，失败回退", color: "#22c55e" },
};

export const presetColors: Record<string, string> = {
  deepseek: "#4d6bfe",
  openai: "#10a37f",
  anthropic: "#d4a574",
  google: "#4285f4",
  doubao: "#3370ff",
  qwen: "#6236ff",
  zhipu: "#2563eb",
  moonshot: "#7c3aed",
  minimax: "#ff6600",
  ollama: "#ffffff",
  openrouter: "#6366f1",
  custom: "#6b7280",
  siliconflow: "#00b4d8",
  gitcode: "#fc5531",
};

export const capMeta: Record<string, { label: string; color: string; icon: string }> = {
  vision: { label: "视觉", color: "#a855f7", icon: "👁" },
  reasoning: { label: "推理", color: "#f59e0b", icon: "🧠" },
  function_calling: { label: "工具", color: "#3b82f6", icon: "🔧" },
  structured_output: { label: "结构化", color: "#06b6d4", icon: "📋" },
  long_context: { label: "长文本", color: "#10b981", icon: "📜" },
  web_search: { label: "搜索", color: "#ef4444", icon: "🔍" },
  code_interpreter: { label: "代码", color: "#8b5cf6", icon: "💻" },
  computer_use: { label: "操控", color: "#ec4899", icon: "🖥" },
  audio_in: { label: "语音", color: "#14b8a6", icon: "🎙" },
  video_in: { label: "视频", color: "#f97316", icon: "🎬" },
  image_gen: { label: "生图", color: "#d946ef", icon: "🎨" },
  streaming: { label: "流式", color: "#64748b", icon: "⚡" },
  prompt_caching: { label: "缓存", color: "#84cc16", icon: "💾" },
  mcp: { label: "MCP", color: "#6366f1", icon: "🔌" },
};

const keyCapabilities = ["vision", "reasoning", "web_search", "code_interpreter", "computer_use", "audio_in", "video_in", "image_gen", "mcp", "function_calling"];

export function providerTitle(provider: ProviderLike) {
  return provider.display_name || provider.id || provider.model || "Provider";
}

export function providerInitial(name: string) {
  const s = (name || "P").trim();
  return s.slice(0, 1).toUpperCase();
}

export function providerColor(id?: string) {
  if (!id) return "#6b7280";
  const lower = id.toLowerCase();
  return Object.entries(presetColors).find(([key]) => lower.includes(key))?.[1] || "#6b7280";
}

export function statusTone(provider?: ProviderLike) {
  if (!provider) return { color: "var(--yunque-text-muted)", label: "未选择" };
  if (!provider.enabled) return { color: "#64748b", label: "已停用" };
  if (provider.breaker_state === "open") return { color: "#ef4444", label: "熔断" };
  if (provider.breaker_state === "half-open") return { color: "#f59e0b", label: "半开" };
  return { color: "#22c55e", label: "启用" };
}

export function orderedCapabilities(caps?: string[], max = 5) {
  if (!caps?.length) return [];
  const important = caps.filter((c) => keyCapabilities.includes(c));
  const rest = caps.filter((c) => !keyCapabilities.includes(c));
  return [...important, ...rest].slice(0, max);
}

export function capabilityOverflow(caps?: string[], max = 5) {
  return Math.max(0, (caps?.length || 0) - orderedCapabilities(caps, max).length);
}

export function searchMatch(parts: Array<unknown>, query: string) {
  const tokens = query
    .trim()
    .toLowerCase()
    .split(/\s+/)
    .filter(Boolean);
  if (tokens.length === 0) return true;

  const values: Array<string | number | boolean> = [];
  const collect = (part: unknown) => {
    if (Array.isArray(part)) {
      part.forEach(collect);
      return;
    }
    if (typeof part === "string" || typeof part === "number" || typeof part === "boolean") {
      if (String(part).trim() !== "") values.push(part);
    }
  };
  parts.forEach(collect);

  const text = values.join(" ").toLowerCase();
  return tokens.some((token) => text.includes(token));
}

export function normalizeProviderTestResult(result?: ProviderTestLike | null): NormalizedProviderTestResult {
  const ok = result?.status === "ok" || result?.success === true;
  if (ok && !result?.error) {
    return {
      status: "ok",
      latency_ms: result?.latency_ms ?? 0,
      model: result?.model,
    };
  }
  return {
    status: "error",
    latency_ms: result?.latency_ms ?? 0,
    model: result?.model,
    error: result?.error || (result?.status && result.status !== "ok" ? result.status : "检测未通过"),
  };
}
