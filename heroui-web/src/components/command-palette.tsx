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
  { id: "nav-dashboard", label: "概览", group: "导航", icon: <LayoutDashboard size={16} />, keywords: "dashboard home 主页 仪表盘" },
  { id: "nav-chat", label: "对话", group: "导航", icon: <MessageCircle size={16} />, keywords: "chat 聊天 会话" },
  { id: "nav-missions", label: "任务中心", group: "导航", icon: <Zap size={16} />, keywords: "missions tasks 任务" },
  { id: "nav-task-run", label: "执行视图", group: "导航", icon: <Terminal size={16} />, keywords: "task run 执行 运行" },
  { id: "nav-workflows", label: "工作流", group: "导航", icon: <Blocks size={16} />, keywords: "workflow 工作流 流程" },
  { id: "nav-inbox", label: "收件箱", group: "导航", icon: <MailWarning size={16} />, keywords: "inbox 消息 通知" },
  { id: "nav-knowledge", label: "知识库", group: "导航", icon: <BookOpen size={16} />, keywords: "knowledge 知识 RAG" },
  { id: "nav-memory", label: "记忆", group: "导航", icon: <Brain size={16} />, keywords: "memory 记忆" },
  { id: "nav-graph", label: "知识图谱", group: "导航", icon: <Share2 size={16} />, keywords: "graph 图谱 关系 知识" },
  { id: "nav-reflect", label: "反思", group: "导航", icon: <Lightbulb size={16} />, keywords: "reflect 反思 思考" },
  { id: "nav-persona", label: "角色", group: "导航", icon: <ScanFace size={16} />, keywords: "persona 人设 角色" },
  { id: "nav-emotions", label: "情绪", group: "导航", icon: <SmilePlus size={16} />, keywords: "emotion 情感 情绪" },
  { id: "nav-reverie", label: "内心独白", group: "导航", icon: <BrainCircuit size={16} />, keywords: "reverie 遐想 独白" },
  { id: "nav-heartbeat", label: "心跳", group: "导航", icon: <HeartPulse size={16} />, keywords: "heartbeat 心跳" },
  { id: "nav-skills", label: "技能", group: "导航", icon: <Package size={16} />, keywords: "skills 技能" },
  { id: "nav-plugins", label: "插件", group: "导航", icon: <Puzzle size={16} />, keywords: "plugins 插件" },
  { id: "nav-tools", label: "终端", group: "导航", icon: <Wrench size={16} />, keywords: "tools terminal shell 工具" },
  { id: "nav-browser", label: "浏览器", group: "导航", icon: <Globe size={16} />, keywords: "browser 浏览器 connector" },
  { id: "nav-models", label: "模型管理", group: "导航", icon: <Cpu size={16} />, keywords: "models 模型 LLM" },
  { id: "nav-providers", label: "提供商", group: "导航", icon: <Globe size={16} />, keywords: "providers 提供商 api key" },
  { id: "nav-metrics", label: "指标", group: "导航", icon: <BarChart3 size={16} />, keywords: "metrics 统计 指标" },
  { id: "nav-audit", label: "审计", group: "导航", icon: <Shield size={16} />, keywords: "audit 日志 审计" },
  { id: "nav-trust", label: "信任", group: "导航", icon: <ShieldCheck size={16} />, keywords: "trust 安全 信任" },
  { id: "nav-backup", label: "备份", group: "导航", icon: <HardDriveDownload size={16} />, keywords: "backup 导出 备份" },
  { id: "nav-settings", label: "设置", group: "导航", icon: <Settings size={16} />, keywords: "settings 偏好 设置" },
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
            placeholder="搜索页面、知识、任务..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
          />
          <kbd className="cmd-kbd">ESC</kbd>
        </div>

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

        <div className="cmd-footer">
          <span><kbd className="cmd-kbd-sm">↑↓</kbd> 选择</span>
          <span><kbd className="cmd-kbd-sm">Enter</kbd> 确认</span>
          <span><kbd className="cmd-kbd-sm">ESC</kbd> 关闭</span>
        </div>
      </div>
    </div>
  );
}
