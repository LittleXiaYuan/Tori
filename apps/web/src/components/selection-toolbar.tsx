"use client";

import { useState, useEffect, useCallback, useRef, useLayoutEffect } from "react";
import { Search, Languages, Lightbulb, BookOpen, Copy, GripVertical } from "lucide-react";

export interface SelectionAction {
  id: string;
  icon: typeof Search;
  label: string;
}

export const selectionActions: SelectionAction[] = [
  { id: "ai_search", icon: Search, label: "AI 搜索" },
  { id: "translate", icon: Languages, label: "翻译" },
  { id: "explain", icon: Lightbulb, label: "解释" },
  { id: "save", icon: BookOpen, label: "存储" },
  { id: "copy", icon: Copy, label: "复制" },
];

interface ToolbarPos {
  x: number;
  y: number;
  placement: "above" | "below";
}

function computeToolbarPos(rect: DOMRect, w: number, h: number): ToolbarPos {
  const pad = 4;
  const vw = window.innerWidth;
  let x = rect.left + rect.width / 2;
  let placement: "above" | "below" = "above";
  let y = rect.top - pad;

  if (rect.top < h + pad + 12) {
    placement = "below";
    y = rect.bottom + pad;
  }

  const half = w / 2;
  x = Math.max(half + 4, Math.min(vw - half - 4, x));
  return { x, y, placement };
}

async function copyText(text: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.cssText = "position:fixed;left:-9999px;top:0;opacity:0";
    document.body.appendChild(ta);
    ta.select();
    document.execCommand("copy");
    document.body.removeChild(ta);
  }
}

interface SelectionToolbarBarProps {
  text: string;
  pos?: ToolbarPos;
  onAction: (action: string, text: string) => void;
  onDismiss?: () => void;
  className?: string;
  inline?: boolean;
}

function clearDocumentSelection(): void {
  const sel = window.getSelection();
  if (sel && sel.rangeCount > 0) {
    sel.removeAllRanges();
  }
}

export function SelectionToolbarBar({
  text,
  pos,
  onAction,
  onDismiss,
  className = "",
  inline = false,
}: SelectionToolbarBarProps) {
  const barRef = useRef<HTMLDivElement>(null);

  const finish = useCallback(() => {
    clearDocumentSelection();
    onDismiss?.();
  }, [onDismiss]);

  const handleAction = useCallback(
    (actionId: string) => {
      if (actionId === "copy") {
        void copyText(text).then(() => finish());
        return;
      }
      onAction(actionId, text);
      finish();
    },
    [text, onAction, finish],
  );

  const posStyle = inline || !pos
    ? undefined
    : {
        left: pos.x,
        top: pos.y,
        transform: pos.placement === "above" ? "translate(-50%, -100%)" : "translate(-50%, 0)",
      };

  return (
    <div
      ref={barRef}
      className={`selection-toolbar ${inline ? "selection-toolbar--inline" : "fixed z-[150]"} ${className}`}
      style={posStyle}
      onMouseDown={(e) => e.preventDefault()}
    >
      <div className="selection-toolbar__bar flex items-center gap-0.5 rounded-lg px-1 py-0.5">
        <span className="selection-toolbar__grip" aria-hidden>
          <GripVertical size={14} />
        </span>
        {selectionActions.map(({ id, icon: Icon, label }) => (
          <button
            key={id}
            type="button"
            onMouseDown={(e) => {
              e.preventDefault();
              e.stopPropagation();
              handleAction(id);
            }}
            className="selection-toolbar__btn flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium whitespace-nowrap"
          >
            <Icon size={14} strokeWidth={1.75} />
            <span>{label}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

interface SelectionToolbarProps {
  onAction: (action: string, text: string) => void;
}

export function SelectionToolbar({ onAction }: SelectionToolbarProps) {
  const [visible, setVisible] = useState(false);
  const [pos, setPos] = useState<ToolbarPos>({ x: 0, y: 0, placement: "above" });
  const [selectedText, setSelectedText] = useState("");
  const suppressUntilRef = useRef(0);
  const rafRef = useRef(0);

  const updateFromSelection = useCallback(() => {
    if (Date.now() < suppressUntilRef.current) {
      return;
    }
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || sel.rangeCount === 0) {
      setVisible(false);
      return;
    }

    const text = sel.toString().trim();
    if (text.length < 2) {
      setVisible(false);
      return;
    }

    const range = sel.getRangeAt(0);
    const rect = range.getBoundingClientRect();
    if (rect.width === 0 && rect.height === 0) {
      setVisible(false);
      return;
    }

    const next = computeToolbarPos(rect, 380, 40);
    setSelectedText(text);
    setPos(next);
    setVisible(true);
  }, []);

  const scheduleUpdate = useCallback(() => {
    cancelAnimationFrame(rafRef.current);
    rafRef.current = requestAnimationFrame(updateFromSelection);
  }, [updateFromSelection]);

  useEffect(() => {
    const onPointerUp = () => scheduleUpdate();
    const onKeyUp = (e: KeyboardEvent) => {
      if (e.key === "Shift" || e.key.startsWith("Arrow") || e.key === "Home" || e.key === "End") {
        scheduleUpdate();
      }
    };
    const onSelectionChange = () => {
      const sel = window.getSelection();
      if (!sel || sel.isCollapsed) {
        setVisible(false);
      }
    };

    document.addEventListener("pointerup", onPointerUp);
    document.addEventListener("keyup", onKeyUp);
    document.addEventListener("selectionchange", onSelectionChange);

    return () => {
      cancelAnimationFrame(rafRef.current);
      document.removeEventListener("pointerup", onPointerUp);
      document.removeEventListener("keyup", onKeyUp);
      document.removeEventListener("selectionchange", onSelectionChange);
    };
  }, [scheduleUpdate]);

  useEffect(() => {
    if (!visible) return;
    const onPointerDown = (e: PointerEvent) => {
      const target = e.target as Node;
      if (document.querySelector(".selection-toolbar")?.contains(target)) return;
      setVisible(false);
    };
    document.addEventListener("pointerdown", onPointerDown, true);
    return () => document.removeEventListener("pointerdown", onPointerDown, true);
  }, [visible]);

  useLayoutEffect(() => {
    if (!visible) return;
    const sel = window.getSelection();
    if (!sel || sel.rangeCount === 0) return;
    const rect = sel.getRangeAt(0).getBoundingClientRect();
    const bar = document.querySelector(".selection-toolbar__bar");
    const w = bar?.getBoundingClientRect().width ?? 380;
    const h = bar?.getBoundingClientRect().height ?? 40;
    setPos(computeToolbarPos(rect, w, h));
  }, [visible, selectedText]);

  if (!visible) return null;

  return (
    <SelectionToolbarBar
      text={selectedText}
      pos={pos}
      onAction={onAction}
      onDismiss={() => {
        suppressUntilRef.current = Date.now() + 900;
        setVisible(false);
      }}
    />
  );
}
