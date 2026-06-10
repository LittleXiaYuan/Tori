"use client";

import { useEffect, useState, type ReactNode } from "react";
import { AlertTriangle } from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
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

/** Aurora (northern-lights) brand mark — flat monoline curtains in a single
 *  accent color (no gradient), so it reads as a calm, friendly glyph. */
function AuroraMark() {
  return (
    <svg viewBox="0 0 48 48" width="26" height="26" fill="none" aria-hidden>
      <g stroke="currentColor" strokeLinecap="round" fill="none">
        <path d="M14 38 C 11 28, 19 23, 15 10" strokeWidth="2.4" opacity="0.95" />
        <path d="M24 40 C 21 27, 30 21, 25 9" strokeWidth="2.4" opacity="0.7" />
        <path d="M34 38 C 33 28, 39 23, 35 12" strokeWidth="2.4" opacity="0.48" />
      </g>
    </svg>
  );
}

/** Returns the time-of-day bucket key; the caller localizes it via i18n. */
function useGreetingKey(): string {
  const [key, setKey] = useState("hello");
  useEffect(() => {
    const h = new Date().getHours();
    setKey(h < 5 ? "late" : h < 11 ? "morning" : h < 13 ? "noon" : h < 18 ? "afternoon" : "evening");
  }, []);
  return key;
}

export function ChatEmptyState({ setupNeeded, chatD, inputRef, composer }: ChatEmptyStateProps) {
  const { t } = useI18n();
  // null = still loading (show skeleton); array = resolved (LLM or fallback).
  const [chips, setChips] = useState<StarterChip[] | null>(null);
  const greetingKey = useGreetingKey();
  const greeting = t(`chat.empty.greet.${greetingKey}`);

  useEffect(() => {
    let alive = true;
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
            <AlertTriangle size={15} style={{ color: "#f59e0b" }} /> {t("chat.empty.setupTitle")}
          </div>
          <p className="chat-empty__setup-desc">{t("chat.empty.setupDesc")}</p>
          <a href="/settings/providers" className="chat-empty__setup-link">{t("chat.empty.setupLink")}</a>
        </div>
      )}

      <div className="chat-empty__hero">
        <div className="chat-empty__hello-row">
          <span className="chat-empty__mark" aria-hidden>
            <AuroraMark />
          </span>
          <h1 className="chat-empty__greeting">{t("chat.empty.greetTpl").replace("{g}", greeting)}</h1>
        </div>
      </div>

      <div className="chat-empty__composer">{composer}</div>

      <div className="chat-empty__chips" aria-label={t("chat.empty.suggestions")}>
        <span className="chat-empty__chips-label">{t("chat.empty.try")}</span>
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
