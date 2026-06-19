"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Card, Chip, Input, Spinner, TextArea, TextField } from "@heroui/react";
import { AlertTriangle, Download, FileJson, GitCompare, PackageSearch, RefreshCw, ShieldCheck } from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import { createSBOMDriftClient as createSBOMDriftPackClient, type SBOMDriftBaselineArtifactSourcePlan, type SBOMDriftCIBaselineWriteback, type SBOMDriftCIGatePlan, type SBOMDriftCIWorkflowWritebackPlan, type SBOMDriftCycloneDXDocument, type SBOMDriftDiff, type SBOMDriftSnapshotSummary, type SBOMDriftStatusResponse as SBOMDriftStatus } from "yunque-client/sbom-drift";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";

const sbomDriftPack = createSBOMDriftPackClient(createYunqueSDKClientOptions());

function riskTone(risk?: string): { bg: string; fg: string } {
  switch (risk) {
    case "critical": return { bg: "rgba(239,68,68,0.16)", fg: "#ef4444" };
    case "high": return { bg: "rgba(249,115,22,0.16)", fg: "#fb923c" };
    case "medium": return { bg: "rgba(250,204,21,0.14)", fg: "#facc15" };
    case "low": return { bg: "rgba(56,189,248,0.12)", fg: "#38bdf8" };
    default: return { bg: "rgba(34,197,94,0.12)", fg: "#22c55e" };
  }
}

function defaultSnapshotId() {
  return `baseline-${new Date().toISOString().slice(0, 10).replaceAll("-", "")}`;
}

const userFacingSteps = [
  {
    title: "1. 建一个依赖基线",
    body: "保存当前 Go/npm 依赖快照，作为以后发版或升级前的对比点。",
  },
  {
    title: "2. 看依赖漂移",
    body: "对比当前工作树，找出新增、移除、版本变化和风险等级。",
  },
  {
    title: "3. 生成 CI 交接计划",
    body: "把漂移报告、CycloneDX 和 CI gate 计划导出给后续流水线审核。",
  },
];

const boundaryItems = [
  "不会修改 GitHub Actions 或 CI 配置。",
  "不会联网拉取漏洞库或执行 govulncheck。",
  "不会把计划结果写成真实发布阻断。",
  "不会替代正式供应链安全扫描。",
];

export default function SBOMDriftPackPage() {
  const [status, setStatus] = useState<SBOMDriftStatus | null>(null);
  const [snapshots, setSnapshots] = useState<SBOMDriftSnapshotSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"snapshot" | "diff" | "cyclonedx" | "ciGate" | "artifactSourcePlan" | "ciBaselineWriteback" | "ciWorkflowPlan" | "evidence" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [snapshotId, setSnapshotId] = useState(defaultSnapshotId);
  const [source, setSource] = useState("manual-baseline");
  const [baseId, setBaseId] = useState("");
  const [diff, setDiff] = useState<SBOMDriftDiff | null>(null);
  const [cycloneDX, setCycloneDX] = useState<SBOMDriftCycloneDXDocument | null>(null);
  const [ciGatePlan, setCIGatePlan] = useState<SBOMDriftCIGatePlan | null>(null);
  const [artifactSourcePlan, setArtifactSourcePlan] = useState<SBOMDriftBaselineArtifactSourcePlan | null>(null);
  const [ciBaselineWriteback, setCIBaselineWriteback] = useState<SBOMDriftCIBaselineWriteback | null>(null);
  const [ciWorkflowPlan, setCIWorkflowPlan] = useState<SBOMDriftCIWorkflowWritebackPlan | null>(null);

  const selectedBase = useMemo(() => snapshots.find((snapshot) => snapshot.id === baseId) || snapshots[0] || null, [baseId, snapshots]);
  const tone = riskTone(diff?.risk_level);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [statusRes, snapshotsRes] = await Promise.all([sbomDriftPack.status(), sbomDriftPack.snapshots()]);
      setStatus(statusRes);
      setSnapshots(snapshotsRes.snapshots || []);
      if (!baseId && snapshotsRes.snapshots?.[0]?.id) setBaseId(snapshotsRes.snapshots[0].id);
    } catch (e) {
      const msg = formatErrorMessage(e, "加载 SBOM Drift Pack 失败");
      setError(msg.includes("pack route is not enabled") ? "SBOM Drift Pack 当前未启用。请到「能力包」控制台启用 yunque.pack.sbom-drift 后再使用。" : msg);
    } finally {
      setLoading(false);
    }
  }, [baseId]);

  useEffect(() => { load(); }, [load]);

  const createSnapshot = async () => {
    setBusy("snapshot");
    setError(null);
    try {
      const res = await sbomDriftPack.createSnapshot({ id: snapshotId, source });
      setBaseId(res.snapshot.id);
      showToast("SBOM 依赖快照已保存", "success");
      await load();
    } catch (e) {
      setError(formatErrorMessage(e, "保存 SBOM 快照失败"));
    } finally {
      setBusy(null);
    }
  };

  const runDiff = async () => {
    const id = baseId || selectedBase?.id;
    if (!id) {
      setError("请先创建或选择一个基线快照。可先点击“生成快照”。");
      return;
    }
    setBusy("diff");
    setError(null);
    try {
      const res = await sbomDriftPack.diff({ base_id: id, target_current: true });
      setDiff(res.diff);
      showToast("已生成依赖漂移报告", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "依赖漂移检测失败"));
    } finally {
      setBusy(null);
    }
  };

  const exportCycloneDX = async () => {
    const id = baseId || selectedBase?.id || "current";
    setBusy("cyclonedx");
    setError(null);
    try {
      const res = await sbomDriftPack.cycloneDX(id);
      setCycloneDX(res.bom);
      showToast("已生成 CycloneDX SBOM JSON", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 CycloneDX SBOM 失败"));
    } finally {
      setBusy(null);
    }
  };

  const planCIGate = async () => {
    const id = baseId || selectedBase?.id;
    if (!id) {
      setError("请先创建或选择一个基线快照。CI gate plan 需要 baseline。");
      return;
    }
    setBusy("ciGate");
    setError(null);
    try {
      const res = await sbomDriftPack.ciGatePlan({ base_id: id, target_current: true, fail_on_risk: "high", requested_by: "pack-console" });
      setCIGatePlan(res.plan);
      setDiff(res.plan.diff);
      showToast(res.plan.blocked ? "CI gate plan 将阻断当前漂移" : "CI gate plan 允许当前漂移", res.plan.blocked ? "warning" : "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 CI gate plan 失败"));
    } finally {
      setBusy(null);
    }
  };

  const planArtifactSource = async () => {
    const id = (baseId || selectedBase?.id || "artifact-baseline").toLowerCase().replace(/[^a-z0-9_-]+/g, "-").replace(/^-+|-+$/g, "") || "artifact-baseline";
    setBusy("artifactSourcePlan");
    setError(null);
    try {
      const res = await sbomDriftPack.baselineArtifactSourcePlan({
        source_name: "release-artifact-baseline",
        provider: "artifact-repository",
        artifact_url: `artifact://sbom-drift/baselines/${id}`,
        artifact_name: "sbom-baseline-evidence.json",
        baseline_id: id,
        requested_by: "pack-console",
      });
      setArtifactSourcePlan(res.plan);
      showToast("已生成 Artifact Baseline Source Plan", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 artifact baseline source plan 失败"));
    } finally {
      setBusy(null);
    }
  };

  const writeCIBaseline = async () => {
    const id = baseId || selectedBase?.id;
    if (!id) {
      setError("请先创建或选择一个基线快照。CI baseline write-back 需要 baseline。");
      return;
    }
    setBusy("ciBaselineWriteback");
    setError(null);
    try {
      const res = await sbomDriftPack.ciBaselineWriteback({ base_id: id, target_current: true, fail_on_risk: "high", requested_by: "pack-console", request_key: `sbom-baseline-${id}` });
      setCIBaselineWriteback(res.writeback);
      setCIGatePlan(res.writeback.ci_gate_plan);
      setDiff(res.writeback.ci_gate_plan.diff);
      showToast("已写入 pack-local CI baseline gate 记录", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "写入 CI baseline gate 记录失败"));
    } finally {
      setBusy(null);
    }
  };

  const planCIWorkflowWriteback = async () => {
    const key = ciBaselineWriteback?.request_key || (baseId || selectedBase?.id ? `sbom-baseline-${baseId || selectedBase?.id}` : "");
    if (!key) {
      setError("请先写入 Baseline Store。CI workflow handoff plan 需要 pack-local baseline 记录。");
      return;
    }
    setBusy("ciWorkflowPlan");
    setError(null);
    try {
      const res = await sbomDriftPack.ciWorkflowWritebackPlan({ request_key: key, workflow_path: ".github/workflows/sbom-drift.yml", job_name: "sbom-drift-gate", requested_by: "pack-console" });
      setCIWorkflowPlan(res.plan);
      showToast("已生成 CI workflow write-back handoff plan", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 CI workflow handoff plan 失败"));
    } finally {
      setBusy(null);
    }
  };

  const exportEvidence = async () => {
    const id = baseId || selectedBase?.id;
    if (!id) return;
    setBusy("evidence");
    setError(null);
    try {
      const evidence = await sbomDriftPack.evidence(id);
      const blob = new Blob([JSON.stringify(evidence, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${id}-sbom-drift-evidence.json`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      showToast("SBOM 证据包已导出", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "导出 SBOM 证据包失败"));
    } finally {
      setBusy(null);
    }
  };

  if (loading) {
    return <div className="flex h-[60vh] items-center justify-center"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader icon={<ShieldCheck size={20} />} title="SBOM 依赖漂移" />

      <Card className="section-card overflow-hidden p-0">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="p-5">
            <div className="flex flex-wrap items-center gap-2">
              <Chip size="sm" style={{ background: "rgba(245,158,11,0.12)", color: "var(--yunque-warning)" }}>实验中</Chip>
              <Chip size="sm" variant="soft">可保存基线</Chip>
              <Chip size="sm" variant="soft">CI 只生成计划</Chip>
            </div>
            <div className="mt-3 text-base font-semibold" style={{ color: "var(--yunque-text)" }}>
              这个能力包现在适合做什么
            </div>
            <div className="mt-2 max-w-3xl text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
              它用于在发版、升级依赖或接入第三方包之前，先记录依赖基线，再对比当前工作树的变化。当前可以保存快照、生成漂移报告、导出 CycloneDX 和证据包；CI gate / workflow 写回仍是交接计划，不会直接改你的仓库或阻断发布。
            </div>
            <div className="mt-4 grid gap-3 md:grid-cols-3">
              {userFacingSteps.map((item) => (
                <div key={item.title} className="rounded-lg p-3" style={{ background: "var(--yunque-bg-hover)", border: "1px solid var(--yunque-border)" }}>
                  <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{item.title}</div>
                  <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>{item.body}</div>
                </div>
              ))}
            </div>
          </div>
          <div className="p-5" style={{ background: "rgba(245,158,11,0.08)", borderLeft: "1px solid var(--yunque-border)" }}>
            <div className="mb-3 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>当前不会做什么</div>
            <div className="space-y-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
              {boundaryItems.map((item) => <div key={item}>{item}</div>)}
            </div>
          </div>
        </div>
      </Card>

      <Card className="section-card p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="mb-3 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>技术状态</div>
            <div className="mb-1 flex items-center gap-2">
              <Chip size="sm" style={{ background: status?.scanner_ready ? "rgba(34,197,94,0.12)" : "rgba(250,204,21,0.12)", color: status?.scanner_ready ? "#22c55e" : "#facc15" }}>
                {status?.scanner_ready ? "Snapshot scanner" : "Pack shell"}
              </Chip>
              <Chip size="sm" style={{ background: status?.govulncheck_plan_ready ? "rgba(56,189,248,0.12)" : "rgba(250,204,21,0.12)", color: status?.govulncheck_plan_ready ? "#38bdf8" : "#facc15" }}>
                {status?.govulncheck_plan_ready ? "govulncheck plan" : "vuln plan pending"}
              </Chip>
              <Chip size="sm" style={{ background: status?.ci_baseline_writeback_ready ? "rgba(56,189,248,0.12)" : "rgba(250,204,21,0.12)", color: status?.ci_baseline_writeback_ready ? "#38bdf8" : "#facc15" }}>
                {status?.artifact_source_plan_ready ? "artifact baseline plan" : (status?.ci_baseline_writeback_ready ? "baseline store" : "baseline pending")}
              </Chip>
              <Chip size="sm" style={{ background: status?.ci_workflow_writeback_plan_ready ? "rgba(56,189,248,0.12)" : "rgba(250,204,21,0.12)", color: status?.ci_workflow_writeback_plan_ready ? "#38bdf8" : "#facc15" }}>
                {status?.ci_workflow_writeback_plan_ready ? "workflow handoff" : "workflow pending"}
              </Chip>
              <Chip size="sm">{status?.stage || "pack-shell"}</Chip>
              <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{status?.pack_id || "yunque.pack.sbom-drift"}</span>
            </div>
            <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>
              当前切片已完成 Go/npm 依赖快照、漂移 diff、CycloneDX JSON 导出、CI gate plan、artifact baseline source handoff plan、pack-local CI baseline gate write-back、CI workflow write-back handoff plan、govulncheck plan preview 和证据包导出。artifact repository 拉取、govulncheck 执行、CI workflow 写回和真实 CI 阻断仍保持后续接入。
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
        <Card className="section-card p-4"><div className="kpi-label">扫描器</div><div className="kpi-value text-lg">{status?.scanner_ready ? "ready" : "shell"}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">CycloneDX / CI / Vuln</div><div className="kpi-value text-lg">{status?.cyclonedx_ready && status?.ci_gate_plan_ready && status?.govulncheck_plan_ready ? "plan" : "pending"}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">Baseline Store</div><div className="kpi-value text-lg">{status?.ci_baseline_store?.record_count ?? 0}</div></Card>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_1fr]">
        <Card className="section-card overflow-hidden">
          <div className="flex items-center justify-between border-b px-4 py-3" style={{ borderColor: "var(--yunque-border)" }}>
            <div className="flex items-center gap-2 text-sm font-semibold"><PackageSearch size={16} />已保存快照</div>
            <Chip size="sm">{snapshots.length}</Chip>
          </div>
          <div className="max-h-[520px] divide-y overflow-auto" style={{ borderColor: "var(--yunque-border)" }}>
            {snapshots.length === 0 ? <div className="p-6 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>还没有基线。可以先生成当前依赖快照。</div> : snapshots.map((snapshot) => (
              <button key={snapshot.id} onClick={() => setBaseId(snapshot.id)} className="block w-full px-4 py-3 text-left hover:bg-white/5">
                <div className="flex items-center justify-between gap-2"><div className="font-medium">{snapshot.id}</div><Chip size="sm">{snapshot.component_count} deps</Chip></div>
                <div className="mt-1 truncate text-xs" style={{ color: "var(--yunque-text-muted)" }}>{snapshot.source}</div>
              </button>
            ))}
          </div>
        </Card>

        <div className="space-y-4">
          <Card className="section-card p-4">
            <div className="mb-3 flex items-center gap-2 text-sm font-semibold"><PackageSearch size={16} />创建基线快照</div>
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
              <TextField value={snapshotId} onChange={setSnapshotId}><Input placeholder="baseline-20260515" /></TextField>
              <TextField value={source} onChange={setSource}><Input placeholder="manual-baseline" /></TextField>
            </div>
            <div className="mt-3 flex justify-end"><Button className="btn-accent" isPending={busy === "snapshot"} onPress={createSnapshot}>生成快照</Button></div>
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold"><GitCompare size={16} />与当前工作树对比</div>
                <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>基线：{selectedBase?.id || baseId || "尚未选择"}</div>
              </div>
              <div className="flex gap-2">
                <Button variant="outline" isPending={busy === "cyclonedx"} onPress={exportCycloneDX} isDisabled={!selectedBase && !baseId}><FileJson size={14} />CycloneDX</Button>
                <Button variant="outline" isPending={busy === "ciGate"} onPress={planCIGate} isDisabled={!selectedBase && !baseId}>CI Gate Plan</Button>
                <Button variant="outline" isPending={busy === "artifactSourcePlan"} onPress={planArtifactSource}>Artifact Source Plan</Button>
                <Button variant="outline" isPending={busy === "ciBaselineWriteback"} onPress={writeCIBaseline} isDisabled={!selectedBase && !baseId}>写 Baseline Store</Button>
                <Button variant="outline" isPending={busy === "evidence"} onPress={exportEvidence} isDisabled={!selectedBase && !baseId}><Download size={14} />导出证据包</Button>
                <Button className="btn-accent" isPending={busy === "diff"} onPress={runDiff} isDisabled={!selectedBase && !baseId}>生成漂移报告</Button>
              </div>
            </div>
            {diff ? (
              <Card className="p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
                <div className="mb-2 flex items-center gap-2 text-sm font-medium">
                  <Chip size="sm" style={{ background: tone.bg, color: tone.fg }}>risk: {diff.risk_level}</Chip>
                  <span>{diff.added.length} added / {diff.changed.length} changed / {diff.removed.length} removed</span>
                </div>
                <TextField value={JSON.stringify(diff, null, 2)} onChange={() => undefined}>
                  <TextArea rows={14} aria-label="SBOM Drift JSON" className="font-mono text-xs" readOnly />
                </TextField>
              </Card>
            ) : (
              <div className="rounded-xl border border-dashed p-6 text-center text-sm" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>选择或创建一个基线后，可以生成与当前工作树的依赖漂移报告。</div>
            )}
          </Card>

          {(cycloneDX || ciGatePlan || artifactSourcePlan || ciBaselineWriteback || ciWorkflowPlan) && (
            <Card className="section-card p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-semibold"><FileJson size={16} />CycloneDX / CI gate / artifact baseline / workflow handoff 预览</div>
              <div className="mb-3 flex flex-wrap gap-2">
                {cycloneDX && <Chip size="sm">CycloneDX {cycloneDX.specVersion}</Chip>}
                {ciGatePlan && <Chip size="sm" style={{ background: ciGatePlan.blocked ? "rgba(239,68,68,0.16)" : "rgba(34,197,94,0.12)", color: ciGatePlan.blocked ? "#ef4444" : "#22c55e" }}>{ciGatePlan.status}</Chip>}
                {ciGatePlan && <Chip size="sm">threshold: {ciGatePlan.fail_on_risk}</Chip>}
                {ciGatePlan && <Chip size="sm">govulncheck_plan_ready: {String(ciGatePlan.govulncheck_plan_ready)}</Chip>}
                {ciGatePlan && <Chip size="sm">govulncheck_ready: {String(ciGatePlan.govulncheck_ready)}</Chip>}
                {ciGatePlan?.govulncheck_plan && <Chip size="sm">writes_files: {String(ciGatePlan.govulncheck_plan.writes_files)}</Chip>}
                {ciGatePlan?.govulncheck_plan && <Chip size="sm">{ciGatePlan.govulncheck_plan.report_artifact}</Chip>}
                {artifactSourcePlan && <Chip size="sm">artifact_source_plan: {String(artifactSourcePlan.artifact_source_plan_ready)}</Chip>}
                {artifactSourcePlan && <Chip size="sm">fetches_artifact: {String(artifactSourcePlan.fetches_artifact_baseline)}</Chip>}
                {artifactSourcePlan && <Chip size="sm">writes_baseline: {String(artifactSourcePlan.writes_baseline_snapshot)}</Chip>}
                {artifactSourcePlan && <Chip size="sm">{artifactSourcePlan.baseline_fetch_handoff_plan?.source?.artifact_name}</Chip>}
                {ciBaselineWriteback && <Chip size="sm">baseline_store: {String(ciBaselineWriteback.writes_ci_baseline_store)}</Chip>}
                {ciBaselineWriteback && <Chip size="sm">ci_workflow: {String(ciBaselineWriteback.writes_ci_workflow)}</Chip>}
                {ciBaselineWriteback && <Chip size="sm">govulncheck_exec: {String(ciBaselineWriteback.executes_govulncheck)}</Chip>}
                {ciWorkflowPlan && <Chip size="sm">workflow_plan: {String(ciWorkflowPlan.ci_workflow_writeback_plan_ready)}</Chip>}
                {ciWorkflowPlan && <Chip size="sm">consumes_store: {String(ciWorkflowPlan.consumes_ci_baseline_store)}</Chip>}
                {ciWorkflowPlan && <Chip size="sm">release_block: {String(ciWorkflowPlan.blocks_release)}</Chip>}
              </div>
              {ciBaselineWriteback && !ciWorkflowPlan && (
                <div className="mb-3 flex justify-end">
                  <Button variant="outline" isPending={busy === "ciWorkflowPlan"} onPress={planCIWorkflowWriteback}>生成 Workflow Handoff</Button>
                </div>
              )}
              <TextField value={JSON.stringify({ cyclonedx: cycloneDX, ci_gate_plan: ciGatePlan, baseline_artifact_source_plan: artifactSourcePlan, ci_baseline_writeback: ciBaselineWriteback, ci_workflow_writeback_plan: ciWorkflowPlan }, null, 2)} onChange={() => undefined}>
                <TextArea rows={12} aria-label="SBOM CycloneDX and CI Gate JSON" className="font-mono text-xs" readOnly />
              </TextField>
              <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                artifact baseline source plan 只描述未来制品仓库 baseline 拉取契约，不联网、不读凭据、不覆盖快照；baseline store 只写 pack-local JSON handoff；workflow handoff plan 只消费该记录并生成 CI writer / release blocker 契约。当前不会修改 GitHub Actions、不会执行 govulncheck、不会拉取漏洞库，也不会真实阻断 release。
              </div>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}
