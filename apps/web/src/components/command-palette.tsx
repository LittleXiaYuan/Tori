"use client";

import { useRouter } from "next/navigation";
import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import {
  Puzzle, Search, FileText, ArrowRight, Sparkles, Layers,
} from "lucide-react";
import { api, SearchResult } from "@/lib/api";
import { NAV_ITEMS, NAV_GROUP_ORDER, NAV_GROUP_LABEL_KEYS, navItemLabel, filterNavItemsByProfile, type NavItem, type NavGroup } from "@/lib/nav-items";
import { useI18n } from "@/lib/i18n";
import { buildPackNavItems, fetchEnabledPacks } from "@/lib/pack-sync";
import { PROFILE_MODE_KEY, readProfileMode, writeProfileMode } from "@/lib/profile-mode";

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
  const { t } = useI18n();
  const groupLabel = (g: string): string => {
    if (g in NAV_GROUP_LABEL_KEYS) return t(NAV_GROUP_LABEL_KEYS[g as NavGroup]);
    if (g === "操作") return t("cmd.group.action");
    if (g === "搜索") return t("cmd.group.search");
    return g;
  };
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [activeIdx, setActiveIdx] = useState(0);
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [extItems, setExtItems] = useState<NavItem[]>([]);
  const [packItems, setPackItems] = useState<NavItem[]>([]);
  const [profileMode, setProfileMode] = useState(readProfileMode);
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
            layer: "pack" as const,
            icon: <Puzzle size={16} />,
            keywords: `${t.label} ${t.label_en || ""} extension ${t.key}`,
          })),
        );
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    fetchEnabledPacks()
      .then((packs) => {
        setPackItems(buildPackNavItems(packs).map((item) => ({
          id: `pack-${item.packId}-${item.href}`,
          href: item.href,
          label: item.label,
          group: "扩展",
          layer: "pack" as const,
          defaultVisible: true,
          icon: item.icon,
          keywords: item.keywords,
        })));
      })
      .catch(() => setPackItems([]));
  }, []);

  useEffect(() => {
    const handleProfileModeChange = (event: StorageEvent) => {
      if (event.key === PROFILE_MODE_KEY) setProfileMode(readProfileMode());
    };
    window.addEventListener("storage", handleProfileModeChange);
    return () => window.removeEventListener("storage", handleProfileModeChange);
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

  const visibleNavItems = useMemo(
    () => filterNavItemsByProfile([...NAV_ITEMS, ...packItems, ...extItems], profileMode),
    [extItems, packItems, profileMode],
  );

  const navCommands: CommandItem[] = useMemo(() => {
    const isEasy = profileMode === "easy";
    const profileCmd: CommandItem = {
      id: "profile-toggle",
      label: isEasy ? "切换到完整模式" : "切换到轻松模式",
      group: "操作",
      icon: isEasy ? <Layers size={16} /> : <Sparkles size={16} />,
      keywords: "easy full simple profile mode 简洁 轻松 完整 专家 switch toggle",
      action: () => {
        const next = isEasy ? "full" : "easy";
        writeProfileMode(next);
        window.location.reload();
        close();
      },
    };
    const onboardingCmd: CommandItem = {
      id: "open-onboarding",
      label: "新手引导",
      group: "操作",
      icon: <Sparkles size={16} />,
      keywords: "onboarding guide intro 引导 新手 教程 初始化 setup welcome",
      action: () => {
        window.dispatchEvent(new CustomEvent("yunque:open-onboarding"));
        close();
      },
    };
    return [
      profileCmd,
      onboardingCmd,
      ...visibleNavItems.map((item) => ({
        ...item,
        action: () => {
          if (item.href) router.push(item.href);
          close();
        },
      })),
    ];
  }, [profileMode, router, close, visibleNavItems]);

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
    for (const it of visibleNavItems) {
      (out[it.group] ??= []).push(it);
    }
    return out;
  }, [visibleNavItems]);

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
            placeholder="搜索任务、记忆、页面，或打开可选能力…"
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
                  <h3 className="cmd-browse-group-title">{groupLabel(g)}</h3>
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
                        <span className="cmd-browse-tile-label">{navItemLabel(it, t)}</span>
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
            <div className="cmd-empty">{searching ? t("cmd.searching") : t("cmd.noResults")}</div>
          )}
          {Object.entries(groups).map(([group, items]) => (
            <div key={group}>
              <div className="cmd-group-label">{groupLabel(group)}</div>
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
                    <span className="cmd-item-label">{navItemLabel(item, t)}</span>
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
