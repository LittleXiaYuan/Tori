"use client";

import { useEffect, useState } from "react";
import { api, type CronJob } from "@/lib/api";
import { Clock, Play, Trash2, Plus, X, Timer } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function CronPage() {
  const [jobs, setJobs] = useState<CronJob[]>([]);
  const [showAdd, setShowAdd] = useState(false);
  const [name, setName] = useState("");
  const [schedType, setSchedType] = useState<"every" | "cron">("every");
  const [everyMin, setEveryMin] = useState("60");
  const [cronExpr, setCronExpr] = useState("*/5 * * * *");
  const [message, setMessage] = useState("");
  const [runOutput, setRunOutput] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const { t } = useI18n();

  const refresh = () => {
    api.cronList().then((r) => setJobs(r.jobs || [])).catch(() => {}).finally(() => setLoading(false));
  };

  useEffect(() => { refresh(); }, []);

  const doAdd = async () => {
    if (!name.trim() || !message.trim()) return;
    const schedule = schedType === "every"
      ? { type: "every", every_ms: parseInt(everyMin) * 60000 }
      : { type: "cron", cron_expr: cronExpr };
    await api.cronAdd(name, schedule, { kind: "agentTurn", message });
    setShowAdd(false);
    setName("");
    setMessage("");
    refresh();
  };

  const doRun = async (id: string) => {
    const r = await api.cronRun(id);
    setRunOutput(r.run.output || r.run.error || "No output");
    refresh();
  };

  const doRemove = async (id: string) => {
    await api.cronRemove(id);
    refresh();
  };

  const fmtTime = (ts?: string) => ts ? new Date(ts).toLocaleString("zh-CN") : "—";

  return (
    <div className="animate-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: "var(--accent-subtle)" }}>
            <Clock size={20} style={{ color: "var(--accent)" }} />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">{t("cron.title")}</h1>
            <p className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>{jobs.length} jobs</p>
          </div>
        </div>
        <button onClick={() => setShowAdd(!showAdd)}
          className={showAdd ? "px-4 py-2.5 rounded-xl text-xs font-medium flex items-center gap-1.5 border" : "btn-glow px-4 py-2.5 rounded-xl text-xs font-medium flex items-center gap-1.5"}
          style={showAdd ? { borderColor: "var(--border)", color: "var(--text-muted)" } : {}}>
          {showAdd ? <X size={13} /> : <Plus size={13} />}
          {showAdd ? t("cron.cancel") : t("cron.addJob")}
        </button>
      </div>

      {/* Add form */}
      {showAdd && (
        <div className="animate-in rounded-xl border p-6 mb-6 space-y-4"
          style={{ background: "var(--bg-card)", borderColor: "var(--border)", boxShadow: "var(--shadow-md)" }}>
          <input value={name} onChange={(e) => setName(e.target.value)}
            placeholder={t("cron.jobName")}
            className="w-full px-4 py-3 rounded-xl border text-sm outline-none"
            style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }} />
          <div className="flex gap-3 items-center">
            <select value={schedType} onChange={(e) => setSchedType(e.target.value as "every" | "cron")}
              className="px-4 py-3 rounded-xl border text-sm outline-none"
              style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }}>
              <option value="every">{t("cron.everyNMin")}</option>
              <option value="cron">{t("cron.cronExpr")}</option>
            </select>
            {schedType === "every" ? (
              <input value={everyMin} onChange={(e) => setEveryMin(e.target.value)}
                type="number" min="1" placeholder="Minutes"
                className="w-28 px-4 py-3 rounded-xl border text-sm outline-none"
                style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }} />
            ) : (
              <input value={cronExpr} onChange={(e) => setCronExpr(e.target.value)}
                placeholder="*/5 * * * *"
                className="flex-1 px-4 py-3 rounded-xl border text-sm outline-none font-mono"
                style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }} />
            )}
          </div>
          <input value={message} onChange={(e) => setMessage(e.target.value)}
            placeholder={t("cron.prompt")}
            className="w-full px-4 py-3 rounded-xl border text-sm outline-none"
            style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }} />
          <button onClick={doAdd} className="btn-glow px-5 py-3 rounded-xl text-sm font-medium">
            {t("cron.create")}
          </button>
        </div>
      )}

      {/* Run output */}
      {runOutput && (
        <div className="animate-in rounded-xl border p-5 mb-5 relative"
          style={{ background: "var(--bg-card)", borderColor: "var(--accent)", boxShadow: "var(--shadow-glow)" }}>
          <button onClick={() => setRunOutput(null)} className="absolute top-4 right-4 p-1 rounded-lg hover:bg-[var(--bg-hover)]" style={{ color: "var(--text-muted)" }}>
            <X size={14} />
          </button>
          <div className="text-[11px] uppercase tracking-wider mb-2 font-medium" style={{ color: "var(--accent)" }}>Output</div>
          <pre className="text-xs font-mono whitespace-pre-wrap max-h-48 overflow-auto" style={{ color: "var(--text-secondary)" }}>{runOutput}</pre>
        </div>
      )}

      {/* Loading */}
      {loading && (
        <div className="space-y-3">
          <div className="skeleton h-20 w-full" />
          <div className="skeleton h-20 w-full" />
        </div>
      )}

      {/* Job list */}
      {!loading && (
        <div className="space-y-2 stagger">
          {jobs.map((j) => (
            <div key={j.id} className="card-hover animate-in rounded-xl border px-5 py-4 flex items-center gap-4"
              style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              <div className="w-9 h-9 rounded-lg flex items-center justify-center" style={{ background: j.enabled ? "var(--accent-subtle)" : "var(--bg-hover)" }}>
                <Timer size={16} style={{ color: j.enabled ? "var(--accent)" : "var(--text-muted)" }} />
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium">{j.name}</div>
                <div className="text-xs flex gap-3 mt-1" style={{ color: "var(--text-muted)" }}>
                  <span className="badge" style={{ background: "var(--bg-hover)" }}>
                    {j.schedule.type === "every" ? `${(j.schedule.every_ms || 0) / 60000}m` : j.schedule.cron_expr}
                  </span>
                  <span>{t("cron.runs")}: {j.run_count}</span>
                  <span>{t("cron.last")}: {fmtTime(j.last_run_at)}</span>
                </div>
              </div>
              <button onClick={() => doRun(j.id)}
                className="p-2.5 rounded-lg hover:bg-[var(--accent-subtle)]"
                style={{ color: "var(--accent)" }} title="Run now">
                <Play size={15} />
              </button>
              <button onClick={() => doRemove(j.id)}
                className="p-2.5 rounded-lg hover:bg-[var(--danger-bg)]"
                style={{ color: "var(--text-muted)" }} title="Delete">
                <Trash2 size={15} />
              </button>
            </div>
          ))}
          {jobs.length === 0 && (
            <div className="text-sm text-center py-16 rounded-xl border" style={{ color: "var(--text-muted)", borderColor: "var(--border)", borderStyle: "dashed" }}>
              <Clock size={32} className="mx-auto mb-3 opacity-30" />
              {t("cron.noJobs")}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
