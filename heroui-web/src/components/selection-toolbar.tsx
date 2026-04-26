"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { Search, Languages, Lightbulb, BookOpen, Copy, Sparkles } from "lucide-react";

interface SelectionToolbarProps {
  onAction: (action: string, text: string) => void;
}

const actions = [
  { id: "ai_search", icon: Search, label: "AI 搜索", labelEn: "AI Search" },
  { id: "translate", icon: Languages, label: "翻译", labelEn: "Translate" },
  { id: "explain", icon: Lightbulb, label: "解释", labelEn: "Explain" },
  { id: "save", icon: BookOpen, label: "存储", labelEn: "Save" },
  { id: "copy", icon: Copy, label: "复制", labelEn: "Copy" },
];

export function SelectionToolbar({ onAction }: SelectionToolbarProps) {
  const [visible, setVisible] = useState(false);
  const [pos, setPos] = useState({ x: 0, y: 0 });
  const [selectedText, setSelectedText] = useState("");
  const toolbarRef = useRef<HTMLDivElement>(null);

  const handleSelection = useCallback(() => {
    const sel = window.getSelection();
    const text = sel?.toString().trim() || "";

    if (text.length < 2) {
      setVisible(false);
      return;
    }

    const range = sel?.getRangeAt(0);
    if (!range) return;

    const mainContent = document.getElementById("main-content");
    if (!mainContent?.contains(range.commonAncestorContainer)) return;

    const rect = range.getBoundingClientRect();
    setSelectedText(text);
    setPos({
      x: rect.left + rect.width / 2,
      y: rect.top - 8,
    });
    setVisible(true);
  }, []);

  useEffect(() => {
    document.addEventListener("mouseup", handleSelection);
    document.addEventListener("keyup", handleSelection);

    const hideOnScroll = () => setVisible(false);
    document.addEventListener("scroll", hideOnScroll, true);

    return () => {
      document.removeEventListener("mouseup", handleSelection);
      document.removeEventListener("keyup", handleSelection);
      document.removeEventListener("scroll", hideOnScroll, true);
    };
  }, [handleSelection]);

  useEffect(() => {
    if (!visible) return;
    const onClick = (e: MouseEvent) => {
      if (toolbarRef.current?.contains(e.target as Node)) return;
      setVisible(false);
    };
    const timer = setTimeout(() => document.addEventListener("mousedown", onClick), 50);
    return () => { clearTimeout(timer); document.removeEventListener("mousedown", onClick); };
  }, [visible]);

  const handleClick = useCallback((actionId: string) => {
    if (actionId === "copy") {
      navigator.clipboard.writeText(selectedText);
      setVisible(false);
      return;
    }
    onAction(actionId, selectedText);
    setVisible(false);
  }, [selectedText, onAction]);

  if (!visible) return null;

  return (
    <div
      ref={toolbarRef}
      className="fixed z-[150] animate-fade-in-up"
      style={{
        left: pos.x,
        top: pos.y,
        transform: "translate(-50%, -100%)",
      }}
    >
      <div
        className="flex items-center gap-0.5 rounded-xl px-1.5 py-1"
        style={{
          background: "rgba(255,255,255,0.95)",
          boxShadow: "0 8px 32px rgba(0,0,0,0.15), 0 0 0 1px rgba(0,0,0,0.06)",
          backdropFilter: "blur(12px)",
        }}
      >
        {actions.map(({ id, icon: Icon, label }) => (
          <button
            key={id}
            onClick={() => handleClick(id)}
            className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[12px] font-medium transition-colors hover:bg-gray-100"
            style={{ color: "#374151" }}
          >
            <Icon size={14} style={{ color: "#6b7280" }} />
            {label}
          </button>
        ))}
      </div>
      <div
        className="mx-auto w-2 h-2 rotate-45"
        style={{ background: "rgba(255,255,255,0.95)", marginTop: -1 }}
      />
    </div>
  );
}
