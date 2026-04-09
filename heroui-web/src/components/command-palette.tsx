"use client";

import { useRouter } from "next/navigation";
import { useState, useEffect, useCallback, useRef } from "react";
import {
  MessageCircle, Zap, BookOpen, ScanFace, Package, Settings,
  MailWarning, Puzzle, Brain, BrainCircuit,
  Shield, ShieldCheck, BarChart3, Globe, Blocks, HardDriveDownload,
  Terminal, Cpu, LayoutDashboard, Wrench, SmilePlus, HeartPulse,
  Lightbulb, Share2, Search, FileText, ArrowRight,
} from "lucide-react";
import { api, SearchResult } from "@/lib/api";

interface CommandItem {
  id: string;
  label: string;
  group: string;
  icon: React.ReactNode;
  action: () => void;
  keywords?: string;
}

const NAV_ITEMS: Omit<CommandItem, "action">[] = [
  { id: "nav-dashboard", label: "??", group: "??", icon: <LayoutDashboard size={16} />, keywords: "dashboard home ?? ???" },
  { id: "nav-chat", label: "??", group: "??", icon: <MessageCircle size={16} />, keywords: "chat ?? ??" },
  { id: "nav-missions", label: "????", group: "??", icon: <Zap size={16} />, keywords: "missions tasks ??" },
  { id: "nav-task-run", label: "????", group: "??", icon: <Terminal size={16} />, keywords: "task run ?? ??" },
  { id: "nav-workflows", label: "???", group: "??", icon: <Blocks size={16} />, keywords: "workflow ??? ???" },
  { id: "nav-inbox", label: "???", group: "??", icon: <MailWarning size={16} />, keywords: "inbox ?? ??" },
  { id: "nav-knowledge", label: "???", group: "??", icon: <BookOpen size={16} />, keywords: "knowledge ?? RAG" },
  { id: "nav-memory", label: "??", group: "??", icon: <Brain size={16} />, keywords: "memory ??" },
  { id: "nav-graph", label: "????", group: "??", icon: <Share2 size={16} />, keywords: "graph ?? ?? ??" },
  { id: "nav-reflect", label: "??", group: "??", icon: <Lightbulb size={16} />, keywords: "reflect ?? ??" },
  { id: "nav-persona", label: "??", group: "??", icon: <ScanFace size={16} />, keywords: "persona ?? ??" },
  { id: "nav-emotions", label: "??", group: "??", icon: <SmilePlus size={16} />, keywords: "emotion ?? ??" },
  { id: "nav-reverie", label: "????", group: "??", icon: <BrainCircuit size={16} />, keywords: "reverie ?? ??" },
  { id: "nav-heartbeat", label: "??", group: "??", icon: <HeartPulse size={16} />, keywords: "heartbeat ??" },
  { id: "nav-skills", label: "??", group: "??", icon: <Package size={16} />, keywords: "skills ??" },
  { id: "nav-plugins", label: "??", group: "??", icon: <Puzzle size={16} />, keywords: "plugins ??" },
  { id: "nav-tools", label: "??", group: "??", icon: <Wrench size={16} />, keywords: "tools terminal shell ??" },
  { id: "nav-browser", label: "???", group: "??", icon: <Globe size={16} />, keywords: "browser ??? connector" },
  { id: "nav-models", label: "????", group: "??", icon: <Cpu size={16} />, keywords: "models ?? LLM" },
  { id: "nav-providers", label: "???", group: "??", icon: <Globe size={16} />, keywords: "providers ??? api key" },
  { id: "nav-metrics", label: "??", group: "??", icon: <BarChart3 size={16} />, keywords: "metrics ?? ??" },
  { id: "nav-audit", label: "??", group: "??", icon: <Shield size={16} />, keywords: "audit ?? ??" },
  { id: "nav-trust", label: "??", group: "??", icon: <ShieldCheck size={16} />, keywords: "trust ?? ??" },
  { id: "nav-backup", label: "??", group: "??", icon: <HardDriveDownload size={16} />, keywords: "backup ?? ??" },
  { id: "nav-settings", label: "??", group: "??", icon: <Settings size={16} />, keywords: "settings ?? ??" },
];

export default function CommandPalette() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [activeIdx, setActiveIdx] = useState(0);
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const timerRef = useRef<NodeJS.Timeout | undefined>(undefined);

  const close = useCallback(() => {
    setOpen(false);
    setQuery("");
    setSearchResults([]);
    setActiveIdx(0);
  }, []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((prev) => !prev);
      }
      if (e.key === "Escape") close();
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [close]);

  useEffect(() => {
    if (open) {
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [open]);

  useEffect(() => {
    if (!query || query.length < 2) {
      setSearchResults([]);
      return;
    }
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(async () => {
      setSearching(true);
      try {
        const res = await api.search(query, 5);
        setSearchResults(res.results || []);
      } catch {
        setSearchResults([]);
      } finally {
        setSearching(false);
      }
    }, 250);
    return () => clearTimeout(timerRef.current);
  }, [query]);

  const navCommands: CommandItem[] = NAV_ITEMS.map((item) => ({
    ...item,
    action: () => {
      const href = item.id.replace("nav-", "/").replace("providers", "settings/providers");
      router.push(href);
      close();
    },
  }));

  const q = query.toLowerCase();
  const filteredNav = q
    ? navCommands.filter((c) => c.label.toLowerCase().includes(q) || (c.keywords && c.keywords.toLowerCase().includes(q)))
    : navCommands;

  const allItems: CommandItem[] = [
    ...filteredNav,
    ...searchResults.map((r, i) => ({
      id: `search-${i}`,
      label: r.title || r.content.slice(0, 60),
      group: "????",
      icon: <FileText size={16} />,
      action: () => {
        if (r.type === "memory") router.push("/memory");
        else if (r.type === "knowledge") router.push("/knowledge");
        else if (r.type === "task") router.push(`/task-run?id=${r.id}`);
        else router.push("/dashboard");
        close();
      },
      keywords: r.content,
    })),
  ];

  useEffect(() => {
    setActiveIdx(0);
  }, [query]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIdx((prev) => Math.min(prev + 1, allItems.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIdx((prev) => Math.max(prev - 1, 0));
    } else if (e.key === "Enter" && allItems[activeIdx]) {
      e.preventDefault();
      allItems[activeIdx].action();
    }
  };

  useEffect(() => {
    const el = listRef.current?.querySelector(`[data-idx="${activeIdx}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [activeIdx]);

  if (!open) return null;

  const groups: Record<string, CommandItem[]> = {};
  for (const item of allItems) {
    (groups[item.group] ??= []).push(item);
  }

  let flatIdx = 0;

  return (
    <div className="cmd-overlay" onClick={close}>
      <div className="cmd-palette" onClick={(e) => e.stopPropagation()}>
        <div className="cmd-input-wrap">
          <Search size={16} style={{ color: "var(--yunque-text-muted)", flexShrink: 0 }} />
          <input
            ref={inputRef}
            className="cmd-input"
            placeholder="?????????????..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
          />
          <kbd className="cmd-kbd">ESC</kbd>
        </div>

        <div className="cmd-list" ref={listRef}>
          {allItems.length === 0 && (
            <div className="cmd-empty">{searching ? "???..." : "??????"}</div>
          )}
          {Object.entries(groups).map(([group, items]) => (
            <div key={group}>
              <div className="cmd-group-label">{group}</div>
              {items.map((item) => {
                const idx = flatIdx++;
                return (
                  <button
                    key={item.id}
                    data-idx={idx}
                    className="cmd-item"
                    data-active={idx === activeIdx || undefined}
                    onClick={item.action}
                    onMouseEnter={() => setActiveIdx(idx)}
                  >
                    <span className="cmd-item-icon">{item.icon}</span>
                    <span className="cmd-item-label">{item.label}</span>
                    {idx === activeIdx && <ArrowRight size={12} className="cmd-item-arrow" />}
                  </button>
                );
              })}
            </div>
          ))}
        </div>

        <div className="cmd-footer">
          <span><kbd className="cmd-kbd-sm">??</kbd> ??</span>
          <span><kbd className="cmd-kbd-sm">Enter</kbd> ??</span>
          <span><kbd className="cmd-kbd-sm">ESC</kbd> ??</span>
        </div>
      </div>
    </div>
  );
}
