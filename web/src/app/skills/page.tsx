"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { api, type SkillInfo, type SkillHubItem, type SkillHubInstalledItem } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  Package, Search, Download, Trash2, Star, ChevronDown, ChevronRight, ChevronLeft,
  Wrench, TrendingUp, Globe, HardDrive, Loader2, Shield, BarChart2,
} from "lucide-react";
import PermissionApproval from "@/components/permission-approval";
import Link from "next/link";

type Tab = "installed" | "market";
const PAGE_SIZE = 48; // fetch page size from ClawHub API
const DISPLAY_PAGE_SIZE = 24; // items per display page

export default function SkillsPage() {
  const [tab, setTab] = useState<Tab>("installed");
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [hubInstalled, setHubInstalled] = useState<SkillHubInstalledItem[]>([]);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  // Market state — infinite scroll
  const [query, setQuery] = useState("");
  const [searching, setSearching] = useState(false);
  const [results, setResults] = useState<SkillHubItem[]>([]);
  const [allItems, setAllItems] = useState<SkillHubItem[]>([]); // flat list of ALL loaded items
  const [nextCursor, setNextCursor] = useState<string>("");
  const [marketLoaded, setMarketLoaded] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [installing, setInstalling] = useState<string | null>(null);
  const [source, setSource] = useState<"" | "clawhub" | "torihub">(""); // "" = all
  const [browsePage, setBrowsePage] = useState(1);
  const [searchPage, setSearchPage] = useState(1);
  const loadingRef = useRef(false); // prevent concurrent loads
  const [pendingInstall, setPendingInstall] = useState<string | null>(null);

  const refreshInstalled = useCallback(() => {
    return Promise.all([
      api.skills().then(setSkills).catch(() => {}),
      api.skillHubInstalled().then((r) => setHubInstalled(Array.isArray(r.skills) ? r.skills : [])).catch(() => {}),
    ]);
  }, []);

  useEffect(() => {
    refreshInstalled().finally(() => setLoading(false));
  }, [refreshInstalled]);

  // Fetch one page, returns { items, cursor }
  const fetchPage = useCallback(async (cursor: string) => {
    const r = await api.skillHubTrending(PAGE_SIZE, source, cursor);
    const items = Array.isArray(r.skills) ? r.skills : [];
    return { items, cursor: r.next_cursor || "" };
  }, [source]);

  // Initial load: one big page
  useEffect(() => {
    if (tab !== "market") return;
    let cancelled = false;
    setMarketLoaded(false);
    setAllItems([]);
    setNextCursor("");

    fetchPage("").then((page) => {
      if (cancelled) return;
      setAllItems(page.items);
      setNextCursor(page.cursor);
    }).catch(() => {}).finally(() => { if (!cancelled) setMarketLoaded(true); });

    return () => { cancelled = true; };
  }, [tab, source, fetchPage]);

  // Load next page (for infinite scroll or manual trigger)
  const loadMore = useCallback(async () => {
    if (!nextCursor || loadingRef.current) return;
    loadingRef.current = true;
    setLoadingMore(true);
    try {
      const page = await fetchPage(nextCursor);
      setAllItems((prev) => [...prev, ...page.items]);
      setNextCursor(page.cursor);
    } catch { /* */ }
    finally {
      setLoadingMore(false);
      loadingRef.current = false;
    }
  }, [nextCursor, fetchPage]);

  // Auto-fetch all remaining pages after initial load (no manual scroll needed)
  useEffect(() => {
    if (!marketLoaded || !nextCursor || loadingRef.current) return;
    const timer = setTimeout(() => loadMore(), 100);
    return () => clearTimeout(timer);
  }, [marketLoaded, nextCursor, loadMore, allItems.length]);

  // Reset page when filter/source changes
  useEffect(() => { setBrowsePage(1); }, [query, source]);

  const [hasSearched, setHasSearched] = useState(false); // distinguishes "no results" from "not searched"
  useEffect(() => { setSearchPage(1); }, [hasSearched]);

  const doSearch = async () => {
    if (!query.trim()) return;
    setSearching(true);
    setHasSearched(true);
    try {
      const r = await api.skillHubSearch(query.trim(), 200, source);
      setResults(Array.isArray(r.results) ? r.results : []);
    } catch { setResults([]); }
    finally { setSearching(false); }
  };

  // Client-side keyword filter for Browse list
  const filteredItems = query.trim() && !hasSearched
    ? allItems.filter((s) => {
        const q = query.trim().toLowerCase();
        return (s.name && s.name.toLowerCase().includes(q))
          || (s.description && s.description.toLowerCase().includes(q))
          || (s.author && s.author.toLowerCase().includes(q));
      })
    : allItems;

  // Client-side pagination
  const browsePageCount = Math.max(1, Math.ceil(filteredItems.length / DISPLAY_PAGE_SIZE));
  const pagedBrowseItems = filteredItems.slice((browsePage - 1) * DISPLAY_PAGE_SIZE, browsePage * DISPLAY_PAGE_SIZE);
  const searchPageCount = Math.max(1, Math.ceil(results.length / DISPLAY_PAGE_SIZE));
  const pagedSearchItems = results.slice((searchPage - 1) * DISPLAY_PAGE_SIZE, searchPage * DISPLAY_PAGE_SIZE);

  const install = async (name: string) => {
    setPendingInstall(name);
  };

  const doInstall = async (name: string) => {
    setPendingInstall(null);
    setInstalling(name);
    try {
      await api.skillHubInstall(name);
      await refreshInstalled();
      const mark = (list: SkillHubItem[]) =>
        list.map((s) => s.name === name ? { ...s, installed: true } : s);
      setResults(mark);
      setAllItems(mark);
    } catch { /* */ }
    finally { setInstalling(null); }
  };

  const uninstall = async (name: string) => {
    setInstalling(name);
    try {
      await api.skillHubUninstall(name);
      await refreshInstalled();
      const mark = (list: SkillHubItem[]) =>
        list.map((s) => s.name === name ? { ...s, installed: false } : s);
      setResults(mark);
      setAllItems(mark);
    } catch { /* */ }
    finally { setInstalling(null); }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin"
          style={{ borderColor: "var(--text-muted)", borderTopColor: "transparent" }} />
      </div>
    );
  }

  return (
    <div className="max-w-4xl">
      <BlurFade delay={0}>
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <Package size={20} />
            <h1 className="text-xl font-semibold tracking-tight">Skills</h1>
            <Link href="/skill-policy" className="p-1.5 rounded-lg hover:bg-[var(--bg-hover)] transition-colors"
              title="安全策略" style={{ color: "var(--text-muted)" }}>
              <Shield size={14} />
            </Link>
            <Link href="/skill-analytics" className="p-1.5 rounded-lg hover:bg-[var(--bg-hover)] transition-colors"
              title="市场分析" style={{ color: "var(--text-muted)" }}>
              <BarChart2 size={14} />
            </Link>
          </div>
          <div className="flex gap-1 p-1 rounded-full border" style={{ borderColor: "var(--border)", background: "var(--bg-card)" }}>
            {([["installed", "Installed"], ["market", "Market"]] as const).map(([key, label]) => (
              <button
                key={key}
                onClick={() => setTab(key)}
                className="px-4 py-1.5 rounded-full text-xs font-medium transition-all cursor-pointer"
                style={{
                  background: tab === key ? "var(--text)" : "transparent",
                  color: tab === key ? "var(--bg)" : "var(--text-muted)",
                }}
              >
                {label}
              </button>
            ))}
          </div>
        </div>
      </BlurFade>

      {tab === "installed" && (
        <BlurFade delay={0.05}>
          {/* SkillHub installed skills */}
          {hubInstalled.length > 0 && (
            <div className="mb-4">
              <div className="text-xs font-medium uppercase tracking-wider mb-3 flex items-center gap-2"
                style={{ color: "var(--text-muted)" }}>
                <Globe size={12} />
                Marketplace ({hubInstalled.length})
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {hubInstalled.map((s) => (
                  <div key={s.slug} className="rounded-xl border p-4 flex flex-col justify-between"
                    style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                    <div>
                      <div className="flex items-start justify-between mb-2">
                        <div className="flex items-center gap-2 min-w-0">
                          <Package size={14} style={{ color: "var(--accent)" }} />
                          <span className="text-sm font-medium truncate">{s.name || s.slug}</span>
                        </div>
                        <span className="text-[10px] px-1.5 py-0.5 rounded-full" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
                          v{s.version}
                        </span>
                      </div>
                      <p className="text-xs mb-2 line-clamp-2" style={{ color: "var(--text-muted)" }}>
                        {s.description || "No description"}
                      </p>
                      <div className="flex items-center gap-3 text-[10px]" style={{ color: "var(--text-muted)" }}>
                        <span>{s.source}</span>
                        {s.security_score > 0 && <span>Security: {s.security_score}/100</span>}
                      </div>
                    </div>
                    <div className="mt-3 flex justify-end">
                      <button
                        onClick={() => uninstall(s.slug)}
                        disabled={installing === s.slug}
                        className="flex items-center gap-1.5 px-3 py-1.5 rounded-full text-[11px] font-medium transition-all cursor-pointer border"
                        style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}
                      >
                        {installing === s.slug ? <Loader2 size={10} className="animate-spin" /> : <Trash2 size={10} />}
                        Uninstall
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Plugin skills */}
          {skills.length > 0 && (
            <div>
              <div className="text-xs font-medium uppercase tracking-wider mb-3 flex items-center gap-2"
                style={{ color: "var(--text-muted)" }}>
                <Wrench size={12} />
                Plugins ({skills.length})
              </div>
              <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                <div className="space-y-1">
                  {skills.map((s) => (
                    <div key={s.name} className="rounded-lg transition-colors" style={{ background: "var(--bg-hover)" }}>
                      <button
                        className="w-full flex items-center gap-3 px-4 py-3 text-left cursor-pointer"
                        onClick={() => setExpanded(expanded === s.name ? null : s.name)}
                      >
                        <Wrench size={14} style={{ color: "var(--accent)" }} />
                        <span className="font-medium text-sm flex-1">{s.name}</span>
                        <span className="text-xs truncate max-w-[200px]" style={{ color: "var(--text-muted)" }}>
                          {s.description}
                        </span>
                        {expanded === s.name ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                      </button>
                      {expanded === s.name && (
                        <div className="px-4 pb-3">
                          <pre className="text-xs p-3 rounded-lg overflow-auto"
                            style={{ background: "var(--bg)", color: "var(--text-muted)" }}>
                            {JSON.stringify(s.parameters, null, 2)}
                          </pre>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}

          {skills.length === 0 && hubInstalled.length === 0 && (
            <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              <div className="text-center py-12">
                <Wrench size={40} className="mx-auto mb-3" style={{ color: "var(--text-muted)", opacity: 0.3 }} />
                <div className="text-sm" style={{ color: "var(--text-muted)" }}>No skills loaded</div>
                <div className="text-xs mt-1" style={{ color: "var(--text-muted)", opacity: 0.6 }}>
                  Install skills from the Market tab or enable plugins
                </div>
              </div>
            </div>
          )}

          <div className="text-xs mt-4 text-right" style={{ color: "var(--text-muted)" }}>
            {skills.length + hubInstalled.length} skill{skills.length + hubInstalled.length !== 1 ? "s" : ""} total
          </div>
        </BlurFade>
      )}

      {tab === "market" && (
        <BlurFade delay={0.05}>
          {/* Source selector */}
          <div className="flex gap-1 p-1 rounded-full border mb-4" style={{ borderColor: "var(--border)", background: "var(--bg-card)", width: "fit-content" }}>
            {([["" as const, "All"], ["clawhub" as const, "ClawHub"], ["torihub" as const, "ToriHub"]] as const).map(([key, label]) => (
              <button
                key={key}
                onClick={() => setSource(key)}
                className="px-3 py-1 rounded-full text-[11px] font-medium transition-all cursor-pointer flex items-center gap-1.5"
                style={{
                  background: source === key ? "var(--bg-hover)" : "transparent",
                  color: source === key ? "var(--text)" : "var(--text-muted)",
                }}
              >
                {key === "clawhub" && <Globe size={10} />}
                {key === "torihub" && <HardDrive size={10} />}
                {label}
              </button>
            ))}
          </div>

          {/* Search bar */}
          <div className="flex gap-2 mb-6">
            <div className="flex-1 flex items-center gap-2 px-4 py-2.5 rounded-full border"
              style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              <Search size={14} style={{ color: "var(--text-muted)" }} />
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && doSearch()}
                placeholder="Search skills..."
                className="bg-transparent text-sm flex-1 focus:outline-none"
              />
            </div>
            <button
              onClick={doSearch}
              disabled={searching || !query.trim()}
              className="px-5 py-2.5 rounded-full text-xs font-medium transition-all cursor-pointer flex items-center gap-2"
              style={{ background: "var(--text)", color: "var(--bg)", opacity: !query.trim() ? 0.5 : 1 }}
            >
              {searching ? <Loader2 size={12} className="animate-spin" /> : <Search size={12} />}
              Search
            </button>
          </div>

          {/* Search results — replaces browse when active */}
          {hasSearched ? (
            <div>
              <div className="flex items-center justify-between mb-3">
                <div className="text-xs font-medium uppercase tracking-wider flex items-center gap-2"
                  style={{ color: "var(--text-muted)" }}>
                  <Search size={12} />
                  Search Results ({results.length})
                </div>
                <button
                  onClick={() => { setResults([]); setQuery(""); setHasSearched(false); }}
                  className="text-[11px] px-3 py-1 rounded-full border cursor-pointer transition-all"
                  style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}
                >
                  Clear
                </button>
              </div>
              {results.length > 0 ? (
                <>
                  <SkillGrid items={pagedSearchItems} installing={installing} onInstall={install} onUninstall={uninstall} />
                  <Pagination page={searchPage} pageCount={searchPageCount} onPageChange={setSearchPage} />
                </>
              ) : (
                <div className="rounded-xl border p-8 text-center" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                  <Search size={32} className="mx-auto mb-3" style={{ color: "var(--text-muted)", opacity: 0.3 }} />
                  <div className="text-sm" style={{ color: "var(--text-muted)" }}>未找到匹配「{query}」的技能</div>
                  <div className="text-xs mt-1" style={{ color: "var(--text-muted)", opacity: 0.6 }}>请尝试其他关键词</div>
                </div>
              )}
            </div>
          ) : (
          /* Browse — infinite scroll with real-time keyword filtering */
          <div>
            <div className="text-xs font-medium uppercase tracking-wider mb-3 flex items-center gap-2"
              style={{ color: "var(--text-muted)" }}>
              <TrendingUp size={12} />
              Browse {filteredItems.length > 0 ? `(${filteredItems.length}${query.trim() ? " matched" : ""} skills)` : ""}
            </div>
            {filteredItems.length === 0 && allItems.length > 0 ? (
              <div className="rounded-xl border p-8 text-center" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                <Search size={32} className="mx-auto mb-3" style={{ color: "var(--text-muted)", opacity: 0.3 }} />
                <div className="text-sm" style={{ color: "var(--text-muted)" }}>未找到匹配「{query}」的技能</div>
                <div className="text-xs mt-1" style={{ color: "var(--text-muted)", opacity: 0.6 }}>按 Enter 搜索远程仓库，或清空关键词</div>
              </div>
            ) : filteredItems.length === 0 ? (
              <div className="rounded-xl border p-8 text-center" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                <Globe size={36} className="mx-auto mb-3" style={{ color: "var(--text-muted)", opacity: 0.3 }} />
                <div className="text-sm" style={{ color: "var(--text-muted)" }}>
                  {marketLoaded ? "No skills available" : "Loading..."}
                </div>
                <div className="text-xs mt-1" style={{ color: "var(--text-muted)", opacity: 0.6 }}>
                  {source === "torihub" ? "ToriHub" : source === "clawhub" ? "ClawHub" : "ClawHub / ToriHub"} connection needed for remote skills
                </div>
              </div>
            ) : (
              <>
                <SkillGrid items={pagedBrowseItems} installing={installing} onInstall={install} onUninstall={uninstall} />
                {(loadingMore || nextCursor) && (
                  <div className="flex justify-center items-center gap-2 py-4">
                    <Loader2 size={14} className="animate-spin" style={{ color: "var(--text-muted)" }} />
                    <span className="text-xs" style={{ color: "var(--text-muted)" }}>Loading more skills... ({allItems.length} loaded)</span>
                  </div>
                )}
                <Pagination page={browsePage} pageCount={browsePageCount} onPageChange={setBrowsePage} />
                {!nextCursor && allItems.length > 0 && (
                  <div className="text-center text-xs py-2" style={{ color: "var(--text-muted)", opacity: 0.5 }}>
                    All {allItems.length} skills loaded
                  </div>
                )}
              </>
            )}
          </div>
          )}
        </BlurFade>
      )}

      {pendingInstall && (
        <PermissionApproval
          slug={pendingInstall}
          onApprove={() => doInstall(pendingInstall)}
          onCancel={() => setPendingInstall(null)}
        />
      )}
    </div>
  );
}

function SkillGrid({
  items, installing, onInstall, onUninstall,
}: {
  items: SkillHubItem[];
  installing: string | null;
  onInstall: (name: string) => void;
  onUninstall: (name: string) => void;
}) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
      {items.map((s) => (
        <div key={s.name} className="rounded-xl border p-4 flex flex-col justify-between"
          style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div>
            <div className="flex items-start justify-between mb-2">
              <div className="flex items-center gap-2 min-w-0">
                <Package size={14} style={{ color: "var(--accent)" }} />
                <span className="text-sm font-medium truncate">{s.name}</span>
              </div>
              <div className="flex items-center gap-1 shrink-0 ml-2">
                {s.source === "clawhub" ? (
                  <Globe size={10} style={{ color: "var(--text-muted)" }} />
                ) : (
                  <HardDrive size={10} style={{ color: "var(--text-muted)" }} />
                )}
                <span className="text-[10px]" style={{ color: "var(--text-muted)" }}>{s.source}</span>
              </div>
            </div>
            <p className="text-xs mb-3 line-clamp-2" style={{ color: "var(--text-muted)" }}>
              {s.description || "No description"}
            </p>
            <div className="flex items-center gap-3 text-[10px]" style={{ color: "var(--text-muted)" }}>
              {s.author && <span>by {s.author}</span>}
              {s.version && <span>v{s.version}</span>}
              {s.rating > 0 && (
                <span className="flex items-center gap-0.5">
                  <Star size={9} fill="currentColor" /> {s.rating.toFixed(1)}
                </span>
              )}
            </div>
          </div>
          <div className="mt-3 flex justify-end">
            {s.installed ? (
              <button
                onClick={() => onUninstall(s.name)}
                disabled={installing === s.name}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-full text-[11px] font-medium transition-all cursor-pointer border"
                style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}
              >
                {installing === s.name ? <Loader2 size={10} className="animate-spin" /> : <Trash2 size={10} />}
                Uninstall
              </button>
            ) : (
              <button
                onClick={() => onInstall(s.name)}
                disabled={installing === s.name}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-full text-[11px] font-medium transition-all cursor-pointer"
                style={{ background: "var(--text)", color: "var(--bg)" }}
              >
                {installing === s.name ? <Loader2 size={10} className="animate-spin" /> : <Download size={10} />}
                Install
              </button>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

function Pagination({ page, pageCount, onPageChange }: { page: number; pageCount: number; onPageChange: (p: number) => void }) {
  if (pageCount <= 1) return null;

  // Build visible page numbers with ellipsis
  const pages: (number | "...")[] = [];
  const delta = 2; // pages around current
  for (let i = 1; i <= pageCount; i++) {
    if (i === 1 || i === pageCount || (i >= page - delta && i <= page + delta)) {
      pages.push(i);
    } else if (pages[pages.length - 1] !== "...") {
      pages.push("...");
    }
  }

  const btnBase = "px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer";

  return (
    <div className="flex items-center justify-center gap-1 pt-4 pb-2">
      <button
        onClick={() => onPageChange(Math.max(1, page - 1))}
        disabled={page <= 1}
        className={`${btnBase} flex items-center gap-1`}
        style={{ color: page <= 1 ? "var(--text-muted)" : "var(--text)", opacity: page <= 1 ? 0.4 : 1 }}
      >
        <ChevronLeft size={12} /> Prev
      </button>
      {pages.map((p, i) =>
        p === "..." ? (
          <span key={`e${i}`} className="px-1 text-xs" style={{ color: "var(--text-muted)" }}>…</span>
        ) : (
          <button
            key={p}
            onClick={() => onPageChange(p)}
            className={btnBase}
            style={{
              background: p === page ? "var(--text)" : "transparent",
              color: p === page ? "var(--bg)" : "var(--text-muted)",
              minWidth: "2rem",
            }}
          >
            {p}
          </button>
        )
      )}
      <button
        onClick={() => onPageChange(Math.min(pageCount, page + 1))}
        disabled={page >= pageCount}
        className={`${btnBase} flex items-center gap-1`}
        style={{ color: page >= pageCount ? "var(--text-muted)" : "var(--text)", opacity: page >= pageCount ? 0.4 : 1 }}
      >
        Next <ChevronRight size={12} />
      </button>
    </div>
  );
}
