"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useI18n } from "@/lib/i18n";
import {
  Brain,
  ChevronRight,
  CheckCircle2,
  Circle,
  FileCode,
  Globe,
  Loader2,
  Monitor,
  Sparkles,
  Terminal,
  X,
} from "lucide-react";
import type { AgentEvent } from "@/components/execution-trace";

type ComputerTab = "timeline" | "terminal" | "browser" | "editor" | "thinking";

interface OutputFile {
  path: string;
  name: string;
  size?: number;
  time: number;
}

interface BrowserFrame {
  image: string;
  url: string;
  action?: string;
  content?: string;
  time: number;
}

interface TerminalLine {
  type: "cmd" | "output" | "error" | "info";
  text: string;
  skill?: string;
  time: number;
}

interface EditorFile {
  path: string;
  content: string;
  language: string;
  operation: string;
  time: number;
}

interface ThinkingEntry {
  text: string;
  type: "thought" | "observation" | "plan";
  time: number;
}

type TimelineKind = "terminal" | "browser" | "file" | "thinking" | "cloud" | "ide" | "workspace";
type TimelineStatus = "running" | "done" | "error" | "info";

interface TimelineItem {
  id: string;
  kind: TimelineKind;
  status: TimelineStatus;
  label: string;
  evidence?: string;
  skill?: string;
  time: number;
}

export interface TaskStep {
  id?: string;
  action?: string;
  skill_name?: string;
  status?: string;
  result?: string;
  error?: string;
  args?: Record<string, string>;
}

export interface ComputerPanelProps {
  steps?: TaskStep[];
  traceEvents?: AgentEvent[];
  taskStatus?: string;
  taskName?: string;
  isLive?: boolean;
  suggestedTab?: ComputerTab;
  className?: string;
  onClose?: () => void;
}

function detectLanguage(path: string): string {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  const map: Record<string, string> = {
    go: "go", py: "python", js: "javascript", ts: "typescript", tsx: "tsx", jsx: "jsx",
    rs: "rust", rb: "ruby", java: "java", c: "c", cpp: "cpp", h: "c",
    html: "html", css: "css", json: "json", yaml: "yaml", yml: "yaml",
    md: "markdown", sql: "sql", sh: "bash", bash: "bash", zsh: "bash",
    toml: "toml", xml: "xml", csv: "csv",
  };
  return map[ext] || "text";
}

function safePreview(value: string, max = 3000) {
  return (value || "").slice(0, max);
}

function compactText(value: unknown, max = 900) {
  if (typeof value === "string") return safePreview(value, max);
  if (value == null) return "";
  try {
    return safePreview(JSON.stringify(value), max);
  } catch {
    return "";
  }
}

function classifyWorkKind(skill: string, type = "", summary = ""): TimelineKind {
  const text = `${skill} ${type} ${summary}`.toLowerCase();
  if (text.includes("browser") || text.includes("navigate") || text.includes("screenshot") || text.includes("search") || text.includes("searx")) return "browser";
  if (text.includes("shell") || text.includes("exec") || text.includes("command") || text.includes("run") || text.includes("terminal") || text.includes("npm") || text.includes("test")) return "terminal";
  if (text.includes("read") || text.includes("write") || text.includes("edit") || text.includes("file") || text.includes("grep") || text.includes("find") || text.includes("workspace")) return "file";
  if (text.includes("think") || text.includes("plan") || text.includes("reason") || text.includes("reflect")) return "thinking";
  if (text.includes("sandbox") || text.includes("computer") || text.includes("daytona") || text.includes("e2b") || text.includes("cloud")) return "cloud";
  if (text.includes("worker") || text.includes("ide") || text.includes("cursor") || text.includes("windsurf")) return "ide";
  return "workspace";
}

function workKindLabel(kind: TimelineKind) {
  switch (kind) {
    case "terminal": return "终端";
    case "browser": return "浏览器";
    case "file": return "文件";
    case "thinking": return "思考";
    case "cloud": return "云电脑";
    case "ide": return "AI IDE";
    default: return "工作现场";
  }
}

function timelineIcon(kind: TimelineKind) {
  switch (kind) {
    case "terminal": return <Terminal size={13} />;
    case "browser": return <Globe size={13} />;
    case "file": return <FileCode size={13} />;
    case "thinking": return <Brain size={13} />;
    case "cloud": return <Monitor size={13} />;
    case "ide": return <Sparkles size={13} />;
    default: return <Circle size={13} />;
  }
}

function statusFromStep(step: TaskStep): TimelineStatus {
  const status = (step.status || "").toLowerCase();
  if (step.error || status.includes("fail") || status.includes("error")) return "error";
  if (status === "done" || status === "completed" || status === "success") return "done";
  if (status === "running" || status === "pending") return "running";
  return step.result ? "done" : "info";
}

function timelineLabelFromStep(step: TaskStep) {
  const skill = step.skill_name || step.action || "执行步骤";
  const args = step.args || {};
  const command = args.command || args.cmd;
  const url = args.url;
  const path = args.path || args.file;
  if (command) return `运行命令：${command}`;
  if (url) return `访问网页：${url}`;
  if (path) return `处理文件：${path}`;
  return step.action || skill;
}

function parseStepTimeline(step: TaskStep, index: number): TimelineItem {
  const skill = step.skill_name || step.action || "";
  return {
    id: step.id || `step-${index}`,
    kind: classifyWorkKind(skill, step.status || "", step.action || ""),
    status: statusFromStep(step),
    label: timelineLabelFromStep(step),
    evidence: safePreview(step.error || step.result || "", 900),
    skill: step.skill_name,
    time: Date.now() + index,
  };
}

function statusFromEvent(evt: AgentEvent): TimelineStatus {
  const type = (evt.type || "").toLowerCase();
  const summary = (evt.summary || "").toLowerCase();
  if (type.includes("fail") || type.includes("error") || summary.includes("failed") || summary.includes("error")) return "error";
  if (type === "tool_result" || type.includes("done") || type.includes("complete") || type === "approved") return "done";
  if (type === "tool_start" || type.includes("start") || type === "thinking" || type.includes("plan")) return "running";
  return "info";
}

function parseEventTimeline(evt: AgentEvent, index: number): TimelineItem {
  const detail = evt.detail as Record<string, unknown> | undefined;
  const args = ((detail?.args as Record<string, unknown>) || {}) as Record<string, unknown>;
  const command = compactText(args.command || args.cmd, 180);
  const url = compactText(args.url, 180);
  const path = compactText(args.path || args.file, 180);
  const result = compactText(detail?.result, 900);
  const skill = evt.meta?.skill || "";
  const kind = classifyWorkKind(skill, evt.type, evt.summary);
  const label = command
    ? `运行命令：${command}`
    : url
      ? `访问网页：${url}`
      : path
        ? `处理文件：${path}`
        : evt.summary || `${workKindLabel(kind)}活动`;
  return {
    id: evt.id || `event-${index}`,
    kind,
    status: statusFromEvent(evt),
    label,
    evidence: result || (label !== evt.summary ? safePreview(evt.summary || "", 500) : ""),
    skill,
    time: new Date(evt.ts).getTime() || Date.now() + index,
  };
}

function parseStepResult(step: TaskStep): {
  terminal?: TerminalLine[];
  browser?: BrowserFrame;
  editor?: EditorFile;
  thinking?: ThinkingEntry;
  files?: OutputFile[];
} {
  const skill = (step.skill_name || step.action || "").toLowerCase();
  const result = step.result || "";
  const error = step.error || "";
  const now = Date.now();

  if (skill.startsWith("browser_") || skill.includes("navigate")) {
    let url = step.args?.url || "";
    if (!url && result) {
      try {
        const parsed = JSON.parse(result);
        url = parsed.url || parsed.data?.url || "";
      } catch {
        url = result.match(/https?:\/\/\S+/)?.[0] || "";
      }
    }
    return {
      browser: { image: "", url, action: skill.replace("browser_", ""), content: safePreview(result, 800), time: now } as BrowserFrame,
      terminal: [{ type: "info" as const, text: `${skill || "browser"}: ${url || result || "working"}`, skill: step.skill_name, time: now }],
    };
  }

  if (skill.includes("shell") || skill.includes("exec") || skill.includes("command") || skill.includes("run")) {
    const cmd = step.args?.command || step.args?.cmd || step.action || "";
    return {
      terminal: [
        { type: "cmd" as const, text: `$ ${cmd}`, skill: step.skill_name, time: now },
        ...(result ? [{ type: "output" as const, text: safePreview(result), time: now }] : []),
        ...(error ? [{ type: "error" as const, text: error, time: now }] : []),
      ],
    };
  }

  if (skill.includes("search") || skill.includes("web_search") || skill.includes("searx")) {
    const query = step.args?.query || step.args?.q || step.action || "";
    return {
      browser: { image: "", url: query ? `search://${query}` : "", action: "search", content: safePreview(result), time: now } as BrowserFrame,
      terminal: [{ type: "info" as const, text: `Search: ${query}`, skill: step.skill_name, time: now }],
    };
  }

  if (skill.includes("read") || skill.includes("write") || skill.includes("edit") || skill.includes("file") || skill.includes("grep") || skill.includes("find") || skill.includes("ls")) {
    const path = step.args?.path || step.args?.file || "";
    const operation = skill.includes("write") ? "write" : skill.includes("edit") ? "edit" : skill.includes("grep") ? "grep" : skill.includes("find") ? "find" : skill.includes("ls") ? "list" : "read";
    return {
      editor: { path, content: safePreview(result || error, 5000), language: detectLanguage(path), operation, time: now } as EditorFile,
    };
  }

  if (skill.includes("think") || skill.includes("plan") || skill.includes("reason") || skill.includes("reflect")) {
    return { thinking: { text: result || step.action || "", type: "thought", time: now } as ThinkingEntry };
  }

  return {
    terminal: [
      { type: error ? "error" : "info", text: safePreview(error || result || step.action || skill || "Tool activity"), skill: step.skill_name, time: now },
    ],
  };
}

function parseAgentEvent(evt: AgentEvent): {
  terminal?: TerminalLine[];
  browser?: BrowserFrame;
  editor?: EditorFile;
  thinking?: ThinkingEntry;
  files?: OutputFile[];
} {
  const skill = (evt.meta?.skill || "").toLowerCase();
  const type = (evt.type || "").toLowerCase();
  const summary = evt.summary || "";
  const detail = evt.detail as Record<string, unknown> | undefined;
  const now = new Date(evt.ts).getTime() || Date.now();
  const args = ((detail?.args as Record<string, unknown>) || {}) as Record<string, unknown>;
  const toolResult = typeof detail?.result === "string" ? detail.result : summary;

  if (type === "thinking" || type === "reflect" || type.includes("plan")) {
    return {
      thinking: {
        text: summary,
        type: type === "reflect" ? "observation" : type.includes("plan") ? "plan" : "thought",
        time: now,
      } as ThinkingEntry,
    };
  }

  if (type === "tool_start" || type === "tool_result") {
    if (skill.startsWith("browser_") || skill.includes("navigate") || skill.includes("screenshot")) {
      const url = (args.url as string) || "";
      return {
        browser: { image: "", url, action: skill.replace("browser_", ""), content: safePreview(toolResult, 1000), time: now } as BrowserFrame,
      };
    }

    if (skill.includes("shell") || skill.includes("exec") || skill.includes("command") || skill.includes("run")) {
      const cmd = ((args.command || args.cmd) as string) || "";
      return {
        terminal: [
          { type: type === "tool_start" ? "cmd" : "output", text: type === "tool_start" ? `$ ${cmd || skill}` : safePreview(toolResult), skill: evt.meta?.skill, time: now },
        ],
      };
    }

    if (skill.includes("search") || skill.includes("web_search") || skill.includes("searx")) {
      const query = ((args.query || args.q) as string) || "";
      return {
        browser: { image: "", url: query ? `search://${query}` : "", action: "search", content: safePreview(toolResult), time: now } as BrowserFrame,
        terminal: [{ type: "info" as const, text: `Search: ${query}`, skill: evt.meta?.skill, time: now }],
      };
    }

    if (skill.includes("read") || skill.includes("write") || skill.includes("edit") || skill.includes("file") || skill.includes("grep")) {
      const path = ((args.path || args.file) as string) || "";
      const files: OutputFile[] = [];
      if (Array.isArray(detail?.files)) {
        for (const f of detail?.files as Array<{ path: string; name: string; size?: number }>) {
          files.push({ path: f.path, name: f.name || f.path.split("/").pop() || f.path, size: f.size, time: now });
        }
      }
      return {
        editor: { path, content: safePreview(toolResult, 5000), language: detectLanguage(path), operation: skill, time: now } as EditorFile,
        ...(files.length ? { files } : {}),
      };
    }
  }

  return summary
    ? { terminal: [{ type: "info" as const, text: safePreview(summary), skill: evt.meta?.skill, time: now }] }
    : {};
}

function EmptyState({ icon, title, desc }: { icon: React.ReactNode; title: string; desc: string }) {
  return (
    <div className="flex h-full items-center justify-center p-6">
      <div className="max-w-[260px] text-center">
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[20px] border" style={{ background: "rgba(255,255,255,0.03)", borderColor: "var(--yunque-border)", color: "var(--yunque-text-secondary)" }}>
          {icon}
        </div>
        <div className="mt-4 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{title}</div>
        <div className="mt-2 text-xs leading-6" style={{ color: "var(--yunque-text-muted)" }}>{desc}</div>
      </div>
    </div>
  );
}

function TimelineView({ items, isLive }: { items: TimelineItem[]; isLive?: boolean }) {
  if (!items.length) {
    return <EmptyState icon={<Monitor size={22} />} title="暂无工作现场" desc="Agent 调用本地沙箱、浏览器、云电脑或 AI IDE 时，会在这里显示推进过程和执行证据。" />;
  }

  const statusTone: Record<TimelineStatus, { color: string; bg: string; label: string }> = {
    running: { color: "#60a5fa", bg: "rgba(59,130,246,0.14)", label: "进行中" },
    done: { color: "#34d399", bg: "rgba(52,211,153,0.12)", label: "完成" },
    error: { color: "#f87171", bg: "rgba(248,113,113,0.12)", label: "异常" },
    info: { color: "#cbd5e1", bg: "rgba(203,213,225,0.09)", label: "记录" },
  };

  return (
    <div className="h-full overflow-y-auto p-4">
      <div className="mb-4 rounded-[24px] border p-4" style={{ background: "linear-gradient(135deg, rgba(59,130,246,0.10), rgba(14,165,233,0.04))", borderColor: "rgba(59,130,246,0.18)" }}>
        <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "#e0f2fe" }}>
          {isLive ? <Loader2 size={15} className="animate-spin" /> : <CheckCircle2 size={15} />}
          <span>{isLive ? "云雀正在推进任务" : "工作过程已记录"}</span>
        </div>
        <div className="mt-2 text-xs leading-6" style={{ color: "#94a3b8" }}>
          这里把本地沙箱、浏览器、云电脑和 AI IDE 的动作收敛成同一条时间线，优先展示推进状态和可验证证据。
        </div>
      </div>

      <div className="space-y-3">
        {items.map((item, index) => {
          const displayStatus = item.status === "running" && !isLive ? "done" : item.status;
          const tone = statusTone[displayStatus];
          return (
            <div key={`${item.id}-${index}`} className="relative pl-8">
              {index < items.length - 1 && <div className="absolute left-[13px] top-8 bottom-[-14px] w-px" style={{ background: "rgba(148,163,184,0.18)" }} />}
              <div className="absolute left-0 top-1 flex h-7 w-7 items-center justify-center rounded-full" style={{ background: tone.bg, color: tone.color, boxShadow: displayStatus === "running" ? `0 0 12px ${tone.color}55` : undefined }}>
                {displayStatus === "running" ? <Loader2 size={13} className="animate-spin" /> : timelineIcon(item.kind)}
              </div>
              <div className="rounded-[22px] border px-4 py-3" style={{ background: "rgba(255,255,255,0.035)", borderColor: "rgba(255,255,255,0.07)" }}>
                <div className="flex items-start gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="rounded-full px-2.5 py-1 text-[10px] font-medium" style={{ background: tone.bg, color: tone.color }}>{tone.label}</span>
                      <span className="rounded-full px-2.5 py-1 text-[10px]" style={{ background: "rgba(255,255,255,0.06)", color: "#cbd5e1" }}>{workKindLabel(item.kind)}</span>
                      {item.skill && <span className="max-w-[160px] truncate rounded-full px-2.5 py-1 text-[10px]" style={{ background: "rgba(255,255,255,0.04)", color: "#94a3b8" }}>{item.skill}</span>}
                    </div>
                    <div className="mt-2 text-sm leading-6" style={{ color: "#f8fafc" }}>{item.label}</div>
                    {item.evidence && (
                      <pre className="mt-2 max-h-[140px] overflow-y-auto whitespace-pre-wrap break-words rounded-2xl px-3 py-2 text-[11px] leading-5" style={{ background: "rgba(4,7,16,0.48)", color: "#b6c3d1" }}>{item.evidence}</pre>
                    )}
                  </div>
                  <span className="shrink-0 text-[10px] font-mono" style={{ color: "#64748b" }}>
                    {new Date(item.time).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", second: "2-digit" })}
                  </span>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function TerminalView({ lines }: { lines: TerminalLine[] }) {
  const endRef = useRef<HTMLDivElement>(null);
  useEffect(() => { endRef.current?.scrollIntoView({ behavior: "smooth" }); }, [lines.length]);

  const { t } = useI18n();
  if (!lines.length) {
    return <EmptyState icon={<Terminal size={22} />} title={t("computerPanel.terminal.empty")} desc={t("computerPanel.terminal.emptyDesc")} />;
  }

  return (
    <div className="h-full overflow-y-auto p-4 font-mono text-[12px]" style={{ background: "rgba(4,7,16,0.92)", color: "#cbd5e1" }}>
      {lines.map((line, i) => (
        <div key={`${line.time}-${i}`} className="mb-2 rounded-xl px-3 py-2" style={{ background: line.type === "cmd" ? "rgba(34,197,94,0.08)" : line.type === "error" ? "rgba(239,68,68,0.09)" : "rgba(255,255,255,0.03)" }}>
          <div className="flex items-center gap-2">
            <span style={{ color: line.type === "cmd" ? "#4ade80" : line.type === "error" ? "#f87171" : "#93c5fd" }}>
              {line.type === "cmd" ? "$" : line.type === "error" ? "!" : ">"}
            </span>
            <span className="flex-1 whitespace-pre-wrap break-all">{line.text.replace(/^\$\s*/, "")}</span>
            {line.skill && <span className="rounded-full px-2 py-0.5 text-[10px]" style={{ background: "rgba(255,255,255,0.06)", color: "#94a3b8" }}>{line.skill}</span>}
          </div>
        </div>
      ))}
      <div ref={endRef} />
    </div>
  );
}

function BrowserView({ frames, sseImage, sseUrl }: { frames: BrowserFrame[]; sseImage: string; sseUrl: string }) {
  const latest = frames[frames.length - 1];
  const url = sseUrl || latest?.url || "";
  const image = sseImage || latest?.image || "";
  const content = latest?.content || "";

  const { t } = useI18n();
  if (!url && !image && !content) {
    return <EmptyState icon={<Globe size={22} />} title={t("computerPanel.browser.empty")} desc={t("computerPanel.browser.emptyDesc")} />;
  }

  return (
    <div className="flex h-full flex-col overflow-hidden" style={{ background: "rgba(255,255,255,0.02)" }}>
      <div className="border-b px-4 py-3" style={{ borderColor: "var(--yunque-border)" }}>
        <div className="rounded-2xl border px-3 py-2 text-xs" style={{ background: "rgba(255,255,255,0.03)", borderColor: "var(--yunque-border)", color: "var(--yunque-text-secondary)" }}>
          {url || "Browser connected"}
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-4">
        {image ? (
          <img src={image} alt="browser frame" className="w-full rounded-[20px] border object-cover" style={{ borderColor: "var(--yunque-border)" }} />
        ) : (
          <div className="rounded-[24px] border p-4" style={{ background: "rgba(255,255,255,0.025)", borderColor: "var(--yunque-border)" }}>
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{latest?.action ? `Action: ${latest.action}` : "Browser session"}</div>
            <div className="mt-2 text-xs leading-6" style={{ color: "var(--yunque-text-secondary)" }}>{content || "Waiting for screenshot frames or extracted browser content."}</div>
          </div>
        )}
      </div>
    </div>
  );
}

function EditorView({ files }: { files: EditorFile[] }) {
  const [selected, setSelected] = useState(0);
  const file = files[selected] || files[files.length - 1];
  useEffect(() => { if (selected >= files.length) setSelected(Math.max(0, files.length - 1)); }, [files.length, selected]);

  const { t } = useI18n();
  if (!files.length || !file) {
    return <EmptyState icon={<FileCode size={22} />} title={t("computerPanel.file.empty")} desc={t("computerPanel.file.emptyDesc")} />;
  }

  return (
    <div className="flex h-full overflow-hidden">
      <div className="w-[180px] shrink-0 overflow-y-auto border-r p-2" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.018)" }}>
        {files.map((entry, i) => (
          <button
            key={`${entry.path}-${entry.time}-${i}`}
            onClick={() => setSelected(i)}
            className="mb-2 w-full rounded-2xl border px-3 py-3 text-left transition-all last:mb-0"
            style={{
              background: i === selected ? "rgba(59,130,246,0.12)" : "rgba(255,255,255,0.02)",
              borderColor: i === selected ? "rgba(59,130,246,0.28)" : "var(--yunque-border)",
            }}
          >
            <div className="truncate text-xs font-semibold" style={{ color: "var(--yunque-text)" }}>{entry.path.split("/").pop() || entry.path || "Untitled"}</div>
            <div className="mt-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{entry.operation}</div>
          </button>
        ))}
      </div>
      <div className="min-w-0 flex-1 overflow-hidden">
        <div className="border-b px-4 py-3" style={{ borderColor: "var(--yunque-border)" }}>
          <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{file.path || "Untitled file"}</div>
          <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>{file.language} ? {file.operation}</div>
        </div>
        <pre className="h-full overflow-y-auto p-4 text-[12px] leading-6" style={{ background: "rgba(4,7,16,0.88)", color: "#dbe4f0" }}>{file.content || "No preview available."}</pre>
      </div>
    </div>
  );
}

function ThinkingView({ entries }: { entries: ThinkingEntry[] }) {
  const { t } = useI18n();
  if (!entries.length) {
    return <EmptyState icon={<Brain size={22} />} title={t("computerPanel.reasoning.empty")} desc={t("computerPanel.reasoning.emptyDesc")} />;
  }

  const tone = {
    thought: { bg: "rgba(59,130,246,0.08)", border: "rgba(59,130,246,0.18)", label: "Thought" },
    observation: { bg: "rgba(245,158,11,0.08)", border: "rgba(245,158,11,0.18)", label: "Observation" },
    plan: { bg: "rgba(168,85,247,0.08)", border: "rgba(168,85,247,0.18)", label: "Plan" },
  } as const;

  return (
    <div className="h-full overflow-y-auto p-4">
      {entries.map((entry, i) => (
        <div key={`${entry.time}-${i}`} className="mb-3 rounded-[22px] border p-4 last:mb-0" style={{ background: tone[entry.type].bg, borderColor: tone[entry.type].border }}>
          <div className="mb-2 inline-flex rounded-full px-2.5 py-1 text-[11px]" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-secondary)" }}>{tone[entry.type].label}</div>
          <div className="text-sm leading-7" style={{ color: "var(--yunque-text)" }}>{entry.text}</div>
        </div>
      ))}
    </div>
  );
}

function humanFileSize(size?: number) {
  if (!size) return "";
  if (size >= 1024 * 1024) return `${(size / 1024 / 1024).toFixed(1)} MB`;
  if (size >= 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${size} B`;
}

export function ComputerPanel({ steps, traceEvents, taskStatus, taskName, isLive, className, onClose, suggestedTab }: ComputerPanelProps) {
  const [activeTab, setActiveTab] = useState<ComputerTab>(suggestedTab || "timeline");
  const [sseImage, setSseImage] = useState("");
  const [sseUrl, setSseUrl] = useState("");

  useEffect(() => {
    if (suggestedTab) setActiveTab(suggestedTab);
  }, [suggestedTab]);

  const parsed = useMemo(() => {
    const terminal: TerminalLine[] = [];
    const browser: BrowserFrame[] = [];
    const editor: EditorFile[] = [];
    const thinking: ThinkingEntry[] = [];
    const files: OutputFile[] = [];
    const timeline: TimelineItem[] = [];

    const mergeEditor = (next: EditorFile) => {
      const idx = editor.findIndex((item) => item.path === next.path);
      if (idx >= 0) editor[idx] = next;
      else editor.push(next);
    };

    for (const step of steps || []) {
      if (step.status === "pending") continue;
      timeline.push(parseStepTimeline(step, timeline.length));
      const next = parseStepResult(step);
      if (next.terminal) terminal.push(...next.terminal);
      if (next.browser) browser.push(next.browser);
      if (next.editor) mergeEditor(next.editor);
      if (next.thinking) thinking.push(next.thinking);
      if (next.files) files.push(...next.files);
    }

    for (const evt of traceEvents || []) {
      timeline.push(parseEventTimeline(evt, timeline.length));
      const next = parseAgentEvent(evt);
      if (next.terminal) terminal.push(...next.terminal);
      if (next.browser) browser.push(next.browser);
      if (next.editor) mergeEditor(next.editor);
      if (next.thinking) thinking.push(next.thinking);
      if (next.files) files.push(...next.files);
    }

    timeline.sort((a, b) => a.time - b.time);

    return { terminal, browser, editor, thinking, files, timeline };
  }, [steps, traceEvents]);

  useEffect(() => {
    let cancelled = false;
    const headers: Record<string, string> = {};
    const token = typeof window !== "undefined" ? localStorage.getItem("yunque_token") || "" : "";
    const key = typeof window !== "undefined" ? localStorage.getItem("yunque_api_key") || "" : "";
    if (token) headers.Authorization = `Bearer ${token}`;
    else if (key) headers["X-API-Key"] = key;

    (async () => {
      try {
        const res = await fetch("/v1/events/stream", { headers });
        if (!res.ok || !res.body) return;
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        while (!cancelled) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split("\\n");
          buffer = lines.pop() || "";
          for (const line of lines) {
            if (!line.startsWith("data: ")) continue;
            try {
              const data = JSON.parse(line.slice(6));
              if (data.event === "browser.screenshot") {
                if (data.data?.image) setSseImage(data.data.image);
                if (data.data?.url) setSseUrl(data.data.url);
                setActiveTab("browser");
              }
            } catch {}
          }
        }
      } catch {}
    })();

    return () => { cancelled = true; };
  }, []);

  const tabs = [
    { key: "timeline" as const, label: "Timeline", icon: <Monitor size={14} />, count: parsed.timeline.length },
    { key: "terminal" as const, label: "Terminal", icon: <Terminal size={14} />, count: parsed.terminal.length },
    { key: "browser" as const, label: "Browser", icon: <Globe size={14} />, count: parsed.browser.length + (sseUrl || sseImage ? 1 : 0) },
    { key: "editor" as const, label: "Files", icon: <FileCode size={14} />, count: parsed.editor.length },
    { key: "thinking" as const, label: "Thinking", icon: <Brain size={14} />, count: parsed.thinking.length },
  ];

  const progressTotal = (steps || []).length;
  const progressDone = (steps || []).filter((item) => item.status === "done" || item.status === "completed").length;
  const progressPct = progressTotal ? Math.round((progressDone / progressTotal) * 100) : 0;
  const activeSummary = isLive || taskStatus === "running" || taskStatus === "planning";
  const latestTimeline = parsed.timeline[parsed.timeline.length - 1];
  const activeLabel = latestTimeline?.label || "等待 Agent 开始使用本地沙箱、浏览器、云电脑或 AI IDE。";

  return (
    <div className={`flex h-full flex-col overflow-hidden ${className || ""}`} style={{ background: "linear-gradient(180deg, rgba(12,18,30,0.98), rgba(8,10,18,0.98))" }}>
      <div className="border-b px-4 py-4" style={{ borderColor: "rgba(255,255,255,0.07)" }}>
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="mb-2 inline-flex items-center gap-2 rounded-full px-3 py-1 text-[11px]" style={{ background: "rgba(59,130,246,0.12)", color: "#93c5fd" }}>
              <Monitor size={12} />
              <span>Computer workspace</span>
            </div>
            <div className="text-sm font-semibold" style={{ color: "#f8fafc" }}>{taskName || "工作现场"}</div>
            <div className="mt-1 text-xs leading-5" style={{ color: "#94a3b8" }}>{activeSummary ? `当前：${activeLabel}` : "查看 Agent 刚刚使用过的终端、浏览器、文件和思考证据。"}</div>
          </div>
          {onClose && (
            <button onClick={onClose} className="flex h-9 w-9 items-center justify-center rounded-2xl transition-colors" style={{ background: "rgba(255,255,255,0.05)", color: "#94a3b8" }}>
              <X size={16} />
            </button>
          )}
        </div>
      </div>

      {(progressTotal > 1 || activeSummary) && (
        <div className="border-b px-4 py-3" style={{ borderColor: "rgba(255,255,255,0.06)", background: "rgba(255,255,255,0.02)" }}>
          {progressTotal > 1 && (
            <>
              <div className="mb-2 flex items-center justify-between text-[11px]" style={{ color: "#94a3b8" }}>
                <span>{progressDone}/{progressTotal} steps complete</span>
                <span>{progressPct}%</span>
              </div>
              <div className="h-1.5 overflow-hidden rounded-full" style={{ background: "rgba(255,255,255,0.07)" }}>
                <div className="h-full rounded-full transition-all duration-500" style={{ width: `${progressPct}%`, background: progressPct >= 100 ? "#22c55e" : "#3b82f6" }} />
              </div>
            </>
          )}
          {activeSummary && (
            <div className={`${progressTotal > 1 ? "mt-3" : ""} flex items-center gap-2 text-[11px]`} style={{ color: "#cbd5e1" }}>
              <span className="flex h-5 w-5 items-center justify-center rounded-full" style={{ background: "rgba(59,130,246,0.14)", color: "#60a5fa" }}><Sparkles size={11} /></span>
              <span>{tabs.find((tab) => tab.key === activeTab)?.label || "Timeline"} is active</span>
              <ChevronRight size={12} style={{ color: "#3b82f6" }} />
              <span className="min-w-0 truncate" style={{ color: "#94a3b8" }}>{taskStatus || (isLive ? "running" : "ready")}</span>
            </div>
          )}
        </div>
      )}

      <div className="border-b px-3 py-2" style={{ borderColor: "rgba(255,255,255,0.06)" }}>
        <div className="flex flex-wrap gap-2">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className="inline-flex items-center gap-2 rounded-full px-3 py-2 text-[11px] transition-all"
              style={{
                background: activeTab === tab.key ? "rgba(59,130,246,0.16)" : "rgba(255,255,255,0.04)",
                color: activeTab === tab.key ? "#dbeafe" : "#94a3b8",
                border: `1px solid ${activeTab === tab.key ? "rgba(59,130,246,0.28)" : "rgba(255,255,255,0.06)"}`,
              }}
            >
              {tab.icon}
              <span>{tab.label}</span>
              {tab.count > 0 && <span className="rounded-full px-1.5 py-0.5 text-[10px]" style={{ background: "rgba(255,255,255,0.08)" }}>{tab.count}</span>}
            </button>
          ))}
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-hidden">
        {activeTab === "timeline" && <TimelineView items={parsed.timeline} isLive={activeSummary} />}
        {activeTab === "terminal" && <TerminalView lines={parsed.terminal} />}
        {activeTab === "browser" && <BrowserView frames={parsed.browser} sseImage={sseImage} sseUrl={sseUrl} />}
        {activeTab === "editor" && <EditorView files={parsed.editor} />}
        {activeTab === "thinking" && <ThinkingView entries={parsed.thinking} />}
      </div>

      {parsed.files.length > 0 && (
        <div className="border-t px-4 py-3" style={{ borderColor: "rgba(255,255,255,0.06)", background: "rgba(255,255,255,0.02)" }}>
          <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "#64748b" }}>Generated files</div>
          <div className="space-y-2">
            {parsed.files.map((file, index) => (
              <a key={`${file.path}-${index}`} href={`/api/files/download?path=${encodeURIComponent(file.path)}`} download={file.name} className="flex items-center gap-3 rounded-[18px] border px-3 py-3 transition-all" style={{ borderColor: "rgba(59,130,246,0.18)", background: "rgba(59,130,246,0.08)" }}>
                <div className="flex h-9 w-9 items-center justify-center rounded-xl" style={{ background: "rgba(59,130,246,0.16)", color: "#93c5fd" }}><FileCode size={15} /></div>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium" style={{ color: "#dbeafe" }}>{file.name}</div>
                  <div className="mt-0.5 text-[11px]" style={{ color: "#94a3b8" }}>{humanFileSize(file.size) || "Ready to download"}</div>
                </div>
                <span className="rounded-full px-2.5 py-1 text-[10px]" style={{ background: "rgba(255,255,255,0.08)", color: "#cbd5e1" }}>Download</span>
              </a>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

export default ComputerPanel;
