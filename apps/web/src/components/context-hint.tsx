"use client";

import { useState, useEffect, useCallback } from "react";
import { Lightbulb, X, ArrowRight } from "lucide-react";
import { useRouter } from "next/navigation";

const DISMISSED_KEY = "yunque_hints_dismissed";

interface ContextHintProps {
  id: string;
  title: string;
  description: string;
  actionLabel: string;
  actionHref?: string;
  onAction?: () => void;
  autoHideMs?: number;
}

function getDismissedHints(): Set<string> {
  if (typeof window === "undefined") return new Set();
  try {
    const raw = localStorage.getItem(DISMISSED_KEY);
    return raw ? new Set(JSON.parse(raw)) : new Set();
  } catch { return new Set(); }
}

function dismissHint(id: string) {
  const set = getDismissedHints();
  set.add(id);
  localStorage.setItem(DISMISSED_KEY, JSON.stringify([...set]));
}

export function ContextHint({ id, title, description, actionLabel, actionHref, onAction, autoHideMs = 10000 }: ContextHintProps) {
  const router = useRouter();
  const [visible, setVisible] = useState(false);
  const [exiting, setExiting] = useState(false);

  useEffect(() => {
    if (getDismissedHints().has(id)) return;
    const timer = setTimeout(() => setVisible(true), 500);
    return () => clearTimeout(timer);
  }, [id]);

  useEffect(() => {
    if (!visible || autoHideMs <= 0) return;
    const timer = setTimeout(() => handleClose(), autoHideMs);
    return () => clearTimeout(timer);
  }, [visible, autoHideMs]);

  const handleClose = useCallback(() => {
    setExiting(true);
    setTimeout(() => {
      setVisible(false);
      setExiting(false);
    }, 200);
  }, []);

  const handleDismissForever = useCallback(() => {
    dismissHint(id);
    handleClose();
  }, [id, handleClose]);

  const handleAction = useCallback(() => {
    dismissHint(id);
    if (onAction) onAction();
    else if (actionHref) router.push(actionHref);
    handleClose();
  }, [id, onAction, actionHref, router, handleClose]);

  if (!visible) return null;

  return (
    <div
      className="context-hint-toast"
      data-exiting={exiting || undefined}
      role="status"
      aria-live="polite"
    >
      <div className="flex items-start gap-3">
        <Lightbulb size={16} style={{ color: "var(--yunque-accent)", marginTop: 2, flexShrink: 0 }} />
        <div className="flex-1 min-w-0">
          <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{title}</div>
          <div className="text-xs mt-1 leading-relaxed" style={{ color: "var(--yunque-text-secondary)" }}>{description}</div>
          <div className="mt-3 flex items-center gap-3">
            <button
              onClick={handleAction}
              className="flex items-center gap-1 text-xs font-medium"
              style={{ color: "var(--yunque-accent)" }}
            >
              {actionLabel} <ArrowRight size={10} />
            </button>
            <button
              onClick={handleDismissForever}
              className="text-xs"
              style={{ color: "var(--yunque-text-muted)" }}
            >
              不再提示
            </button>
          </div>
        </div>
        <button onClick={handleClose} className="p-1 rounded" style={{ color: "var(--yunque-text-disabled)" }} aria-label="关闭提示">
          <X size={12} />
        </button>
      </div>
    </div>
  );
}
