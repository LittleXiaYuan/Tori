"use client";

import { useEffect, useState } from "react";
import { api, type ToolSession } from "@/lib/api";
import { TerminalSquare, Play, Square, RefreshCw, ChevronRight } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function ToolsPage() {
  const [command, setCommand] = useState("");
  const [cwd, setCwd] = useState("");
  const [bg, setBg] = useState(false);
  const [output, setOutput] = useState("");
  const [running, setRunning] = useState(false);
  const [sessions, setSessions] = useState<ToolSession[]>([]);
  const [pollId, setPollId] = useState<string | null>(null);
  const [pollLines, setPollLines] = useState<string[]>([]);
  const { t } = useI18n();

  const refreshSessions = () => {
    api.toolList().then((r) => setSessions(r.sessions || [])).catch(() => {});
  };

  useEffect(() => { refreshSessions(); }, []);

  const doExec = async () => {
    if (!command.trim()) return;
    setRunning(true);
    setOutput("");
    try {
      const r = await api.toolExec(command, cwd || undefined, bg);
      setOutput(r.output || (r.session_id ? `Backgrounded: ${r.session_id}` : "Done"));
      refreshSessions();
    } catch (e: unknown) {
      setOutput(String(e));
    }
    setRunning(false);
  };

  const doPoll = async (id: string) => {
    setPollId(id);
    const r = await api.toolPoll(id);
    setPollLines(r.lines || []);
  };

  const doKill = async (id: string) => {
    await api.toolKill(id);
    refreshSessions();
  };

  return (
    <div className="animate-in">
      {/* Header */}
      <div className="flex items-center gap-3 mb-8">
        <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: "var(--accent-subtle)" }}>
          <TerminalSquare size={20} style={{ color: "var(--accent)" }} />
        </div>
        <div>
          <h1 className="text-xl font-semibold tracking-tight">{t("tools.title")}</h1>
          <p className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>Execute commands on the agent server</p>
        </div>
      </div>

      {/* Terminal-style command input */}
      <div className="rounded-xl border p-5 mb-6 space-y-4"
        style={{ background: "var(--bg-card)", borderColor: "var(--border)", boxShadow: "var(--shadow-md)" }}>
        <div className="flex gap-2">
          <div className="flex-1 relative">
            <ChevronRight size={14} className="absolute left-4 top-1/2 -translate-y-1/2" style={{ color: "var(--accent)" }} />
            <input value={command} onChange={(e) => setCommand(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && doExec()}
              placeholder={t("tools.commandPlaceholder")}
              className="w-full pl-10 pr-4 py-3 rounded-xl border text-sm outline-none font-mono"
              style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }} />
          </div>
          <button onClick={doExec} disabled={running || !command.trim()}
            className="btn-glow px-5 py-3 rounded-xl text-sm font-medium flex items-center gap-2">
            <Play size={14} /> {running ? t("tools.running") : t("tools.run")}
          </button>
        </div>
        <div className="flex gap-3 items-center">
          <input value={cwd} onChange={(e) => setCwd(e.target.value)}
            placeholder={t("tools.cwdPlaceholder")}
            className="flex-1 px-4 py-2.5 rounded-xl border text-xs outline-none"
            style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }} />
          <label className="flex items-center gap-2 text-xs cursor-pointer select-none px-3 py-2 rounded-lg" style={{ color: "var(--text-muted)", background: "var(--bg-hover)" }}>
            <input type="checkbox" checked={bg} onChange={(e) => setBg(e.target.checked)} className="accent-[var(--accent)]" />
            {t("tools.background")}
          </label>
        </div>
      </div>

      {/* Output */}
      {output && (
        <div className="animate-in rounded-xl border p-5 mb-6"
          style={{ background: "var(--bg-card)", borderColor: "var(--accent)", boxShadow: "var(--shadow-glow)" }}>
          <div className="text-[11px] uppercase tracking-wider mb-2 font-medium" style={{ color: "var(--accent)" }}>Output</div>
          <pre className="text-xs font-mono whitespace-pre-wrap max-h-80 overflow-auto leading-relaxed"
            style={{ color: "var(--text-secondary)" }}>{output}</pre>
        </div>
      )}

      {/* Sessions */}
      {sessions.length > 0 && (
        <div className="animate-in">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-medium" style={{ color: "var(--text-muted)" }}>{t("tools.bgSessions")}</h2>
            <button onClick={refreshSessions} className="p-2 rounded-lg hover:bg-[var(--bg-hover)]" style={{ color: "var(--text-muted)" }}>
              <RefreshCw size={13} />
            </button>
          </div>
          <div className="space-y-2 stagger">
            {sessions.map((s) => (
              <div key={s.id} className="card-hover animate-in rounded-xl border px-5 py-4 flex items-center gap-3"
                style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                <div className={`w-2.5 h-2.5 rounded-full ${s.state === "running" ? "pulse-dot" : ""}`}
                  style={{ background: s.state === "running" ? "var(--success)" : "var(--text-muted)" }} />
                <div className="flex-1 min-w-0">
                  <div className="text-xs font-mono truncate">{s.command}</div>
                  <div className="text-[11px] mt-1 flex gap-2" style={{ color: "var(--text-muted)" }}>
                    <span className="badge" style={{ background: s.state === "running" ? "var(--success-bg)" : "var(--bg-hover)", color: s.state === "running" ? "var(--success)" : "var(--text-muted)" }}>
                      {s.state}
                    </span>
                    {s.state !== "running" && <span>exit {s.exit_code}</span>}
                  </div>
                </div>
                <button onClick={() => doPoll(s.id)}
                  className="text-xs px-3 py-1.5 rounded-lg font-medium hover:bg-[var(--accent-subtle)]"
                  style={{ color: "var(--accent)" }}>{t("tools.poll")}</button>
                {s.state === "running" && (
                  <button onClick={() => doKill(s.id)}
                    className="p-2 rounded-lg hover:bg-[var(--danger-bg)]"
                    style={{ color: "var(--danger)" }} title="Kill">
                    <Square size={13} />
                  </button>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Poll output */}
      {pollId && pollLines.length > 0 && (
        <div className="animate-in rounded-xl border p-5 mt-5"
          style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="text-[11px] uppercase tracking-wider mb-2 font-medium" style={{ color: "var(--text-muted)" }}>
            Poll: {pollId.slice(0, 8)}...
          </div>
          <pre className="text-xs font-mono whitespace-pre-wrap max-h-60 overflow-auto leading-relaxed"
            style={{ color: "var(--text-secondary)" }}>{pollLines.join("\n")}</pre>
        </div>
      )}
    </div>
  );
}
