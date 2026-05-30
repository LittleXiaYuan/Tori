"use client";

import { useEffect, useState, type ReactNode } from "react";
import { Sparkles, AlertTriangle } from "lucide-react";
import { api } from "@/lib/api";
import type { ChatDispatch } from "@/lib/chat-state";
import { PRODUCT_SCENARIOS } from "@/lib/product-scenarios";

interface StarterChip {
  label: string;
  prompt: string;
}

interface ChatEmptyStateProps {
  setupNeeded: boolean;
  chatD: ChatDispatch;
  inputRef: React.RefObject<HTMLTextAreaElement | null>;
  /** The chat composer, injected by the page so it can be centered here when
   *  there is no conversation yet (Claude.ai-style empty screen). */
  composer: ReactNode;
}

const FALLBACK_CHIPS: StarterChip[] = PRODUCT_SCENARIOS.slice(0, 4).map((s) => ({
  label: s.label,
  prompt: s.prompt,
}));

function greeting(): string {
  const h = new Date().getHours();
  if (h < 6) return "夜深了";
  if (h < 11) return "早上好";
  if (h < 13) return "中午好";
  if (h < 18) return "下午好";
  return "晚上好";
}

export function ChatEmptyState({ setupNeeded, chatD, inputRef, composer }: ChatEmptyStateProps) {
  // null = still loading (show skeleton); array = resolved (LLM or fallback).
  const [chips, setChips] = useState<StarterChip[] | null>(null);

  useEffect(() => {
    let alive = true;
    // Don't spend an LLM call before a model is even configured — go straight
    // to the curated set so the screen is still useful during onboarding.
    if (setupNeeded) {
      setChips(FALLBACK_CHIPS);
      return;
    }
    api
      .starterSuggestions()
      .then((res) => {
        if (!alive) return;
        setChips(res.suggestions?.length ? res.suggestions : FALLBACK_CHIPS);
      })
      .catch(() => {
        if (alive) setChips(FALLBACK_CHIPS);
      });
    return () => {
      alive = false;
    };
  }, [setupNeeded]);

  const pickChip = (prompt: string) => {
    chatD({ type: "SET_INPUT", value: prompt });
    inputRef.current?.focus();
  };

  return (
    <div className="chat-empty">
      {setupNeeded && (
        <div className="chat-empty__setup">
          <div className="chat-empty__setup-title">
            <AlertTriangle size={15} style={{ color: "#f59e0b" }} /> 先完成模型配置
          </div>
          <p className="chat-empty__setup-desc">请先在设置中添加模型提供商 API Key，再开始第一轮对话。</p>
          <a href="/settings/providers" className="chat-empty__setup-link">前往配置提供商 →</a>
        </div>
      )}

      <div className="chat-empty__hero">
        <span className="chat-empty__mark">
          <Sparkles size={22} style={{ color: "var(--yunque-accent)" }} />
        </span>
        <h1 className="chat-empty__greeting">{greeting()}</h1>
        <p className="chat-empty__sub">我是云雀，有什么可以帮你的？</p>
      </div>

      <div className="chat-empty__composer">{composer}</div>

      <div className="chat-empty__chips" aria-label="开场建议">
        {chips === null
          ? Array.from({ length: 4 }).map((_, i) => (
              <span key={i} className="chat-empty__chip chat-empty__chip--skeleton" aria-hidden />
            ))
          : chips.map((chip) => (
              <button
                key={chip.label}
                type="button"
                className="chat-empty__chip"
                onClick={() => pickChip(chip.prompt)}
                title={chip.prompt}
              >
                {chip.label}
              </button>
            ))}
      </div>
    </div>
  );
}
