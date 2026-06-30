"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { api, type KBSource, type KBChunk, type KBStats, type KBImportTreeNode } from "@/lib/api";
import {
  Button, Spinner, SearchField, Tooltip, Chip, Checkbox,
  Modal, TextField, Label, Input, TextArea, NumberField,
} from "@heroui/react";
import { KPI, KPIGroup, DropZone, Segment } from "@heroui-pro/react";
import {
  BookOpen, Upload, File, RefreshCw, FileText, Database,
  Globe, GitBranch, Trash2, X, Link, FolderGit, Filter,
  ChevronRight, ChevronDown, ExternalLink, HardDrive, Search,
  FileCode, BarChart3, Pencil,
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

const UPLOAD_ACCEPT = ".txt,.md,.pdf,.docx,.csv,.json,.html,.py,.go,.js,.ts,.java,.c,.cpp,.rs,.rb";

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

  // Shared upload: preserves the original per-file api.kbUpload loop.
  // Wired to both the drop-zone drag handler and the file-picker select handler.
  const uploadFiles = useCallback(async (files: FileList | File[]) => {
    const list = Array.from(files);
    if (!list.length) return;
    setUploading(true);
    try {
      for (let i = 0; i < list.length; i++) {
        await api.kbUpload(list[i]);
      }
      refresh();
    } catch (e) { showToast(e instanceof Error ? e.message : "上传失败", "error"); }
    setUploading(false);
    if (fileRef.current) fileRef.current.value = "";
  }, [refresh]);

  const handleUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) uploadFiles(e.target.files);
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
            <Button isIconOnly aria-label="刷新知识库" variant="ghost" size="sm" onPress={refresh}><RefreshCw size={14} /></Button>
            <Tooltip.Content>刷新</Tooltip.Content>
          </Tooltip>
          <input ref={fileRef} type="file" multiple className="hidden" onChange={handleUpload} accept={UPLOAD_ACCEPT} />
          <Button size="sm" isPending={uploading} onPress={() => fileRef.current?.click()} className="btn-accent">
            <Upload size={14} /> 上传文件
          </Button>
        </div>
      </div>

      {/* Stats — Pro KPI cards */}
      <KPIGroup className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <KPI>
          <KPI.Header>
            <KPI.Icon><Database size={16} /></KPI.Icon>
            <KPI.Title>知识源</KPI.Title>
          </KPI.Header>
          <KPI.Content>
            <KPI.Value value={stats?.sources ?? sources.length} maximumFractionDigits={0} />
          </KPI.Content>
        </KPI>
        <KPI>
          <KPI.Header>
            <KPI.Icon status="success"><FileText size={16} /></KPI.Icon>
            <KPI.Title>总片段</KPI.Title>
          </KPI.Header>
          <KPI.Content>
            <KPI.Value value={totalChunks} maximumFractionDigits={0} />
          </KPI.Content>
        </KPI>
        <KPI>
          <KPI.Header>
            <KPI.Icon status="warning"><HardDrive size={16} /></KPI.Icon>
            <KPI.Title>总字数</KPI.Title>
          </KPI.Header>
          <KPI.Content>
            <KPI.Value value={totalChars} notation="compact" maximumFractionDigits={1} />
          </KPI.Content>
        </KPI>
        <KPI>
          <KPI.Header>
            <KPI.Icon><BarChart3 size={16} /></KPI.Icon>
            <KPI.Title>搜索结果</KPI.Title>
          </KPI.Header>
          <KPI.Content>
            <KPI.Value value={searchResults.length} maximumFractionDigits={0} />
          </KPI.Content>
        </KPI>
      </KPIGroup>

      {/* Upload — Pro DropZone (drag + click both reuse uploadFiles) */}
      <DropZone>
        <DropZone.Area onDrop={async (e) => {
          const files: File[] = [];
          for (const item of e.items) {
            if (item.kind === "file") {
              try { files.push(await item.getFile()); } catch { /* skip non-file */ }
            }
          }
          if (files.length) uploadFiles(files);
        }}>
          <DropZone.Icon />
          <DropZone.Label>拖拽文件到此处，或点击下方按钮选择</DropZone.Label>
          <DropZone.Description>支持 txt / md / pdf / docx / csv / json / 代码文件</DropZone.Description>
          <DropZone.Trigger>选择文件</DropZone.Trigger>
        </DropZone.Area>
        <DropZone.Input multiple accept={UPLOAD_ACCEPT} onSelect={(files) => uploadFiles(files)} />
      </DropZone>

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
        <div className="section-card rounded-xl p-4 space-y-3 animate-scale-in">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium flex items-center gap-2"><Globe size={14} /> URL 导入</span>
            <Tooltip delay={0}><Button isIconOnly aria-label="关闭" variant="ghost" size="sm" onPress={() => { setImportMode(null); setImportTree(null); }}><X size={14} /></Button><Tooltip.Content>关闭</Tooltip.Content></Tooltip>
          </div>
          <div className="flex gap-2 items-end">
            <TextField className="flex-1" value={importUrl} onChange={setImportUrl} aria-label="导入 URL">
              <Input placeholder="https://example.com/docs" onKeyDown={(e) => e.key === "Enter" && handleImportUrl()} />
            </TextField>
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
              <NumberField
                value={parseInt(maxPages) || 0}
                onChange={(v) => setMaxPages(Number.isNaN(v) ? "" : String(v))}
                minValue={1} maxValue={100} aria-label="最大页数" className="w-32"
              >
                <Label className="text-xs" style={{ color: "var(--yunque-text-secondary)" }}>最大页数</Label>
                <NumberField.Group>
                  <NumberField.Input />
                  <NumberField.DecrementButton />
                  <NumberField.IncrementButton />
                </NumberField.Group>
              </NumberField>
            )}
          </div>
          {importTree && (
            <div className="rounded-lg p-3" style={{ background: "var(--yunque-bg-muted)", border: "1px solid var(--yunque-border)" }}>
              <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text-muted)" }}>导入结构</div>
              <ImportTreeView node={importTree} />
            </div>
          )}
        </div>
      )}

      {/* Repo Import Panel */}
      {importMode === "repo" && (
        <div className="section-card rounded-xl p-4 space-y-3 animate-scale-in">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium flex items-center gap-2"><FolderGit size={14} /> 仓库导入</span>
            <Tooltip delay={0}><Button isIconOnly aria-label="关闭" variant="ghost" size="sm" onPress={() => setImportMode(null)}><X size={14} /></Button><Tooltip.Content>关闭</Tooltip.Content></Tooltip>
          </div>
          <div className="flex gap-2 items-end">
            <TextField className="flex-1" value={repoPath} onChange={setRepoPath} aria-label="本地仓库路径">
              <Input placeholder="本地仓库路径, 如 /home/user/project" onKeyDown={(e) => e.key === "Enter" && handleImportRepo()} />
            </TextField>
            <Button size="sm" isPending={importingRepo} onPress={handleImportRepo} className="btn-accent">
              <GitBranch size={13} /> 导入
            </Button>
          </div>
          <NumberField
            value={parseInt(maxFiles) || 0}
            onChange={(v) => setMaxFiles(Number.isNaN(v) ? "" : String(v))}
            minValue={1} maxValue={1000} aria-label="最大文件数" className="w-40"
          >
            <Label className="text-xs" style={{ color: "var(--yunque-text-secondary)" }}>最大文件数</Label>
            <NumberField.Group>
              <NumberField.Input />
              <NumberField.DecrementButton />
              <NumberField.IncrementButton />
            </NumberField.Group>
          </NumberField>
        </div>
      )}

      {/* Text Ingest Panel */}
      {importMode === "text" && (
        <div className="section-card rounded-xl p-4 space-y-3 animate-scale-in">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium flex items-center gap-2"><FileCode size={14} /> 文本导入</span>
            <Tooltip delay={0}><Button isIconOnly aria-label="关闭" variant="ghost" size="sm" onPress={() => setImportMode(null)}><X size={14} /></Button><Tooltip.Content>关闭</Tooltip.Content></Tooltip>
          </div>
          <TextField value={ingestName} onChange={setIngestName} aria-label="知识名称" fullWidth>
            <Input placeholder="知识名称" />
          </TextField>
          <TextField value={ingestTrigger} onChange={setIngestTrigger} aria-label="使用时机" fullWidth>
            <Input placeholder="使用时机（可选）, 如 当需要设计或排版时" />
          </TextField>
          <TextField value={ingestContent} onChange={setIngestContent} aria-label="文本内容" fullWidth>
            <TextArea placeholder="粘贴文本内容..." rows={6} />
          </TextField>
          <Button size="sm" isPending={ingesting} onPress={handleIngest} isDisabled={!ingestName.trim() || !ingestContent.trim()} className="btn-accent">
            导入文本
          </Button>
        </div>
      )}

      {/* Search with filters */}
      <div className="section-card rounded-xl p-4 space-y-3">
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
        <div className="flex gap-3 items-end flex-wrap">
          <div className="flex items-center gap-1.5 text-[11px] pb-2" style={{ color: "var(--yunque-text-muted)" }}>
            <Filter size={12} /> 过滤:
          </div>
          <TextField value={fileFilter} onChange={setFileFilter} aria-label="按文件名过滤" className="w-[150px]">
            <Input placeholder="文件名…" />
          </TextField>
          <TextField value={langFilter} onChange={setLangFilter} aria-label="按语言过滤" className="w-[150px]">
            <Input placeholder="语言 (go/py/js)..." />
          </TextField>
          {(fileFilter || langFilter) && (
            <Button size="sm" variant="danger" onPress={() => { setFileFilter(""); setLangFilter(""); }} className="text-[11px]">
              <X size={10} /> 清除过滤
            </Button>
          )}
        </div>
      </div>

      {/* Search results */}
      {searchResults.length > 0 && (
        <div className="space-y-2 animate-fade-in">
          <h3 className="text-sm font-medium flex items-center gap-2">
            搜索结果
            <Chip size="sm" variant="soft" color="accent">{searchResults.length}</Chip>
          </h3>
          {searchResults.map((r, i) => (
            <div key={i} className="section-card rounded-xl p-3 hover-lift transition-all duration-200">
              <div className="text-xs leading-relaxed whitespace-pre-wrap" style={{ color: "var(--yunque-text-secondary)" }}>{r.content}</div>
              <div className="mt-2 flex items-center gap-2 flex-wrap">
                <Chip size="sm" variant="soft">源: {r.source_id}</Chip>
                <Chip size="sm" variant="soft">#{r.index}</Chip>
                {r.metadata && Object.entries(r.metadata).slice(0, 3).map(([k, v]) => (
                  <Chip key={k} size="sm" variant="soft">{k}: {v}</Chip>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Type filter — Pro Segment */}
      {types.length > 2 && (
        <Segment
          size="sm"
          selectedKey={typeFilter}
          onSelectionChange={(k) => setTypeFilter(String(k))}
        >
          {types.map((t) => (
            <Segment.Item key={t} id={t}>{t === "all" ? "全部" : t}</Segment.Item>
          ))}
        </Segment>
      )}

      {/* Sources grid */}
      <div>
        <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
          知识源
          <Chip size="sm" variant="soft" color="accent">{filteredSources.length}</Chip>
        </h3>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3 stagger-children">
          {filteredSources.map((s) => {
            const typeIcon = s.type === "url" ? <Globe size={18} style={{ color: "var(--yunque-info)" }} /> :
                             s.type === "repo" ? <FolderGit size={18} style={{ color: "var(--yunque-accent)" }} /> :
                             s.type === "code" ? <FileCode size={18} style={{ color: "var(--yunque-success)" }} /> :
                             <File size={18} style={{ color: "var(--yunque-accent)" }} />;
            return (
              <div key={s.id || s.name} className="section-card rounded-xl p-3 hover-lift transition-all duration-200 group">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-lg flex items-center justify-center shrink-0" style={{ background: "var(--yunque-accent-soft)" }}>
                    {typeIcon}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium truncate">{s.name}</div>
                    <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                      <Chip size="sm" variant="soft">{s.chunk_count} 片段</Chip>
                      <Chip size="sm" variant="soft">{s.type}</Chip>
                      {s.trigger && <Chip size="sm" variant="soft" color="success">{s.trigger}</Chip>}
                      {s.added_at && (
                        <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{new Date(s.added_at).toLocaleDateString()}</span>
                      )}
                    </div>
                  </div>
                  <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Tooltip delay={0}>
                      <Button isIconOnly aria-label={`编辑知识源 ${s.name}`} variant="ghost" size="sm" onPress={() => openEdit(s)}>
                        <Pencil size={13} style={{ color: "var(--yunque-accent)" }} />
                      </Button>
                      <Tooltip.Content>编辑</Tooltip.Content>
                    </Tooltip>
                    <Tooltip delay={0}>
                      <Button
                        isIconOnly variant="ghost" size="sm"
                        aria-label={`删除知识源 ${s.name}`}
                        isPending={deleting === s.id}
                        onPress={() => handleDelete(s.id)}
                      >
                        <Trash2 size={13} style={{ color: "var(--yunque-danger)" }} />
                      </Button>
                      <Tooltip.Content>删除</Tooltip.Content>
                    </Tooltip>
                  </div>
                </div>
              </div>
            );
          })}
          {filteredSources.length === 0 && (
            <div className="col-span-full">
              <EmptyState icon={<BookOpen size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无知识源" description="上传文件、导入 URL 或仓库开始" />
            </div>
          )}
        </div>
      </div>

      {/* Edit Knowledge Modal — OSS Modal */}
      <Modal.Backdrop isOpen={!!editSource} onOpenChange={(o) => { if (!o) setEditSource(null); }} variant="blur">
        <Modal.Container size="md" placement="center">
          <Modal.Dialog className="sm:max-w-[480px]">
            <Modal.CloseTrigger />
            <Modal.Header>
              <Modal.Heading>编辑知识</Modal.Heading>
            </Modal.Header>
            <Modal.Body className="space-y-3">
              <TextField value={editName} onChange={setEditName} aria-label="名称" fullWidth>
                <Label>名称</Label>
                <Input />
              </TextField>
              <TextField value={editTrigger} onChange={setEditTrigger} aria-label="使用时机" fullWidth>
                <Label>使用时机</Label>
                <Input placeholder="何时调取这条知识" />
              </TextField>
              <TextField value={editContent} onChange={setEditContent} aria-label="内容" fullWidth>
                <Label>内容</Label>
                <TextArea placeholder="留空则只更新名称和使用时机" rows={6} maxLength={2000} />
                <span className="self-end text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{editContent.length} / 2000</span>
              </TextField>
            </Modal.Body>
            <Modal.Footer className="justify-between">
              <Button size="sm" variant="ghost" onPress={() => editSource && handleDelete(editSource.id).then(() => setEditSource(null))} style={{ color: "var(--yunque-danger)" }}>删除</Button>
              <div className="flex gap-2">
                <Button size="sm" variant="ghost" slot="close">取消</Button>
                <Button size="sm" isPending={saving} onPress={handleSaveEdit} className="btn-accent">保存</Button>
              </div>
            </Modal.Footer>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </div>
  );
}
