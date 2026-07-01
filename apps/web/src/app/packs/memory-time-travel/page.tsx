"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Card, Chip, Input, Label, ListBox, Select, Spinner, TextArea, TextField } from "@heroui/react";
import { AlertTriangle, Clock3, DatabaseZap, Download, GitCompare, History, Link2, RefreshCw, RotateCcw, Save, ShieldCheck, Trash2, UnlockKeyhole } from "lucide-react";
import PageHeader from "@/components/page-header";
import ReadinessBadges from "@/components/readiness-badges";
import { showToast } from "@/components/toast-provider";
import { confirmAction } from "@/components/confirm-dialog";
import { JsonViewer } from "@/components/json-viewer";
import { formatErrorMessage } from "@/lib/error-utils";
import { createMemoryTimeTravelClient as createMemoryTimeTravelPackClient, type MemoryTimeTravelApprovedRollbackPlan, type MemoryTimeTravelAuditVerificationResponse as MemoryTimeTravelAuditVerification, type MemoryTimeTravelDiffReport, type MemoryTimeTravelKVAuditLinksResponse as MemoryTimeTravelKVAuditLinksReport, type MemoryTimeTravelKVAuditProofLinkPreview, type MemoryTimeTravelKVAuditProofLinkWritebackPlan, type MemoryTimeTravelKVAuditProofLinkWritebackStore, type MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlan, type MemoryTimeTravelKVHistoryCutoverPlan, type MemoryTimeTravelKVHistoryCutoverReadiness, type MemoryTimeTravelKVHistoryDualReadParity, type MemoryTimeTravelNativeKVHistoryMigrationPreview, type MemoryTimeTravelNativeKVHistoryPlan, type MemoryTimeTravelRetentionPlan, type MemoryTimeTravelRetentionPruneExecute, type MemoryTimeTravelRetentionPrunePlan, type MemoryTimeTravelRollbackWritebackExecutorPlan, type MemoryTimeTravelRollbackWritebackStore, type MemoryTimeTravelSnapshotAtResponse, type MemoryTimeTravelSnapshotSummary, type MemoryTimeTravelStatusResponse as MemoryTimeTravelStatus } from "yunque-client/memory-time-travel";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { PackAbout, type PackBoundaryItem, type PackStep } from "@/components/packs/pack-page-kit";

const memoryTimeTravelPack = createMemoryTimeTravelPackClient(createYunqueSDKClientOptions());

type ChipColor = "success" | "warning" | "danger" | "default";

function riskColor(risk?: string): ChipColor {
  switch (risk) {
    case "high": return "danger";
    case "medium": return "warning";
    case "low": return "default";
    default: return "success";
  }
}

function defaultSnapshotId() {
  return `memory-${new Date().toISOString().slice(0, 16).replace(/[-:T]/g, "")}`;
}

function sampleValues() {
  return JSON.stringify({
    goal: "继续推进 Pack Runtime 轻内核 + 可选能力包",
    persona: "谨慎、可回滚、验证后提交",
    memory_layer: "long",
  }, null, 2);
}

const userFacingSteps: PackStep[] = [
  { key: "save", label: "保存记忆快照", detail: "把当前记忆键值保存成可对比的 baseline，后续可以回看每次变化。" },
  { key: "diff", label: "对比漂移与回溯", detail: "选择两个快照或一个时间点，查看哪些记忆新增、修改、删除。" },
  { key: "evidence", label: "生成回滚证据", detail: "输出 dry-run 回滚计划、审计证明和迁移交接材料，供人工确认。" },
];

const boundaryItems: PackBoundaryItem[] = [
  { key: "live", label: "不改 live memory", detail: "不会直接修改 live memory 或 Ledger KV。", tone: "warning" },
  { key: "adapter", label: "不切 adapter", detail: "不会自动切换 kv_history adapter 或执行 schema 迁移。", tone: "warning" },
  { key: "rollback", label: "不跳审批", detail: "不会跳过审批执行真实回滚。", tone: "warning" },
  { key: "proof", label: "不冒充证明", detail: "不会把审计证明预览当成已写入的 Merkle 证明。", tone: "warning" },
];

const commonNamespaces = ["memory_snapshot", "persona_memory", "knowledge_base", "chat_context"];

export default function MemoryTimeTravelPackPage() {
  const [status, setStatus] = useState<MemoryTimeTravelStatus | null>(null);
  const [auditVerification, setAuditVerification] = useState<MemoryTimeTravelAuditVerification | null>(null);
  const [auditLinks, setAuditLinks] = useState<MemoryTimeTravelKVAuditLinksReport | null>(null);
  const [auditLinkPreview, setAuditLinkPreview] = useState<MemoryTimeTravelKVAuditProofLinkPreview | null>(null);
  const [auditLinkWritebackPlan, setAuditLinkWritebackPlan] = useState<MemoryTimeTravelKVAuditProofLinkWritebackPlan | null>(null);
  const [auditLinkWritebackStore, setAuditLinkWritebackStore] = useState<MemoryTimeTravelKVAuditProofLinkWritebackStore | null>(null);
  const [auditLinkWritebackExecutorPlan, setAuditLinkWritebackExecutorPlan] = useState<MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlan | null>(null);
  const [nativeKVHistoryPlan, setNativeKVHistoryPlan] = useState<MemoryTimeTravelNativeKVHistoryPlan | null>(null);
  const [nativeKVHistoryPreview, setNativeKVHistoryPreview] = useState<MemoryTimeTravelNativeKVHistoryMigrationPreview | null>(null);
  const [kvHistoryDualReadParity, setKVHistoryDualReadParity] = useState<MemoryTimeTravelKVHistoryDualReadParity | null>(null);
  const [kvHistoryCutoverPlan, setKVHistoryCutoverPlan] = useState<MemoryTimeTravelKVHistoryCutoverPlan | null>(null);
  const [kvHistoryCutoverReadiness, setKVHistoryCutoverReadiness] = useState<MemoryTimeTravelKVHistoryCutoverReadiness | null>(null);
  const [approvedRollbackPlan, setApprovedRollbackPlan] = useState<MemoryTimeTravelApprovedRollbackPlan | null>(null);
  const [rollbackWritebackStore, setRollbackWritebackStore] = useState<MemoryTimeTravelRollbackWritebackStore | null>(null);
  const [rollbackWritebackExecutorPlan, setRollbackWritebackExecutorPlan] = useState<MemoryTimeTravelRollbackWritebackExecutorPlan | null>(null);
  const [retentionPlan, setRetentionPlan] = useState<MemoryTimeTravelRetentionPlan | null>(null);
  const [retentionPrunePlan, setRetentionPrunePlan] = useState<MemoryTimeTravelRetentionPrunePlan | null>(null);
  const [retentionPruneExecute, setRetentionPruneExecute] = useState<MemoryTimeTravelRetentionPruneExecute | null>(null);
  const [snapshots, setSnapshots] = useState<MemoryTimeTravelSnapshotSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"save" | "snapshot-at" | "diff" | "rollback" | "approved-rollback" | "rollback-writeback-store" | "rollback-writeback-executor" | "evidence" | "audit" | "audit-links" | "audit-link-preview" | "audit-link-writeback" | "audit-link-writeback-store" | "audit-link-writeback-executor" | "native-kv-history" | "native-kv-history-preview" | "kv-history-dual-read-parity" | "kv-history-cutover" | "kv-history-cutover-readiness" | "retention" | "retention-prune" | "retention-prune-execute" | null>(null);
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
  const namespaceOptions = useMemo(() => {
    const values = [namespace, ...commonNamespaces, ...snapshots.map((item) => item.namespace)];
    return [...new Set(values.map((item) => item.trim()).filter(Boolean))];
  }, [namespace, snapshots]);
  const diffRiskColor = riskColor(diff?.risk_level);

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
      setError(msg.includes("pack route is not enabled") ? "Memory Time Travel Pack 当前未启用。请到「能力包」控制台启用 yunque.pack.memory-time-travel 后再使用。" : msg);
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

  const writeRollbackWritebackStore = async () => {
    const target = baseId || selectedBase?.id;
    if (!target) return;
    setBusy("rollback-writeback-store");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.rollbackWritebackStore({
        namespace,
        snapshot_id: target,
        requested_by: "pack-console",
        reason: "persist approved rollback write-back handoff",
        approval_id: approvalId,
        dry_run: true,
      });
      setRollbackWritebackStore(res.writeback);
      showToast("已写入 rollback-writeback-store.json（仅 pack-local handoff）", "success");
      await load();
    } catch (e) {
      setError(formatErrorMessage(e, "写入 rollback write-back handoff store 失败"));
    } finally {
      setBusy(null);
    }
  };

  const buildRollbackWritebackExecutorPlan = async () => {
    setBusy("rollback-writeback-executor");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.rollbackWritebackExecutorPlan({
        request_key: approvalId,
        namespace,
        snapshot_id: baseId || selectedBase?.id,
        requested_by: "pack-console",
        reason: "plan rollback executor handoff from pack-local store",
        dry_run: true,
      });
      setRollbackWritebackExecutorPlan(res.plan);
      showToast("已生成 rollback executor handoff plan（未写 Ledger）", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 rollback executor handoff plan 失败"));
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

  const executePackLocalRetentionPrune = async () => {
    const candidateIds = retentionPrunePlan?.candidates?.map((item) => item.id) || retentionPlan?.candidates?.map((item) => item.id) || [];
    if (candidateIds.length === 0) {
      showToast("当前没有可清理的 pack-local 快照候选。", "info");
      return;
    }
    const confirmed = await confirmAction({
      title: "执行 pack-local 快照清理？",
      body: `将删除 ${candidateIds.length} 个 pack-local snapshot 候选。此操作只限本地快照目录，但删除后这些快照无法从界面恢复。`,
      confirmLabel: "删除快照",
      cancelLabel: "取消",
      tone: "danger",
    });
    if (!confirmed) return;
    setBusy("retention-prune-execute");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.retentionPruneExecute({
        namespace,
        candidate_ids: candidateIds,
        requested_by: "pack-console",
        reason: "approved pack-local snapshot retention cleanup",
        approval_id: "approval-pack-local-retention-prune",
        approved: true,
      });
      setRetentionPruneExecute(res.prune);
      showToast(
        res.prune.deleted_candidate_count > 0 ? `已删除 ${res.prune.deleted_candidate_count} 个 pack-local 快照` : "没有删除快照，请查看 blocked_by / skipped_candidates",
        res.prune.deleted_candidate_count > 0 ? "success" : "info",
      );
      await load();
    } catch (e) {
      setError(formatErrorMessage(e, "执行 pack-local retention prune 失败"));
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

  const previewAuditLinks = async () => {
    setBusy("audit-link-preview");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.auditLinksPreview({
        namespace,
        at,
        limit: 50,
        dry_run: true,
      });
      setAuditLinkPreview(res.preview);
      showToast(
        `已生成 KV audit proof-link preview（匹配 ${res.preview.matched_link_count}/${res.preview.candidate_link_count}，未写 Ledger）`,
        res.preview.matched_link_count > 0 ? "success" : "info",
      );
    } catch (e) {
      setError(formatErrorMessage(e, "生成 KV audit proof-link preview 失败"));
    } finally {
      setBusy(null);
    }
  };

  const buildAuditLinkWritebackPlan = async () => {
    setBusy("audit-link-writeback");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.auditLinksWritebackPlan({
        namespace,
        at,
        limit: 50,
        requested_by: "pack-console",
        reason: "operator audit proof-link write-back review",
        approval_id: `${approvalId}-audit-link`,
        dry_run: true,
      });
      setAuditLinkWritebackPlan(res.plan);
      showToast(
        `已生成 KV audit proof-link writeback plan（动作 ${res.plan.action_count}，仍未写 Ledger）`,
        res.plan.action_count > 0 ? "success" : "info",
      );
    } catch (e) {
      setError(formatErrorMessage(e, "生成 KV audit proof-link writeback plan 失败"));
    } finally {
      setBusy(null);
    }
  };

  const writeAuditLinkWritebackStore = async () => {
    setBusy("audit-link-writeback-store");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.auditLinksWritebackStore({
        namespace,
        at,
        limit: 50,
        requested_by: "pack-console",
        reason: "operator audit proof-link handoff store review",
        approval_id: `${approvalId}-audit-link`,
        dry_run: true,
      });
      setAuditLinkWritebackStore(res.writeback);
      showToast(
        `已写入 pack-local handoff store（记录 ${res.writeback.audit_link_writeback_store.record_count}，未写 native/Ledger）`,
        "success",
      );
    } catch (e) {
      setError(formatErrorMessage(e, "写入 KV audit proof-link handoff store 失败"));
    } finally {
      setBusy(null);
    }
  };

  const buildAuditLinkWritebackExecutorPlan = async () => {
    setBusy("audit-link-writeback-executor");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.auditLinksWritebackExecutorPlan({
        namespace,
        request_key: `${approvalId}-audit-link`,
        requested_by: "pack-console",
        reason: "operator audit proof-link executor handoff review",
        dry_run: true,
      });
      setAuditLinkWritebackExecutorPlan(res.plan);
      showToast(
        `已生成 executor handoff plan（动作 ${res.plan.action_count}，仍未写 native/Ledger/Merkle）`,
        "success",
      );
    } catch (e) {
      setError(formatErrorMessage(e, "生成 KV audit proof-link executor handoff plan 失败，请先写入 handoff store"));
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

  const runKVHistoryCutoverReadiness = async () => {
    setBusy("kv-history-cutover-readiness");
    setError(null);
    try {
      const res = await memoryTimeTravelPack.kvHistoryCutoverReadiness({
        namespace,
        at,
        requested_by: "pack-console",
        reason: "operator cutover readiness gate review",
        limit: 500,
        dry_run: true,
      });
      setKVHistoryCutoverReadiness(res.readiness);
      showToast(
        res.readiness.cutover_ready ? "cutover readiness 已通过" : `cutover readiness 仍阻断：${res.readiness.blocked_gate_count} 个 gate`,
        res.readiness.cutover_ready ? "success" : "warning",
      );
    } catch (e) {
      setError(formatErrorMessage(e, "运行 kv_history cutover readiness gate 失败"));
    } finally {
      setBusy(null);
    }
  };

  if (loading) {
    return <div className="flex h-[60vh] items-center justify-center"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader
        icon={<History size={20} />}
        title="Memory Time Travel"
        actions={<Button size="sm" variant="ghost" onPress={load}><RefreshCw size={14} />刷新</Button>}
      />

      <PackAbout
        chips={<>
          <Chip size="sm" color="warning">实验中</Chip>
          <Chip size="sm" variant="soft">可保存快照</Chip>
          <Chip size="sm" variant="soft">回滚只生成计划</Chip>
        </>}
        description="给云雀的记忆做版本快照、时间点回看和漂移对比。当前可以保存 pack-local 快照、生成 diff、导出证据包和 dry-run 回滚计划；真实 Ledger 写回、自动回滚、原生 kv_history 切换与定时清理仍需要人工审批和后续接入。"
        boundaries={boundaryItems}
      />

      <div className="grid gap-3 md:grid-cols-3">
        {userFacingSteps.map((item, idx) => (
          <div key={item.key} className="rounded-xl border border-border bg-surface-secondary p-4">
            <div className="text-sm font-medium text-foreground">{idx + 1}. {item.label}</div>
            <div className="mt-2 text-xs leading-5 text-muted">{item.detail}</div>
          </div>
        ))}
      </div>

      <Card variant="default">
        <Card.Content className="p-4">
          <div className="mb-3 text-sm font-semibold text-foreground">技术状态</div>
          <div className="mb-1 flex items-center gap-2">
            <Chip size="sm" color={status?.ledger_history_ready ? "success" : "warning"}>
              {status?.ledger_history_ready ? "Ledger history ready" : "Pack shell"}
            </Chip>
            <span className="text-xs text-muted">{status?.pack_id || "yunque.pack.memory-time-travel"}</span>
          </div>
          <div className="text-sm text-muted">
            当前切片完成记忆快照存储、时间点回溯、漂移 diff、dry-run 回滚计划、approved rollback write-back plan、pack-local rollback writeback store / executor handoff plan、retention dry-run/prune plan、pack-local retention prune executor、Native kv_history plan / migration preview / dual-read parity gate / cutover readiness gate / cutover plan、KV audit proof-link schema / proof-link preview gate / proof-link writeback plan / pack-local handoff store / executor handoff plan、证据包导出和只读 Merkle 审计链验证。原生 Ledger kv_history 表写入、adapter 切换、Ledger temporal prune/cron、逐条 KV 审计证明真实写回和真实回滚执行仍作为后续切片推进。
          </div>
        </Card.Content>
      </Card>

      {error && (
        <Card variant="secondary">
          <Card.Content className="flex items-center gap-2 p-4 text-sm text-danger"><AlertTriangle size={16} />{error}</Card.Content>
        </Card>
      )}

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Card className="section-card p-4"><div className="kpi-label">快照数量</div><div className="kpi-value">{status?.snapshot_count ?? snapshots.length}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">命名空间</div><div className="kpi-value">{status?.namespace_count ?? 0}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">阶段</div><div className="mt-1 text-sm font-medium text-foreground">{status?.stage || "pack-shell"}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">能力数</div><div className="kpi-value">{status?.capabilities?.length ?? 0}</div></Card>
      </div>

      <Card className="section-card p-4">
        <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
          <ShieldCheck size={16} className="text-accent" />
          就绪状态
        </div>
        <p className="mt-1 text-xs text-muted">
          各回溯 / 保留 / Native kv_history 环节的就绪情况 —— 绿色表示已就绪，灰色表示仍待接通。
        </p>
        <div className="mt-3">
          <ReadinessBadges
            flags={[
              { label: "回溯计划", hint: "时间点回溯 dry-run 计划就绪", ready: status?.approved_rollback_plan_ready },
              { label: "写回计划", hint: "approved rollback write-back plan 就绪", ready: status?.rollback_writeback_plan_ready },
              { label: "Retention dry-run", hint: "retention dry-run 计划就绪", ready: status?.retention_plan_ready },
              { label: "Retention prune", hint: "pack-local retention prune executor 就绪", ready: status?.retention_pack_local_prune_ready },
              { label: "kv_history 计划", hint: "Native kv_history plan 就绪", ready: status?.native_kv_history_plan_ready },
              { label: "迁移预览", hint: "native kv_history migration preview 就绪", ready: status?.native_kv_history_preview_ready },
              { label: "parity gate", hint: "dual-read parity gate 就绪", ready: status?.dual_read_parity_check_ready },
              { label: "cutover", hint: "cutover readiness / plan 就绪", ready: status?.kv_history_cutover_readiness_ready || status?.kv_history_cutover_plan_ready },
            ]}
          />
        </div>
      </Card>

      <Card className="section-card p-4">
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold"><UnlockKeyhole size={16} />Approved rollback write-back plan</div>
            <div className="mt-1 text-xs text-muted">
              将选中的 snapshot 映射为未来 Ledger KV versioned put 与全局 Approval Manager 请求形态；当前先输出 approved-rollback-plan.json / rollback-writeback-plan.json / approval-request-plan.json，现在也可显式写入 pack-local rollback-writeback-store.json / rollback-writeback-record.json，并生成 rollback-writeback-executor-plan.json / rollback-executor-handoff-plan.json / rollback-executor-audit-plan.json。仍不写 Ledger、不追加 Merkle、不修改 live memory。
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" isPending={busy === "approved-rollback"} onPress={buildApprovedRollbackPlan} isDisabled={!selectedBase && !baseId}><UnlockKeyhole size={14} />生成写回计划</Button>
            <Button variant="outline" isPending={busy === "rollback-writeback-store"} onPress={writeRollbackWritebackStore} isDisabled={!selectedBase && !baseId}><Save size={14} />写入 handoff store</Button>
            <Button variant="outline" isPending={busy === "rollback-writeback-executor"} onPress={buildRollbackWritebackExecutorPlan}><DatabaseZap size={14} />生成 executor handoff plan</Button>
          </div>
        </div>
        <div className="mb-3 grid grid-cols-1 gap-3 md:grid-cols-3">
          <TextField value={baseId} onChange={setBaseId}>
            <Label>目标快照 ID（回滚到）</Label>
            <Input placeholder="target snapshot id" />
          </TextField>
          <TextField value={approvalId} onChange={setApprovalId}>
            <Label>审批 ID / 请求键</Label>
            <Input placeholder="approval id / request key" />
          </TextField>
          <div className="rounded-xl border border-border p-3 text-xs text-muted">
            store ready: {String(status?.rollback_writeback_store_ready ?? false)} · executor plan: {String(status?.rollback_writeback_executor_plan_ready ?? false)} · writeback ready: {String(status?.rollback_writeback_ready ?? false)}
          </div>
        </div>
        {approvedRollbackPlan ? (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-[280px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color={approvedRollbackPlan.rollback_writeback_ready ? "success" : "warning"}>{approvedRollbackPlan.status}</Chip>
              <div className="mt-3 text-2xl font-semibold">{approvedRollbackPlan.action_count}</div>
              <div className="text-xs text-muted">writeback actions · approval {approvedRollbackPlan.approval_required ? "required" : "not required"}</div>
              <div className="mt-2 text-xs text-muted">Ledger writes {approvedRollbackPlan.writes_ledger_kv ? "enabled" : "blocked"} · Merkle {approvedRollbackPlan.merkle_append_ready ? "ready" : "blocked"}</div>
            </div>
            <JsonViewer title="已审批回滚计划 JSON" value={approvedRollbackPlan} rows={10} />
          </div>
        ) : (
          <div className="rounded-xl border border-dashed border-border p-4 text-sm text-muted">
            尚未生成 approved rollback write-back plan。该入口只塑形审批与写回契约，用于后续接入真实 Approval Manager + Ledger KV executor。
          </div>
        )}
      </Card>

      <Card className="section-card p-4">
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold"><DatabaseZap size={16} />Native kv_history plan</div>
            <div className="mt-1 text-xs text-muted">
              从当前 reserved `__kv_history__` TemporalKV adapter 推导未来原生 Ledger `kv_history` 表、索引、migration plan、row preview、dual-read parity gate、cutover readiness gate 和 dual-read/dual-write cutover plan；只输出 native-kv-history-plan.json / kv-history-migration-plan.json / kv-history-index-plan.json / kv-history-migration-preview.json / kv-history-dual-read-parity.json / kv-history-cutover-readiness.json / kv-history-cutover-plan.json / kv-history-dual-read-plan.json / kv-history-dual-write-plan.json，不建表、不迁移、不写 native rows、不切换 adapter。
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" isPending={busy === "native-kv-history"} onPress={buildNativeKVHistoryPlan}><DatabaseZap size={14} />生成 native 计划</Button>
            <Button variant="outline" isPending={busy === "native-kv-history-preview"} onPress={previewNativeKVHistoryMigration}><DatabaseZap size={14} />预览迁移行</Button>
            <Button variant="outline" isPending={busy === "kv-history-dual-read-parity"} onPress={runKVHistoryDualReadParity}><GitCompare size={14} />运行 parity gate</Button>
            <Button variant="outline" isPending={busy === "kv-history-cutover-readiness"} onPress={runKVHistoryCutoverReadiness}><ShieldCheck size={14} />运行 readiness gate</Button>
            <Button variant="outline" isPending={busy === "kv-history-cutover"} onPress={buildKVHistoryCutoverPlan}><DatabaseZap size={14} />生成 cutover 计划</Button>
          </div>
        </div>
        {nativeKVHistoryPlan ? (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color={nativeKVHistoryPlan.native_kv_history_ready ? "success" : "warning"}>{nativeKVHistoryPlan.status}</Chip>
              <div className="mt-3 text-2xl font-semibold">{nativeKVHistoryPlan.schema_plan.length}</div>
              <div className="text-xs text-muted">columns · indexes {nativeKVHistoryPlan.kv_history_index_plan.length}</div>
              <div className="mt-2 text-xs text-muted">writes_native_kv_history {String(nativeKVHistoryPlan.writes_native_kv_history)} · migrates {String(nativeKVHistoryPlan.migrates_kv_history)}</div>
            </div>
            <JsonViewer title="Native kv_history 计划 JSON" value={nativeKVHistoryPlan} rows={10} />
          </div>
        ) : (
          <div className="rounded-xl border border-dashed border-border p-4 text-sm text-muted">
            尚未生成 Native kv_history plan。该入口用于把当前 `__kv_history__` 适配层升级路径固化成可审计契约，真实 Ledger schema migration 会在后续切片单独接入。
          </div>
        )}
        {nativeKVHistoryPreview && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color={nativeKVHistoryPreview.native_kv_history_preview_ready ? "default" : "warning"} variant="soft">
                {nativeKVHistoryPreview.status || "preview"}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{nativeKVHistoryPreview.returned_row_count}</div>
              <div className="text-xs text-muted">returned rows · total {nativeKVHistoryPreview.preview_row_count}</div>
              <div className="mt-2 text-xs text-muted">documents {nativeKVHistoryPreview.scanned_document_count} · writes {String(nativeKVHistoryPreview.writes_native_kv_history)}</div>
            </div>
            <JsonViewer title="Native kv_history 预览 JSON" value={nativeKVHistoryPreview} rows={10} />
          </div>
        )}
        {kvHistoryDualReadParity && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color={kvHistoryDualReadParity.parity_passed ? "success" : "warning"}>
                {kvHistoryDualReadParity.status}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{kvHistoryDualReadParity.mismatch_count}</div>
              <div className="text-xs text-muted">mismatches · matched {kvHistoryDualReadParity.matched_key_count}</div>
              <div className="mt-2 text-xs text-muted">dual_read_parity_ready {String(kvHistoryDualReadParity.dual_read_parity_ready)}</div>
              <div className="mt-2 text-xs text-muted">switches adapter {String(kvHistoryDualReadParity.switches_temporal_adapter)} · writes Ledger {String(kvHistoryDualReadParity.writes_ledger_kv)}</div>
            </div>
            <JsonViewer title="双读校验 JSON" value={kvHistoryDualReadParity} rows={10} />
          </div>
        )}
        {kvHistoryCutoverPlan && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color={kvHistoryCutoverPlan.cutover_ready ? "success" : "warning"}>
                {kvHistoryCutoverPlan.status}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{kvHistoryCutoverPlan.phases.length}</div>
              <div className="text-xs text-muted">cutover phases · preview rows {kvHistoryCutoverPlan.returned_preview_row_count}</div>
              <div className="mt-2 text-xs text-muted">dual_read_ready {String(kvHistoryCutoverPlan.dual_read_ready)} · dual_write_ready {String(kvHistoryCutoverPlan.dual_write_ready)}</div>
              <div className="mt-2 text-xs text-muted">switches adapter {String(kvHistoryCutoverPlan.switches_temporal_adapter)} · writes Ledger {String(kvHistoryCutoverPlan.dual_write_plan.writes_ledger_kv)}</div>
            </div>
            <JsonViewer title="Cutover 计划 JSON" value={kvHistoryCutoverPlan} rows={10} />
          </div>
        )}
        {kvHistoryCutoverReadiness && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color={kvHistoryCutoverReadiness.cutover_ready ? "success" : "warning"}>
                {kvHistoryCutoverReadiness.status}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{kvHistoryCutoverReadiness.passed_gate_count}/{kvHistoryCutoverReadiness.required_gate_count}</div>
              <div className="text-xs text-muted">passed gates · blocked {kvHistoryCutoverReadiness.blocked_gate_count}</div>
              <div className="mt-2 text-xs text-muted">cutover_ready {String(kvHistoryCutoverReadiness.cutover_ready)} · parity {String(kvHistoryCutoverReadiness.parity_passed)}</div>
              <div className="mt-2 text-xs text-muted">switches adapter {String(kvHistoryCutoverReadiness.switches_temporal_adapter)} · writes Ledger {String(kvHistoryCutoverReadiness.writes_ledger_kv)}</div>
            </div>
            <JsonViewer title="Cutover 就绪检查 JSON" value={kvHistoryCutoverReadiness} rows={10} />
          </div>
        )}
      </Card>

      <Card className="section-card p-4">
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold"><Trash2 size={16} />Retention dry-run plan</div>
            <div className="mt-1 text-xs text-muted">
              先计算 pack-local snapshots 的保留策略与审批计划；现在可在显式 approved=true 后只删除 pack-local snapshot 目录，并输出 retention-prune-execute.json。Ledger temporal KV、native kv_history、Merkle append 与 cron 仍保持阻断。
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" isPending={busy === "retention"} onPress={buildRetentionPlan}><Trash2 size={14} />生成保留计划</Button>
            <Button variant="outline" isPending={busy === "retention-prune"} onPress={buildRetentionPrunePlan}><ShieldCheck size={14} />生成审批计划</Button>
            <Button variant="outline" isPending={busy === "retention-prune-execute"} onPress={executePackLocalRetentionPrune}><Trash2 size={14} />执行 pack-local 清理</Button>
          </div>
        </div>
        {retentionPlan ? (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color="default" variant="soft">{retentionPlan.status}</Chip>
              <div className="mt-3 text-2xl font-semibold">{retentionPlan.candidate_count}</div>
              <div className="text-xs text-muted">candidates · keep {retentionPlan.keep_count}/{retentionPlan.snapshot_count}</div>
              <div className="mt-2 text-xs text-muted">reclaimable {retentionPlan.reclaimable_bytes} bytes · prune {retentionPlan.temporal_prune_ready ? "ready" : "not wired"}</div>
            </div>
            <JsonViewer title="Retention 计划 JSON" value={retentionPlan} rows={8} />
          </div>
        ) : (
          <div className="rounded-xl border border-dashed border-border p-4 text-sm text-muted">
            尚未生成保留计划。建议先 dry-run 确认候选，再执行只限 pack-local snapshot 目录的 approved 清理。
          </div>
        )}
        {retentionPrunePlan && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color="warning">{retentionPrunePlan.status}</Chip>
              <div className="mt-3 text-2xl font-semibold">{retentionPrunePlan.selected_candidate_count}</div>
              <div className="text-xs text-muted">selected · approval {retentionPrunePlan.approval_required ? "required" : "not required"}</div>
              <div className="mt-2 text-xs text-muted">prune {retentionPrunePlan.prune_ready ? "ready" : "blocked"} · reclaimable {retentionPrunePlan.reclaimable_bytes} bytes</div>
            </div>
            <JsonViewer title="Retention prune dry-run JSON" value={retentionPrunePlan} rows={8} />
          </div>
        )}
        {retentionPruneExecute && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color={retentionPruneExecute.writes_pack_local_snapshot_store ? "success" : "warning"}>
                {retentionPruneExecute.status}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{retentionPruneExecute.deleted_candidate_count}/{retentionPruneExecute.selected_candidate_count}</div>
              <div className="text-xs text-muted">deleted pack-local snapshots · skipped {retentionPruneExecute.skipped_candidate_count}</div>
              <div className="mt-2 text-xs text-muted">writes local store {String(retentionPruneExecute.writes_pack_local_snapshot_store)} · writes Ledger {String(retentionPruneExecute.writes_ledger_kv)}</div>
              <div className="mt-2 text-xs text-muted">writes native {String(retentionPruneExecute.writes_native_kv_history)} · cron {String(retentionPruneExecute.cron_ready)}</div>
            </div>
            <JsonViewer title="Retention prune execute JSON" value={retentionPruneExecute} rows={8} />
          </div>
        )}
        {rollbackWritebackStore && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color="default" variant="soft">
                pack-local rollback store
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{rollbackWritebackStore.rollback_writeback_store.record_count}</div>
              <div className="text-xs text-muted">records · status {rollbackWritebackStore.status}</div>
              <div className="mt-2 text-xs text-muted">rollback_writeback_store_ready {String(rollbackWritebackStore.rollback_writeback_store_ready)} · store writes {String(rollbackWritebackStore.writes_rollback_writeback_store)}</div>
              <div className="mt-2 text-xs text-muted">Ledger {String(rollbackWritebackStore.writes_ledger_kv)} · Temporal {String(rollbackWritebackStore.writes_temporal_kv)} · Merkle {String(rollbackWritebackStore.merkle_append_ready)}</div>
              <div className="mt-2 text-xs text-muted">artifact rollback-writeback-store.json · rollback-writeback-record.json</div>
            </div>
            <JsonViewer title="Rollback 写回 store JSON" value={rollbackWritebackStore} rows={10} />
          </div>
        )}
        {rollbackWritebackExecutorPlan && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color={rollbackWritebackExecutorPlan.rollback_executor_ready ? "success" : "warning"}>
                {rollbackWritebackExecutorPlan.status}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{rollbackWritebackExecutorPlan.action_count}</div>
              <div className="text-xs text-muted">rollback_writeback_executor_plan_ready {String(rollbackWritebackExecutorPlan.rollback_writeback_executor_plan_ready)} · consumes store {String(rollbackWritebackExecutorPlan.consumes_rollback_writeback_store)}</div>
              <div className="mt-2 text-xs text-muted">executor ready {String(rollbackWritebackExecutorPlan.rollback_executor_ready)} · input contract {String(rollbackWritebackExecutorPlan.executor_input_contract_ready)}</div>
              <div className="mt-2 text-xs text-muted">writes Ledger {String(rollbackWritebackExecutorPlan.writes_ledger_kv)} · audit chain {String(rollbackWritebackExecutorPlan.writes_audit_chain)}</div>
              <div className="mt-2 text-xs text-muted">artifact rollback-writeback-executor-plan.json · rollback-executor-handoff-plan.json · rollback-executor-audit-plan.json</div>
            </div>
            <JsonViewer title="Rollback executor handoff JSON" value={rollbackWritebackExecutorPlan} rows={10} />
          </div>
        )}
      </Card>

      <Card className="section-card p-4">
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold"><Link2 size={16} />KV audit proof-link schema / preview gate</div>
            <div className="mt-1 text-xs text-muted">
              先暴露逐条 KV 证明链接的稳定 schema，并用 `audit-link-preview.json` 把 native kv_history row preview 与 Merkle audit record 做只读候选关联；`audit-link-writeback-plan.json` 只把已匹配候选映射为未来 audit_seq/audit_hash 回填动作和 Approval Manager 请求形态；`audit-link-writeback-store.json` / `audit-link-writeback-record.json` 只保存 pack-local durable handoff record；`audit-link-writeback-executor-plan.json` / `audit-link-executor-handoff-plan.json` / `audit-link-executor-audit-plan.json` 只生成 executor 输入与审计追加计划，当前不会回填 audit_seq/audit_hash、不会写 native/Ledger、不会追加 Merkle，也不会声称已有 per-KV proof。
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" isPending={busy === "audit-links"} onPress={loadAuditLinks}><Link2 size={14} />读取 schema</Button>
            <Button variant="outline" isPending={busy === "audit-link-preview"} onPress={previewAuditLinks}><ShieldCheck size={14} />生成 preview gate</Button>
            <Button variant="outline" isPending={busy === "audit-link-writeback"} onPress={buildAuditLinkWritebackPlan}><UnlockKeyhole size={14} />生成 writeback plan</Button>
            <Button variant="outline" isPending={busy === "audit-link-writeback-store"} onPress={writeAuditLinkWritebackStore}><Save size={14} />写入 handoff store</Button>
            <Button variant="outline" isPending={busy === "audit-link-writeback-executor"} onPress={buildAuditLinkWritebackExecutorPlan}><DatabaseZap size={14} />生成 executor handoff plan</Button>
          </div>
        </div>
        {auditLinks ? (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-[240px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color="default" variant="soft">{auditLinks.schema_ready ? "schema ready" : "pending"}</Chip>
              <div className="mt-3 text-2xl font-semibold">{auditLinks.kv_audit_links.length}</div>
              <div className="text-xs text-muted">links · linkage {auditLinks.linkage_ready ? "ready" : "not wired"}</div>
              <div className="mt-2 text-xs text-muted">native kv_history {auditLinks.native_kv_history_ready ? "ready" : "not wired"}</div>
            </div>
            <JsonViewer title="KV audit proof links JSON" value={auditLinks} rows={8} />
          </div>
        ) : (
          <div className="rounded-xl border border-dashed border-border p-4 text-sm text-muted">
            尚未读取 KV audit proof-link schema。这个入口当前只返回 schema/空 links，为后续 native kv_history + Merkle 证明关联预留稳定契约。
          </div>
        )}
        {auditLinkPreview && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color={auditLinkPreview.linkage_ready ? "success" : "warning"}>
                {auditLinkPreview.status}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{auditLinkPreview.matched_link_count}/{auditLinkPreview.candidate_link_count}</div>
              <div className="text-xs text-muted">matched links · pending {auditLinkPreview.pending_link_count}</div>
              <div className="mt-2 text-xs text-muted">linkage_ready {String(auditLinkPreview.linkage_ready)} · merkle_append {String(auditLinkPreview.merkle_append_ready)}</div>
              <div className="mt-2 text-xs text-muted">writes Ledger {String(auditLinkPreview.writes_ledger_kv)} · writes native {String(auditLinkPreview.writes_native_kv_history)}</div>
            </div>
            <JsonViewer title="KV audit proof-link 预览 JSON" value={auditLinkPreview} rows={10} />
          </div>
        )}
        {auditLinkWritebackPlan && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color={auditLinkWritebackPlan.kv_audit_link_writeback_ready ? "success" : "warning"}>
                {auditLinkWritebackPlan.status}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{auditLinkWritebackPlan.action_count}</div>
              <div className="text-xs text-muted">writeback actions · matched {auditLinkWritebackPlan.matched_link_count}</div>
              <div className="mt-2 text-xs text-muted">backfills audit_seq {String(auditLinkWritebackPlan.backfills_audit_seq)} · audit_hash {String(auditLinkWritebackPlan.backfills_audit_hash)}</div>
              <div className="mt-2 text-xs text-muted">writes Ledger {String(auditLinkWritebackPlan.writes_ledger_kv)} · global enqueue {String(auditLinkWritebackPlan.global_approval_enqueue_ready)}</div>
            </div>
            <JsonViewer title="KV audit proof-link 写回计划 JSON" value={auditLinkWritebackPlan} rows={10} />
          </div>
        )}
        {auditLinkWritebackStore && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color="default" variant="soft">
                pack-local handoff store
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{auditLinkWritebackStore.audit_link_writeback_store.record_count}</div>
              <div className="text-xs text-muted">records · status {auditLinkWritebackStore.status}</div>
              <div className="mt-2 text-xs text-muted">kv_audit_link_writeback_store_ready {String(auditLinkWritebackStore.kv_audit_link_writeback_store_ready)} · store writes {String(auditLinkWritebackStore.writes_audit_link_writeback_store)}</div>
              <div className="mt-2 text-xs text-muted">proof ready {String(auditLinkWritebackStore.kv_audit_link_writeback_ready)} · record audit-link-writeback-record.json</div>
              <div className="mt-2 text-xs text-muted">Ledger {String(auditLinkWritebackStore.writes_ledger_kv)} · native {String(auditLinkWritebackStore.writes_native_kv_history)} · Merkle {String(auditLinkWritebackStore.appends_merkle)}</div>
            </div>
            <JsonViewer title="KV audit proof-link 写回 store JSON" value={auditLinkWritebackStore} rows={10} />
          </div>
        )}
        {auditLinkWritebackExecutorPlan && (
          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[260px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color={auditLinkWritebackExecutorPlan.audit_proof_link_executor_ready ? "success" : "warning"}>
                {auditLinkWritebackExecutorPlan.status}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{auditLinkWritebackExecutorPlan.action_count}</div>
              <div className="text-xs text-muted">kv_audit_link_writeback_executor_plan_ready {String(auditLinkWritebackExecutorPlan.kv_audit_link_writeback_executor_plan_ready)} · consumes store {String(auditLinkWritebackExecutorPlan.consumes_audit_link_writeback_store)}</div>
              <div className="mt-2 text-xs text-muted">executor ready {String(auditLinkWritebackExecutorPlan.audit_proof_link_executor_ready)} · input contract {String(auditLinkWritebackExecutorPlan.executor_input_contract_ready)}</div>
              <div className="mt-2 text-xs text-muted">writes audit chain {String(auditLinkWritebackExecutorPlan.writes_audit_chain)} · audit append {String(auditLinkWritebackExecutorPlan.audit_append_plan_ready)}</div>
              <div className="mt-2 text-xs text-muted">artifact audit-link-writeback-executor-plan.json · audit-link-executor-handoff-plan.json · audit-link-executor-audit-plan.json</div>
            </div>
            <JsonViewer title="KV audit executor handoff JSON" value={auditLinkWritebackExecutorPlan} rows={10} />
          </div>
        )}
      </Card>

      <Card className="section-card p-4">
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold"><ShieldCheck size={16} />Merkle 审计链验证</div>
            <div className="mt-1 text-xs text-muted">
              通过 Pack Runtime 只读验证内存中的 audit chain，并展示最近记录摘要；这里不执行回滚写回，也不声称 KV history 已具备逐条审计证明。
            </div>
          </div>
          <Button variant="outline" isPending={busy === "audit"} onPress={verifyAuditChain}><ShieldCheck size={14} />验证审计链</Button>
        </div>
        {auditVerification ? (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-[220px_1fr]">
            <div className="rounded-xl border border-border bg-surface-secondary p-3">
              <Chip size="sm" color={auditVerification.ready && auditVerification.valid ? "success" : "warning"}>
                {auditVerification.ready ? (auditVerification.valid ? "valid" : "invalid") : "not attached"}
              </Chip>
              <div className="mt-3 text-2xl font-semibold">{auditVerification.record_count}</div>
              <div className="text-xs text-muted">records · last seq {auditVerification.last_seq ?? "-"}</div>
              <div className="mt-2 truncate text-xs text-muted">{auditVerification.last_hash || "no hash"}</div>
            </div>
            <JsonViewer title="Audit verification JSON" value={auditVerification} rows={8} />
          </div>
        ) : (
          <div className="rounded-xl border border-dashed border-border p-4 text-sm text-muted">
            尚未运行本页验证。启用 pack 后可通过 `/v1/memory-time-travel/audit/verify` 获取只读验证结果。
          </div>
        )}
      </Card>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_1fr]">
        <Card className="section-card overflow-hidden">
          <div className="flex items-center justify-between border-b border-border px-4 py-3">
            <div className="flex items-center gap-2 text-sm font-semibold"><Clock3 size={16} />记忆快照</div>
            <Chip size="sm">{snapshots.length}</Chip>
          </div>
          <div className="max-h-[560px] divide-y divide-border overflow-auto">
            {snapshots.length === 0 ? <div className="p-6 text-center text-sm text-muted">还没有快照。可以先保存右侧样例作为 baseline。</div> : snapshots.map((snapshot) => (
              <button key={`${snapshot.namespace}:${snapshot.id}`} onClick={() => { setBaseId(snapshot.id); if (!targetId || targetId === snapshot.id) setTargetId(snapshots.find((item) => item.id !== snapshot.id)?.id || snapshot.id); }} className="block w-full px-4 py-3 text-left hover:bg-white/5">
                <div className="flex items-center justify-between gap-2"><div className="font-medium">{snapshot.id}</div><Chip size="sm">{snapshot.key_count} keys</Chip></div>
                <div className="mt-1 truncate text-xs text-muted">{snapshot.namespace} · {snapshot.hash.slice(0, 12)}</div>
              </button>
            ))}
          </div>
        </Card>

        <div className="space-y-4">
          <Card className="section-card p-4">
            <div className="mb-3 flex items-center gap-2 text-sm font-semibold"><Save size={16} />保存记忆快照</div>
            <div className="mb-3 grid grid-cols-1 gap-3 md:grid-cols-2">
              <Select
                selectedKey={namespace}
                onSelectionChange={(key) => setNamespace(String(key))}
              >
                <Label>命名空间</Label>
                <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                <Select.Popover>
                  <ListBox>
                    {namespaceOptions.map((item) => (
                      <ListBox.Item key={item} id={item} textValue={item}>
                        <span className="font-mono text-xs">{item}</span>
                      </ListBox.Item>
                    ))}
                  </ListBox>
                </Select.Popover>
              </Select>
              <TextField value={snapshotId} onChange={setSnapshotId}>
                <Label>快照 ID</Label>
                <Input placeholder="baseline id" />
              </TextField>
            </div>
            <TextField value={valuesJSON} onChange={setValuesJSON}>
              <Label>快照值 JSON</Label>
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
              <TextField value={baseId} onChange={setBaseId}>
                <Label>Base 快照 ID</Label>
                <Input placeholder="base snapshot id" />
              </TextField>
              <TextField value={targetId} onChange={setTargetId}>
                <Label>Target 快照 ID</Label>
                <Input placeholder="target snapshot id" />
              </TextField>
              <TextField value={at} onChange={setAt}>
                <Label>回溯时间点</Label>
                <Input placeholder="2026-05-15T12:00:00Z" />
              </TextField>
            </div>
            <div className="mt-3 flex justify-end"><Button variant="outline" isPending={busy === "snapshot-at"} onPress={reconstruct}><Clock3 size={14} />重建时间点</Button></div>

            {snapshotAt && (
              <Card className="mt-3 bg-surface-secondary p-3">
                <div className="mb-2 text-sm font-medium">snapshot-at: {snapshotAt.status} {snapshotAt.matched_id ? `· ${snapshotAt.matched_id}` : ""}</div>
                <JsonViewer title="时间点快照 JSON" value={snapshotAt.values} rows={6} />
              </Card>
            )}

            {diff ? (
              <Card className="mt-3 bg-surface-secondary p-3">
                <div className="mb-2 flex flex-wrap items-center gap-2 text-sm font-medium">
                  <Chip size="sm" color={diffRiskColor}>risk: {diff.risk_level}</Chip>
                  <span>{diff.added_count} added / {diff.changed_count} changed / {diff.removed_count} removed · drift {diff.drift_score}</span>
                </div>
                <JsonViewer title="记忆漂移 diff JSON" value={diff} rows={12} />
              </Card>
            ) : (
              <div className="mt-3 rounded-xl border border-dashed border-border p-6 text-center text-sm text-muted">保存或选择两个快照后，可以生成记忆漂移 diff 和回滚建议。</div>
            )}

            {rollbackPlan.length > 0 && (
              <Card className="mt-3 bg-surface-secondary p-3">
                <div className="mb-2 flex items-center gap-2 text-sm font-medium"><RotateCcw size={15} />Dry-run rollback plan</div>
                <pre className="max-h-56 overflow-auto whitespace-pre-wrap text-xs text-muted">{rollbackPlan.join("\n")}</pre>
              </Card>
            )}
          </Card>
        </div>
      </div>
    </div>
  );
}
