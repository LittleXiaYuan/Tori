"use client";

import { useEffect, useState, useRef } from "react";
import { api, type KBChunk, type KBImportTreeNode, type KBSource, type KBStats } from "@/lib/api";
import { BookOpen, Search, Upload, Trash2, FileText, Database, Sparkles } from "lucide-react";
import { useI18n } from "@/lib/i18n";

function ImportTree({ node }: { node: KBImportTreeNode }) {
  return (
    <div className="space-y-2">
      <div className="text-sm">
        {node.url ? <a href={node.url} target="_blank" rel="noreferrer" className="underline underline-offset-2">{node.title}</a> : node.title}
      </div>
      {node.children && node.children.length > 0 && (
        <div className="pl-4 border-l space-y-2" style={{ borderColor: "var(--border)" }}>
          {node.children.map((child) => (
            <ImportTree key={`${child.path || child.title}-${child.url || ""}`} node={child} />
          ))}
        </div>
      )}
    </div>
  );
}

export default function KnowledgePage() {
  const [sources, setSources] = useState<KBSource[]>([]);
  const [stats, setStats] = useState<KBStats | null>(null);
  const [query, setQuery] = useState("");
  const [fileFilter, setFileFilter] = useState("");
  const [langFilter, setLangFilter] = useState("");
  const [results, setResults] = useState<KBChunk[]>([]);
  const [searching, setSearching] = useState(false);
  const [urlInput, setUrlInput] = useState("");
  const [urlName, setUrlName] = useState("");
  const [crawlChildren, setCrawlChildren] = useState(false);
  const [maxPages, setMaxPages] = useState(5);
  const [importingURL, setImportingURL] = useState(false);
  const [repoPath, setRepoPath] = useState("");
  const [repoMaxFiles, setRepoMaxFiles] = useState(200);
  const [importingRepo, setImportingRepo] = useState(false);
  const [lastImportTree, setLastImportTree] = useState<KBImportTreeNode | null>(null);
  const [lastImportedCount, setLastImportedCount] = useState(0);
  const [ingestName, setIngestName] = useState("");
  const [ingestContent, setIngestContent] = useState("");
  const [tab, setTab] = useState<"search" | "sources" | "ingest">("search");
  const [loading, setLoading] = useState(true);
  const fileRef = useRef<HTMLInputElement>(null);
  const { t } = useI18n();

  const refresh = () => {
    Promise.all([
      api.kbSources().then((r) => setSources(r.sources || [])).catch(() => {}),
      api.kbStats().then(setStats).catch(() => {}),
    ]).finally(() => setLoading(false));
  };

  useEffect(() => { refresh(); }, []);

  const doSearch = async () => {
    if (!query.trim()) return;
    setSearching(true);
    try {
      const r = await api.kbSearch(query, 10, { file: fileFilter.trim(), lang: langFilter.trim() });
      setResults(r.chunks || []);
    } catch { setResults([]); }
    setSearching(false);
  };

  const doIngest = async () => {
    if (!ingestContent.trim()) return;
    await api.kbIngest(ingestName || "inline", ingestContent);
    setIngestName("");
    setIngestContent("");
    refresh();
  };

  const doImportURL = async () => {
    if (!urlInput.trim()) return;
    setImportingURL(true);
    try {
      const result = await api.kbImportURL(urlInput.trim(), urlName.trim() || undefined, {
        crawlChildren,
        maxPages,
      });
      setLastImportTree(result.tree || null);
      setLastImportedCount(result.imported || 1);
      setUrlInput("");
      setUrlName("");
      refresh();
    } finally {
      setImportingURL(false);
    }
  };

  const doUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    await api.kbUpload(file);
    refresh();
    if (fileRef.current) fileRef.current.value = "";
  };

  const doDelete = async (id: string) => {
    await api.kbDelete(id);
    refresh();
  };

  const doImportRepo = async () => {
    if (!repoPath.trim()) return;
    setImportingRepo(true);
    try {
      await api.kbImportRepo(repoPath.trim(), repoMaxFiles);
      refresh();
    } finally {
      setImportingRepo(false);
    }
  };

  return (
    <div className="animate-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: "var(--accent-subtle)" }}>
            <BookOpen size={20} style={{ color: "var(--accent)" }} />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">{t("kb.title")}</h1>
            {stats && (
              <p className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>
                {stats.sources} {t("kb.sources").toLowerCase()} · {stats.chunks} {t("kb.chunks")}
              </p>
            )}
          </div>
        </div>
        {stats && (
          <div className="flex gap-3">
            <div className="px-3 py-2 rounded-lg text-center" style={{ background: "var(--bg-hover)" }}>
              <div className="text-lg font-bold" style={{ color: "var(--accent)" }}>{stats.sources}</div>
              <div className="text-[10px] uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>{t("kb.sources")}</div>
            </div>
            <div className="px-3 py-2 rounded-lg text-center" style={{ background: "var(--bg-hover)" }}>
              <div className="text-lg font-bold">{stats.chunks}</div>
              <div className="text-[10px] uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>{t("kb.chunks")}</div>
            </div>
          </div>
        )}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-6 p-1 rounded-xl w-fit" style={{ background: "var(--bg-hover)", boxShadow: "var(--shadow-sm)" }}>
        {(["search", "sources", "ingest"] as const).map((tb) => (
          <button key={tb} onClick={() => setTab(tb)}
            className="px-5 py-2 rounded-lg text-xs font-medium"
            style={{
              background: tab === tb ? "var(--bg-elevated)" : "transparent",
              color: tab === tb ? "var(--text)" : "var(--text-muted)",
              boxShadow: tab === tb ? "var(--shadow-sm)" : "none",
            }}>
            {tb === "search" ? t("kb.search") : tb === "sources" ? t("kb.sources") : t("kb.ingest")}
          </button>
        ))}
      </div>

      {/* Loading */}
      {loading && (
        <div className="space-y-3">
          <div className="skeleton h-12 w-full" />
          <div className="skeleton h-24 w-full" />
          <div className="skeleton h-24 w-full" />
        </div>
      )}

      {/* Search tab */}
      {!loading && tab === "search" && (
        <div className="animate-in">
          <div className="flex gap-2 mb-5">
            <div className="flex-1 relative">
              <Search size={16} className="absolute left-4 top-1/2 -translate-y-1/2" style={{ color: "var(--text-muted)" }} />
              <input value={query} onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && doSearch()}
                placeholder={t("kb.searchPlaceholder")}
                className="w-full pl-11 pr-4 py-3 rounded-xl border text-sm outline-none"
                style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }} />
            </div>
            <button onClick={doSearch} disabled={searching}
              className="btn-glow px-5 py-3 rounded-xl text-sm font-medium flex items-center gap-2">
              <Sparkles size={14} /> {searching ? "..." : t("kb.search")}
            </button>
          </div>
          <div className="grid gap-2 md:grid-cols-2 mb-5">
            <input value={fileFilter} onChange={(e) => setFileFilter(e.target.value)}
              placeholder={t("kb.filePlaceholder")}
              className="w-full px-4 py-3 rounded-xl border text-sm outline-none"
              style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }} />
            <input value={langFilter} onChange={(e) => setLangFilter(e.target.value)}
              placeholder={t("kb.language")}
              className="w-full px-4 py-3 rounded-xl border text-sm outline-none"
              style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }} />
          </div>
          <div className="space-y-3 stagger">
            {results.map((c, i) => (
              <div key={i} className="card-hover rounded-xl border p-5 animate-in"
                style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                <div className="flex items-center gap-2 mb-3">
                  <span className="badge" style={{ background: "var(--accent-subtle)", color: "var(--accent)" }}>
                    {c.source_id}
                  </span>
                  <span className="text-[11px]" style={{ color: "var(--text-muted)" }}>{t("kb.chunk")} #{c.index}</span>
                  {c.metadata?.lang && <span className="text-[11px]" style={{ color: "var(--text-muted)" }}>{c.metadata.lang}</span>}
                </div>
                {c.metadata?.file && (
                  <div className="text-[11px] mb-2" style={{ color: "var(--text-muted)" }}>{c.metadata.file}</div>
                )}
                <p className="text-sm leading-relaxed" style={{ color: "var(--text-secondary)" }}>{c.content}</p>
              </div>
            ))}
            {results.length === 0 && query && !searching && (
              <div className="text-sm text-center py-12 rounded-xl border" style={{ color: "var(--text-muted)", borderColor: "var(--border)", borderStyle: "dashed" }}>{t("kb.noResults")}</div>
            )}
          </div>
        </div>
      )}

      {/* Sources tab */}
      {!loading && tab === "sources" && (
        <div className="animate-in">
          <div className="mb-5">
            <label className="btn-glow px-5 py-3 rounded-xl text-sm font-medium cursor-pointer inline-flex items-center gap-2">
              <Upload size={14} /> {t("kb.uploadFile")}
              <input ref={fileRef} type="file" accept=".txt,.md,.csv,.json,.pdf" onChange={doUpload} className="hidden" />
            </label>
          </div>
          <div className="space-y-2 stagger">
            {sources.map((s) => (
              <div key={s.id} className="card-hover rounded-xl border flex items-center gap-4 px-5 py-4 animate-in"
                style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                <div className="w-9 h-9 rounded-lg flex items-center justify-center" style={{ background: "var(--accent-subtle)" }}>
                  <FileText size={16} style={{ color: "var(--accent)" }} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium truncate">{s.name}</div>
                  <div className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>{s.chunk_count} {t("kb.chunks")}</div>
                </div>
                <button onClick={() => doDelete(s.id)}
                  className="p-2 rounded-lg hover:bg-[var(--danger-bg)]"
                  style={{ color: "var(--text-muted)" }}>
                  <Trash2 size={14} />
                </button>
              </div>
            ))}
            {sources.length === 0 && (
              <div className="text-sm text-center py-16 rounded-xl border" style={{ color: "var(--text-muted)", borderColor: "var(--border)", borderStyle: "dashed" }}>
                <Database size={32} className="mx-auto mb-3 opacity-30" />
                {t("kb.noSources")}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Ingest tab */}
      {!loading && tab === "ingest" && (
        <div className="animate-in space-y-4">
          <div className="rounded-xl border p-4 space-y-3" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>{t("kb.importRepo")}</div>
            <input value={repoPath} onChange={(e) => setRepoPath(e.target.value)}
              placeholder={t("kb.repoPlaceholder")}
              className="w-full px-4 py-3 rounded-xl border text-sm outline-none"
              style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }} />
            <div className="flex items-center gap-2">
              <span className="text-xs" style={{ color: "var(--text-muted)" }}>{t("kb.maxFiles")}</span>
              <input
                type="number"
                min={10}
                max={1000}
                value={repoMaxFiles}
                onChange={(e) => setRepoMaxFiles(Math.min(1000, Math.max(10, Number(e.target.value) || 10)))}
                className="w-24 px-3 py-2 rounded-lg border text-sm outline-none"
                style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }}
              />
            </div>
            <button onClick={doImportRepo} disabled={!repoPath.trim() || importingRepo}
              className="btn-glow px-5 py-3 rounded-xl text-sm font-medium">
              {importingRepo ? "..." : t("kb.importRepo")}
            </button>
          </div>
          <div className="rounded-xl border p-4 space-y-3" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <input value={urlName} onChange={(e) => setUrlName(e.target.value)}
              placeholder={t("kb.sourceName")}
              className="w-full px-4 py-3 rounded-xl border text-sm outline-none"
              style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }} />
            <input value={urlInput} onChange={(e) => setUrlInput(e.target.value)}
              placeholder={t("kb.urlPlaceholder")}
              className="w-full px-4 py-3 rounded-xl border text-sm outline-none"
              style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }} />
            <div className="flex flex-wrap items-center gap-3">
              <label className="flex items-center gap-2 text-sm cursor-pointer" style={{ color: "var(--text)" }}>
                <input
                  type="checkbox"
                  checked={crawlChildren}
                  onChange={(e) => setCrawlChildren(e.target.checked)}
                />
                {t("kb.crawlChildren")}
              </label>
              <div className="flex items-center gap-2">
                <span className="text-xs" style={{ color: "var(--text-muted)" }}>{t("kb.maxPages")}</span>
                <input
                  type="number"
                  min={1}
                  max={20}
                  value={maxPages}
                  onChange={(e) => setMaxPages(Math.min(20, Math.max(1, Number(e.target.value) || 1)))}
                  disabled={!crawlChildren}
                  className="w-20 px-3 py-2 rounded-lg border text-sm outline-none"
                  style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }}
                />
              </div>
            </div>
            <p className="text-xs" style={{ color: "var(--text-muted)" }}>{t("kb.urlHint")}</p>
            <button onClick={doImportURL} disabled={!urlInput.trim() || importingURL}
              className="btn-glow px-5 py-3 rounded-xl text-sm font-medium">
              {importingURL ? "..." : t("kb.importUrl")}
            </button>
            {lastImportTree && (
              <div className="rounded-xl border p-4 space-y-3" style={{ borderColor: "var(--border)", background: "var(--bg-hover)" }}>
                <div className="text-sm font-medium">{t("kb.importedSummary")} ({lastImportedCount})</div>
                <ImportTree node={lastImportTree} />
              </div>
            )}
          </div>
          <input value={ingestName} onChange={(e) => setIngestName(e.target.value)}
            placeholder={t("kb.sourceName")}
            className="w-full px-4 py-3 rounded-xl border text-sm outline-none"
            style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }} />
          <textarea value={ingestContent} onChange={(e) => setIngestContent(e.target.value)}
            placeholder={t("kb.pasteContent")}
            rows={12}
            className="w-full px-4 py-3 rounded-xl border text-sm outline-none resize-none"
            style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }} />
          <button onClick={doIngest} disabled={!ingestContent.trim()}
            className="btn-glow px-5 py-3 rounded-xl text-sm font-medium">
            {t("kb.ingestText")}
          </button>
        </div>
      )}
    </div>
  );
}
