"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import {
  Monitor,
  Terminal,
  FileCode,
  Globe,
  Brain,
  Sparkles,
  Camera,
  ChevronRight,
  CheckCircle2,
  XCircle,
  Clock,
  Loader2,
  Eye,
  FileText,
  FolderTree,
  AlertTriangle,
} from "lucide-react";
import type { TaskStep } from "@/lib/api";
import type { AgentEvent } from "@/components/execution-trace";

/* ── Types ── */

type ComputerTab = "terminal" | "browser" | "editor" | "thinking";

interface BrowserFrame {
  image: string;       // base64
  url: string;
  action?: string;
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
  operation: string;   // read / write / edit / grep
  time: number;
}

interface ThinkingEntry {
  text: string;
  type: "thought" | "observation" | "plan";
  time: number;
}

/* ── Helpers ── */

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

function parseStepResult(step: TaskStep): {
  terminal?: TerminalLine[];
  browser?: BrowserFrame;
  editor?: EditorFile;
  thinking?: ThinkingEntry;
} {
  const skill = (step.skill_name || step.action || "").toLowerCase();
  const result = step.result || "";
  const error = step.error || "";
  const now = Date.now();

  // Browser skills
  if (skill.startsWith("browser_")) {
    if (skill === "browser_screenshot" || skill === "browser_navigate") {
      // Try to extract URL from result
      let url = "";
      try {
        const parsed = JSON.parse(result);
        url = parsed.url || parsed.data?.url || "";
      } catch {
        url = result.match(/https?:\/\/\S+/)?.[0] || "";
      }
      return {
        browser: { image: "", url, action: skill.replace("browser_", ""), time: now },
      };
    }
    return {
      terminal: [
        { type: "cmd", text: `🌐 ${skill}`, skill: step.skill_name || undefined, time: now },
        ...(result ? [{ type: "output" as const, text: result.slice(0, 2000), time: now }] : []),
        ...(error ? [{ type: "error" as const, text: error, time: now }] : []),
      ],
    };
  }

  // Shell / exec skills
  if (skill.includes("shell") || skill.includes("exec") || skill.includes("command") || skill.includes("run")) {
    const cmd = step.args?.command || step.args?.cmd || step.action || "";
    return {
      terminal: [
        { type: "cmd", text: `$ ${cmd}`, skill: step.skill_name || undefined, time: now },
        ...(result ? [{ type: "output" as const, text: result.slice(0, 3000), time: now }] : []),
        ...(error ? [{ type: "error" as const, text: error, time: now }] : []),
      ],
    };
  }

  // File skills
  if (skill.includes("read") || skill.includes("write") || skill.includes("edit") || skill.includes("file") || skill.includes("grep") || skill.includes("find") || skill.includes("ls")) {
    const path = step.args?.path || step.args?.file || "";
    const operation = skill.includes("write") ? "write" :
                      skill.includes("edit") ? "edit" :
                      skill.includes("grep") ? "grep" :
                      skill.includes("find") ? "find" :
                      skill.includes("ls") ? "ls" : "read";
    return {
      editor: {
        path,
        content: result.slice(0, 5000) || error || "",
        language: detectLanguage(path),
        operation,
        time: now,
      },
    };
  }

  // Thinking / planning
  if (skill.includes("think") || skill.includes("plan") || skill.includes("reason") || skill.includes("reflect")) {
    return {
      thinking: { text: result || step.action, type: "thought", time: now },
    };
  }

  // Default: treat as terminal output
  return {
    terminal: [
      { type: "info", text: `⚡ ${step.action}`, skill: step.skill_name || undefined, time: now },
      ...(result ? [{ type: "output" as const, text: result.slice(0, 2000), time: now }] : []),
      ...(error ? [{ type: "error" as const, text: error, time: now }] : []),
    ],
  };
}

/* ── AgentEvent → view data (for chat integration) ── */

function parseAgentEvent(evt: AgentEvent): {
  terminal?: TerminalLine[];
  browser?: BrowserFrame;
  editor?: EditorFile;
  thinking?: ThinkingEntry;
} {
  const skill = (evt.meta?.skill || "").toLowerCase();
  const type = (evt.type || "").toLowerCase();
  const summary = evt.summary || "";
  const detail = evt.detail as Record<string, unknown> | undefined;
  const now = new Date(evt.ts).getTime() || Date.now();

  // Thinking / reflect / planning events
  if (type === "thinking" || type === "reflect" || type.includes("plan")) {
    return {
      thinking: {
        text: summary,
        type: type === "reflect" ? "observation" : type.includes("plan") ? "plan" : "thought",
        time: now,
      },
    };
  }

  // Tool start/result events
  if (type === "tool_start" || type === "tool_result") {
    // Browser skills
    if (skill.startsWith("browser_")) {
      let url = "";
      if (detail && typeof detail === "object") {
        url = (detail.url as string) || "";
      }
      return {
        browser: { image: "", url, action: skill.replace("browser_", ""), time: now },
      };
    }

    // Shell / exec
    if (skill.includes("shell") || skill.includes("exec") || skill.includes("command") || skill.includes("run")) {
      const cmd = detail && typeof detail === "object" ? ((detail.command || detail.cmd || "") as string) : "";
      const result = type === "tool_result" ? summary : "";
      return {
        terminal: [
          { type: "cmd", text: cmd ? `$ ${cmd}` : `⚡ ${skill}`, skill: evt.meta?.skill || undefined, time: now },
          ...(result ? [{ type: "output" as const, text: result.slice(0, 3000), time: now }] : []),
        ],
      };
    }

    // File skills
    if (skill.includes("read") || skill.includes("write") || skill.includes("edit") || skill.includes("file") || skill.includes("grep")) {
      const path = detail && typeof detail === "object" ? ((detail.path || detail.file || "") as string) : "";
      const operation = skill.includes("write") ? "write" :
                        skill.includes("edit") ? "edit" :
                        skill.includes("grep") ? "grep" : "read";
      return {
        editor: {
          path,
          content: summary.slice(0, 5000),
          language: detectLanguage(path),
          operation,
          time: now,
        },
      };
    }

    // Generic tool event → terminal
    return {
      terminal: [
        { type: type === "tool_start" ? "cmd" : "output", text: `⚡ ${skill || type}: ${summary}`.slice(0, 2000), skill: evt.meta?.skill || undefined, time: now },
      ],
    };
  }

  // Workflow node events
  if (type.startsWith("node_")) {
    return {
      terminal: [
        { type: "info", text: `🔄 ${evt.meta?.node_name || type}: ${summary}`, time: now },
      ],
    };
  }

  // Default: show as thinking observation
  if (summary) {
    return {
      thinking: { text: summary, type: "observation", time: now },
    };
  }

  return {};
}

/* ─────────────────────────────────────────────
   Terminal View
   ───────────────────────────────────────────── */
function TerminalView({ lines }: { lines: TerminalLine[] }) {
  const endRef = useRef<HTMLDivElement>(null);
  useEffect(() => { endRef.current?.scrollIntoView({ behavior: "smooth" }); }, [lines.length]);

  return (
    <div className="flex flex-col h-full">
      {/* Terminal header bar */}
      <div
        className="flex items-center gap-2 px-3 py-2 flex-shrink-0"
        style={{ background: "#1a1a2e", borderBottom: "1px solid #2a2a3e" }}
      >
        <div className="flex gap-1.5">
          <span className="w-3 h-3 rounded-full" style={{ background: "#ff5f57" }} />
          <span className="w-3 h-3 rounded-full" style={{ background: "#febc2e" }} />
          <span className="w-3 h-3 rounded-full" style={{ background: "#28c840" }} />
        </div>
        <span className="text-[10px] font-mono ml-2" style={{ color: "#6b7280" }}>
          yunque@computer ~ terminal
        </span>
      </div>

      {/* Terminal content */}
      <div
        className="flex-1 overflow-y-auto p-3 font-mono text-xs leading-relaxed"
        style={{ background: "#0d1117" }}
      >
        {lines.length === 0 ? (
          <div className="flex items-center justify-center h-full" style={{ color: "#4b5563" }}>
            <span>等待 Agent 执行命令...</span>
          </div>
        ) : (
          lines.map((line, i) => (
            <div key={i} className="mb-0.5">
              {line.type === "cmd" ? (
                <div className="flex items-start gap-1">
                  <span style={{ color: "#22c55e" }}>❯</span>
                  <span style={{ color: "#e5e7eb" }}>{line.text}</span>
                  {line.skill && (
                    <span className="ml-2 text-[9px] px-1 py-0.5 rounded" style={{ background: "#1e3a5f", color: "#60a5fa" }}>
                      {line.skill}
                    </span>
                  )}
                </div>
              ) : line.type === "error" ? (
                <span style={{ color: "#f87171" }}>{line.text}</span>
              ) : line.type === "info" ? (
                <span style={{ color: "#60a5fa" }}>{line.text}</span>
              ) : (
                <span style={{ color: "#9ca3af" }}>{line.text}</span>
              )}
            </div>
          ))
        )}
        <div ref={endRef} />
      </div>
    </div>
  );
}

/* ─────────────────────────────────────────────
   Browser View
   ───────────────────────────────────────────── */
function BrowserView({ frames, sseImage, sseUrl }: {
  frames: BrowserFrame[];
  sseImage: string;
  sseUrl: string;
}) {
  const latestFrame = frames[frames.length - 1];
  const displayImage = sseImage || latestFrame?.image;
  const displayUrl = sseUrl || latestFrame?.url || "";

  return (
    <div className="flex flex-col h-full">
      {/* Browser chrome */}
      <div
        className="flex items-center gap-2 px-3 py-2 flex-shrink-0"
        style={{ background: "#1a1a2e", borderBottom: "1px solid #2a2a3e" }}
      >
        <div className="flex gap-1.5">
          <span className="w-3 h-3 rounded-full" style={{ background: "#ff5f57" }} />
          <span className="w-3 h-3 rounded-full" style={{ background: "#febc2e" }} />
          <span className="w-3 h-3 rounded-full" style={{ background: "#28c840" }} />
        </div>
        <div
          className="flex-1 px-3 py-1 rounded-md text-[11px] font-mono truncate ml-2"
          style={{ background: "#0d1117", color: "#9ca3af" }}
        >
          {displayUrl || "about:blank"}
        </div>
      </div>

      {/* Browser content */}
      <div className="flex-1 overflow-hidden flex items-center justify-center" style={{ background: "#0d1117" }}>
        {displayImage ? (
          <img
            src={displayImage.startsWith("data:") ? displayImage : `data:image/png;base64,${displayImage}`}
            alt="Browser screenshot"
            className="max-w-full max-h-full object-contain"
          />
        ) : (
          <div className="text-center" style={{ color: "#4b5563" }}>
            <Globe size={40} className="mx-auto mb-3 opacity-30" />
            <p className="text-xs">Agent 尚未打开浏览器</p>
            <p className="text-[10px] mt-1">执行浏览器任务时将在此显示实时画面</p>
          </div>
        )}
      </div>

      {/* Action log */}
      {frames.length > 0 && (
        <div
          className="flex-shrink-0 px-3 py-2 text-[10px] font-mono overflow-x-auto whitespace-nowrap"
          style={{ background: "#1a1a2e", borderTop: "1px solid #2a2a3e", color: "#6b7280" }}
        >
          {frames.slice(-3).map((f, i) => (
            <span key={i} className="mr-3">
              <span style={{ color: "#60a5fa" }}>{f.action}</span> {f.url ? `→ ${f.url.slice(0, 40)}` : ""}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

/* ─────────────────────────────────────────────
   Editor View
   ───────────────────────────────────────────── */
function EditorView({ files }: { files: EditorFile[] }) {
  const [activeIdx, setActiveIdx] = useState(0);

  // Auto-focus latest file
  useEffect(() => { setActiveIdx(Math.max(0, files.length - 1)); }, [files.length]);

  const file = files[activeIdx];
  if (!file) {
    return (
      <div className="flex items-center justify-center h-full" style={{ background: "#0d1117", color: "#4b5563" }}>
        <div className="text-center">
          <FileCode size={40} className="mx-auto mb-3 opacity-30" />
          <p className="text-xs">Agent 尚未操作文件</p>
        </div>
      </div>
    );
  }

  const opColors: Record<string, string> = {
    read: "#60a5fa", write: "#22c55e", edit: "#f59e0b", grep: "#a78bfa", find: "#06b6d4", ls: "#6b7280",
  };

  return (
    <div className="flex flex-col h-full">
      {/* Tab bar */}
      <div
        className="flex items-center gap-0.5 px-2 py-1 flex-shrink-0 overflow-x-auto"
        style={{ background: "#1a1a2e", borderBottom: "1px solid #2a2a3e" }}
      >
        {files.map((f, i) => {
          const name = f.path.split("/").pop() || f.path || f.operation;
          return (
            <button
              key={i}
              onClick={() => setActiveIdx(i)}
              className="flex items-center gap-1 px-2.5 py-1.5 text-[10px] font-mono rounded-t whitespace-nowrap"
              style={{
                background: i === activeIdx ? "#0d1117" : "transparent",
                color: i === activeIdx ? "#e5e7eb" : "#6b7280",
                borderBottom: i === activeIdx ? "2px solid var(--accent)" : "2px solid transparent",
              }}
            >
              <span className="w-1.5 h-1.5 rounded-full" style={{ background: opColors[f.operation] || "#6b7280" }} />
              {name}
            </button>
          );
        })}
      </div>

      {/* File info */}
      <div
        className="flex items-center gap-2 px-3 py-1.5 text-[10px] flex-shrink-0"
        style={{ background: "#141422", color: "#6b7280", borderBottom: "1px solid #1e1e30" }}
      >
        <span className="px-1.5 py-0.5 rounded" style={{ background: `${opColors[file.operation] || "#6b7280"}20`, color: opColors[file.operation] || "#6b7280" }}>
          {file.operation}
        </span>
        <span className="truncate flex-1">{file.path}</span>
        <span style={{ color: "#4b5563" }}>{file.language}</span>
      </div>

      {/* Code content */}
      <div
        className="flex-1 overflow-auto p-3 font-mono text-xs leading-relaxed"
        style={{ background: "#0d1117" }}
      >
        {file.content.split("\n").map((line, i) => (
          <div key={i} className="flex">
            <span className="w-10 text-right mr-3 select-none flex-shrink-0" style={{ color: "#3b3b50" }}>
              {i + 1}
            </span>
            <span style={{ color: "#e5e7eb" }}>{line}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

/* ─────────────────────────────────────────────
   Thinking View (Yunque's inner monologue)
   ───────────────────────────────────────────── */
function ThinkingView({ entries }: { entries: ThinkingEntry[] }) {
  const endRef = useRef<HTMLDivElement>(null);
  useEffect(() => { endRef.current?.scrollIntoView({ behavior: "smooth" }); }, [entries.length]);

  const typeIcon: Record<string, { icon: React.ReactNode; color: string; label: string }> = {
    thought:     { icon: <Brain size={12} />,     color: "#a78bfa", label: "思考" },
    observation: { icon: <Eye size={12} />,       color: "#22c55e", label: "观察" },
    plan:        { icon: <Sparkles size={12} />,  color: "#f59e0b", label: "计划" },
  };

  return (
    <div className="flex flex-col h-full" style={{ background: "#0d1117" }}>
      <div
        className="flex items-center gap-2 px-3 py-2 flex-shrink-0"
        style={{ background: "#1a1a2e", borderBottom: "1px solid #2a2a3e" }}
      >
        <Sparkles size={12} style={{ color: "var(--accent)" }} />
        <span className="text-[10px]" style={{ color: "#9ca3af" }}>云雀的思维过程</span>
      </div>

      <div className="flex-1 overflow-y-auto p-4 space-y-3">
        {entries.length === 0 ? (
          <div className="flex items-center justify-center h-full" style={{ color: "#4b5563" }}>
            <div className="text-center">
              <Brain size={32} className="mx-auto mb-3 opacity-30" />
              <p className="text-xs">等待 Agent 思考...</p>
            </div>
          </div>
        ) : (
          entries.map((entry, i) => {
            const t = typeIcon[entry.type] || typeIcon.thought;
            return (
              <div
                key={i}
                className="flex items-start gap-2 text-xs animate-in"
                style={{ animationDelay: `${i * 50}ms` }}
              >
                <div
                  className="w-5 h-5 rounded-full flex items-center justify-center flex-shrink-0 mt-0.5"
                  style={{ background: `${t.color}20`, color: t.color }}
                >
                  {t.icon}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-[10px] mb-0.5" style={{ color: t.color }}>{t.label}</div>
                  <div className="whitespace-pre-wrap" style={{ color: "#d1d5db" }}>
                    {entry.text}
                  </div>
                </div>
              </div>
            );
          })
        )}
        <div ref={endRef} />
      </div>
    </div>
  );
}

/* ─────────────────────────────────────────────
   Computer Panel — Main exported component
   ───────────────────────────────────────────── */
export interface ComputerPanelProps {
  steps?: TaskStep[];
  traceEvents?: AgentEvent[];
  taskStatus?: string;
  isLive?: boolean;
  className?: string;
}

export function ComputerPanel({ steps, traceEvents, taskStatus, isLive, className }: ComputerPanelProps) {
  const [activeTab, setActiveTab] = useState<ComputerTab>("terminal");
  const [sseImage, setSseImage] = useState("");
  const [sseUrl, setSseUrl] = useState("");

  // Parse all steps into view data
  const terminalLines: TerminalLine[] = [];
  const browserFrames: BrowserFrame[] = [];
  const editorFiles: EditorFile[] = [];
  const thinkingEntries: ThinkingEntry[] = [];

  // From TaskStep[] (task-run page)
  if (steps) {
    for (const step of steps) {
      if (step.status === "pending") continue;
      const parsed = parseStepResult(step);
      if (parsed.terminal) terminalLines.push(...parsed.terminal);
      if (parsed.browser) browserFrames.push(parsed.browser);
      if (parsed.editor) {
        const existingIdx = editorFiles.findIndex(f => f.path === parsed.editor!.path);
        if (existingIdx >= 0) {
          editorFiles[existingIdx] = parsed.editor;
        } else {
          editorFiles.push(parsed.editor);
        }
      }
      if (parsed.thinking) thinkingEntries.push(parsed.thinking);
    }
  }

  // From AgentEvent[] (chat page)
  if (traceEvents) {
    for (const evt of traceEvents) {
      const parsed = parseAgentEvent(evt);
      if (parsed.terminal) terminalLines.push(...parsed.terminal);
      if (parsed.browser) browserFrames.push(parsed.browser);
      if (parsed.editor) {
        const existingIdx = editorFiles.findIndex(f => f.path === parsed.editor!.path);
        if (existingIdx >= 0) {
          editorFiles[existingIdx] = parsed.editor;
        } else {
          editorFiles.push(parsed.editor);
        }
      }
      if (parsed.thinking) thinkingEntries.push(parsed.thinking);
    }
  }

  // SSE: listen for browser screenshots
  useEffect(() => {
    if (typeof window === "undefined") return;

    const key = localStorage.getItem("yunque_api_key") || localStorage.getItem("yunque_token") || "";
    let es: EventSource | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    function connect() {
      es = new EventSource(`/v1/events/stream?key=${encodeURIComponent(key)}`);

      es.addEventListener("browser.screenshot", (e) => {
        try {
          const data = JSON.parse(e.data);
          if (data.data?.image) {
            setSseImage(data.data.image);
            setActiveTab("browser");
          }
          if (data.data?.url) setSseUrl(data.data.url);
        } catch { /* ignore */ }
      });

      es.addEventListener("browser.action", (e) => {
        try {
          const data = JSON.parse(e.data);
          if (data.data?.action) setActiveTab("browser");
        } catch { /* ignore */ }
      });

      es.onerror = () => {
        es?.close();
        // Reconnect after 3 seconds
        reconnectTimer = setTimeout(connect, 3000);
      };
    }

    connect();

    return () => {
      es?.close();
      if (reconnectTimer) clearTimeout(reconnectTimer);
    };
  }, []);

  // Auto-switch tab based on latest step or trace event
  useEffect(() => {
    // From steps (task-run page)
    if (steps && steps.length > 0) {
      const lastActive = [...steps].reverse().find(s => s.status === "running" || s.status === "done");
      if (!lastActive) return;
      const skill = (lastActive.skill_name || lastActive.action || "").toLowerCase();
      if (skill.startsWith("browser_")) setActiveTab("browser");
      else if (skill.includes("shell") || skill.includes("exec") || skill.includes("command")) setActiveTab("terminal");
      else if (skill.includes("read") || skill.includes("write") || skill.includes("edit") || skill.includes("file") || skill.includes("grep")) setActiveTab("editor");
      else if (skill.includes("think") || skill.includes("plan") || skill.includes("reason")) setActiveTab("thinking");
      return;
    }
    // From trace events (chat page)
    if (traceEvents && traceEvents.length > 0) {
      const last = traceEvents[traceEvents.length - 1];
      const skill = (last.meta?.skill || "").toLowerCase();
      const type = (last.type || "").toLowerCase();
      if (skill.startsWith("browser_")) setActiveTab("browser");
      else if (skill.includes("shell") || skill.includes("exec") || skill.includes("command")) setActiveTab("terminal");
      else if (skill.includes("read") || skill.includes("write") || skill.includes("edit") || skill.includes("file") || skill.includes("grep")) setActiveTab("editor");
      else if (type === "thinking" || type === "reflect" || type.includes("plan")) setActiveTab("thinking");
      else if (type === "tool_start" || type === "tool_result") setActiveTab("terminal");
    }
  }, [steps, traceEvents]);

  const isActive = isLive || taskStatus === "running" || taskStatus === "planning";

  const tabs: { key: ComputerTab; icon: React.ReactNode; label: string; count: number }[] = [
    { key: "terminal", icon: <Terminal size={13} />, label: "终端", count: terminalLines.filter(l => l.type === "cmd").length },
    { key: "browser",  icon: <Globe size={13} />,    label: "浏览器", count: browserFrames.length },
    { key: "editor",   icon: <FileCode size={13} />, label: "编辑器", count: editorFiles.length },
    { key: "thinking", icon: <Brain size={13} />,    label: "思维", count: thinkingEntries.length },
  ];

  return (
    <div className={`flex flex-col h-full ${className || ""}`} style={{ background: "#0d1117" }}>
      {/* Computer header */}
      <div
        className="flex items-center justify-between px-3 py-2 flex-shrink-0"
        style={{ background: "#111827", borderBottom: "1px solid #1f2937" }}
      >
        <div className="flex items-center gap-2">
          <Monitor size={14} style={{ color: "var(--accent)" }} />
          <span className="text-xs font-medium" style={{ color: "#e5e7eb" }}>云雀的电脑</span>
          {isActive && (
            <span className="flex items-center gap-1 text-[10px] px-2 py-0.5 rounded-full" style={{ background: "#3b82f620", color: "#60a5fa" }}>
              <span className="w-1.5 h-1.5 rounded-full animate-pulse" style={{ background: "#3b82f6" }} />
              LIVE
            </span>
          )}
        </div>
      </div>

      {/* Tab bar */}
      <div
        className="flex items-center gap-1 px-2 py-1.5 flex-shrink-0"
        style={{ background: "#111827", borderBottom: "1px solid #1f2937" }}
      >
        {tabs.map(({ key, icon, label, count }) => (
          <button
            key={key}
            onClick={() => setActiveTab(key)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-full text-[11px] transition-all duration-200"
            style={{
              background: activeTab === key ? "var(--accent)" : "transparent",
              color: activeTab === key ? "#ffffff" : "#6b7280",
              fontWeight: activeTab === key ? 500 : 400,
            }}
          >
            {icon}
            <span>{label}</span>
            {count > 0 && (
              <span
                className="text-[9px] px-1.5 py-0 rounded-full"
                style={{
                  background: activeTab === key ? "rgba(255,255,255,0.2)" : "#374151",
                  color: activeTab === key ? "#fff" : "#9ca3af",
                }}
              >
                {count}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* View content */}
      <div className="flex-1 min-h-0">
        {activeTab === "terminal" && <TerminalView lines={terminalLines} />}
        {activeTab === "browser" && <BrowserView frames={browserFrames} sseImage={sseImage} sseUrl={sseUrl} />}
        {activeTab === "editor" && <EditorView files={editorFiles} />}
        {activeTab === "thinking" && <ThinkingView entries={thinkingEntries} />}
      </div>
    </div>
  );
}

export default ComputerPanel;
