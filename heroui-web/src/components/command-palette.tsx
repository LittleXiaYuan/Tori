"use client";

import { useRouter } from "next/navigation";
import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import {
  Puzzle, Search, FileText, ArrowRight,
} from "lucide-react";
import { api, SearchResult } from "@/lib/api";
import { NAV_ITEMS, NAV_GROUP_ORDER, type NavItem, type NavGroup } from "@/lib/nav-items";

interface CommandItem {
  id: string;
  label: string;
  group: string;
  icon: React.ReactNode;
  action: () => void;
  href?: string;
  keywords?: string;
}

export default function CommandPalette() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [activeIdx, setActiveIdx] = useState(0);
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [extItems, setExtItems] = useState<NavItem[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const timerRef = useRef<NodeJS.Timeout | undefined>(undefined);

  // Lazy-load plugin UI tabs as a "扩展" group, mirroring sidebar.tsx behaviour.
  useEffect(() => {
    api
      .pluginUITabs()
      .then((res) => {
        if (!res?.tabs?.length) return;
        setExtItems(
          res.tabs.map((t) => ({
            id: `ext-${t.key}`,
            href: `/ext/${t.key}`,
            label: t.label,
            group: "扩展",
            icon: <Puzzle size={16} />,
            keywords: `${t.label} ${t.label_en || ""} extension ${t.key}`,
          })),
        );
      })
      .catch(() => {});
  }, []);

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
    const openHandler = () => setOpen(true);
    document.addEventListener("keydown", handler);
    document.addEventListener("yunque:open-command-palette", openHandler);
    return () => {
      document.removeEventListener("keydown", handler);
      document.removeEventListener("yunque:open-command-palette", openHandler);
    };
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

  const navCommands: CommandItem[] = useMemo(() => {
    const all = [...NAV_ITEMS, ...extItems];
    return all.map((item) => ({
      ...item,
      action: () => {
        if (item.href) router.push(item.href);
        close();
      },
    }));
  }, [extItems, router, close]);

  const q = query.toLowerCase();
  const filteredNav = q
    ? navCommands.filter((c) => c.label.toLowerCase().includes(q) || (c.keywords && c.keywords.toLowerCase().includes(q)))
    : navCommands;

  const allItems: CommandItem[] = [
    ...filteredNav,
    ...searchResults.map((r, i) => ({
      id: `search-${i}`,
      label: r.title || r.content.slice(0, 60),
      group: "搜索结果",
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

  const navByGroup = useMemo(() => {
    const out: Record<string, NavItem[]> = {};
    for (const it of [...NAV_ITEMS, ...extItems]) {
      (out[it.group] ??= []).push(it);
    }
    return out;
  }, [extItems]);

  if (!open) return null;

  const groups: Record<string, CommandItem[]> = {};
  for (const item of allItems) {
    (groups[item.group] ??= []).push(item);
  }

  let flatIdx = 0;
  const showBrowseMode = !query.trim();

  return (
    <div className="cmd-overlay" onClick={close}>
      <div className="cmd-palette" data-mode={showBrowseMode ? "browse" : "search"} onClick={(e) => e.stopPropagation()}>
        <div className="cmd-input-wrap">
          <Search size={16} style={{ color: "var(--yunque-text-muted)", flexShrink: 0 }} />
          <input
            ref={inputRef}
            className="cmd-input"
            placeholder="搜索页面、知识、任务，或浏览下方分类…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
          />
          <kbd className="cmd-kbd">ESC</kbd>
        </div>

        {showBrowseMode ? (
          <div className="cmd-browse" ref={listRef}>
            {NAV_GROUP_ORDER.map((g) => {
              const items = navByGroup[g];
              if (!items || items.length === 0) return null;
              return (
                <section key={g} className="cmd-browse-group">
                  <h3 className="cmd-browse-group-title">{g}</h3>
                  <div className="cmd-browse-grid">
                    {items.map((it) => (
                      <button
                        key={it.id}
                        type="button"
                        className="cmd-browse-tile"
                        onClick={() => {
                          if (it.href) router.push(it.href);
                          close();
                        }}
                      >
                        <span className="cmd-browse-tile-icon">{it.icon}</span>
                        <span className="cmd-browse-tile-label">{it.label}</span>
                      </button>
                    ))}
                  </div>
                </section>
              );
            })}
          </div>
        ) : (
        <div className="cmd-list" ref={listRef}>
          {allItems.length === 0 && (
            <div className="cmd-empty">{searching ? "搜索中..." : "没有匹配结果"}</div>
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
        )}

        <div className="cmd-footer">
          {showBrowseMode ? (
            <span>输入字符以搜索 · <kbd className="cmd-kbd-sm">ESC</kbd> 关闭</span>
          ) : (
            <>
              <span><kbd className="cmd-kbd-sm">↑↓</kbd> 选择</span>
              <span><kbd className="cmd-kbd-sm">Enter</kbd> 确认</span>
              <span><kbd className="cmd-kbd-sm">ESC</kbd> 关闭</span>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
