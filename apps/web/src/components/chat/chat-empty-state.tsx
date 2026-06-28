"use client";

import { type ReactNode, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { AlertTriangle, Pencil } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import type { ChatDispatch } from "@/lib/chat-state";
import { api } from "@/lib/api";
import { buildHero, getNickname, setNickname } from "@/lib/chat-hero";

interface ChatEmptyStateProps {
  setupNeeded: boolean;
  // chatD/inputRef are kept on the interface for callers that still pass them;
  // the centered empty state no longer uses them (no starter chips).
  chatD?: ChatDispatch;
  inputRef?: React.RefObject<HTMLTextAreaElement | null>;
  /** The chat composer, injected by the page so it can be centered here when
   *  there is no conversation yet. */
  composer: ReactNode;
}

export function ChatEmptyState({ setupNeeded, composer, chatD, inputRef }: ChatEmptyStateProps) {
  const { t } = useI18n();
  const [nickname, setNick] = useState<string | null>(null);
  const [recentTitle, setRecent] = useState<string | null>(null);
  const [hydrated, setHydrated] = useState(false);

  // Load nickname + most-recent conversation on mount.
  useEffect(() => {
    setNick(getNickname());
    api
      .conversations(false)
      .then((res) => {
        const list = res.sessions || [];
        // Prefer a session with a real human-set name; fall back to summary
        // when the user hasn't named the thread yet.
        const pick = list.find((c) => c.name && !c.name.startsWith("new-")) || list[0];
        const title = pick?.name && !pick.name.startsWith("new-")
          ? pick.name
          : pick?.summary?.trim() || null;
        setRecent(title);
      })
      .catch(() => setRecent(null))
      .finally(() => setHydrated(true));
  }, []);

  const hero = hydrated
    ? buildHero({ nickname, recentTitle })
    : t("chat.empty.heroTitle");

  const editNickname = useCallback(() => {
    const next = window.prompt("怎么称呼你？", nickname || "");
    if (next === null) return;
    setNickname(next);
    setNick(next.trim() || null);
  }, [nickname]);

  // 点击建议把文案填进输入框并聚焦，让用户在此基础上改或直接发。
  const fillPrompt = useCallback((text: string) => {
    chatD?.({ type: "SET_INPUT", value: text });
    inputRef?.current?.focus();
  }, [chatD, inputRef]);

  // 对话式建议（用户视角，不暴露功能/架构名）。有上次对话时动态加一条续聊。
  const suggestions: { label: string; prompt: string }[] = [
    ...(recentTitle ? [{ label: `继续：${recentTitle.length > 12 ? recentTitle.slice(0, 12) + "…" : recentTitle}`, prompt: `我们继续上次「${recentTitle}」的话题吧。` }] : []),
    { label: "帮我写点东西", prompt: "帮我写一段" },
    { label: "问个问题", prompt: "" },
    { label: "理理思路", prompt: "帮我梳理一下这件事的思路：" },
  ];

  return (
    <div className="chat-empty chat-empty--centered">
      {/* Soft dark-blue radial glow centered behind the prompt. Pure decoration. */}
      <div aria-hidden className="chat-empty__glow" />

      {setupNeeded && (
        <div className="chat-empty__setup">
          <div className="chat-empty__setup-title">
            <AlertTriangle size={15} style={{ color: "#f59e0b" }} /> {t("chat.empty.setupTitle")}
          </div>
          <p className="chat-empty__setup-desc">{t("chat.empty.setupDesc")}</p>
          <Link href="/setup" className="chat-empty__setup-link">{t("chat.empty.setupLink")}</Link>
        </div>
      )}

      <div className="chat-empty__center">
        <h1 className="chat-empty__hero-title" suppressHydrationWarning>
          <span>{hero}</span>
          <button
            type="button"
            className="chat-empty__hero-edit"
            onClick={editNickname}
            aria-label="设置称呼"
            title="设置称呼"
          >
            <Pencil size={12} />
          </button>
        </h1>
        <div className="chat-empty__suggestions">
          {suggestions.map((s) => (
            <button
              key={s.label}
              type="button"
              className="chat-empty__suggestion"
              onClick={() => fillPrompt(s.prompt)}
            >
              {s.label}
            </button>
          ))}
        </div>
        <div className="chat-empty__composer chat-empty__composer--centered">{composer}</div>
      </div>
    </div>
  );
}
