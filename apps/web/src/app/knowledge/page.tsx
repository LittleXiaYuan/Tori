"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { api, type KBSource, type KBChunk, type KBStats, type KBImportTreeNode } from "@/lib/api";
import { Card, Button, Spinner, SearchField, Tooltip, Chip, Checkbox, Input } from "@heroui/react";
import {
  BookOpen, Upload, File, RefreshCw, FileText, Database,
  Globe, GitBranch, Trash2, X, Link, FolderGit, Filter,
  ChevronRight, ChevronDown, ExternalLink, HardDrive, Search,
  FileCode, BarChart3, Type as TextFieldIcon, Pencil,
} from "lucide-react";
import { showToast } from "@/components/toast-provider";
import EmptyState from "@/components/empty-state";

type ImportMode = null | "url" | "repo" | "text";

function ImportTreeView({ node, depth = 0 }: { node: KBImportTreeNode; depth?: number }) {
  const [expanded, setExpanded] = useState(depth < 2);
  const hasChildren = node.children && node.children.length > 0;

  return (
    <div style={{ paddingLeft: depth * 16 }}>
      <div
        className="flex items-center gap-1.5 py-1 text-xs cursor-pointer hover:opacity-80"
        style={{ color: "var(--yunque-text-secondary)" }}
        onClick={() => hasChildren && setExpanded(!expanded)}
      >
        {hasChildren ? (expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />) : <FileText size={12} style={{ opacity: 0.4 }} />}
        <span className="truncate flex-1">{node.title || node.url || node.path}</span>
        {node.url && (
          <a href={node.url} target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()}>
            <ExternalLink size={10} style={{ color: "var(--yunque-text-muted)" }} />
          </a>
        )}
      </div>
      {expanded && hasChildren && node.children!.map((child, i) => (
        <ImportTreeView key={i} node={child} depth={depth + 1} />
      ))}
    </div>
  );
}

export default function KnowledgePage() {
  const [sources, setSources] = useState<KBSource[]>([]);
  const [stats, setStats] = useState<KBStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState("");
  const [searchResults, setSearchResults] = useState<KBChunk[]>([]);
  const [searching, setSearching] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [importMode, setImportMode] = useState<ImportMode>(null);

  // Import URL state
  const [importUrl, setImportUrl] = useState("");
  const [crawlChildren, setCrawlChildren] = useState(false);
  const [maxPages, setMaxPages] = useState("10");
  const [importingUrl, setImportingUrl] = useState(false);
  const [importTree, setImportTree] = useState<KBImportTreeNode | null>(null);

  // Import Repo state
  const [repoPath, setRepoPath] = useState("");
  const [maxFiles, setMaxFiles] = useState("100");
  const [importingRepo, setImportingRepo] = useState(false);

  // Ingest text state
  const [ingestName, setIngestName] = useState("");
  const [ingestTrigger, setIngestTrigger] = useState("");
  const [ingestContent, setIngestContent] = useState("");
  const [ingesting, setIngesting] = useState(false);

  // Filter state
  const [fileFilter, setFileFilter] = useState("");
  const [langFilter, setLangFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState<string>("all");

  // Delete state
  const [deleting, setDeleting] = useState<string | null>(null);

  // Edit modal state
  const [editSource, setEditSource] = useState<KBSource | null>(null);
  const [editName, setEditName] = useState("");
  const [editTrigger, setEditTrigger] = useState("");
  const [editContent, setEditContent] = useState("");
  const [saving, setSaving] = useState(false);

  const fileRef = useRef<HTMLInputElement>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [srcRes, statsRes] = await Promise.all([api.kbSources(), api.kbStats()]);
      setSources(Array.isArray(srcRes.sources) ? srcRes.sources : []);
      setStats(statsRes);
    } catch (e) { showToast(e instanceof Error ? e.message : "加载知识库失败", "error"); }
    setLoading(false);
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const handleSearch = async () => {
    if (!query.trim()) return;
    setSearching(true);
    try {
      const filters: { file?: string; lang?: string } = {};
      if (fileFilter) filters.file = fileFilter;
      if (langFilter) filters.lang = langFilter;
      const r = await api.kbSearch(query, 20, filters);
      setSearchResults(Array.isArray(r.chunks) ? r.chunks : []);
    } catch (e) { showToast(e instanceof Error ? e.message : "搜索失败，请重试", "error"); }
    setSearching(false);
  };

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files?.length) return;
    setUploading(true);
    try {
      for (let i = 0; i < files.length; i++) {
        await api.kbUpload(files[i]);
      }
      refresh();
    } catch (e) { showToast(e instanceof Error ? e.message : "上传失败", "error"); }
    setUploading(false);
    if (fileRef.current) fileRef.current.value = "";
  };

  const handleImportUrl = async () => {
    if (!importUrl.trim()) return;
    setImportingUrl(true);
    setImportTree(null);
    try {
      const r = await api.kbImportURL(importUrl, undefined, {
        crawlChildren,
        maxPages: crawlChildren ? parseInt(maxPages) || 10 : undefined,
      });
      if (r.tree) setImportTree(r.tree);
      setImportUrl("");
      refresh();
    } catch (e) { showToast(e instanceof Error ? e.message : "导入失败", "error"); }
    setImportingUrl(false);
  };

  const handleImportRepo = async () => {
    if (!repoPath.trim()) return;
    setImportingRepo(true);
    try {
      await api.kbImportRepo(repoPath, parseInt(maxFiles) || 100);
      setRepoPath("");
      refresh();
    } catch (e) { showToast(e instanceof Error ? e.message : "导入失败", "error"); }
    setImportingRepo(false);
  };

  const handleIngest = async () => {
    if (!ingestName.trim() || !ingestContent.trim()) return;
    setIngesting(true);
    try {
      await api.kbIngest(ingestName, ingestContent, ingestTrigger || undefined);
      setIngestName("");
      setIngestTrigger("");
      setIngestContent("");
      refresh();
    } catch (e) { showToast(e instanceof Error ? e.message : "写入失败", "error"); }
    setIngesting(false);
  };

  const handleDelete = async (id: string) => {
    setDeleting(id);
    try {
      await api.kbDelete(id);
      setSources((prev) => prev.filter((s) => s.id !== id));
    } catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); }
    setDeleting(null);
  };

  const openEdit = (s: KBSource) => {
    setEditSource(s);
    setEditName(s.name);
    setEditTrigger(s.trigger || "");
    setEditContent("");
  };

  const handleSaveEdit = async () => {
    if (!editSource || !editName.trim()) return;
    setSaving(true);
    try {
      await api.kbUpdate(editSource.id, editName, editTrigger, editContent);
      showToast("知识已更新", "success");
      setEditSource(null);
      refresh();
    } catch (e) { showToast(e instanceof Error ? e.message : "保存失败", "error"); }
    setSaving(false);
  };

  // Derived data
  const typeSet = new Set(sources.map((s) => s.type));
  const types = ["all", ...Array.from(typeSet)];
  const filteredSources = sources.filter((s) => typeFilter === "all" || s.type === typeFilter);
  const totalChunks = stats?.chunks ?? sources.reduce((acc, s) => acc + (s.chunk_count || 0), 0);
  const totalChars = stats?.total_chars ?? 0;

  if (loading) return <div className="flex-1 flex items-center justify-center"><Spinner size="lg" /></div>;

  return (
    <div className="page-root space-y-5 animate-fade-in-up" style={{ color: "var(--yunque-text)" }}>
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="page-title flex items-center gap-2"><BookOpen size={20} /> 知识库</h1>
        <div className="flex gap-2">
          <Tooltip delay={0}>
            <Button variant="ghost" size="sm" onPress={refresh}><RefreshCw size={14} /></Button>
            <Tooltip.Content>刷新</Tooltip.Content>
          </Tooltip>
          <input ref={fileRef} type="file" multiple className="hidden" onChange={handleUpload} accept=".txt,.md,.pdf,.docx,.csv,.json,.html,.py,.go,.js,.ts,.java,.c,.cpp,.rs,.rb" />
          <Button size="sm" isPending={uploading} onPress={() => fileRef.current?.click()} className="btn-accent">
            <Upload size={14} /> 上传文件
          </Button>
        </div>
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 stagger-children">
        <Card className="section-card hover-lift">
          <Card.Content className="flex items-center gap-3 py-3">
            <Database size={18} style={{ color: "var(--yunque-accent)" }} />
            <div>
              <div className="kpi-value" style={{ fontSize: "var(--text-xl)" }}>{stats?.sources ?? sources.length}</div>
              <div className="kpi-sub">知识源</div>
            </div>
          </Card.Content>
        </Card>
        <Card className="section-card hover-lift">
          <Card.Content className="flex items-center gap-3 py-3">
            <FileText size={18} style={{ color: "#22c55e" }} />
            <div>
              <div className="kpi-value" style={{ fontSize: "var(--text-xl)" }}>{totalChunks}</div>
              <div className="kpi-sub">总片段</div>
            </div>
          </Card.Content>
        </Card>
        <Card className="section-card hover-lift">
          <Card.Content className="flex items-center gap-3 py-3">
            <HardDrive size={18} style={{ color: "#f59e0b" }} />
            <div>
              <div className="text-lg font-bold">{totalChars > 1_000_000 ? `${(totalChars / 1_000_000).toFixed(1)}M` : totalChars > 1000 ? `${(totalChars / 1000).toFixed(0)}K` : totalChars}</div>
              <div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>总字数</div>
            </div>
          </Card.Content>
        </Card>
        <Card className="section-card hover-lift">
          <Card.Content className="flex items-center gap-3 py-3">
            <BarChart3 size={18} style={{ color: "#a78bfa" }} />
            <div>
              <div className="text-lg font-bold">{searchResults.length}</div>
              <div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>搜索结果</div>
            </div>
          </Card.Content>
        </Card>
      </div>

      {/* Import tools */}
      <div className="flex gap-2 flex-wrap">
        <Button
          size="sm" variant={importMode === "url" ? "primary" : "ghost"}
          onPress={() => setImportMode(importMode === "url" ? null : "url")}
          className="rounded-lg text-xs"
        >
          <Globe size={13} /> URL 导入
        </Button>
        <Button
          size="sm" variant={importMode === "repo" ? "primary" : "ghost"}
          onPress={() => setImportMode(importMode === "repo" ? null : "repo")}
          className="rounded-lg text-xs"
        >
          <FolderGit size={13} /> 仓库导入
        </Button>
        <Button
          size="sm" variant={importMode === "text" ? "primary" : "ghost"}
          onPress={() => setImportMode(importMode === "text" ? null : "text")}
          className="rounded-lg text-xs"
        >
          <FileCode size={13} /> 文本导入
        </Button>
      </div>

      {/* URL Import Panel */}
      {importMode === "url" && (
        <Card className="section-card animate-scale-in">
          <Card.Header className="flex items-center justify-between pb-2">
            <span className="text-sm font-medium flex items-center gap-2"><Globe size={14} /> URL 导入</span>
            <Button isIconOnly aria-label="关闭" variant="ghost" size="sm" onPress={() => { setImportMode(null); setImportTree(null); }}><X size={14} /></Button>
          </Card.Header>
          <Card.Content className="space-y-3 pt-0">
            <div className="flex gap-2">
              <input
                value={importUrl}
                onChange={(e) => setImportUrl(e.target.value)}
                placeholder="https://example.com/docs"
                onKeyDown={(e) => e.key === "Enter" && handleImportUrl()}
                className="flex-1 px-3 py-2 rounded-lg text-sm bg-transparent outline-none"
                style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
              />
              <Button size="sm" isPending={importingUrl} onPress={handleImportUrl} className="btn-accent">
                <Link size={13} /> 导入
              </Button>
            </div>
            <div className="flex items-center gap-4 text-xs">
              <Checkbox isSelected={crawlChildren} onChange={setCrawlChildren}>
                <Checkbox.Control><Checkbox.Indicator /></Checkbox.Control>
                <Checkbox.Content><span className="text-xs" style={{ color: "var(--yunque-text-secondary)" }}>爬取子页面</span></Checkbox.Content>
              </Checkbox>
              {crawlChildren && (
                <label className="flex items-center gap-1.5" style={{ color: "var(--yunque-text-secondary)" }}>
                  最大页数:
                  <input
                    type="number" value={maxPages} onChange={(e) => setMaxPages(e.target.value)}
                    className="w-16 px-2 py-0.5 rounded text-xs bg-transparent outline-none"
                    style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                    min="1" max="100"
                  />
                </label>
              )}
            </div>
            {importTree && (
              <div className="rounded-lg p-3" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
                <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text-muted)" }}>导入结构</div>
                <ImportTreeView node={importTree} />
              </div>
            )}
          </Card.Content>
        </Card>
      )}

      {/* Repo Import Panel */}
      {importMode === "repo" && (
        <Card className="section-card animate-scale-in">
          <Card.Header className="flex items-center justify-between pb-2">
            <span className="text-sm font-medium flex items-center gap-2"><FolderGit size={14} /> 仓库导入</span>
            <Button isIconOnly aria-label="关闭" variant="ghost" size="sm" onPress={() => setImportMode(null)}><X size={14} /></Button>
          </Card.Header>
          <Card.Content className="space-y-3 pt-0">
            <div className="flex gap-2">
              <input
                value={repoPath}
                onChange={(e) => setRepoPath(e.target.value)}
                placeholder="本地仓库路径, 如 /home/user/project"
                onKeyDown={(e) => e.key === "Enter" && handleImportRepo()}
                className="flex-1 px-3 py-2 rounded-lg text-sm bg-transparent outline-none"
                style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
              />
              <Button size="sm" isPending={importingRepo} onPress={handleImportRepo} className="btn-accent">
                <GitBranch size={13} /> 导入
              </Button>
            </div>
            <label className="flex items-center gap-1.5 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
              最大文件数:
              <input
                type="number" value={maxFiles} onChange={(e) => setMaxFiles(e.target.value)}
                className="w-20 px-2 py-0.5 rounded text-xs bg-transparent outline-none"
                style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                min="1" max="1000"
              />
            </label>
          </Card.Content>
        </Card>
      )}

      {/* Text Ingest Panel */}
      {importMode === "text" && (
        <Card className="section-card animate-scale-in">
          <Card.Header className="flex items-center justify-between pb-2">
            <span className="text-sm font-medium flex items-center gap-2"><FileCode size={14} /> 文本导入</span>
            <Button isIconOnly aria-label="关闭" variant="ghost" size="sm" onPress={() => setImportMode(null)}><X size={14} /></Button>
          </Card.Header>
          <Card.Content className="space-y-3 pt-0">
            <input
              value={ingestName}
              onChange={(e) => setIngestName(e.target.value)}
              placeholder="知识名称"
              className="w-full px-3 py-2 rounded-lg text-sm bg-transparent outline-none"
              style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
            />
            <input
              value={ingestTrigger}
              onChange={(e) => setIngestTrigger(e.target.value)}
              placeholder="使用时机（可选）, 如 当需要设计或排版时"
              className="w-full px-3 py-2 rounded-lg text-sm bg-transparent outline-none"
              style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
            />
            <textarea
              value={ingestContent}
              onChange={(e) => setIngestContent(e.target.value)}
              placeholder="粘贴文本内容..."
              rows={6}
              className="w-full resize-none px-3 py-2 text-sm rounded-lg outline-none bg-transparent"
              style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
            />
            <Button size="sm" isPending={ingesting} onPress={handleIngest} isDisabled={!ingestName.trim() || !ingestContent.trim()} className="btn-accent">
              导入文本
            </Button>
          </Card.Content>
        </Card>
      )}

      {/* Search with filters */}
      <Card className="section-card">
        <Card.Content className="space-y-3">
          <div className="flex flex-row gap-3 items-end">
            <SearchField className="flex-1" name="kb-search" value={query} onChange={setQuery} onSubmit={handleSearch}>
              <SearchField.Group>
                <SearchField.SearchIcon />
                <SearchField.Input placeholder="搜索知识库内容…" />
                <SearchField.ClearButton />
              </SearchField.Group>
            </SearchField>
            <Button size="sm" isPending={searching} onPress={handleSearch} className="btn-accent">
              <Search size={13} /> 搜索
            </Button>
          </div>
          <div className="flex gap-3 items-center flex-wrap">
            <div className="flex items-center gap-1.5 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
              <Filter size={12} /> 过滤:
            </div>
            <Input
              value={fileFilter}
              onChange={(e) => setFileFilter(e.target.value)}
              placeholder="文件名…" className="w-[140px]"
            />
            <Input
              value={langFilter}
              onChange={(e) => setLangFilter(e.target.value)}
              placeholder="语言 (go/py/js)..." className="w-[140px]"
            />
            {(fileFilter || langFilter) && (
              <Button size="sm" variant="ghost" onPress={() => { setFileFilter(""); setLangFilter(""); }} className="text-[11px]" style={{ color: "#ef4444" }}>
                <X size={10} /> 清除过滤
              </Button>
            )}
          </div>
        </Card.Content>
      </Card>

      {/* Search results */}
      {searchResults.length > 0 && (
        <div className="space-y-2 animate-fade-in">
          <h3 className="text-sm font-medium flex items-center gap-2">
            搜索结果
            <Chip style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)", fontSize: "var(--text-2xs)" }}>{searchResults.length}</Chip>
          </h3>
          {searchResults.map((r, i) => (
            <Card key={i} className="section-card hover-lift transition-all duration-200">
              <Card.Content className="py-3">
                <div className="text-xs leading-relaxed whitespace-pre-wrap" style={{ color: "var(--yunque-text-secondary)" }}>{r.content}</div>
                <div className="mt-2 flex items-center gap-2 flex-wrap">
                  <Chip style={{ background: "rgba(0,111,238,0.05)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>源: {r.source_id}</Chip>
                  <Chip style={{ background: "rgba(0,111,238,0.05)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>#{r.index}</Chip>
                  {r.metadata && Object.entries(r.metadata).slice(0, 3).map(([k, v]) => (
                    <Chip key={k} style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>{k}: {v}</Chip>
                  ))}
                </div>
              </Card.Content>
            </Card>
          ))}
        </div>
      )}

      {/* Type filter tabs */}
      {types.length > 2 && (
        <div className="flex gap-1.5 flex-wrap">
          {types.map((t) => (
            <button
              key={t}
              onClick={() => setTypeFilter(t)}
              className="px-2.5 py-1 rounded-lg text-[11px] font-medium transition-all"
              style={{
                background: typeFilter === t ? "var(--yunque-accent)" : "rgba(255,255,255,0.04)",
                color: typeFilter === t ? "#fff" : "var(--yunque-text-muted)",
              }}
            >
              {t === "all" ? "全部" : t}
            </button>
          ))}
        </div>
      )}

      {/* Sources grid */}
      <div>
        <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
          知识源
          <Chip style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)", fontSize: "var(--text-2xs)" }}>{filteredSources.length}</Chip>
        </h3>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3 stagger-children">
          {filteredSources.map((s) => {
            const typeIcon = s.type === "url" ? <Globe size={18} style={{ color: "#06b6d4" }} /> :
                             s.type === "repo" ? <FolderGit size={18} style={{ color: "#a78bfa" }} /> :
                             s.type === "code" ? <FileCode size={18} style={{ color: "#22c55e" }} /> :
                             <File size={18} style={{ color: "var(--yunque-accent)" }} />;
            return (
              <Card key={s.id || s.name} className="section-card hover-lift transition-all duration-200 group">
                <Card.Content className="flex items-center gap-3 py-3">
                  <div className="w-10 h-10 rounded-lg flex items-center justify-center shrink-0" style={{ background: "rgba(0,111,238,0.08)" }}>
                    {typeIcon}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium truncate">{s.name}</div>
                    <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                      <Chip style={{ background: "rgba(0,111,238,0.05)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>{s.chunk_count} 片段</Chip>
                      <Chip style={{ background: "rgba(0,111,238,0.05)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>{s.type}</Chip>
                      {s.trigger && <Chip style={{ background: "rgba(34,197,94,0.08)", color: "#22c55e", fontSize: "var(--text-2xs)" }}>{s.trigger}</Chip>}
                      {s.added_at && (
                        <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{new Date(s.added_at).toLocaleDateString()}</span>
                      )}
                    </div>
                  </div>
                  <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Tooltip delay={0}>
                      <Button isIconOnly variant="ghost" size="sm" onPress={() => openEdit(s)}>
                        <Pencil size={13} style={{ color: "var(--yunque-accent)" }} />
                      </Button>
                      <Tooltip.Content>编辑</Tooltip.Content>
                    </Tooltip>
                    <Tooltip delay={0}>
                      <Button
                        isIconOnly variant="ghost" size="sm"
                        isPending={deleting === s.id}
                        onPress={() => handleDelete(s.id)}
                      >
                        <Trash2 size={13} style={{ color: "#ef4444" }} />
                      </Button>
                      <Tooltip.Content>删除</Tooltip.Content>
                    </Tooltip>
                  </div>
                </Card.Content>
              </Card>
            );
          })}
          {filteredSources.length === 0 && (
            <div className="col-span-full">
              <EmptyState icon={<BookOpen size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无知识源" description="上传文件、导入 URL 或仓库开始" />
            </div>
          )}
        </div>
      </div>

      {/* Edit Knowledge Modal */}
      {editSource && (
        <div className="fixed inset-0 z-50 flex items-center justify-center" style={{ background: "rgba(0,0,0,0.5)", backdropFilter: "blur(4px)" }} onClick={() => setEditSource(null)}>
          <div className="w-[480px] max-h-[80vh] rounded-2xl p-6 space-y-4 animate-scale-in" style={{ background: "var(--glass-card, var(--yunque-card))", border: "1px solid var(--glass-edge, var(--yunque-border))", color: "var(--yunque-text)", backdropFilter: "blur(var(--yunque-glass-blur)) saturate(var(--yunque-glass-saturate))" }} onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between">
              <h3 className="text-base font-semibold">编辑知识</h3>
              <Button isIconOnly variant="ghost" size="sm" onPress={() => setEditSource(null)}><X size={16} /></Button>
            </div>
            <div className="space-y-3">
              <div>
                <label className="text-xs font-medium mb-1 block" style={{ color: "var(--yunque-text-secondary)" }}>名称</label>
                <input value={editName} onChange={(e) => setEditName(e.target.value)} className="w-full px-3 py-2 rounded-lg text-sm bg-transparent outline-none" style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }} />
              </div>
              <div>
                <label className="text-xs font-medium mb-1 block" style={{ color: "var(--yunque-text-secondary)" }}>使用时机</label>
                <input value={editTrigger} onChange={(e) => setEditTrigger(e.target.value)} placeholder="何时调取这条知识" className="w-full px-3 py-2 rounded-lg text-sm bg-transparent outline-none" style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }} />
              </div>
              <div>
                <label className="text-xs font-medium mb-1 block" style={{ color: "var(--yunque-text-secondary)" }}>内容</label>
                <div className="relative">
                  <textarea value={editContent} onChange={(e) => setEditContent(e.target.value)} placeholder="留空则只更新名称和使用时机" rows={6} maxLength={2000} className="w-full resize-none px-3 py-2 text-sm rounded-lg outline-none bg-transparent" style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }} />
                  <span className="absolute bottom-2 right-3 text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{editContent.length} / 2000</span>
                </div>
              </div>
            </div>
            <div className="flex items-center justify-between pt-2">
              <Button size="sm" variant="ghost" onPress={() => handleDelete(editSource.id).then(() => setEditSource(null))} style={{ color: "#ef4444" }}>删除</Button>
              <div className="flex gap-2">
                <Button size="sm" variant="ghost" onPress={() => setEditSource(null)}>取消</Button>
                <Button size="sm" isPending={saving} onPress={handleSaveEdit} className="btn-accent">保存</Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
