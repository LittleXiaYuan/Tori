"use client";

import { useState } from "react";
import { Card, Button, Spinner, Chip, Tooltip } from "@heroui/react";
import { api, type AuditRecord, type AuditStats } from "@/lib/api";
import { Shield, RefreshCw, CheckCircle, XCircle, AlertTriangle, Link2, User } from "lucide-react";
import PageHeader from "@/components/page-header";
import { useApiData } from "@/lib/use-api-data";
import { formatErrorMessage } from "@/lib/error-utils";

export default function AuditPage() {
  const { data, loading, refresh } = useApiData(
    async () => {
      const [tail, st] = await Promise.all([
        api.auditTail(50),
        api.auditStats().catch(() => null),
      ]);
      return { records: tail.records || [] as AuditRecord[], stats: st };
    },
    { records: [] as AuditRecord[], stats: null as AuditStats | null },
  );
  const { records, stats } = data;
  const [verification, setVerification] = useState<{ valid: boolean; checked?: number; broken_at?: number; error?: string } | null>(null);
  const [verifying, setVerifying] = useState(false);

  const verify = async () => {
    setVerifying(true);
    try {
      const res = await api.auditVerify();
      setVerification(res);
    } catch (e: unknown) {
      setVerification({ valid: false, error: formatErrorMessage(e, "审计链验证失败") });
    }
    setVerifying(false);
  };

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader
        icon={<Shield size={20} />}
        title="审计日志"
        onRefresh={refresh}
        actions={
          <Button variant="ghost" size="sm" isPending={verifying} onPress={verify}
            style={{ color: "var(--yunque-text-muted)" }}>
            <Link2 size={14} /> {"验证链"}</Button>
        }
      />

      {/* Verification result */}
      {verification && (
        <Card className="p-4 animate-scale-in" style={{
          background: verification.valid ? "rgba(34,197,94,0.05)" : "rgba(239,68,68,0.05)",
          borderColor: verification.valid ? "var(--yunque-success)" : "var(--yunque-danger)",
        }}>
          <div className="flex items-center justify-between gap-3 flex-wrap">
            <div className="flex items-center gap-2">
            {verification.valid ? <CheckCircle size={16} style={{ color: "var(--yunque-success)" }} /> : <XCircle size={16} style={{ color: "var(--yunque-danger)" }} />}
            <span className="text-sm" style={{ color: verification.valid ? "var(--yunque-success)" : "var(--yunque-danger)" }}>
              {verification.valid ? "链验证通过 ✔" : `链验证失败: ${verification.error || ""}`}
            </span>
            </div>
            {"checked" in verification && typeof verification.checked === "number" && (
              <Chip size="sm" variant="soft">{`已检查 ${verification.checked} 条`}</Chip>
            )}
          </div>
        </Card>
      )}

      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 stagger-children">
          <Card className="section-card p-4 hover-lift">
            <div className="kpi-label">{"总记录"}</div>
            <div className="kpi-value">{stats.total || 0}</div>
          </Card>
          <Card className="section-card p-4 hover-lift">
            <div className="kpi-label">{"内存记录"}</div>
            <div className="kpi-value">{stats.in_memory || 0}</div>
          </Card>
          <Card className="section-card p-4 hover-lift">
            <div className="kpi-label">{"首条记录"}</div>
            <div className="text-sm font-semibold truncate" style={{ color: "var(--yunque-text)" }}>
              {stats.first_at ? new Date(stats.first_at).toLocaleDateString() : "—"}
            </div>
          </Card>
          <Card className="section-card p-4 hover-lift">
            <div className="kpi-label">{"持久化"}</div>
            <div className="text-sm font-semibold truncate" style={{ color: "var(--yunque-text)" }}>
              {stats.has_file ? "JSONL 已启用" : "仅内存"}
            </div>
          </Card>
        </div>
      )}

      {stats && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <Card className="section-card p-5">
            <div className="text-sm font-semibold mb-3" style={{ color: "var(--yunque-text)" }}>事件类型分布</div>
            {stats.type_counts && Object.keys(stats.type_counts).length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {Object.entries(stats.type_counts).map(([type, count]) => (
                  <Chip key={type} size="sm" variant="soft">{`${type}: ${count}`}</Chip>
                ))}
              </div>
            ) : (
              <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>暂无类型统计</div>
            )}
          </Card>
          <Card className="section-card p-5">
            <div className="text-sm font-semibold mb-3" style={{ color: "var(--yunque-text)" }}>操作者分布</div>
            {stats.actors && Object.keys(stats.actors).length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {Object.entries(stats.actors).map(([actor, count]) => (
                  <Chip key={actor} size="sm" variant="soft">{`${actor}: ${count}`}</Chip>
                ))}
              </div>
            ) : (
              <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>暂无操作者统计</div>
            )}
          </Card>
        </div>
      )}

      {/* Records */}
      <Card className="section-card p-5">
        {records.length === 0 ? (
          <div className="text-sm text-center py-12" style={{ color: "var(--yunque-text-muted)" }}>{"暂无审计记录"}</div>
        ) : (
          <div className="space-y-1 stagger-children">
            {records.map((r, i) => {
              const actionColor = r.action?.includes("delete") || r.action?.includes("remove") ? "#ef4444"
                : r.action?.includes("create") || r.action?.includes("add") ? "#22c55e"
                : r.action?.includes("update") || r.action?.includes("set") || r.action?.includes("edit") ? "#f59e0b"
                : "var(--yunque-accent)";
              return (
              <div key={i} className="flex items-center gap-3 p-3 rounded-lg transition-colors hover-lift"
                style={{ background: "rgba(255,255,255,0.02)", borderLeft: `3px solid ${actionColor}20` }}>
                <div className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0"
                  style={{ background: `${actionColor}15` }}>
                  <User size={14} style={{ color: actionColor }} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{r.action}</span>
                    {r.type && <Chip size="sm" variant="soft">{r.type}</Chip>}
                    <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)", fontSize: 10 }}>{r.actor || "system"}</Chip>
                  </div>
                  {r.detail && (
                    <div className="text-xs truncate mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>{r.detail}</div>
                  )}
                </div>
                <div className="text-xs shrink-0" style={{ color: "var(--yunque-text-muted)" }}>
                  {r.timestamp ? new Date(r.timestamp).toLocaleString() : ""}
                </div>
                {r.hash && (
                  <Tooltip delay={0}>
                    <span className="text-[10px] font-mono" style={{ color: "var(--yunque-text-muted)" }}>
                      {`#${r.seq} ${r.hash.slice(0, 8)}`}
                    </span>
                    <Tooltip.Content>{r.hash}</Tooltip.Content>
                  </Tooltip>
                )}
              </div>
              );
            })}
          </div>
        )}
      </Card>
    </div>
  );
}
