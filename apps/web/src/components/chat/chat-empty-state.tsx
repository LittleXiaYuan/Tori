"use client";

import { useMemo, useState, type ReactNode } from "react";
import { AlertTriangle } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import type { ChatDispatch } from "@/lib/chat-state";
import { CHAT_AGENT_SCENES, CHAT_EMPTY_SCENARIOS, PRODUCT_SCENARIOS, type ProductScenario } from "@/lib/product-scenarios";

interface StarterChip {
  id: string;
  label: string;
  description: string;
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

/** Scenario ids used as offline/fallback starter chips. Labels and prompts are
 *  localized at render via i18n so English users don't see Chinese chips. */
const FALLBACK_SCENARIO_IDS = CHAT_EMPTY_SCENARIOS.map((s) => s.id);
const SCENARIO_BY_ID = new Map(PRODUCT_SCENARIOS.map((s) => [s.id, s]));

/** Aurora (northern-lights) brand mark — flat monoline curtains in a single
 *  accent color (no gradient), so it reads as a calm, friendly glyph. */
function AuroraMark() {
  return (
    <svg viewBox="0 0 48 48" width="30" height="30" fill="none" aria-hidden>
      <g stroke="currentColor" strokeLinecap="round" fill="none">
        <path d="M14 38 C 11 28, 19 23, 15 10" strokeWidth="2.4" opacity="0.95" />
        <path d="M24 40 C 21 27, 30 21, 25 9" strokeWidth="2.4" opacity="0.7" />
        <path d="M34 38 C 33 28, 39 23, 35 12" strokeWidth="2.4" opacity="0.48" />
      </g>
    </svg>
  );
}

/** Maps the current hour to a time-of-day bucket key. */
function greetingKeyForNow(): string {
  const h = new Date().getHours();
  return h < 5 ? "late" : h < 11 ? "morning" : h < 13 ? "noon" : h < 18 ? "afternoon" : "evening";
}

/** Returns the time-of-day bucket key; the caller localizes it via i18n.
 *  Computed once on the first render (lazy init) so the empty state paints the
 *  correct greeting immediately instead of flashing from a "hello" default. */
function useGreetingKey(): string {
  const [key] = useState(greetingKeyForNow);
  return key;
}

export function ChatEmptyState({ setupNeeded, chatD, inputRef, composer }: ChatEmptyStateProps) {
  const { t } = useI18n();
  const [activeSceneId, setActiveSceneId] = useState(CHAT_AGENT_SCENES[0]?.id || "general");
  const greetingKey = useGreetingKey();
  const greeting = t(`chat.empty.greet.${greetingKey}`);
  const activeScene = CHAT_AGENT_SCENES.find((scene) => scene.id === activeSceneId) || CHAT_AGENT_SCENES[0];
  const chips = useMemo<StarterChip[]>(() => {
    const ids = activeScene?.promptIds?.length ? activeScene.promptIds : FALLBACK_SCENARIO_IDS;
    return ids
      .map((id) => SCENARIO_BY_ID.get(id))
      .filter((s): s is ProductScenario => Boolean(s))
      .map((s) => ({
        id: s.id,
        label: t(`scenario.${s.id}.label`),
        description: s.description,
        prompt: t(`scenario.${s.id}.prompt`),
      }));
  }, [activeScene, t]);

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
          <a href="/setup" className="chat-empty__setup-link">{t("chat.empty.setupLink")}</a>
        </div>
      )}

      <div className="chat-empty__hero">
        <div className="chat-empty__hello-row">
          <span className="chat-empty__mark" aria-hidden>
            <AuroraMark />
          </span>
          <h1 className="chat-empty__greeting" suppressHydrationWarning>{t("chat.empty.greetTpl").replace("{g}", greeting)}</h1>
        </div>
        <p className="chat-empty__sub">{t("chat.empty.subtitle")}</p>
      </div>

      <div className="chat-empty__composer">{composer}</div>

      <section className="chat-empty__scene" aria-labelledby="chat-scene-title">
        <div className="chat-empty__scene-copy">
          <p className="chat-empty__scene-eyebrow">{t("chat.scene.eyebrow")}</p>
          <h2 id="chat-scene-title" className="chat-empty__scene-title">{t("chat.scene.title")}</h2>
          <p className="chat-empty__scene-desc">{t(`chat.scene.${activeScene.id}.desc`)}</p>
        </div>

        <ul className="chat-empty__scene-list" aria-label={t("chat.scene.options")}>
          {CHAT_AGENT_SCENES.map((scene) => {
            const active = scene.id === activeScene.id;
            return (
              <li key={scene.id}>
                <button
                  type="button"
                  className="chat-empty__scene-btn"
                  data-active={active ? "true" : undefined}
                  aria-current={active ? "true" : undefined}
                  onClick={() => setActiveSceneId(scene.id)}
                >
                  <span className="chat-empty__scene-icon" aria-hidden>{scene.icon}</span>
                  <span>{t(`chat.scene.${scene.id}.label`)}</span>
                </button>
              </li>
            );
          })}
        </ul>

        <ul className="chat-empty__chips" aria-label={t("chat.empty.suggestions")}>
          {chips.map((chip) => (
            <li key={chip.id}>
              <button
                type="button"
                className="chat-empty__chip"
                aria-labelledby={`chat-empty-chip-title-${chip.id}`}
                aria-describedby={`chat-empty-chip-desc-${chip.id}`}
                onClick={() => pickChip(chip.prompt)}
                title={chip.prompt}
              >
                <span id={`chat-empty-chip-title-${chip.id}`} className="chat-empty__chip-title">{chip.label}</span>
                <span id={`chat-empty-chip-desc-${chip.id}`} className="chat-empty__chip-desc">{chip.description}</span>
              </button>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
