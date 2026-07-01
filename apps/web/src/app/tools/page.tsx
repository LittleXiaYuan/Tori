"use client";

import { useEffect, useState, useRef } from "react";
import { Card, Button, Tooltip } from "@heroui/react";
import { api } from "@/lib/api";
import { Terminal, Send, Plus, FolderOpen, Play, XCircle, RefreshCw } from "lucide-react";
import { showToast } from "@/components/toast-provider";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";

interface ToolSession {
  id: string;
  command: string;
  output: string;
  running: boolean;
  sessionId?: string; // backend session ID for poll/kill
}

export default function ToolsPage() {
  const [sessions, setSessions] = useState<ToolSession[]>([]);
  const [activeIdx, setActiveIdx] = useState(0);
  const [input, setInput] = useState("");
  const [cwd, setCwd] = useState("");
  const [bgMode, setBgMode] = useState(false);
  const [loading, setLoading] = useState(false);
  const outputRef = useRef<HTMLDivElement>(null);
  const commandInputRef = useRef<HTMLInputElement>(null);

  const addSession = () => {
    const newSession: ToolSession = {
      id: Date.now().toString(),
      command: "",
      output: "",
      running: false,
    };
    setSessions((prev) => [...prev, newSession]);
    setActiveIdx(sessions.length);
  };

  useEffect(() => {
    if (sessions.length === 0) addSession();
  }, []);

  const executeCommand = async () => {
    if (!input.trim() || loading) return;
    const cmd = input.trim();
    setInput("");
    setLoading(true);

    setSessions((prev) => {
      const next = [...prev];
      if (next[activeIdx]) {
        next[activeIdx] = {
          ...next[activeIdx],
          command: cmd,
          output: next[activeIdx].output + `\n$ ${cmd}\n`,
          running: true,
        };
      }
      return next;
    });

    try {
      const res = await api.toolExec(cmd, cwd || undefined, bgMode);
      const sessionId = res.session_id || (res as any).sessionId;
      setSessions((prev) => {
        const next = [...prev];
        if (next[activeIdx]) {
          next[activeIdx] = {
            ...next[activeIdx],
            output: next[activeIdx].output + (res.output || (res as any).result || "done") + "\n",
            running: bgMode && !!sessionId,
            sessionId: sessionId || next[activeIdx].sessionId,
          };
        }
        return next;
      });
    } catch (e: unknown) {
      const friendly = formatErrorMessage(e, "执行失败");
      setSessions((prev) => {
        const next = [...prev];
        if (next[activeIdx]) {
          next[activeIdx] = {
            ...next[activeIdx],
            output: next[activeIdx].output + `Error: ${friendly}\n`,
            running: false,
          };
        }
        return next;
      });
    }
    setLoading(false);
    setTimeout(() => outputRef.current?.scrollTo({ top: outputRef.current.scrollHeight }), 50);
  };

  const currentSession = sessions[activeIdx];

  const pollSession = async () => {
    if (!currentSession?.sessionId) return;
    try {
      const res = await api.toolPoll(currentSession.sessionId);
      setSessions((prev) => {
        const next = [...prev];
        if (next[activeIdx]) {
          const lines = (res.lines || []).join("\n");
          next[activeIdx] = {
            ...next[activeIdx],
            output: next[activeIdx].output + (lines ? lines + "\n" : ""),
            running: res.state === "running",
          };
        }
        return next;
      });
    } catch { /* ignore */ }
  };

  const killSession = async () => {
    if (!currentSession?.sessionId) return;
    try {
      await api.toolKill(currentSession.sessionId);
      setSessions((prev) => {
        const next = [...prev];
        if (next[activeIdx]) {
          next[activeIdx] = { ...next[activeIdx], output: next[activeIdx].output + "[killed]\n", running: false };
        }
        return next;
      });
    } catch (e) { showToast(e instanceof Error ? e.message : "终止失败", "error"); }
  };

  return (
    <div className="page-root flex flex-col animate-fade-in-up" style={{ minHeight: "calc(100vh - var(--yunque-sidebar-width, 0px))" }}>
      <PageHeader
        icon={<Terminal aria-hidden="true" size={20} />}
        title="工具执行"
        description="验证命令、后台会话和输出。"
        actions={
          <Tooltip delay={0}>
            <Button isIconOnly aria-label="新建工具会话" variant="ghost" size="sm" onPress={addSession}
              style={{ color: "var(--yunque-text-muted)" }}>
              <Plus aria-hidden="true" size={16} />
            </Button>
            <Tooltip.Content>新会话</Tooltip.Content>
          </Tooltip>
        }
      />

      <section
        aria-label="工具恢复入口"
        className="mb-3 flex flex-col gap-3 rounded-lg border p-3 md:flex-row md:items-center md:justify-between"
        style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-card)" }}
      >
        <div className="min-w-0">
          <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>工具失败？</div>
          <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>新会话隔离，直接复现命令。</div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" variant="secondary" onPress={addSession}>
            <Plus aria-hidden="true" size={13} /> 新会话
          </Button>
          <Button size="sm" variant="ghost" onPress={() => commandInputRef.current?.focus()}>
            <Terminal aria-hidden="true" size={13} /> 输命令
          </Button>
        </div>
      </section>

      {/* Session tabs */}
      {sessions.length > 1 && (
        <div className="flex items-center gap-1 mb-3 overflow-x-auto">
          {sessions.map((s, i) => (
            <Button
              key={s.id}
              variant={i === activeIdx ? "secondary" : "ghost"}
              size="sm"
              onPress={() => setActiveIdx(i)}
              style={{
                color: i === activeIdx ? "var(--yunque-accent)" : "var(--yunque-text-muted)",
                background: i === activeIdx ? "rgba(0,111,238,0.08)" : "transparent",
              }}
            >
              <Terminal aria-hidden="true" size={12} /> #{i + 1}
              {s.running && <span className="animate-pulse-dot ml-1" style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--yunque-success)", display: "inline-block" }} />}
            </Button>
          ))}
        </div>
      )}

      {/* CWD and options */}
      <div className="flex items-center gap-3 mb-3">
        <div className="flex items-center gap-2 flex-1 px-3 py-1.5 rounded-lg" style={{ background: "rgba(255,255,255,0.04)" }}>
          <FolderOpen aria-hidden="true" size={13} style={{ color: "var(--yunque-text-muted)" }} />
          <label htmlFor="tools-cwd-input" className="text-xs font-medium whitespace-nowrap" style={{ color: "var(--yunque-text-muted)" }}>
            工作目录
          </label>
          <input
            id="tools-cwd-input"
            value={cwd}
            onChange={(e) => setCwd(e.target.value)}
            placeholder="可选路径"
            className="flex-1 bg-transparent text-xs font-mono outline-none"
            style={{ color: "var(--yunque-text)" }}
          />
        </div>
        <Button
          type="button"
          aria-pressed={bgMode}
          variant="ghost"
          size="sm"
          onPress={() => setBgMode(!bgMode)}
          className="gap-1.5"
          style={{
            background: bgMode ? "rgba(0,111,238,0.12)" : "rgba(255,255,255,0.04)",
            color: bgMode ? "var(--yunque-accent)" : "var(--yunque-text-muted)",
          }}
        >
          <Play aria-hidden="true" size={11} /> 后台
        </Button>
        {currentSession?.running && currentSession?.sessionId && (
          <>
            <Tooltip delay={0}>
              <Button isIconOnly aria-label="刷新当前工具会话输出" variant="ghost" size="sm" onPress={pollSession}
                style={{ color: "var(--yunque-accent)" }}>
                <RefreshCw aria-hidden="true" size={14} />
              </Button>
              <Tooltip.Content>刷新输出</Tooltip.Content>
            </Tooltip>
            <Tooltip delay={0}>
              <Button isIconOnly aria-label="终止当前工具会话" variant="ghost" size="sm" onPress={killSession}
                style={{ color: "var(--yunque-danger)" }}>
                <XCircle aria-hidden="true" size={14} />
              </Button>
              <Tooltip.Content>终止</Tooltip.Content>
            </Tooltip>
          </>
        )}
      </div>

      {/* Terminal output */}
      <Card className="section-card flex-1 flex flex-col overflow-hidden">
        <div
          ref={outputRef}
          className="flex-1 p-4 overflow-y-auto font-mono text-sm custom-scrollbar"
          style={{ color: "var(--yunque-text)", whiteSpace: "pre-wrap", minHeight: 200 }}
        >
          {currentSession?.output || (
            <span style={{ color: "var(--yunque-text-muted)" }}>
              {"输入命令并按 Enter 执行..."}
            </span>
          )}
        </div>

        {/* Input */}
        <div className="p-3 flex items-center gap-2" style={{ borderTop: "1px solid var(--yunque-border)" }}>
          <label htmlFor="tools-command-input" className="text-xs font-medium whitespace-nowrap" style={{ color: "var(--yunque-text-muted)" }}>
            命令
          </label>
          <span className="text-sm font-mono" style={{ color: "var(--yunque-accent)" }}>$</span>
          <input
            id="tools-command-input"
            ref={commandInputRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") executeCommand(); }}
            placeholder={"输入命令..."}
            className="flex-1 bg-transparent text-sm font-mono outline-none"
            style={{ color: "var(--yunque-text)" }}
            disabled={loading}
          />
          <Tooltip delay={0}>
            <Button isIconOnly aria-label="执行命令" variant="ghost" size="sm" onPress={executeCommand}
              isPending={loading}
              style={{ color: "var(--yunque-accent)" }}>
              <Send aria-hidden="true" size={14} />
            </Button>
            <Tooltip.Content>{"执行"}</Tooltip.Content>
          </Tooltip>
        </div>
      </Card>
    </div>
  );
}
