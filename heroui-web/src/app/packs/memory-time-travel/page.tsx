"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Card, Chip, Input, Spinner, TextArea, TextField } from "@heroui/react";
import { AlertTriangle, Clock3, Download, GitCompare, History, RefreshCw, RotateCcw, Save, ShieldCheck } from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import { createMemoryTimeTravelPackClient, type MemoryTimeTravelAuditVerification, type MemoryTimeTravelDiffReport, type MemoryTimeTravelSnapshotAtResponse, type MemoryTimeTravelSnapshotSummary, type MemoryTimeTravelStatus } from "@/lib/memory-time-travel-pack-client";

const memoryTimeTravelPack = createMemoryTimeTravelPackClient();

function riskTone(risk?: string): { bg: string; fg: string } {
  switch (risk) {
    case "high": return { bg: "rgba(239,68,68,0.16)", fg: "#ef4444" };
    case "medium": return { bg: "rgba(250,204,21,0.14)", fg: "#facc15" };
    case "low": return { bg: "rgba(56,189,248,0.12)", fg: "#38bdf8" };
    default: return { bg: "rgba(34,197,94,0.12)", fg: "#22c55e" };
  }
}

function defaultSnapshotId() {
  return `memory-${new Date().toISOString().slice(0, 16).replace(/[-:T]/g, "")}`;
}

function sampleValues() {
  return JSON.stringify({
    goal: "继续推进 Pack Runtime 轻内核 + 可选增量包",
    persona: "谨慎、可回滚、验证后提交",
    memory_layer: "long",
  }, null, 2);
}

export default function MemoryTimeTravelPackPage() {
  const [status, setStatus] = useState<MemoryTimeTravelStatus | null>(null);
  const [auditVerification, setAuditVerification] = useState<MemoryTimeTravelAuditVerification | null>(null);
  const [snapshots, setSnapshots] = useState<MemoryTimeTravelSnapshotSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"save" | "snapshot-at" | "diff" | "rollback" | "evidence" | "audit" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [namespace, setNamespace] = useState("memory_snapshot");
  const [snapshotId, setSnapshotId] = useState(defaultSnapshotId);
  const [valuesJSON, setValuesJSON] = useState(sampleValues);
  const [baseId, setBaseId] = useState("");
  const [targetId, setTargetId] = useState("");
  const [at, setAt] = useState("2026-05-15T12:00:00Z");
  const [snapshotAt, setSnapshotAt] = useState<MemoryTimeTravelSnapshotAtResponse | null>(null);
  const [diff, setDiff] = useState<MemoryTimeTravelDiffReport | null>(null);
  const [rollbackPlan, setRollbackPlan] = useState<string[]>([]);

  const selectedBase = useMemo(() => snapshots.find((item) => item.id === baseId) || snapshots[0] || null, [baseId, snapshots]);
  const selectedTarget = useMemo(() => snapshots.find((item) => item.id === targetId) || snapshots[1] || snapshots[0] || null, [targetId, snapshots]);
  const tone = riskTone(diff?.risk_level);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [statusRes, snapshotsRes] = await Promise.all([memoryTimeTravelPack.status(), memoryTimeTravelPack.snapshots(namespace)]);
      setStatus(statusRes);
      setSnapshots(snapshotsRes.snapshots || []);
      if (!baseId && snapshotsRes.snapshots?.[0]?.id) setBaseId(snapshotsRes.snapshots[0].id);
      if (!targetId && snapshotsRes.snapshots?.[1]?.id) setTargetId(snapshotsRes.snapshots[1].id);
    } catch (e) {
      const msg = formatErrorMessage(e, "加载 Memory Time Travel Pack 失败");
      setError(msg.includes("pack route is not enabled") ? "Memory Time Travel Pack 当前未启用。请到「增量包」控制台启用 yunque.pack.memory-time-travel 后再使用。" : msg);
    } finally {
      setLoading(false);
    }
  }, [baseId, namespace, targetId]);

  useEffect(() => { load(); }, [load]);

  const saveSnapshot = async () => {
    setBusy("save");
    setError(null);
    try {
      const values = JSON.parse(valuesJSON) as Record<string, string>;
      const res = await memoryTimeTravelPack.saveSnapshot({ id: snapshotId, namespace, source: "pack-console", reason: "manual baseline", values });
      setBaseId(res.snapshot.id);
      showToast("记忆快照已保存", "success");
      await load();
    } catch (e) {
      setError(formatErrorMessage(e, "保存记忆快照失败"));
    } finally {
      setBusy(null);
    }
  };

  const reconstruct = async () => {
    setBusy("snapshot-at");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.snapshotAt({ namespace, at });
      setSnapshotAt(res);
      showToast(res.status === "reconstructed" ? "已重建时间点快照" : "该时间点前没有快照", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "重建时间点快照失败"));
    } finally {
      setBusy(null);
    }
  };

  const runDiff = async () => {
    const base = baseId || selectedBase?.id;
    const target = targetId || selectedTarget?.id;
    if (!base || !target) {
      setError("请先保存或选择两个记忆快照。至少需要 base_id 与 target_id。");
      return;
    }
    setBusy("diff");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.diff({ namespace, base_id: base, target_id: target });
      setDiff(res.diff);
      setRollbackPlan(res.diff.rollback_plan || []);
      showToast("已生成记忆漂移报告", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "记忆漂移对比失败"));
    } finally {
      setBusy(null);
    }
  };

  const buildRollbackPlan = async () => {
    const target = baseId || selectedBase?.id;
    if (!target) return;
    setBusy("rollback");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.rollbackPlan({ namespace, snapshot_id: target, dry_run: true });
      setRollbackPlan(res.plan.actions || []);
      showToast("已生成 dry-run 回滚计划", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成回滚计划失败"));
    } finally {
      setBusy(null);
    }
  };

  const exportEvidence = async () => {
    const target = baseId || selectedBase?.id;
    if (!target) return;
    setBusy("evidence");
    setError(null);
    try {
      const evidence = await memoryTimeTravelPack.evidence(target);
      const blob = new Blob([JSON.stringify(evidence, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${target}-memory-time-travel-evidence.json`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      showToast("记忆回溯证据包已导出", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "导出记忆回溯证据包失败"));
    } finally {
      setBusy(null);
    }
  };

  const verifyAuditChain = async () => {
    setBusy("audit");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.auditVerify(8);
      setAuditVerification(res);
      showToast(
        res.ready ? (res.valid ? "Merkle 审计链验证通过" : `Merkle 审计链异常：index ${res.invalid_index}`) : "Merkle 审计链验证器尚未接入",
        !res.ready ? "info" : res.valid ? "success" : "warning",
      );
    } catch (e) {
      setError(formatErrorMessage(e, "Merkle 审计链验证失败"));
    } finally {
      setBusy(null);
    }
  };

  if (loading) {
    return <div className="flex h-[60vh] items-center justify-center"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader icon={<History size={20} />} title="Memory Time Travel" />

      <Card className="section-card p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="mb-1 flex items-center gap-2">
              <Chip size="sm" style={{ background: status?.ledger_history_ready ? "rgba(34,197,94,0.12)" : "rgba(250,204,21,0.12)", color: status?.ledger_history_ready ? "#22c55e" : "#facc15" }}>
                {status?.ledger_history_ready ? "Ledger history ready" : "Pack shell"}
              </Chip>
              <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{status?.pack_id || "yunque.pack.memory-time-travel"}</span>
            </div>
            <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>
              当前切片完成记忆快照存储、时间点回溯、漂移 diff、dry-run 回滚计划、证据包导出和只读 Merkle 审计链验证。原生 Ledger kv_history 表、retention cron 和真实写回仍作为后续切片推进。
            </div>
          </div>
          <Button size="sm" variant="ghost" onPress={load}><RefreshCw size={14} />刷新</Button>
        </div>
      </Card>

      {error && (
        <Card className="p-4" style={{ background: "rgba(239,68,68,0.06)" }}>
          <div className="flex items-center gap-2 text-sm" style={{ color: "var(--yunque-danger)" }}><AlertTriangle size={16} />{error}</div>
        </Card>
      )}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <Card className="section-card p-4"><div className="kpi-label">快照数量</div><div className="kpi-value">{status?.snapshot_count ?? snapshots.length}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">命名空间</div><div className="kpi-value">{status?.namespace_count ?? 0}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">Merkle 验证</div><div className="kpi-value text-lg">{status?.merkle_verification_ready ? "ready" : "pending"}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">阶段</div><div className="kpi-value text-lg">{status?.stage || "pack-shell"}</div></Card>
      </div>

      <Card className="section-card p-4">
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold"><ShieldCheck size={16} />Merkle 审计链验证</div>
            <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
              通过 Pack Runtime 只读验证内存中的 audit chain，并展示最近记录摘要；这里不执行回滚写回，也不声称 KV history 已具备逐条审计证明。
            </div>
          </div>
          <Button variant="outline" isPending={busy === "audit"} onPress={verifyAuditChain}><ShieldCheck size={14} />验证审计链</Button>
        </div>
        {auditVerification ? (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-[220px_1fr]">
            <div className="rounded-xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
              <Chip size="sm" style={{ background: auditVerification.ready && auditVerification.valid ? "rgba(34,197,94,0.12)" : "rgba(250,204,21,0.12)", color: auditVerification.ready && auditVerification.valid ? "#22c55e" : "#facc15" }}>
                {auditVerification.ready ? (auditVerification.valid ? "valid" : "invalid") : "not attached"}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{auditVerification.record_count}</div>
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>records · last seq {auditVerification.last_seq ?? "-"}</div>
              <div className="mt-2 truncate text-xs" style={{ color: "var(--yunque-text-muted)" }}>{auditVerification.last_hash || "no hash"}</div>
            </div>
            <TextField value={JSON.stringify(auditVerification, null, 2)} onChange={() => undefined}>
              <TextArea rows={8} aria-label="Memory Time Travel audit verification JSON" className="font-mono text-xs" readOnly />
            </TextField>
          </div>
        ) : (
          <div className="rounded-xl border border-dashed p-4 text-sm" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
            尚未运行本页验证。启用 pack 后可通过 `/v1/memory-time-travel/audit/verify` 获取只读验证结果。
          </div>
        )}
      </Card>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_1fr]">
        <Card className="section-card overflow-hidden">
          <div className="flex items-center justify-between border-b px-4 py-3" style={{ borderColor: "var(--yunque-border)" }}>
            <div className="flex items-center gap-2 text-sm font-semibold"><Clock3 size={16} />记忆快照</div>
            <Chip size="sm">{snapshots.length}</Chip>
          </div>
          <div className="max-h-[560px] divide-y overflow-auto" style={{ borderColor: "var(--yunque-border)" }}>
            {snapshots.length === 0 ? <div className="p-6 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>还没有快照。可以先保存右侧样例作为 baseline。</div> : snapshots.map((snapshot) => (
              <button key={`${snapshot.namespace}:${snapshot.id}`} onClick={() => { setBaseId(snapshot.id); if (!targetId || targetId === snapshot.id) setTargetId(snapshots.find((item) => item.id !== snapshot.id)?.id || snapshot.id); }} className="block w-full px-4 py-3 text-left hover:bg-white/5">
                <div className="flex items-center justify-between gap-2"><div className="font-medium">{snapshot.id}</div><Chip size="sm">{snapshot.key_count} keys</Chip></div>
                <div className="mt-1 truncate text-xs" style={{ color: "var(--yunque-text-muted)" }}>{snapshot.namespace} · {snapshot.hash.slice(0, 12)}</div>
              </button>
            ))}
          </div>
        </Card>

        <div className="space-y-4">
          <Card className="section-card p-4">
            <div className="mb-3 flex items-center gap-2 text-sm font-semibold"><Save size={16} />保存记忆快照</div>
            <div className="mb-3 grid grid-cols-1 gap-3 md:grid-cols-2">
              <TextField value={namespace} onChange={setNamespace}><Input placeholder="memory_snapshot" /></TextField>
              <TextField value={snapshotId} onChange={setSnapshotId}><Input placeholder="baseline id" /></TextField>
            </div>
            <TextField value={valuesJSON} onChange={setValuesJSON}>
              <TextArea rows={8} aria-label="Memory snapshot values JSON" className="font-mono text-xs" />
            </TextField>
            <div className="mt-3 flex justify-end"><Button className="btn-accent" isPending={busy === "save"} onPress={saveSnapshot}>保存快照</Button></div>
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-sm font-semibold"><GitCompare size={16} />时间点回溯与漂移对比</div>
              <div className="flex gap-2">
                <Button variant="outline" isPending={busy === "evidence"} onPress={exportEvidence} isDisabled={!selectedBase && !baseId}><Download size={14} />导出证据包</Button>
                <Button variant="outline" isPending={busy === "rollback"} onPress={buildRollbackPlan} isDisabled={!selectedBase && !baseId}><RotateCcw size={14} />回滚计划</Button>
                <Button className="btn-accent" isPending={busy === "diff"} onPress={runDiff} isDisabled={!selectedBase || !selectedTarget}>生成 diff</Button>
              </div>
            </div>
            <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
              <TextField value={baseId} onChange={setBaseId}><Input placeholder="base snapshot id" /></TextField>
              <TextField value={targetId} onChange={setTargetId}><Input placeholder="target snapshot id" /></TextField>
              <TextField value={at} onChange={setAt}><Input placeholder="2026-05-15T12:00:00Z" /></TextField>
            </div>
            <div className="mt-3 flex justify-end"><Button variant="outline" isPending={busy === "snapshot-at"} onPress={reconstruct}><Clock3 size={14} />重建时间点</Button></div>

            {snapshotAt && (
              <Card className="mt-3 p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
                <div className="mb-2 text-sm font-medium">snapshot-at: {snapshotAt.status} {snapshotAt.matched_id ? `· ${snapshotAt.matched_id}` : ""}</div>
                <TextField value={JSON.stringify(snapshotAt.values, null, 2)} onChange={() => undefined}>
                  <TextArea rows={6} aria-label="Snapshot at values JSON" className="font-mono text-xs" readOnly />
                </TextField>
              </Card>
            )}

            {diff ? (
              <Card className="mt-3 p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
                <div className="mb-2 flex flex-wrap items-center gap-2 text-sm font-medium">
                  <Chip size="sm" style={{ background: tone.bg, color: tone.fg }}>risk: {diff.risk_level}</Chip>
                  <span>{diff.added_count} added / {diff.changed_count} changed / {diff.removed_count} removed · drift {diff.drift_score}</span>
                </div>
                <TextField value={JSON.stringify(diff, null, 2)} onChange={() => undefined}>
                  <TextArea rows={12} aria-label="Memory Time Travel diff JSON" className="font-mono text-xs" readOnly />
                </TextField>
              </Card>
            ) : (
              <div className="mt-3 rounded-xl border border-dashed p-6 text-center text-sm" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>保存或选择两个快照后，可以生成记忆漂移 diff 和回滚建议。</div>
            )}

            {rollbackPlan.length > 0 && (
              <Card className="mt-3 p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
                <div className="mb-2 flex items-center gap-2 text-sm font-medium"><RotateCcw size={15} />Dry-run rollback plan</div>
                <pre className="max-h-56 overflow-auto whitespace-pre-wrap text-xs" style={{ color: "var(--yunque-text-muted)" }}>{rollbackPlan.join("\n")}</pre>
              </Card>
            )}
          </Card>
        </div>
      </div>
    </div>
  );
}
