"use client";

import { useEffect, useState } from "react";
import { api, type AuditRecord, type AuditStats } from "@/lib/api";
import { Shield, CheckCircle, XCircle, RefreshCw, Lock } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function AuditPage() {
  const [records, setRecords] = useState<AuditRecord[]>([]);
  const [stats, setStats] = useState<AuditStats | null>(null);
  const [verifyResult, setVerifyResult] = useState<{ valid: boolean; checked: number } | null>(null);
  const [verifying, setVerifying] = useState(false);
  const [loading, setLoading] = useState(true);
  const { t } = useI18n();

  const refresh = () => {
    Promise.all([
      api.auditTail(50).then((r) => setRecords(r.records || [])).catch(() => {}),
      api.auditStats().then(setStats).catch(() => {}),
    ]).finally(() => setLoading(false));
  };

  useEffect(() => { refresh(); }, []);

  const doVerify = async () => {
    setVerifying(true);
    try {
      const r = await api.auditVerify();
      setVerifyResult(r);
    } catch { setVerifyResult(null); }
    setVerifying(false);
  };

  return (
    <div className="animate-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: "var(--accent-subtle)" }}>
            <Shield size={20} style={{ color: "var(--accent)" }} />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">{t("audit.title")}</h1>
            <p className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>Merkle chain tamper-evident log</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={doVerify} disabled={verifying}
            className="btn-glow px-4 py-2.5 rounded-xl text-xs font-medium flex items-center gap-1.5">
            <Lock size={12} />
            {verifying ? t("audit.verifying") : t("audit.verifyChain")}
          </button>
          <button onClick={refresh} className="p-2.5 rounded-xl border hover:bg-[var(--bg-hover)]" style={{ color: "var(--text-muted)", borderColor: "var(--border)" }}>
            <RefreshCw size={14} />
          </button>
        </div>
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-3 gap-4 mb-6 stagger">
        <div className="card-hover animate-in rounded-xl border p-5"
          style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="text-[11px] uppercase tracking-wider mb-2" style={{ color: "var(--text-muted)" }}>{t("audit.totalRecords")}</div>
          <div className="text-3xl font-bold" style={{ color: "var(--accent)" }}>{stats?.total ?? "—"}</div>
        </div>
        {verifyResult && (
          <div className="card-hover animate-in rounded-xl border p-5 flex items-center gap-4"
            style={{
              background: "var(--bg-card)",
              borderColor: verifyResult.valid ? "var(--success)" : "var(--danger)",
              boxShadow: verifyResult.valid ? "0 0 20px rgba(34,197,94,0.1)" : "0 0 20px rgba(239,68,68,0.1)",
            }}>
            {verifyResult.valid ? <CheckCircle size={28} style={{ color: "var(--success)" }} /> : <XCircle size={28} style={{ color: "var(--danger)" }} />}
            <div>
              <div className="text-sm font-semibold" style={{ color: verifyResult.valid ? "var(--success)" : "var(--danger)" }}>
                {verifyResult.valid ? t("audit.chainValid") : t("audit.chainBroken")}
              </div>
              <div className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>{verifyResult.checked} {t("audit.recordsChecked")}</div>
            </div>
          </div>
        )}
        {stats && Object.keys(stats.actors || {}).length > 0 && (
          <div className="card-hover animate-in rounded-xl border p-5"
            style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-[11px] uppercase tracking-wider mb-2" style={{ color: "var(--text-muted)" }}>Top Actors</div>
            <div className="space-y-1">
              {Object.entries(stats.actors).slice(0, 3).map(([actor, count]) => (
                <div key={actor} className="flex justify-between text-xs">
                  <span style={{ color: "var(--text-secondary)" }}>{actor}</span>
                  <span className="font-mono" style={{ color: "var(--text-muted)" }}>{count}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Loading */}
      {loading && (
        <div className="space-y-2">
          <div className="skeleton h-10 w-full" />
          <div className="skeleton h-12 w-full" />
          <div className="skeleton h-12 w-full" />
          <div className="skeleton h-12 w-full" />
        </div>
      )}

      {/* Records table */}
      {!loading && (
        <div className="animate-in rounded-xl border overflow-hidden" style={{ borderColor: "var(--border)", boxShadow: "var(--shadow-sm)" }}>
          <table className="w-full text-sm">
            <thead>
              <tr style={{ background: "var(--bg-hover)" }}>
                <th className="text-left px-4 py-3 text-[11px] font-semibold uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>{t("audit.col.index")}</th>
                <th className="text-left px-4 py-3 text-[11px] font-semibold uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>{t("audit.col.time")}</th>
                <th className="text-left px-4 py-3 text-[11px] font-semibold uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>{t("audit.col.action")}</th>
                <th className="text-left px-4 py-3 text-[11px] font-semibold uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>{t("audit.col.actor")}</th>
                <th className="text-left px-4 py-3 text-[11px] font-semibold uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>{t("audit.col.detail")}</th>
                <th className="text-left px-4 py-3 text-[11px] font-semibold uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>{t("audit.col.hash")}</th>
              </tr>
            </thead>
            <tbody>
              {records.map((r) => (
                <tr key={r.index} className="border-t hover:bg-[var(--bg-hover)]" style={{ borderColor: "var(--border)", transition: "background 0.15s" }}>
                  <td className="px-4 py-3 text-xs font-mono" style={{ color: "var(--text-muted)" }}>{r.index}</td>
                  <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>{new Date(r.timestamp).toLocaleString("zh-CN")}</td>
                  <td className="px-4 py-3 text-xs">
                    <span className="badge" style={{ background: "var(--accent-subtle)", color: "var(--accent)" }}>{r.action}</span>
                  </td>
                  <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>{r.actor}</td>
                  <td className="px-4 py-3 text-xs max-w-[200px] truncate" style={{ color: "var(--text-secondary)" }} title={r.detail}>{r.detail}</td>
                  <td className="px-4 py-3 text-xs font-mono" style={{ color: "var(--text-muted)" }}>{r.hash?.slice(0, 12)}…</td>
                </tr>
              ))}
            </tbody>
          </table>
          {records.length === 0 && (
            <div className="text-sm text-center py-16" style={{ color: "var(--text-muted)" }}>
              <Shield size={32} className="mx-auto mb-3 opacity-30" />
              {t("audit.noRecords")}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
