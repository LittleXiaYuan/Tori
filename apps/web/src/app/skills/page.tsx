"use client";

import { useState, useEffect, useCallback } from "react";
import { api, type SkillInfo, type SkillHubItem, type SkillHubInstalledItem, type DynamicSkillDef } from "@/lib/api";
import { Card, Button, Spinner, Tabs, Chip, SearchField, Tooltip, Badge } from "@heroui/react";
import {
  Package, Download, Trash2, Star, Wrench, Check, RefreshCw, Globe, HardDrive, TrendingUp, GitFork, FolderSearch, ArrowUpDown,
} from "lucide-react";
import { showToast } from "@/components/toast-provider";
import EmptyState from "@/components/empty-state";

type TabKey = "installed" | "market" | "dynamic";
type HubSource = "" | "clawhub" | "torihub";
type SortKey = "usage" | "name" | "success";

interface SkillCatInfo { id: string; name: string; description: string }

export default function SkillsPage() {
  const [tab, setTab] = useState<TabKey>("installed");
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [skillCategories, setSkillCategories] = useState<SkillCatInfo[]>([]);
  const [hubInstalled, setHubInstalled] = useState<SkillHubInstalledItem[]>([]);
  const [dynamicSkills, setDynamicSkills] = useState<DynamicSkillDef[]>([]);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState("");
  const [installedFilter, setInstalledFilter] = useState("");
  const [installedCatFilter, setInstalledCatFilter] = useState("");
  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [results, setResults] = useState<SkillHubItem[]>([]);
  const [allItems, setAllItems] = useState<SkillHubItem[]>([]);
  const [installing, setInstalling] = useState<string | null>(null);
  const [source, setSource] = useState<HubSource>("");
  const [marketLoading, setMarketLoading] = useState(false);
  const [githubSlug, setGithubSlug] = useState("");
  const [scanning, setScanning] = useState(false);

  const refreshInstalled = useCallback(() => {
    return Promise.all([
      api.skills().then((r) => {
        setSkills(Array.isArray(r.skills) ? r.skills : []);
        setSkillCategories(Array.isArray(r.categories) ? r.categories : []);
      }).catch(() => {}),
      api.skillHubInstalled().then((r) => setHubInstalled(Array.isArray(r.skills) ? r.skills : [])).catch(() => {}),
      api.getDynamicSkills().then(setDynamicSkills).catch(() => {}),
    ]);
  }, []);

  useEffect(() => { refreshInstalled().finally(() => setLoading(false)); }, [refreshInstalled]);

  useEffect(() => {
    if (tab !== "market") return;
    setMarketLoading(true);
    // Try trending first; fall back to search-based discovery if trending returns empty
    api.skillHubTrending(48, "", source).then(async (r) => {
      const items = Array.isArray(r.skills) ? r.skills : [];
      if (items.length > 0) { setAllItems(items); setMarketLoading(false); return; }
      // Trending empty — populate with search results from popular categories
      const seeds = ["python", "code", "web", "data", "document", "api", "test", "design"];
      const all: SkillHubItem[] = [];
      const seen = new Set<string>();
      for (const kw of seeds) {
        try {
          const sr = await api.skillHubSearch(kw, 8, source);
          for (const item of (Array.isArray(sr.results) ? sr.results : [])) {
            if (!seen.has(item.name)) { seen.add(item.name); all.push(item); }
          }
        } catch {}
        if (all.length >= 48) break;
      }
      setAllItems(all.slice(0, 48));
    }).catch(() => {}).finally(() => setMarketLoading(false));
  }, [tab, source]);

  const handleSearch = async () => {
    if (!query.trim()) return;
    const r = await api.skillHubSearch(query, 24, source);
    setResults(Array.isArray(r.results) ? r.results : []);
  };

  const handleInstall = async (name: string) => {
    setInstalling(name);
    try { await api.skillHubInstall(name); await refreshInstalled(); showToast("安装成功", "success"); } catch (e) { showToast(e instanceof Error ? e.message : "安装失败", "error"); }
    setInstalling(null);
  };

  const handleUninstall = async (name: string) => {
    try { await api.skillHubUninstall(name); await refreshInstalled(); showToast("已卸载", "success"); } catch (e) { showToast(e instanceof Error ? e.message : "卸载失败", "error"); }
  };

  const handleApprove = async (name: string) => {
    try { await api.approveDynamicSkill(name); await refreshInstalled(); showToast("已批准", "success"); } catch (e) { showToast(e instanceof Error ? e.message : "操作失败", "error"); }
  };

  const handleScanSkills = async () => {
    setScanning(true);
    try {
      const r = await api.scanSkills();
      await refreshInstalled();
      showToast(`扫描完成：加载了 ${r.skills_loaded} 个文件技能`, "success");
    } catch (e) { showToast(e instanceof Error ? e.message : "扫描失败", "error"); }
    setScanning(false);
  };

  const handleGithubInstall = async () => {
    const slug = githubSlug.trim();
    if (!slug || !slug.includes("/")) { showToast("请输入 owner/repo 格式", "error"); return; }
    setInstalling(slug);
    try { await api.skillHubInstall(slug); await refreshInstalled(); showToast(`${slug} 安装成功`, "success"); setGithubSlug(""); } catch (e) { showToast(e instanceof Error ? e.message : "安装失败", "error"); }
    setInstalling(null);
  };

  if (loading) return <div className="flex-1 flex items-center justify-center"><Spinner size="lg" /></div>;

  const items = query.trim() ? results : allItems;
  const filteredItems = source ? items.filter(i => i.source === source) : items;

  const sourceFilters: { key: HubSource; label: string; icon?: React.ReactNode }[] = [
    { key: "", label: "全部" },
    { key: "clawhub", label: "ClawHub", icon: <Globe size={11} /> },
    { key: "torihub", label: "ToriHub", icon: <HardDrive size={11} /> },
  ];

  return (
    <div className="page-root space-y-5 animate-fade-in-up" style={{ color: "var(--yunque-text)" }}>
      <div className="flex items-center justify-between">
        <h1 className="page-title flex items-center gap-2"><Package size={20} /> 技能</h1>
        <div className="flex items-center gap-1">
          <Tooltip delay={0}>
            <Button variant="ghost" size="sm" isPending={scanning} onPress={handleScanSkills}><FolderSearch size={14} /></Button>
            <Tooltip.Content>扫描 data/skills/ 目录</Tooltip.Content>
          </Tooltip>
          <Tooltip delay={0}>
            <Button variant="ghost" size="sm" onPress={() => { refreshInstalled(); }}><RefreshCw size={14} /></Button>
            <Tooltip.Content>刷新</Tooltip.Content>
          </Tooltip>
        </div>
      </div>

      <Tabs selectedKey={tab} onSelectionChange={(k) => setTab(k as TabKey)}>
        <Tabs.ListContainer>
          <Tabs.List aria-label="技能分组">
            <Tabs.Tab id="installed">
              已安装
              <Badge><Chip style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)", fontSize: "var(--text-xs)" }}>{skills.length}</Chip></Badge>
              <Tabs.Indicator />
            </Tabs.Tab>
            <Tabs.Tab id="market">
              <Tabs.Separator />
              技能市场
              <Tabs.Indicator />
            </Tabs.Tab>
            <Tabs.Tab id="dynamic">
              <Tabs.Separator />
              动态技能
              {dynamicSkills.length > 0 && <Chip style={{ background: "rgba(34,197,94,0.1)", color: "#22c55e", fontSize: "var(--text-xs)" }}>{dynamicSkills.length}</Chip>}
              <Tabs.Indicator />
            </Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>
      </Tabs>

      {tab === "installed" && (() => {
        const lowerFilter = installedFilter.toLowerCase();
        const filtered = skills
          .filter((s) => {
            if (installedCatFilter && (s.category || "") !== installedCatFilter) return false;
            if (!lowerFilter) return true;
            return s.name.toLowerCase().includes(lowerFilter) || (s.description || "").toLowerCase().includes(lowerFilter);
          })
          .sort((a, b) => {
            if (sortKey === "usage") {
              const diff = (b.usage_total || 0) - (a.usage_total || 0);
              return diff !== 0 ? diff : a.name.localeCompare(b.name);
            }
            if (sortKey === "success") {
              const diff = (b.success_rate || 0) - (a.success_rate || 0);
              return diff !== 0 ? diff : a.name.localeCompare(b.name);
            }
            return a.name.localeCompare(b.name);
          });
        const sortOptions: { key: SortKey; label: string }[] = [
          { key: "usage", label: "使用量" },
          { key: "success", label: "成功率" },
          { key: "name", label: "名称" },
        ];
        return (
          <div className="space-y-4">
            <div className="flex items-center gap-3 flex-wrap">
              <SearchField className="flex-1 min-w-[200px]" name="installed-search" value={installedFilter} onChange={setInstalledFilter}>
                <SearchField.Group>
                  <SearchField.SearchIcon />
                  <SearchField.Input placeholder="搜索已安装技能..." />
                  <SearchField.ClearButton />
                </SearchField.Group>
              </SearchField>
              <div
                className="flex gap-1 p-1 rounded-full border shrink-0"
                style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-card)" }}
              >
                <ArrowUpDown size={11} style={{ color: "var(--yunque-text-muted)", margin: "auto 4px" }} />
                {sortOptions.map(({ key, label }) => (
                  <button
                    key={key}
                    onClick={() => setSortKey(key)}
                    className="px-2.5 py-1 rounded-full text-[11px] font-medium transition-all"
                    style={{
                      background: sortKey === key ? "rgba(255,255,255,0.08)" : "transparent",
                      color: sortKey === key ? "var(--yunque-text)" : "var(--yunque-text-muted)",
                    }}
                  >
                    {label}
                  </button>
                ))}
              </div>
              {skillCategories.length > 0 && (
                <div
                  className="flex gap-1 p-1 rounded-full border shrink-0"
                  style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-card)" }}
                >
                  <button
                    onClick={() => setInstalledCatFilter("")}
                    className="px-3 py-1 rounded-full text-[11px] font-medium transition-all"
                    style={{
                      background: !installedCatFilter ? "rgba(255,255,255,0.08)" : "transparent",
                      color: !installedCatFilter ? "var(--yunque-text)" : "var(--yunque-text-muted)",
                    }}
                  >
                    全部
                  </button>
                  {skillCategories.map((cat) => (
                    <button
                      key={cat.id}
                      onClick={() => setInstalledCatFilter(installedCatFilter === cat.id ? "" : cat.id)}
                      className="px-3 py-1 rounded-full text-[11px] font-medium transition-all"
                      style={{
                        background: installedCatFilter === cat.id ? "rgba(255,255,255,0.08)" : "transparent",
                        color: installedCatFilter === cat.id ? "var(--yunque-text)" : "var(--yunque-text-muted)",
                      }}
                    >
                      {cat.name}
                    </button>
                  ))}
                </div>
              )}
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 stagger-children">
              {filtered.map((s) => (
                <Card key={s.name} className="section-card hover-lift transition-all duration-200">
                  <Card.Header className="flex items-center justify-between">
                    <div className="flex items-center gap-2 min-w-0">
                      <Wrench size={15} style={{ color: "var(--yunque-accent)", flexShrink: 0 }} />
                      <span className="font-medium truncate" style={{ fontSize: "var(--text-base)" }}>{s.name}</span>
                    </div>
                    <div className="flex items-center gap-1 shrink-0">
                      {s.category && (
                        <Chip size="sm" style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>
                          {skillCategories.find((c) => c.id === s.category)?.name || s.category}
                        </Chip>
                      )}
                      {hubInstalled.find((h) => h.slug === s.name) && (
                        <Tooltip delay={0}>
                          <Button isIconOnly variant="ghost" size="sm" onPress={() => handleUninstall(s.name)}>
                            <Trash2 size={13} />
                          </Button>
                          <Tooltip.Content>卸载</Tooltip.Content>
                        </Tooltip>
                      )}
                    </div>
                  </Card.Header>
                  <Card.Content className="pt-0">
                    <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>{s.description || "无描述"}</div>
                    {(s.usage_total || 0) > 0 && (
                      <div className="flex items-center gap-3 mt-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                        <span>{s.usage_total} 次调用</span>
                        <span style={{ color: (s.success_rate || 0) >= 0.8 ? "var(--yunque-success)" : (s.success_rate || 0) >= 0.5 ? "var(--yunque-warning)" : "var(--yunque-danger)" }}>
                          {Math.round((s.success_rate || 0) * 100)}% 成功
                        </span>
                      </div>
                    )}
                  </Card.Content>
                </Card>
              ))}
              {filtered.length === 0 && (
                <div className="col-span-full">
                  <EmptyState icon={<Package size={24} style={{ color: "var(--yunque-accent)" }} />} title={installedFilter || installedCatFilter ? "无匹配技能" : "暂无已安装技能"} description={installedFilter || installedCatFilter ? "尝试调整搜索词或分类筛选" : "开始对话后，Agent 会自动创建和积累技能；你也可以在「市场」标签中安装社区技能。"} />
                </div>
              )}
            </div>
          </div>
        );
      })()}

      {tab === "market" && (
        <div className="space-y-4">
          {/* Source Filter Tabs */}
          <div className="flex items-center gap-2 flex-wrap">
            <div
              className="flex gap-1 p-1 rounded-full border"
              style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-card)", width: "fit-content" }}
            >
              {sourceFilters.map(({ key, label, icon }) => (
                <button
                  key={key}
                  onClick={() => { setSource(key); setResults([]); setQuery(""); }}
                  className="px-3 py-1 rounded-full text-[11px] font-medium transition-all flex items-center gap-1.5"
                  style={{
                    background: source === key ? "rgba(255,255,255,0.08)" : "transparent",
                    color: source === key ? "var(--yunque-text)" : "var(--yunque-text-muted)",
                  }}
                >
                  {icon}{label}
                </button>
              ))}
            </div>
            {source && (
              <Chip
                size="sm"
                style={{
                  background: source === "clawhub" ? "rgba(0,111,238,0.12)" : "rgba(139,92,246,0.12)",
                  color: source === "clawhub" ? "var(--yunque-accent)" : "#8b5cf6",
                  fontSize: "var(--text-xs)",
                }}
              >
                {source === "clawhub" ? "ClawHub 技能市场" : "ToriHub 技能市场"}
              </Chip>
            )}
          </div>

          {/* Search */}
          <div className="flex gap-2 items-end">
            <SearchField className="flex-1" name="skill-search" value={query} onChange={setQuery} onSubmit={handleSearch}>
              <SearchField.Group>
                <SearchField.SearchIcon />
                <SearchField.Input placeholder="搜索技能.." />
                <SearchField.ClearButton />
              </SearchField.Group>
            </SearchField>
            <Button size="sm" onPress={handleSearch} className="btn-accent">搜索</Button>
          </div>

          {/* GitHub Slug Install */}
          <div
            className="flex gap-2 items-center p-3 rounded-lg border"
            style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-card)" }}
          >
            <GitFork size={16} style={{ color: "var(--yunque-text-muted)", flexShrink: 0 }} />
            <input
              className="flex-1 bg-transparent text-sm outline-none"
              style={{ color: "var(--yunque-text)" }}
              placeholder="从 GitHub 安装: owner/repo"
              value={githubSlug}
              onChange={(e) => setGithubSlug(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleGithubInstall()}
            />
            <Button
              size="sm"
              isDisabled={!githubSlug.trim() || installing === githubSlug.trim()}
              isPending={installing === githubSlug.trim()}
              onPress={handleGithubInstall}
              className="btn-accent shrink-0"
            >
              <Download size={12} /> 安装
            </Button>
          </div>

          {/* Market Loading */}
          {marketLoading && (
            <div className="flex items-center justify-center py-8 gap-2">
              <Spinner size="sm" />
              <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>正在发现技能...</span>
            </div>
          )}

          {/* Grid */}
          {!marketLoading && (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 stagger-children">
            {filteredItems.map((item) => (
              <Card key={item.name} className="section-card hover-lift transition-all duration-200">
                <Card.Header className="flex items-center justify-between gap-2">
                  <span className="font-medium truncate flex-1" style={{ fontSize: "var(--text-base)" }}>{item.name}</span>
                  <div className="flex items-center gap-1.5 shrink-0">
                    <Chip style={{ background: "rgba(245,158,11,0.1)", color: "#f59e0b", fontSize: "var(--text-2xs)" }}>
                      <Star size={11} className="mr-0.5" /> {item.rating || 0}
                    </Chip>
                    <Chip
                      size="sm"
                      style={{
                        background: item.source === "clawhub" ? "rgba(0,111,238,0.08)" : "rgba(139,92,246,0.08)",
                        color: item.source === "clawhub" ? "var(--yunque-accent)" : "#8b5cf6",
                        fontSize: "var(--text-2xs)",
                      }}
                    >
                      {item.source === "clawhub" ? <Globe size={10} /> : <HardDrive size={10} />}
                      {item.source}
                    </Chip>
                  </div>
                </Card.Header>
                <Card.Content className="pt-0 flex items-end justify-between gap-2">
                  <span className="text-sm flex-1 line-clamp-2" style={{ color: "var(--yunque-text-secondary)" }}>{item.description || ""}</span>
                  <Button
                    size="sm"
                    isDisabled={installing === item.name || item.installed}
                    isPending={installing === item.name}
                    onPress={() => handleInstall(item.name)}
                    className="rounded-lg shrink-0"
                    style={{
                      background: item.installed ? "transparent" : "var(--yunque-accent)",
                      color: item.installed ? "var(--yunque-success)" : "#fff",
                    }}
                  >
                    {item.installed ? <><Check size={12} /> 已安装</> : <><Download size={12} /> 安装</>}
                  </Button>
                </Card.Content>
              </Card>
            ))}
            {filteredItems.length === 0 && (
              <div className="col-span-full">
                <EmptyState
                  icon={<Globe size={24} style={{ color: "var(--yunque-accent)" }} />}
                  title={`${source === "torihub" ? "ToriHub" : source === "clawhub" ? "ClawHub" : "ClawHub / ToriHub"} 暂无可用技能`}
                  description="请检查网络连接或稍后重试"
                />
              </div>
            )}
          </div>
          )}
        </div>
      )}

      {tab === "dynamic" && (
        <div className="space-y-3 stagger-children">
          {dynamicSkills.map((ds) => (
            <Card key={ds.name} className="section-card hover-lift transition-all duration-200">
              <Card.Header className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <TrendingUp size={14} style={{ color: "#22c55e" }} />
                  <span className="text-sm font-medium">{ds.name}</span>
                  <Chip style={{
                    background: ds.approval_status === "approved" ? "rgba(34,197,94,0.1)" : "rgba(245,158,11,0.1)",
                    color: ds.approval_status === "approved" ? "#22c55e" : "#f59e0b",
                    fontSize: "var(--text-2xs)",
                  }}>
                    {ds.approval_status === "approved" ? "已批准" : "待审核"}
                  </Chip>
                </div>
                {ds.approval_status !== "approved" && (
                  <Button size="sm" onPress={() => handleApprove(ds.name)} className="btn-accent">
                    <Check size={12} /> {"批准"}
                  </Button>
                )}
              </Card.Header>
              <Card.Content className="pt-0 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
                {ds.description || "AI 自动发现的技能"}
              </Card.Content>
            </Card>
          ))}
          {dynamicSkills.length === 0 && (
            <EmptyState icon={<TrendingUp size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无动态技能" description="Agent 在任务执行过程中会自动归纳、学习新技能，后续将显示在这里。" />
          )}
        </div>
      )}
    </div>
  );
}
