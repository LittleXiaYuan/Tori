"use client";

import { useEffect, useState, useCallback } from "react";

export interface ShortcutDef {
  id: string;
  label: string;
  labelEn: string;
  defaultKey: string;
  category: string;
}

const STORAGE_KEY = "yunque_shortcuts";

export const defaultShortcuts: ShortcutDef[] = [
  { id: "new_chat", label: "新对话", labelEn: "New chat", defaultKey: "Alt+N", category: "chat" },
  { id: "search", label: "搜索", labelEn: "Search", defaultKey: "Ctrl+K", category: "global" },
  { id: "stop", label: "停止生成", labelEn: "Stop generation", defaultKey: "Escape", category: "chat" },
  { id: "focus_input", label: "聚焦输入", labelEn: "Focus input", defaultKey: "Alt+I", category: "chat" },
  { id: "toggle_sidebar", label: "切换侧边栏", labelEn: "Toggle sidebar", defaultKey: "Alt+S", category: "global" },
  { id: "toggle_computer", label: "切换计算机面板", labelEn: "Toggle computer", defaultKey: "Alt+C", category: "chat" },
  { id: "screenshot_analyze", label: "截图分析", labelEn: "Screenshot & analyze", defaultKey: "Alt+P", category: "chat" },
  { id: "copy_last", label: "复制上条回复", labelEn: "Copy last reply", defaultKey: "Alt+Shift+C", category: "chat" },
  { id: "prev_conv", label: "上一个对话", labelEn: "Previous chat", defaultKey: "Alt+ArrowUp", category: "chat" },
  { id: "next_conv", label: "下一个对话", labelEn: "Next chat", defaultKey: "Alt+ArrowDown", category: "chat" },
];

function parseKey(shortcut: string): { ctrl: boolean; alt: boolean; shift: boolean; meta: boolean; key: string } {
  const parts = shortcut.split("+").map((s) => s.trim().toLowerCase());
  return {
    ctrl: parts.includes("ctrl"),
    alt: parts.includes("alt"),
    shift: parts.includes("shift"),
    meta: parts.includes("meta") || parts.includes("cmd"),
    key: parts.filter((p) => !["ctrl", "alt", "shift", "meta", "cmd"].includes(p))[0] || "",
  };
}

function matchEvent(e: KeyboardEvent, parsed: ReturnType<typeof parseKey>): boolean {
  const eventKey = e.key.toLowerCase();
  return (
    e.ctrlKey === parsed.ctrl &&
    e.altKey === parsed.alt &&
    e.shiftKey === parsed.shift &&
    e.metaKey === parsed.meta &&
    (eventKey === parsed.key || e.code.toLowerCase() === parsed.key)
  );
}

export type ShortcutMap = Record<string, string>;

function loadCustomKeys(): ShortcutMap {
  if (typeof window === "undefined") return {};
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

export function useShortcuts(handlers: Record<string, () => void>) {
  const [customKeys, setCustomKeys] = useState<ShortcutMap>(loadCustomKeys);

  const getKey = useCallback(
    (id: string) => customKeys[id] || defaultShortcuts.find((s) => s.id === id)?.defaultKey || "",
    [customKeys],
  );

  const updateKey = useCallback((id: string, newKey: string) => {
    setCustomKeys((prev) => {
      const next = { ...prev, [id]: newKey };
      localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
      return next;
    });
  }, []);

  const resetAll = useCallback(() => {
    setCustomKeys({});
    localStorage.removeItem(STORAGE_KEY);
  }, []);

  useEffect(() => {
    const parsed = defaultShortcuts.map((def) => ({
      id: def.id,
      parsed: parseKey(customKeys[def.id] || def.defaultKey),
    }));

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        if (e.key !== "Escape") return;
      }
      for (const { id, parsed: p } of parsed) {
        if (matchEvent(e, p) && handlers[id]) {
          e.preventDefault();
          handlers[id]();
          return;
        }
      }
    };

    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [handlers, customKeys]);

  return { shortcuts: defaultShortcuts, getKey, updateKey, resetAll };
}
