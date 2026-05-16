"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Card, Chip, Input, Spinner, TextArea, TextField } from "@heroui/react";
import { AlertTriangle, Clock3, DatabaseZap, Download, GitCompare, History, Link2, RefreshCw, RotateCcw, Save, ShieldCheck, Trash2, UnlockKeyhole } from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import { createMemoryTimeTravelPackClient, type MemoryTimeTravelApprovedRollbackPlan, type MemoryTimeTravelAuditVerification, type MemoryTimeTravelDiffReport, type MemoryTimeTravelKVAuditLinksReport, type MemoryTimeTravelKVHistoryCutoverPlan, type MemoryTimeTravelKVHistoryDualReadParity, type MemoryTimeTravelNativeKVHistoryMigrationPreview, type MemoryTimeTravelNativeKVHistoryPlan, type MemoryTimeTravelRetentionPlan, type MemoryTimeTravelRetentionPrunePlan, type MemoryTimeTravelSnapshotAtResponse, type MemoryTimeTravelSnapshotSummary, type MemoryTimeTravelStatus } from "@/lib/memory-time-travel-pack-client";

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
  const [auditLinks, setAuditLinks] = useState<MemoryTimeTravelKVAuditLinksReport | null>(null);
  const [nativeKVHistoryPlan, setNativeKVHistoryPlan] = useState<MemoryTimeTravelNativeKVHistoryPlan | null>(null);
  const [nativeKVHistoryPreview, setNativeKVHistoryPreview] = useState<MemoryTimeTravelNativeKVHistoryMigrationPreview | null>(null);
  const [kvHistoryDualReadParity, setKVHistoryDualReadParity] = useState<MemoryTimeTravelKVHistoryDualReadParity | null>(null);
  const [kvHistoryCutoverPlan, setKVHistoryCutoverPlan] = useState<MemoryTimeTravelKVHistoryCutoverPlan | null>(null);
  const [approvedRollbackPlan, setApprovedRollbackPlan] = useState<MemoryTimeTravelApprovedRollbackPlan | null>(null);
  const [retentionPlan, setRetentionPlan] = useState<MemoryTimeTravelRetentionPlan | null>(null);
  const [retentionPrunePlan, setRetentionPrunePlan] = useState<MemoryTimeTravelRetentionPrunePlan | null>(null);
  const [snapshots, setSnapshots] = useState<MemoryTimeTravelSnapshotSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"save" | "snapshot-at" | "diff" | "rollback" | "approved-rollback" | "evidence" | "audit" | "audit-links" | "native-kv-history" | "native-kv-history-preview" | "kv-history-dual-read-parity" | "kv-history-cutover" | "retention" | "retention-prune" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [namespace, setNamespace] = useState("memory_snapshot");
  const [snapshotId, setSnapshotId] = useState(defaultSnapshotId);
  const [valuesJSON, setValuesJSON] = useState(sampleValues);
  const [baseId, setBaseId] = useState("");
  const [targetId, setTargetId] = useState("");
  const [approvalId, setApprovalId] = useState("approval-memory-rollback-preview");
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

  const buildApprovedRollbackPlan = async () => {
    const target = baseId || selectedBase?.id;
    if (!target) return;
    setBusy("approved-rollback");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.approvedRollbackPlan({
        namespace,
        snapshot_id: target,
        requested_by: "pack-console",
        reason: "operator approved rollback write-back preview",
        approval_id: approvalId,
        dry_run: true,
      });
      setApprovedRollbackPlan(res.plan);
      showToast("已生成 approved rollback write-back plan（未写 Ledger）", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 approved rollback write-back plan 失败"));
    } finally {
      setBusy(null);
    }
  };

  const buildRetentionPlan = async () => {
    setBusy("retention");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.retentionPlan(namespace);
      setRetentionPlan(res.plan);
      showToast(
        res.plan.candidate_count > 0 ? `发现 ${res.plan.candidate_count} 个可清理候选（dry-run）` : "当前策略下无需清理快照",
        "success",
      );
    } catch (e) {
      setError(formatErrorMessage(e, "生成 retention dry-run plan 失败"));
    } finally {
      setBusy(null);
    }
  };

  const buildRetentionPrunePlan = async () => {
    setBusy("retention-prune");
    setError(null);
    try {
      const candidateIds = retentionPlan?.candidates?.map((item) => item.id) || [];
      const res = await memoryTimeTravelPack.retentionPrunePlan({
        namespace,
        candidate_ids: candidateIds,
        requested_by: "pack-console",
        reason: "operator dry-run approval preview",
        dry_run: true,
      });
      setRetentionPrunePlan(res.plan);
      showToast(
        res.plan.selected_candidate_count > 0 ? `已生成 ${res.plan.selected_candidate_count} 个候选的审批计划` : "当前没有可审批清理候选",
        "success",
      );
    } catch (e) {
      setError(formatErrorMessage(e, "生成 retention prune approval plan 失败"));
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

  const loadAuditLinks = async () => {
    setBusy("audit-links");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.auditLinks(namespace);
      setAuditLinks(res.links);
      showToast("已读取 KV audit proof-link schema 占位", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "读取 KV audit proof-link schema 失败"));
    } finally {
      setBusy(null);
    }
  };

  const buildNativeKVHistoryPlan = async () => {
    setBusy("native-kv-history");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.nativeKVHistoryPlan(namespace);
      setNativeKVHistoryPlan(res.plan);
      showToast("已生成 Native kv_history plan（未迁移、未写表）", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 native kv_history plan 失败"));
    } finally {
      setBusy(null);
    }
  };

  const previewNativeKVHistoryMigration = async () => {
    setBusy("native-kv-history-preview");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.nativeKVHistoryMigrationPreview(namespace, 50);
      setNativeKVHistoryPreview(res.kv_history_migration_preview);
      showToast("已生成 Native kv_history migration preview（未迁移、未写表）", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 native kv_history migration preview 失败"));
    } finally {
      setBusy(null);
    }
  };

  const buildKVHistoryCutoverPlan = async () => {
    setBusy("kv-history-cutover");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.kvHistoryCutoverPlan({
        namespace,
        requested_by: "pack-console",
        reason: "operator dual-read/write cutover review",
        limit: 50,
        dry_run: true,
      });
      setKVHistoryCutoverPlan(res.plan);
      showToast("已生成 kv_history cutover plan（未切换 adapter、未写 Ledger）", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 kv_history cutover plan 失败"));
    } finally {
      setBusy(null);
    }
  };

  const runKVHistoryDualReadParity = async () => {
    setBusy("kv-history-dual-read-parity");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.kvHistoryDualReadParity({
        namespace,
        at,
        limit: 500,
      });
      setKVHistoryDualReadParity(res.parity);
      showToast(
        res.parity.parity_passed ? "dual-read parity 已通过（仍未切换 adapter）" : `dual-read parity 未通过：${res.parity.status}`,
        res.parity.parity_passed ? "success" : "warning",
      );
    } catch (e) {
      setError(formatErrorMessage(e, "运行 dual-read parity gate 失败"));
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
              当前切片完成记忆快照存储、时间点回溯、漂移 diff、dry-run 回滚计划、approved rollback write-back plan、retention dry-run/prune plan、Native kv_history plan / migration preview / dual-read parity gate / cutover plan、KV audit proof-link schema 占位、证据包导出和只读 Merkle 审计链验证。原生 Ledger kv_history 表写入、adapter 切换、retention prune/cron、逐条 KV 审计证明和真实写回仍作为后续切片推进。
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
        <Card className="section-card p-4"><div className="kpi-label">Retention</div><div className="kpi-value text-lg">{status?.retention_plan_ready ? "dry-run" : "pending"}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">Native kv_history</div><div className="kpi-value text-lg">{status?.dual_read_parity_check_ready ? "parity gate" : status?.kv_history_cutover_plan_ready ? "cutover plan" : status?.native_kv_history_preview_ready ? "preview" : status?.native_kv_history_plan_ready ? "plan" : "pending"}</div></Card>
      </div>

      <Card className="section-card p-4">
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold"><UnlockKeyhole size={16} />Approved rollback write-back plan</div>
            <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
              将选中的 snapshot 映射为未来 Ledger KV versioned put 与全局 Approval Manager 请求形态；当前只输出 approved-rollback-plan.json / rollback-writeback-plan.json / approval-request-plan.json，不写 Ledger、不追加 Merkle、不修改 live memory。
            </div>
          </div>
          <Button variant="outline" isPending={busy === "approved-rollback"} onPress={buildApprovedRollbackPlan} isDisabled={!selectedBase && !baseId}><UnlockKeyhole size={14} />生成写回计划</Button>
        </div>
        <div className="mb-3 grid grid-cols-1 gap-3 md:grid-cols-3">
          <TextField value={baseId} onChange={setBaseId}><Input placeholder="target snapshot id" /></TextField>
          <TextField value={approvalId} onChange={setApprovalId}><Input placeholder="approval id / request key" /></TextField>
          <div className="rounded-xl border p-3 text-xs" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
            writeback ready: {String(status?.rollback_writeback_ready ?? false)} · global enqueue: {String(status?.global_approval_enqueue_ready ?? false)}
          </div>
        </div>
        {approvedRollbackPlan ? (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-[280px_1fr]">
            <div className="rounded-xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
              <Chip size="sm" style={{ background: approvedRollbackPlan.rollback_writeback_ready ? "rgba(34,197,94,0.12)" : "rgba(250,204,21,0.12)", color: approvedRollbackPlan.rollback_writeback_ready ? "#22c55e" : "#facc15" }}>{approvedRollbackPlan.status}</Chip>
              <div className="mt-3 text-2xl font-semibold">{approvedRollbackPlan.action_count}</div>
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>writeback actions · approval {approvedRollbackPlan.approval_required ? "required" : "not required"}</div>
              <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>Ledger writes {approvedRollbackPlan.writes_ledger_kv ? "enabled" : "blocked"} · Merkle {approvedRollbackPlan.merkle_append_ready ? "ready" : "blocked"}</div>
            </div>
            <TextField value={JSON.stringify(approvedRollbackPlan, null, 2)} onChange={() => undefined}>
              <TextArea rows={10} aria-label="Memory Time Travel approved rollback writeback plan JSON" className="font-mono text-xs" readOnly />
            </TextField>
          </div>
        ) : (
          <div className="rounded-xl border border-dashed p-4 text-sm" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
            尚未生成 approved rollback write-back plan。该入口只塑形审批与写回契约，用于后续接入真实 Approval Manager + Ledger KV executor。
          </div>
        )}
      </Card>

      <Card className="section-card p-4">
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold"><DatabaseZap size={16} />Native kv_history plan</div>
            <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
              从当前 reserved `__kv_history__` TemporalKV adapter 推导未来原生 Ledger `kv_history` 表、索引、migration plan、row preview、dual-read parity gate 和 dual-read/dual-write cutover plan；只输出 native-kv-history-plan.json / kv-history-migration-plan.json / kv-history-index-plan.json / kv-history-migration-preview.json / kv-history-dual-read-parity.json / kv-history-cutover-plan.json / kv-history-dual-read-plan.json / kv-history-dual-write-plan.json，不建表、不迁移、不写 native rows、不切换 adapter。
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" isPending={busy === "native-kv-history"} onPress={buildNativeKVHistoryPlan}><DatabaseZap size={14} />生成 native 计划</Button>
            <Button variant="outline" isPending={busy === "native-kv-history-preview"} onPress={previewNativeKVHistoryMigration}><DatabaseZap size={14} />预览迁移行</Button>
            <Button variant="outline" isPending={busy === "kv-history-dual-read-parity"} onPress={runKVHistoryDualReadParity}><GitCompare size={14} />运行 parity gate</Button>
            <Button variant="outline" isPending={busy === "kv-history-cutover"} onPress={buildKVHistoryCutoverPlan}><DatabaseZap size={14} />生成 cutover 计划</Button>
          </div>
        </div>
        {nativeKVHistoryPlan ? (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
              <Chip size="sm" style={{ background: nativeKVHistoryPlan.native_kv_history_ready ? "rgba(34,197,94,0.12)" : "rgba(250,204,21,0.12)", color: nativeKVHistoryPlan.native_kv_history_ready ? "#22c55e" : "#facc15" }}>{nativeKVHistoryPlan.status}</Chip>
              <div className="mt-3 text-2xl font-semibold">{nativeKVHistoryPlan.schema_plan.length}</div>
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>columns · indexes {nativeKVHistoryPlan.kv_history_index_plan.length}</div>
              <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>writes_native_kv_history {String(nativeKVHistoryPlan.writes_native_kv_history)} · migrates {String(nativeKVHistoryPlan.migrates_kv_history)}</div>
            </div>
            <TextField value={JSON.stringify(nativeKVHistoryPlan, null, 2)} onChange={() => undefined}>
              <TextArea rows={10} aria-label="Memory Time Travel native kv_history plan JSON" className="font-mono text-xs" readOnly />
            </TextField>
          </div>
        ) : (
          <div className="rounded-xl border border-dashed p-4 text-sm" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
            尚未生成 Native kv_history plan。该入口用于把当前 `__kv_history__` 适配层升级路径固化成可审计契约，真实 Ledger schema migration 会在后续切片单独接入。
          </div>
        )}
        {nativeKVHistoryPreview && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
              <Chip size="sm" style={{ background: nativeKVHistoryPreview.native_kv_history_preview_ready ? "rgba(56,189,248,0.12)" : "rgba(250,204,21,0.12)", color: nativeKVHistoryPreview.native_kv_history_preview_ready ? "#38bdf8" : "#facc15" }}>
                {nativeKVHistoryPreview.status || "preview"}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{nativeKVHistoryPreview.returned_row_count}</div>
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>returned rows · total {nativeKVHistoryPreview.preview_row_count}</div>
              <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>documents {nativeKVHistoryPreview.scanned_document_count} · writes {String(nativeKVHistoryPreview.writes_native_kv_history)}</div>
            </div>
            <TextField value={JSON.stringify(nativeKVHistoryPreview, null, 2)} onChange={() => undefined}>
              <TextArea rows={10} aria-label="Memory Time Travel native kv_history migration preview JSON" className="font-mono text-xs" readOnly />
            </TextField>
          </div>
        )}
        {kvHistoryDualReadParity && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
              <Chip size="sm" style={{ background: kvHistoryDualReadParity.parity_passed ? "rgba(34,197,94,0.12)" : "rgba(250,204,21,0.12)", color: kvHistoryDualReadParity.parity_passed ? "#22c55e" : "#facc15" }}>
                {kvHistoryDualReadParity.status}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{kvHistoryDualReadParity.mismatch_count}</div>
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>mismatches · matched {kvHistoryDualReadParity.matched_key_count}</div>
              <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>dual_read_parity_ready {String(kvHistoryDualReadParity.dual_read_parity_ready)}</div>
              <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>switches adapter {String(kvHistoryDualReadParity.switches_temporal_adapter)} · writes Ledger {String(kvHistoryDualReadParity.writes_ledger_kv)}</div>
            </div>
            <TextField value={JSON.stringify(kvHistoryDualReadParity, null, 2)} onChange={() => undefined}>
              <TextArea rows={10} aria-label="Memory Time Travel kv_history dual-read parity JSON" className="font-mono text-xs" readOnly />
            </TextField>
          </div>
        )}
        {kvHistoryCutoverPlan && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
              <Chip size="sm" style={{ background: kvHistoryCutoverPlan.cutover_ready ? "rgba(34,197,94,0.12)" : "rgba(250,204,21,0.12)", color: kvHistoryCutoverPlan.cutover_ready ? "#22c55e" : "#facc15" }}>
                {kvHistoryCutoverPlan.status}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{kvHistoryCutoverPlan.phases.length}</div>
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>cutover phases · preview rows {kvHistoryCutoverPlan.returned_preview_row_count}</div>
              <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>dual_read_ready {String(kvHistoryCutoverPlan.dual_read_ready)} · dual_write_ready {String(kvHistoryCutoverPlan.dual_write_ready)}</div>
              <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>switches adapter {String(kvHistoryCutoverPlan.switches_temporal_adapter)} · writes Ledger {String(kvHistoryCutoverPlan.dual_write_plan.writes_ledger_kv)}</div>
            </div>
            <TextField value={JSON.stringify(kvHistoryCutoverPlan, null, 2)} onChange={() => undefined}>
              <TextArea rows={10} aria-label="Memory Time Travel kv_history cutover plan JSON" className="font-mono text-xs" readOnly />
            </TextField>
          </div>
        )}
      </Card>

      <Card className="section-card p-4">
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold"><Trash2 size={16} />Retention dry-run plan</div>
            <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
              只计算 pack-local snapshots 的保留策略、候选清理动作和审批计划，不删除文件，也不清理 Ledger temporal KV。后续再接 native kv_history purge 与 cron。
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" isPending={busy === "retention"} onPress={buildRetentionPlan}><Trash2 size={14} />生成保留计划</Button>
            <Button variant="outline" isPending={busy === "retention-prune"} onPress={buildRetentionPrunePlan}><ShieldCheck size={14} />生成审批计划</Button>
          </div>
        </div>
        {retentionPlan ? (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
              <Chip size="sm" style={{ background: "rgba(56,189,248,0.12)", color: "#38bdf8" }}>{retentionPlan.status}</Chip>
              <div className="mt-3 text-2xl font-semibold">{retentionPlan.candidate_count}</div>
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>candidates · keep {retentionPlan.keep_count}/{retentionPlan.snapshot_count}</div>
              <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>reclaimable {retentionPlan.reclaimable_bytes} bytes · prune {retentionPlan.temporal_prune_ready ? "ready" : "not wired"}</div>
            </div>
            <TextField value={JSON.stringify(retentionPlan, null, 2)} onChange={() => undefined}>
              <TextArea rows={8} aria-label="Memory Time Travel retention plan JSON" className="font-mono text-xs" readOnly />
            </TextField>
          </div>
        ) : (
          <div className="rounded-xl border border-dashed p-4 text-sm" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
            尚未生成保留计划。此入口当前只做 dry-run，用于上线前确认策略不会误删记忆快照。
          </div>
        )}
        {retentionPrunePlan && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
              <Chip size="sm" style={{ background: "rgba(250,204,21,0.12)", color: "#facc15" }}>{retentionPrunePlan.status}</Chip>
              <div className="mt-3 text-2xl font-semibold">{retentionPrunePlan.selected_candidate_count}</div>
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>selected · approval {retentionPrunePlan.approval_required ? "required" : "not required"}</div>
              <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>prune {retentionPrunePlan.prune_ready ? "ready" : "blocked"} · reclaimable {retentionPrunePlan.reclaimable_bytes} bytes</div>
            </div>
            <TextField value={JSON.stringify(retentionPrunePlan, null, 2)} onChange={() => undefined}>
              <TextArea rows={8} aria-label="Memory Time Travel retention prune plan JSON" className="font-mono text-xs" readOnly />
            </TextField>
          </div>
        )}
      </Card>

      <Card className="section-card p-4">
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold"><Link2 size={16} />KV audit proof-link schema</div>
            <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
              先暴露逐条 KV 证明链接的稳定 schema，用于后续原生 Ledger kv_history 与 Merkle audit record 关联；当前不会声称已有 per-KV proof。
            </div>
          </div>
          <Button variant="outline" isPending={busy === "audit-links"} onPress={loadAuditLinks}><Link2 size={14} />读取 schema</Button>
        </div>
        {auditLinks ? (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-[240px_1fr]">
            <div className="rounded-xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
              <Chip size="sm" style={{ background: "rgba(56,189,248,0.12)", color: "#38bdf8" }}>{auditLinks.schema_ready ? "schema ready" : "pending"}</Chip>
              <div className="mt-3 text-2xl font-semibold">{auditLinks.kv_audit_links.length}</div>
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>links · linkage {auditLinks.linkage_ready ? "ready" : "not wired"}</div>
              <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>native kv_history {auditLinks.native_kv_history_ready ? "ready" : "not wired"}</div>
            </div>
            <TextField value={JSON.stringify(auditLinks, null, 2)} onChange={() => undefined}>
              <TextArea rows={8} aria-label="Memory Time Travel KV audit links JSON" className="font-mono text-xs" readOnly />
            </TextField>
          </div>
        ) : (
          <div className="rounded-xl border border-dashed p-4 text-sm" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
            尚未读取 KV audit proof-link schema。这个入口当前只返回 schema/空 links，为后续 native kv_history + Merkle 证明关联预留稳定契约。
          </div>
        )}
      </Card>

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
