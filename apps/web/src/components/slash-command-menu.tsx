"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Kbd } from "@heroui/react";
import { BookOpen, Brain, Calendar, Camera, Code, FileDown, FileText, GitBranch, Globe, Keyboard, Layers, ListOrdered, Mail, MessageSquare, MousePointer, Search, Sparkles, Terminal, Library } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface SlashCommand {
  id: string;
  command: string;
  title: string;
  description: string;
  icon: React.ElementType;
  category: string;
  placeholder?: string;
}

// title/description/category 存 i18n key，渲染时用 t() 翻译（见下方 render）。
const commands: SlashCommand[] = [
  { id: "navigate", command: "/navigate", title: "slash.cmd.navigate.title", description: "slash.cmd.navigate.desc", icon: Globe, category: "slash.cat.browser", placeholder: "https://example.com" },
  { id: "content", command: "/content", title: "slash.cmd.content.title", description: "slash.cmd.content.desc", icon: FileText, category: "slash.cat.browser" },
  { id: "mark", command: "/mark", title: "slash.cmd.mark.title", description: "slash.cmd.mark.desc", icon: ListOrdered, category: "slash.cat.browser" },
  { id: "click", command: "/click", title: "slash.cmd.click.title", description: "slash.cmd.click.desc", icon: MousePointer, category: "slash.cat.browser", placeholder: "Continue button" },
  { id: "type", command: "/type", title: "slash.cmd.type.title", description: "slash.cmd.type.desc", icon: Keyboard, category: "slash.cat.browser", placeholder: "Email: admin@example.com" },
  { id: "screenshot", command: "/screenshot", title: "slash.cmd.screenshot.title", description: "slash.cmd.screenshot.desc", icon: Camera, category: "slash.cat.browser" },
  { id: "search", command: "/search", title: "slash.cmd.search.title", description: "slash.cmd.search.desc", icon: Search, category: "slash.cat.tools", placeholder: "HeroUI dialog examples" },
  { id: "code", command: "/code", title: "slash.cmd.code.title", description: "slash.cmd.code.desc", icon: Code, category: "slash.cat.tools", placeholder: "Refactor auth middleware" },
  { id: "terminal", command: "/terminal", title: "slash.cmd.terminal.title", description: "slash.cmd.terminal.desc", icon: Terminal, category: "slash.cat.tools", placeholder: "npm run build" },
  { id: "think", command: "/think", title: "slash.cmd.think.title", description: "slash.cmd.think.desc", icon: Brain, category: "slash.cat.tools", placeholder: "Why is setup flow inconsistent?" },
  { id: "github_issues", command: "/github_issues", title: "slash.cmd.github_issues.title", description: "slash.cmd.github_issues.desc", icon: GitBranch, category: "slash.cat.connectors", placeholder: "owner/repo" },
  { id: "gmail_inbox", command: "/gmail_inbox", title: "slash.cmd.gmail_inbox.title", description: "slash.cmd.gmail_inbox.desc", icon: Mail, category: "slash.cat.connectors" },
  { id: "calendar_events", command: "/calendar_events", title: "slash.cmd.calendar_events.title", description: "slash.cmd.calendar_events.desc", icon: Calendar, category: "slash.cat.connectors" },
  { id: "notion_search", command: "/notion_search", title: "slash.cmd.notion_search.title", description: "slash.cmd.notion_search.desc", icon: BookOpen, category: "slash.cat.connectors", placeholder: "release checklist" },
  { id: "slack_send", command: "/slack_send", title: "slash.cmd.slack_send.title", description: "slash.cmd.slack_send.desc", icon: MessageSquare, category: "slash.cat.connectors", placeholder: "#general Release is ready" },
  { id: "skill", command: "/skill", title: "slash.cmd.skill.title", description: "slash.cmd.skill.desc", icon: Sparkles, category: "slash.cat.tools", placeholder: "web_search query about AI" },
  { id: "skills", command: "/skills", title: "slash.cmd.skills.title", description: "slash.cmd.skills.desc", icon: Layers, category: "slash.cat.tools" },
  { id: "workflow", command: "/workflow", title: "slash.cmd.workflow.title", description: "slash.cmd.workflow.desc", icon: Layers, category: "slash.cat.automation", placeholder: "pre-release checks" },
  { id: "research", command: "/research", title: "slash.cmd.research.title", description: "slash.cmd.research.desc", icon: Search, category: "slash.cat.research", placeholder: "Compare pricing of Notion vs Coda vs Confluence" },
  { id: "report", command: "/report", title: "slash.cmd.report.title", description: "slash.cmd.report.desc", icon: FileDown, category: "slash.cat.research", placeholder: "Export as comparison table with sources" },
  { id: "save_knowledge", command: "/save_knowledge", title: "slash.cmd.save_knowledge.title", description: "slash.cmd.save_knowledge.desc", icon: Library, category: "slash.cat.research" },
];

interface Props {
  query: string;
  visible: boolean;
  onSelect: (commandText: string) => void;
  onClose: () => void;
  anchorRef: React.RefObject<HTMLElement | null>;
}

// Compact single-column slash menu (Manus-style). Floats above the composer.
// Hover/keyboard-selected row shows a thin description line; the previous
// two-column preview + "usage" card has been removed (it was loud and
// repeated information already in the row).
export function SlashCommandMenu({ query, visible, onSelect, onClose, anchorRef }: Props) {
  const { t } = useI18n();
  const [selectedIdx, setSelectedIdx] = useState(0);
  const menuRef = useRef<HTMLDivElement>(null);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return commands;
    return commands.filter((command) =>
      [command.id, command.command, t(command.title), t(command.description), t(command.category)]
        .filter(Boolean)
        .some((value) => value.toLowerCase().includes(q))
    );
  }, [query, t]);

  useEffect(() => {
    setSelectedIdx(0);
  }, [query]);

  useEffect(() => {
    if (!visible) return;
    const handler = (event: KeyboardEvent) => {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        setSelectedIdx((prev) => Math.min(prev + 1, filtered.length - 1));
      } else if (event.key === "ArrowUp") {
        event.preventDefault();
        setSelectedIdx((prev) => Math.max(prev - 1, 0));
      } else if (event.key === "Enter" || event.key === "Tab") {
        event.preventDefault();
        const item = filtered[selectedIdx];
        if (item) onSelect(`${item.command} `);
      } else if (event.key === "Escape") {
        event.preventDefault();
        onClose();
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [visible, filtered, selectedIdx, onSelect, onClose]);

  useEffect(() => {
    if (!visible) return;
    const handler = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) onClose();
    };
    const timer = window.setTimeout(() => document.addEventListener("mousedown", handler), 0);
    return () => {
      window.clearTimeout(timer);
      document.removeEventListener("mousedown", handler);
    };
  }, [visible, onClose]);

  useEffect(() => {
    const el = menuRef.current?.querySelector(`[data-idx="${selectedIdx}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [selectedIdx]);

  if (!visible || filtered.length === 0) return null;

  const inputWidth = anchorRef.current?.getBoundingClientRect().width;
  const isFiltering = query.trim().length > 0;
  let lastCategory = "";

  return (
    <div
      ref={menuRef}
      className="animate-command-panel absolute bottom-full left-0 z-50 mb-2 overflow-hidden rounded-2xl border"
      style={{
        width: Math.min(420, Math.max(320, (inputWidth || 480) * 0.6)),
        background: "var(--yunque-elevated)",
        borderColor: "var(--yunque-border)",
        boxShadow: "var(--shadow-lg)",
      }}
    >
      <div
        className="max-h-[360px] overflow-y-auto py-1.5"
        role="listbox"
        aria-label={t("slash.title")}
      >
        {filtered.map((cmd, idx) => {
          const Icon = cmd.icon;
          const active = idx === selectedIdx;
          const showCategoryHeader = !isFiltering && cmd.category !== lastCategory;
          lastCategory = cmd.category;
          return (
            <div key={cmd.id}>
              {showCategoryHeader && (
                <div
                  className="px-3 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-[0.08em]"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  {t(cmd.category)}
                </div>
              )}
              <button
                data-idx={idx}
                type="button"
                role="option"
                aria-selected={active}
                className="interactive-list-item flex w-full items-center gap-2.5 px-3 py-1.5 text-left"
                data-active={active ? "true" : "false"}
                style={{
                  background: active ? "var(--yunque-bg-muted)" : "transparent",
                  color: active ? "var(--yunque-text)" : "var(--yunque-text-secondary)",
                }}
                onMouseEnter={() => setSelectedIdx(idx)}
                onClick={() => onSelect(`${cmd.command} `)}
              >
                <Icon
                  size={14}
                  style={{
                    color: active ? "var(--yunque-text)" : "var(--yunque-text-muted)",
                    flexShrink: 0,
                  }}
                />
                <span
                  className="text-[13px] font-medium truncate"
                  style={{ color: active ? "var(--yunque-text)" : "var(--yunque-text)" }}
                >
                  {t(cmd.title)}
                </span>
                <span
                  className="ml-auto text-[11px] font-mono truncate"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  {cmd.command}
                </span>
              </button>
            </div>
          );
        })}
      </div>

      <div
        className="flex items-center justify-between gap-2 border-t px-3 py-1.5 text-[10px]"
        style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}
      >
        <span>{filtered.length} {t("slash.available")}</span>
        <div className="flex items-center gap-1.5">
          <Kbd><Kbd.Abbr keyValue="up" /></Kbd>
          <Kbd><Kbd.Abbr keyValue="down" /></Kbd>
          <Kbd><Kbd.Abbr keyValue="enter" /></Kbd>
          <span className="ml-1">{t("slash.close")}: esc</span>
        </div>
      </div>
    </div>
  );
}
