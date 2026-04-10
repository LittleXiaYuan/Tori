"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Kbd } from "@heroui/react";
import { BookOpen, Brain, Calendar, Camera, Code, FileText, GitBranch, Globe, Keyboard, Layers, ListOrdered, Mail, MessageSquare, MousePointer, Search, Sparkles, Terminal } from "lucide-react";
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

const commands: SlashCommand[] = [
  { id: "navigate", command: "/navigate", title: "Open page", description: "Open a URL in your connected browser.", icon: Globe, category: "Browser", placeholder: "https://example.com" },
  { id: "content", command: "/content", title: "Read page", description: "Extract the current page into readable content.", icon: FileText, category: "Browser" },
  { id: "mark", command: "/mark", title: "Mark elements", description: "Number interactive elements so you can click them precisely.", icon: ListOrdered, category: "Browser" },
  { id: "click", command: "/click", title: "Click element", description: "Click a marked element or a described target.", icon: MousePointer, category: "Browser", placeholder: "Continue button" },
  { id: "type", command: "/type", title: "Type input", description: "Fill a field in the active page.", icon: Keyboard, category: "Browser", placeholder: "Email: admin@example.com" },
  { id: "screenshot", command: "/screenshot", title: "Capture page", description: "Take a screenshot of the current view.", icon: Camera, category: "Browser" },
  { id: "search", command: "/search", title: "Search web", description: "Search for current information or references.", icon: Search, category: "Tools", placeholder: "HeroUI dialog examples" },
  { id: "code", command: "/code", title: "Write code", description: "Ask the agent to implement or refactor code.", icon: Code, category: "Tools", placeholder: "Refactor auth middleware" },
  { id: "terminal", command: "/terminal", title: "Run command", description: "Execute a shell command or inspect project files.", icon: Terminal, category: "Tools", placeholder: "npm run build" },
  { id: "think", command: "/think", title: "Think deeper", description: "Slow down and reason before taking action.", icon: Brain, category: "Tools", placeholder: "Why is setup flow inconsistent?" },
  { id: "github_issues", command: "/github_issues", title: "GitHub issues", description: "Check issues from a connected repository.", icon: GitBranch, category: "Connectors", placeholder: "owner/repo" },
  { id: "gmail_inbox", command: "/gmail_inbox", title: "Recent email", description: "Read the latest messages from Gmail.", icon: Mail, category: "Connectors" },
  { id: "calendar_events", command: "/calendar_events", title: "Today’s schedule", description: "Read today’s calendar events.", icon: Calendar, category: "Connectors" },
  { id: "notion_search", command: "/notion_search", title: "Search Notion", description: "Find notes and pages from Notion.", icon: BookOpen, category: "Connectors", placeholder: "release checklist" },
  { id: "slack_send", command: "/slack_send", title: "Send Slack message", description: "Draft or send a message to Slack.", icon: MessageSquare, category: "Connectors", placeholder: "#general Release is ready" },
  { id: "skill", command: "/skill", title: "Use skill", description: "Invoke a named skill directly.", icon: Sparkles, category: "Tools", placeholder: "web_search query about AI" },
  { id: "skills", command: "/skills", title: "List skills", description: "Show all available skills and plugins.", icon: Layers, category: "Tools" },
  { id: "workflow", command: "/workflow", title: "Run workflow", description: "Start a reusable workflow from chat.", icon: Layers, category: "Automation", placeholder: "pre-release checks" },
];

interface Props {
  query: string;
  visible: boolean;
  onSelect: (commandText: string) => void;
  onClose: () => void;
  anchorRef: React.RefObject<HTMLElement | null>;
}

export function SlashCommandMenu({ query, visible, onSelect, onClose, anchorRef }: Props) {
  const { t } = useI18n();
  const [selectedIdx, setSelectedIdx] = useState(0);
  const menuRef = useRef<HTMLDivElement>(null);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return commands;
    return commands.filter((command) =>
      [command.id, command.command, command.title, command.description, command.category]
        .filter(Boolean)
        .some((value) => value.toLowerCase().includes(q))
    );
  }, [query]);

  const activeCommand = filtered[selectedIdx] || filtered[0];

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

  return (
    <div
      ref={menuRef}
      className="animate-command-panel absolute bottom-full left-0 z-50 mb-3 overflow-hidden rounded-[22px] border"
      style={{
        width: Math.min(680, Math.max(420, inputWidth || 560)),
        background: "rgba(15,16,20,0.96)",
        borderColor: "rgba(255,255,255,0.08)",
        boxShadow: "0 24px 80px rgba(0,0,0,0.42), 0 0 0 1px rgba(255,255,255,0.03)",
        backdropFilter: "blur(18px)",
      }}
    >
      <div className="border-b px-4 py-3" style={{ borderColor: "var(--yunque-border)" }}>
        <div className="flex items-center justify-between gap-3">
          <div>
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{t("slash.title")}</div>
            <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>{t("slash.subtitle")}</div>
          </div>
          <div className="inline-flex items-center gap-2">
            <Kbd><Kbd.Abbr keyValue="up" /></Kbd>
            <Kbd><Kbd.Abbr keyValue="down" /></Kbd>
            <Kbd><Kbd.Abbr keyValue="enter" /></Kbd>
          </div>
        </div>
      </div>

      <div className="grid max-h-[420px] grid-cols-1 md:grid-cols-[1.1fr_0.9fr]">
        <div className="overflow-y-auto px-2 py-2 md:border-r" style={{ borderColor: "var(--yunque-border)" }}>
          {filtered.map((cmd, idx) => {
            const Icon = cmd.icon;
            const active = idx === selectedIdx;
            return (
              <button
                key={cmd.id}
                data-idx={idx}
                type="button"
                className="interactive-list-item mb-1 flex w-full items-start gap-3 rounded-2xl px-3 py-3 text-left last:mb-0"
                data-active={active ? "true" : "false"}
                style={{
                  background: active ? "rgba(59,130,246,0.12)" : "transparent",
                  border: active ? "1px solid rgba(59,130,246,0.22)" : "1px solid transparent",
                  boxShadow: active ? "0 10px 24px rgba(59,130,246,0.08)" : "none",
                }}
                onMouseEnter={() => setSelectedIdx(idx)}
                onClick={() => onSelect(`${cmd.command} `)}
              >
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl" style={{ background: active ? "rgba(59,130,246,0.18)" : "rgba(255,255,255,0.05)", color: active ? "var(--yunque-accent)" : "var(--yunque-text-secondary)" }}>
                  <Icon size={17} />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{cmd.title}</span>
                    <span className="rounded-full px-2 py-0.5 text-[10px]" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-muted)" }}>{cmd.command}</span>
                  </div>
                  <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>{cmd.description}</div>
                </div>
              </button>
            );
          })}
        </div>

        <div className="animate-content-fade flex flex-col justify-between px-4 py-4">
          {activeCommand && (
            <>
              <div>
                <div className="inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-[11px]" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-secondary)" }}>
                  <Sparkles size={12} />
                  <span>{activeCommand.category}</span>
                </div>

                <div className="interactive-preview-panel mt-4 rounded-[18px] border p-4" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.025)" }}>
                  <div className="text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>{t("slash.insert")}</div>
                  <div className="mt-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{activeCommand.command}{activeCommand.placeholder ? ` ${activeCommand.placeholder}` : ""}</div>
                </div>

                <div className="interactive-preview-panel mt-3 rounded-[18px] border p-4" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.02)" }}>
                  <div className="text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>{t("slash.usage")}</div>
                  <div className="mt-2 text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>{t("slash.usageDesc")}</div>
                </div>
              </div>

              <div className="mt-4 flex items-center justify-between rounded-[18px] border px-4 py-3 text-[11px]" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
                <span>{filtered.length} {t("slash.available")}</span>
                <div className="flex items-center gap-2">
                  <Kbd><Kbd.Abbr keyValue="escape" /></Kbd>
                  <span>{t("slash.close")}</span>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
