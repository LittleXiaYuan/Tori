"use client";

import { useState, useMemo, useRef, useEffect } from "react";
import { Button, Chip, Tooltip } from "@heroui/react";
import { Search, Cpu, ChevronDown, Check, Zap, Star, X, Settings, Sparkles, Heart, MessageCircle } from "lucide-react";

export interface ModelOption {
  id: string;
  model: string;
  display_name?: string;
  enabled: boolean;
  type?: string;
  tier?: string;
  capabilities?: string[];
}

export type ChatMode = "agent" | "fast" | "chat";

interface Props {
  models: ModelOption[];
  currentModelId: string;
  currentModelLabel: string;
  onSelect: (model: ModelOption) => void;
  chatMode?: ChatMode;
  onModeChange?: (mode: ChatMode) => void;
  airiAvailable?: boolean;
}

const PROVIDER_LABELS: Record<string, { label: string; color: string }> = {
  openai:    { label: "OpenAI",    color: "#10a37f" },
  anthropic: { label: "Anthropic", color: "#d97706" },
  claude:    { label: "Claude",    color: "#d97706" },
  gemini:    { label: "Gemini",    color: "#4285f4" },
  google:    { label: "Google",    color: "#4285f4" },
  deepseek:  { label: "DeepSeek",  color: "#6366f1" },
  ollama:    { label: "Ollama",    color: "#888" },
  local:     { label: "本地模型",   color: "#22c55e" },
  tori:      { label: "Tori",      color: "#a855f7" },
  openrouter:{ label: "OpenRouter", color: "#ef4444" },
};

function providerMeta(type?: string) {
  if (!type) return { label: "其他", color: "var(--yunque-text-muted)" };
  const key = type.toLowerCase();
  for (const [k, v] of Object.entries(PROVIDER_LABELS)) {
    if (key.includes(k)) return v;
  }
  return { label: type, color: "var(--yunque-text-muted)" };
}

function tierBadge(tier?: string) {
  if (!tier) return null;
  const t = tier.toLowerCase();
  if (t === "premium" || t === "high") return { label: "高级", color: "#f59e0b", bg: "rgba(245,158,11,0.1)" };
  if (t === "standard" || t === "mid") return { label: "标准", color: "#3b82f6", bg: "rgba(59,130,246,0.1)" };
  if (t === "fast" || t === "low") return { label: "快速", color: "#22c55e", bg: "rgba(34,197,94,0.1)" };
  return { label: tier, color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
}

const MODE_DEFS: { key: ChatMode; label: string; icon: typeof Sparkles; color: string; desc: string }[] = [
  { key: "agent", label: "Agent", icon: Sparkles, color: "var(--yunque-accent, #006fee)", desc: "多步推理 + 工具调用" },
  { key: "fast",  label: "Fast",  icon: Zap,      color: "#f59e0b", desc: "快速响应模式" },
  { key: "chat",  label: "Chat",  icon: MessageCircle, color: "#22c55e", desc: "简单对话" },
];

export function ModelSelectorPopup({ models, currentModelId, currentModelLabel, onSelect, chatMode, onModeChange, airiAvailable }: Props) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const popoverRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open && searchRef.current) {
      setTimeout(() => searchRef.current?.focus(), 80);
    }
    if (!open) setSearch("");
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open]);

  const enabledModels = useMemo(() => models.filter(m => m.enabled), [models]);

  const grouped = useMemo(() => {
    const q = search.toLowerCase().trim();
    const filtered = q
      ? enabledModels.filter(m =>
          (m.model || "").toLowerCase().includes(q) ||
          (m.display_name || "").toLowerCase().includes(q) ||
          (m.type || "").toLowerCase().includes(q)
        )
      : enabledModels;

    const groups: Record<string, ModelOption[]> = {};
    for (const m of filtered) {
      const key = providerMeta(m.type).label;
      if (!groups[key]) groups[key] = [];
      groups[key].push(m);
    }
    return Object.entries(groups).sort(([a], [b]) => a.localeCompare(b));
  }, [enabledModels, search]);

  if (enabledModels.length === 0) {
    return (
      <Chip size="sm" variant="soft" className="text-xs font-mono">
        {currentModelLabel || "未配置模型"}
      </Chip>
    );
  }

  return (
    <div className="relative" style={{ zIndex: 50 }}>
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1.5 rounded-full px-2.5 h-8 text-[11px] font-mono transition-all"
        style={{
          background: open ? "rgba(255,255,255,0.08)" : "rgba(255,255,255,0.04)",
          color: "var(--yunque-text-secondary)",
          border: "1px solid transparent",
        }}
      >
        <Cpu size={12} />
        <span className="max-w-[140px] truncate">{currentModelLabel}</span>
        <ChevronDown size={10} style={{ color: "var(--yunque-text-muted)", transform: open ? "rotate(180deg)" : "none", transition: "transform 0.2s" }} />
      </button>

      {open && (
        <div
          ref={popoverRef}
          className="model-selector-popup"
          style={{
            position: "absolute",
            top: "calc(100% + 6px)",
            left: 0,
            minWidth: 320,
            maxWidth: 400,
            maxHeight: 420,
            background: "var(--yunque-bg-card, #1a1a2e)",
            border: "1px solid rgba(255,255,255,0.08)",
            borderRadius: 14,
            boxShadow: "0 12px 40px rgba(0,0,0,0.4), 0 0 0 1px rgba(255,255,255,0.04)",
            overflow: "hidden",
            animation: "modelPopFadeIn 0.18s ease-out",
          }}
        >
          {/* Mode Switcher */}
          {chatMode && onModeChange && (
            <div style={{ padding: "10px 12px 6px", borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
              <div className="flex items-center gap-1 rounded-lg p-1" style={{ background: "rgba(255,255,255,0.03)" }}>
                {MODE_DEFS.map(md => {
                  const isAiri = md.key === "chat" && airiAvailable;
                  const Icon = isAiri ? Heart : md.icon;
                  const label = isAiri ? "Airi" : md.label;
                  const color = isAiri ? "#d946ef" : md.color;
                  const active = chatMode === md.key;
                  return (
                    <button
                      key={md.key}
                      onClick={() => onModeChange(md.key)}
                      className="flex-1 flex items-center justify-center gap-1.5 rounded-md py-1.5 text-[11px] font-medium transition-all"
                      style={{
                        background: active ? `${color}15` : "transparent",
                        color: active ? color : "var(--yunque-text-muted)",
                        boxShadow: active ? `inset 0 0 0 1px ${color}30` : "none",
                      }}
                    >
                      <Icon size={12} fill={isAiri && active ? "currentColor" : "none"} />
                      {label}
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          {/* Search */}
          <div style={{ padding: chatMode ? "6px 12px 6px" : "10px 12px 6px", borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
            <div className="flex items-center gap-2 rounded-lg px-2.5 py-1.5" style={{ background: "rgba(255,255,255,0.04)" }}>
              <Search size={13} style={{ color: "var(--yunque-text-muted)", flexShrink: 0 }} />
              <input
                ref={searchRef}
                value={search}
                onChange={e => setSearch(e.target.value)}
                placeholder="搜索模型…"
                className="flex-1 bg-transparent border-none outline-none text-xs"
                style={{ color: "var(--yunque-text-primary)", caretColor: "var(--yunque-accent)" }}
              />
              {search && (
                <button onClick={() => setSearch("")} className="opacity-50 hover:opacity-100">
                  <X size={12} />
                </button>
              )}
            </div>
          </div>

          {/* Model list */}
          <div style={{ overflowY: "auto", maxHeight: 340, padding: "4px 0" }} className="chat-scroll-area">
            {grouped.length === 0 && (
              <div className="text-center py-6 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                没有找到匹配的模型
              </div>
            )}
            {grouped.map(([providerName, providerModels]) => {
              const meta = PROVIDER_LABELS[Object.keys(PROVIDER_LABELS).find(k => providerName.toLowerCase().includes(k)) || ""] || { color: "var(--yunque-text-muted)" };
              return (
                <div key={providerName}>
                  <div
                    className="flex items-center gap-2 px-3 py-1.5 text-[10px] font-semibold uppercase tracking-wider"
                    style={{ color: meta.color, opacity: 0.8 }}
                  >
                    <span className="w-1.5 h-1.5 rounded-full" style={{ background: meta.color }} />
                    {providerName}
                    <span className="text-[9px] font-normal" style={{ color: "var(--yunque-text-muted)" }}>
                      {providerModels.length}
                    </span>
                  </div>
                  {providerModels.map(m => {
                    const isActive = m.id === currentModelId;
                    const tb = tierBadge(m.tier);
                    return (
                      <button
                        key={m.id}
                        onClick={() => { onSelect(m); setOpen(false); }}
                        className="w-full flex items-center gap-2 px-3 py-2 text-left transition-all group"
                        style={{
                          background: isActive ? "rgba(0,111,238,0.08)" : "transparent",
                          borderLeft: isActive ? "2px solid var(--yunque-accent)" : "2px solid transparent",
                        }}
                        onMouseEnter={e => { if (!isActive) (e.currentTarget as HTMLElement).style.background = "rgba(255,255,255,0.04)"; }}
                        onMouseLeave={e => { if (!isActive) (e.currentTarget as HTMLElement).style.background = "transparent"; }}
                      >
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-1.5">
                            <span className="text-xs font-medium truncate" style={{ color: isActive ? "var(--yunque-accent)" : "var(--yunque-text-primary)" }}>
                              {m.display_name || m.model || m.id}
                            </span>
                            {tb && (
                              <span className="text-[9px] px-1.5 py-0.5 rounded-full font-medium" style={{ color: tb.color, background: tb.bg }}>
                                {tb.label}
                              </span>
                            )}
                          </div>
                          {m.display_name && m.model && m.display_name !== m.model && (
                            <div className="text-[10px] font-mono truncate mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>
                              {m.model}
                            </div>
                          )}
                        </div>
                        {isActive && <Check size={14} style={{ color: "var(--yunque-accent)", flexShrink: 0 }} />}
                      </button>
                    );
                  })}
                </div>
              );
            })}
          </div>

          {/* Footer */}
          <div style={{ padding: "6px 10px", borderTop: "1px solid rgba(255,255,255,0.06)" }}>
            <a
              href="/settings/providers"
              className="flex items-center gap-1.5 justify-center rounded-lg py-1.5 text-[10px] font-medium transition-all"
              style={{ color: "var(--yunque-text-muted)", background: "rgba(255,255,255,0.02)" }}
              onMouseEnter={e => { (e.currentTarget as HTMLElement).style.color = "var(--yunque-accent)"; (e.currentTarget as HTMLElement).style.background = "rgba(0,111,238,0.06)"; }}
              onMouseLeave={e => { (e.currentTarget as HTMLElement).style.color = "var(--yunque-text-muted)"; (e.currentTarget as HTMLElement).style.background = "rgba(255,255,255,0.02)"; }}
            >
              <Settings size={11} /> 管理模型提供商
            </a>
          </div>
        </div>
      )}
    </div>
  );
}
