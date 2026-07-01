"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Button, Card, Chip, Input, Label, Spinner, TextArea, TextField } from "@heroui/react";
import { Activity, AlertTriangle, ClipboardCheck, ClipboardList, Download, Radar, RefreshCw, Send, ShieldAlert } from "lucide-react";
import PageHeader from "@/components/page-header";
import { JsonViewer } from "@/components/json-viewer";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref } from "@/lib/pack-action-links";
import { PackAbout, PackSectionTitle, PackStepsGrid, type PackBoundaryItem, type PackStep } from "@/components/packs/pack-page-kit";
import { createSkillAnomalyPackClient, type SkillAnomalyApprovalManagerBridgePlan, type SkillAnomalyApprovalQueueWriteback, type SkillAnomalyAuditHookPlan, type SkillAnomalyProfileSummary, type SkillAnomalyResult, type SkillAnomalyStatus } from "@/lib/skill-anomaly-pack-client";

const skillAnomalyPack = createSkillAnomalyPackClient();

type ChipColor = "danger" | "warning" | "success" | "default";

// Map an anomaly severity to a semantic Chip color (no hand-rolled rgba).
function severityColor(severity?: string): ChipColor {
  switch (severity) {
    case "block": return "danger";
    case "needs_approval": return "warning";
    case "learning": return "default";
    default: return "success";
  }
}

function sampleEvent(skillSlug: string, anomalous = false) {
  return JSON.stringify(anomalous ? {
    skill_slug: skillSlug,
    action: "shell_exec",
    params: { command: "whoami", exfil_url: "https://example.invalid" },
    success: false,
    duration_ms: 500,
    dry_run: true,
  } : {
    skill_slug: skillSlug,
    action: "read_file",
    params: { path: "notes.md" },
    success: true,
    duration_ms: 100,
  }, null, 2);
}

const userFacingSteps: PackStep[] = [
  { key: "baseline", label: "建立正常行为画像", detail: "记录 Skill 平时会做什么、耗时多少、是否成功，形成可比较的基线。" },
  { key: "detect", label: "检测可疑调用", detail: "把候选行为拿来 dry-run 检查，判断是否需要人工审批或阻断。" },
  { key: "govern", label: "交给治理流程", detail: "生成审计、审批队列和全局审批桥接计划，供后续接入 Trust。" },
];

const boundaryItems: PackBoundaryItem[] = [
  { key: "trust", label: "不扣 Trust", detail: "不会直接扣 Trust Score。" },
  { key: "release", label: "不放行动作", detail: "不会自动批准或释放 runtime action。" },
  { key: "merkle", label: "不写 Merkle", detail: "不会追加 Merkle Chain 审计记录。" },
  { key: "manager", label: "不调审批中心", detail: "不会调用全局 Approval Manager。" },
];

const workflowLoopItems: PackStep[] = [
  { key: "observe", label: "观察正常行为", detail: "先积累 Skill 的正常调用、耗时和成功率，避免把未知误判成异常。" },
  { key: "chat", label: "带回 Chat", detail: "让云雀解释异常原因，拆成确认参数、收紧权限或补审批规则的任务。" },
  { key: "review", label: "看审批依据", detail: "本地队列和桥接计划是治理证据，不会自动扣分或释放动作。" },
  { key: "extend", label: "继续补能力", detail: "如果误报或漏报明显，把真实样本交给小羽改检测规则和画像逻辑。" },
];

export default function SkillAnomalyPackPage() {
  const [status, setStatus] = useState<SkillAnomalyStatus | null>(null);
  const [profiles, setProfiles] = useState<SkillAnomalyProfileSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"observe" | "detect" | "audit-plan" | "approval-writeback" | "approval-bridge-plan" | "evidence" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [skillSlug, setSkillSlug] = useState("text_processing");
  const [normalJSON, setNormalJSON] = useState(() => sampleEvent("text_processing"));
  const [candidateJSON, setCandidateJSON] = useState(() => sampleEvent("text_processing", true));
  const [result, setResult] = useState<SkillAnomalyResult | null>(null);
  const [auditPlan, setAuditPlan] = useState<SkillAnomalyAuditHookPlan | null>(null);
  const [approvalWriteback, setApprovalWriteback] = useState<SkillAnomalyApprovalQueueWriteback | null>(null);
  const [approvalBridgePlan, setApprovalBridgePlan] = useState<SkillAnomalyApprovalManagerBridgePlan | null>(null);

  const selectedProfile = useMemo(() => profiles.find((profile) => profile.skill_slug === skillSlug) || profiles[0] || null, [profiles, skillSlug]);
  const severityChipColor = severityColor(result?.severity);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [statusRes, profilesRes] = await Promise.all([skillAnomalyPack.status(), skillAnomalyPack.profiles()]);
      setStatus(statusRes);
      setProfiles(profilesRes.profiles || []);
      if (!skillSlug && profilesRes.profiles?.[0]?.skill_slug) setSkillSlug(profilesRes.profiles[0].skill_slug);
    } catch (e) {
      const msg = formatErrorMessage(e, "加载 Skill Anomaly Pack 失败");
      setError(msg.includes("pack route is not enabled") ? "Skill Anomaly Pack 当前未启用。请到「能力包」控制台启用 yunque.pack.skill-anomaly 后再使用。" : msg);
    } finally {
      setLoading(false);
    }
  }, [skillSlug]);

  useEffect(() => { load(); }, [load]);

  const observeEvent = async () => {
    setBusy("observe");
    setError(null);
    try {
      const payload = JSON.parse(normalJSON);
      const res = await skillAnomalyPack.observe(payload);
      setSkillSlug(res.event.skill_slug || skillSlug);
      setResult(res.result);
      showToast(payload.dry_run ? "Skill 行为事件已校验" : "Skill 行为事件已进入基线", "success");
      if (!payload.dry_run) await load();
    } catch (e) {
      setError(formatErrorMessage(e, "记录 Skill 行为事件失败"));
    } finally {
      setBusy(null);
    }
  };

  const detectCandidate = async () => {
    setBusy("detect");
    setError(null);
    try {
      const payload = JSON.parse(candidateJSON);
      const res = await skillAnomalyPack.detect(payload);
      setResult(res.result);
      setAuditPlan(null);
      setApprovalWriteback(null);
      setApprovalBridgePlan(null);
      showToast(res.result.needs_approval ? "已生成 NeedsApproval 异常计划" : "候选行为符合当前基线", "success");
      if (!payload.dry_run) await load();
    } catch (e) {
      setError(formatErrorMessage(e, "检测 Skill 行为异常失败"));
    } finally {
      setBusy(null);
    }
  };

  const planAuditHook = async () => {
    setBusy("audit-plan");
    setError(null);
    try {
      const payload = JSON.parse(candidateJSON);
      const res = await skillAnomalyPack.auditHookPlan({
        ...payload,
        requested_by: payload.requested_by || "operator",
        reason: payload.reason || "review skill anomaly before audit/trust write-back",
      });
      setAuditPlan(res.plan);
      setResult(res.plan.detection);
      setApprovalWriteback(null);
      setApprovalBridgePlan(null);
      showToast(res.plan.approval_required ? "已生成 audit hook / Trust mutation 审批计划" : "当前候选不需要写回计划", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 audit hook / Trust mutation 计划失败"));
    } finally {
      setBusy(null);
    }
  };

  const writeApprovalQueue = async () => {
    setBusy("approval-writeback");
    setError(null);
    try {
      const payload = JSON.parse(candidateJSON);
      const res = await skillAnomalyPack.approvalQueueWriteback({
        ...payload,
        requested_by: payload.requested_by || "operator",
        reason: payload.reason || "persist pack-local approval queue record before audit/trust wiring",
      });
      setApprovalWriteback(res.writeback);
      setAuditPlan(res.writeback.plan_summary);
      setResult(res.writeback.plan_summary.detection);
      setApprovalBridgePlan(null);
      await load();
      showToast("已写入 pack-local Approval queue store", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "写入 pack-local Approval queue 失败"));
    } finally {
      setBusy(null);
    }
  };

  const planApprovalManagerBridge = async () => {
    setBusy("approval-bridge-plan");
    setError(null);
    try {
      const payload = JSON.parse(candidateJSON);
      const requestID = approvalWriteback?.request_id || (typeof payload.request_id === "string" ? payload.request_id : "");
      const requestKey = approvalWriteback?.request_key || (typeof payload.request_key === "string" ? payload.request_key : "");
      const res = await skillAnomalyPack.approvalManagerBridgePlan({
        ...payload,
        requested_by: payload.requested_by || "operator",
        reason: payload.reason || "map pack-local approval queue record to global Approval Manager request",
        ...(requestID ? { request_id: requestID } : {}),
        ...(requestKey ? { request_key: requestKey } : {}),
      });
      setApprovalBridgePlan(res.plan);
      setAuditPlan(res.plan.plan_summary);
      setResult(res.plan.detection);
      showToast("已生成 Approval Manager bridge plan", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 Approval Manager bridge plan 失败"));
    } finally {
      setBusy(null);
    }
  };

  const exportEvidence = async () => {
    const target = skillSlug || selectedProfile?.skill_slug;
    if (!target) return;
    setBusy("evidence");
    setError(null);
    try {
      const evidence = await skillAnomalyPack.evidence(target);
      const blob = new Blob([JSON.stringify(evidence, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${target}-skill-anomaly-evidence.json`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      showToast("Skill 异常证据包已导出", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "导出 Skill 异常证据包失败"));
    } finally {
      setBusy(null);
    }
  };

  if (loading) {
    return <div className="flex h-[60vh] items-center justify-center"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader icon={<Radar size={20} />} title="Skill 行为异常" />

      <PackAbout
        chips={<>
          <Chip size="sm" color="warning">实验中</Chip>
          <Chip size="sm" variant="soft">可检测异常</Chip>
          <Chip size="sm" variant="soft">审批只生成计划</Chip>
        </>}
        description="它用于观察 Skill 的正常行为，并在出现越权参数、异常动作或失败模式时生成 NeedsApproval 计划。当前可以写入行为样本、dry-run 检测异常、导出证据包和生成审批桥接计划；真实 Trust 扣分、全局审批入队和 runtime action 放行仍未自动执行。"
        boundaries={boundaryItems}
      />

      <Card variant="default">
        <Card.Header className="flex-row flex-wrap items-center justify-between gap-2">
          <PackSectionTitle icon={<Radar size={15} />} tone="accent">怎么用</PackSectionTitle>
          <Button size="sm" variant="ghost" onPress={load}><RefreshCw size={14} />刷新</Button>
        </Card.Header>
        <Card.Content className="flex flex-col gap-4">
          <PackStepsGrid steps={userFacingSteps} columns={3} />
          <div className="flex flex-wrap items-center gap-2">
            <Chip size="sm" color={status?.audit_hook_ready ? "success" : "warning"}>
              {status?.audit_hook_ready ? "Audit hook ready" : status?.audit_hook_plan_ready ? "Audit plan ready" : "Pack shell"}
            </Chip>
            <span className="font-mono text-xs text-muted">{status?.pack_id || "yunque.pack.skill-anomaly"}</span>
            <span className="text-xs text-muted">stage {status?.stage || "pack-shell"}</span>
          </div>
          <div className="text-sm leading-6 text-muted">
            当前切片先把 skill 行为基线、滑动窗口异常评分、NeedsApproval 计划、audit hook / Trust mutation 审批计划、pack-local queue 写回、Approval Manager bridge plan 和证据包放进可选 Pack。队列写回只落到 pack-local approval-queue-store.json；bridge plan 只生成 approval-manager-bridge-plan.json，不调用全局 Approval Manager；Merkle Chain append、Trust Score 扣分和 runtime action release 仍保持后续接入。
          </div>
        </Card.Content>
      </Card>

      <Card variant="default">
        <Card.Header className="flex-row flex-wrap items-start justify-between gap-3">
          <div className="flex flex-col gap-1">
            <PackSectionTitle icon={<ShieldAlert size={15} />} tone="accent">从异常检测到人工审批</PackSectionTitle>
            <span className="text-xs leading-5 text-muted">Skill Anomaly 把可疑调用整理成可审查证据：先解释为什么异常，再由你决定是否进入审批、收紧权限或继续训练画像。</span>
          </div>
          <div className="flex flex-wrap gap-2">
            <Link href={chatPromptHref("请根据 Skill Anomaly 最新检测结果，解释可疑调用为什么需要审批，并把确认权限、收紧规则和补样本拆成任务。")}>
              <Button size="sm" className="btn-accent">
                <Send size={13} /> 带回 Chat
              </Button>
            </Link>
            <Link href="/missions">
              <Button size="sm" variant="outline">
                <ClipboardList size={13} /> 看任务
              </Button>
            </Link>
          </div>
        </Card.Header>
        <Card.Content className="flex flex-col gap-3">
          <PackStepsGrid steps={workflowLoopItems} columns={4} />
          <div className="flex flex-wrap gap-2">
            <Link href="/trace"><Button size="sm" variant="ghost">核对执行轨迹</Button></Link>
            <Link href="/packs/studio?packId=yunque.pack.skill-anomaly"><Button size="sm" variant="ghost">让小羽继续改</Button></Link>
          </div>
        </Card.Content>
      </Card>

      {error && (
        <Card variant="secondary">
          <Card.Content className="flex items-center gap-2 text-sm text-danger"><AlertTriangle size={16} />{error}</Card.Content>
        </Card>
      )}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <Card className="section-card p-4"><div className="kpi-label">画像数量</div><div className="kpi-value">{status?.profile_count ?? profiles.length}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">活跃画像</div><div className="kpi-value">{status?.active_profiles ?? profiles.filter((p) => p.observed > 0).length}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">异常次数</div><div className="kpi-value">{status?.anomaly_count ?? profiles.reduce((sum, p) => sum + (p.anomaly_count || 0), 0)}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">队列记录</div><div className="kpi-value">{status?.approval_queue_store?.record_count ?? 0}</div></Card>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <Card className="section-card p-4">
          <div className="kpi-label">Audit hook 计划</div>
          <div className="mt-2"><Chip size="sm" color={status?.audit_hook_plan_ready ? "success" : "default"}>{status?.audit_hook_plan_ready ? "plan ready" : "not ready"}</Chip></div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Trust mutation 计划</div>
          <div className="mt-2"><Chip size="sm" color={status?.trust_mutation_plan_ready ? "success" : "default"}>{status?.trust_mutation_plan_ready ? "plan ready" : "not ready"}</Chip></div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">真实写回</div>
          <div className="mt-2"><Chip size="sm" color={status?.approval_queue_store_ready ? "success" : "warning"}>{status?.approval_queue_store_ready ? "queue ready" : "plan-only"}</Chip></div>
          <div className="mt-2 text-xs text-muted">仅 pack-local JSON store</div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Approval Manager bridge</div>
          <div className="mt-2"><Chip size="sm" color={status?.approval_manager_bridge_plan_ready ? "success" : "default"}>{status?.approval_manager_bridge_plan_ready ? "bridge plan ready" : "not ready"}</Chip></div>
          <div className="mt-2 text-xs text-muted">global enqueue: {status?.global_approval_enqueue_ready ? "ready" : "blocked"}</div>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_1fr]">
        <Card className="section-card overflow-hidden">
          <div className="flex items-center justify-between border-b px-4 py-3 border-border">
            <div className="flex items-center gap-2 text-sm font-semibold"><Activity size={16} />Skill 行为画像</div>
            <Chip size="sm">{profiles.length}</Chip>
          </div>
          <div className="max-h-[520px] divide-y overflow-auto border-border">
            {profiles.length === 0 ? <div className="p-6 text-center text-sm text-muted">还没有画像。可以先写入几条正常行为样本。</div> : profiles.map((profile) => (
              <button key={profile.skill_slug} onClick={() => setSkillSlug(profile.skill_slug)} className="block w-full px-4 py-3 text-left hover:bg-white/5">
                <div className="flex items-center justify-between gap-2"><div className="font-medium">{profile.skill_slug}</div><Chip size="sm">{profile.observed} obs</Chip></div>
                <div className="mt-1 truncate text-xs text-muted">success {Math.round((profile.success_rate || 0) * 100)}% · anomalies {profile.anomaly_count || 0}</div>
              </button>
            ))}
          </div>
        </Card>

        <div className="space-y-4">
          <Card className="section-card p-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-sm font-semibold"><Activity size={16} />写入基线事件</div>
              <TextField className="w-56" value={skillSlug} onChange={(value) => { setSkillSlug(value); setNormalJSON(sampleEvent(value)); setCandidateJSON(sampleEvent(value, true)); }}>
                <Label>Skill 标识</Label>
                <Input placeholder="skill slug" />
              </TextField>
            </div>
            <TextField value={normalJSON} onChange={setNormalJSON}>
              <Label>正常行为事件 JSON</Label>
              <TextArea rows={8} aria-label="Skill anomaly observation JSON" className="font-mono text-xs" />
            </TextField>
            <div className="mt-3 flex justify-end"><Button className="btn-accent" isPending={busy === "observe"} onPress={observeEvent}>写入 / 校验事件</Button></div>
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold"><ShieldAlert size={16} />异常检测计划</div>
                <div className="mt-1 text-xs text-muted">目标画像：{selectedProfile?.skill_slug || skillSlug}</div>
              </div>
              <div className="flex gap-2">
                <Button variant="outline" isPending={busy === "evidence"} onPress={exportEvidence} isDisabled={!selectedProfile && !skillSlug}><Download size={14} />导出证据包</Button>
                <Button variant="outline" isPending={busy === "audit-plan"} onPress={planAuditHook}><ClipboardCheck size={14} />写回计划</Button>
                <Button variant="outline" isPending={busy === "approval-writeback"} onPress={writeApprovalQueue}><ClipboardCheck size={14} />写入审批队列</Button>
                <Button variant="outline" isPending={busy === "approval-bridge-plan"} onPress={planApprovalManagerBridge}><ClipboardCheck size={14} />全局审批桥计划</Button>
                <Button className="btn-accent" isPending={busy === "detect"} onPress={detectCandidate}>Dry-run 检测</Button>
              </div>
            </div>
            <TextField value={candidateJSON} onChange={setCandidateJSON}>
              <Label>候选行为 JSON</Label>
              <TextArea rows={7} aria-label="Skill anomaly candidate JSON" className="font-mono text-xs" />
            </TextField>
            {result ? (
              <Card className="mt-3 p-3 bg-surface-secondary">
                <div className="mb-2 flex items-center gap-2 text-sm font-medium"><Chip size="sm" color={severityChipColor}>{result.severity}</Chip><span>score {result.score}</span></div>
                <JsonViewer title="检测结果 JSON" value={result} rows={12} />
              </Card>
            ) : (
              <div className="mt-3 rounded-xl border border-dashed p-6 text-center text-sm border-border text-muted">基线达到最小观测数后，可以生成候选行为的 NeedsApproval / block 计划。</div>
            )}
            {auditPlan ? (
              <Card className="mt-3 p-3 bg-surface-secondary">
                <div className="mb-2 flex items-center gap-2 text-sm font-medium">
                  <ClipboardCheck size={15} />
                  <span>Audit hook / Trust mutation 计划</span>
                  <Chip size="sm" color={auditPlan.approval_required ? "warning" : "success"}>{auditPlan.status}</Chip>
                </div>
                <div className="mb-2 text-xs text-muted">
                  非破坏性预览：计划路由不会写 Merkle Chain，不追加 Merkle Chain，不会扣 Trust Score，也不会释放 runtime action；真实队列按钮只写 pack-local approval-queue-store.json，不接全局审批中心。
                </div>
                <JsonViewer title="Audit hook 计划 JSON" value={auditPlan} rows={12} />
              </Card>
            ) : null}
            {approvalWriteback ? (
              <Card className="mt-3 p-3 bg-surface-secondary">
                <div className="mb-2 flex flex-wrap items-center gap-2 text-sm font-medium">
                  <ClipboardCheck size={15} />
                  <span>Pack-local Approval queue 写回</span>
                  <Chip size="sm" color="success">approval-queue-store.json</Chip>
                  <Chip size="sm">records {approvalWriteback.approval_queue_store.record_count}</Chip>
                </div>
                <div className="mb-2 text-xs text-muted">
                  已生成 approval-queue-record.json 语义证据；不会追加 Merkle Chain，不会扣 Trust Score，不会批准或释放 runtime action，也不接全局 Approval Manager。
                </div>
                <JsonViewer title="审批队列写回 JSON" value={approvalWriteback} rows={12} />
              </Card>
            ) : null}
            {approvalBridgePlan ? (
              <Card className="mt-3 p-3 bg-surface-secondary">
                <div className="mb-2 flex flex-wrap items-center gap-2 text-sm font-medium">
                  <ClipboardCheck size={15} />
                  <span>Approval Manager bridge plan</span>
                  <Chip size="sm" color="default">approval-manager-bridge-plan.json</Chip>
                  <Chip size="sm">{approvalBridgePlan.proposed_global_approval_request.category}</Chip>
                  <Chip size="sm">risk {approvalBridgePlan.proposed_global_approval_request.risk_level}</Chip>
                </div>
                <div className="mb-2 text-xs text-muted">
                  该桥接计划只把 pack-local approval-queue-record.json 映射成未来全局 Approval Manager 请求形状；global_approval_enqueue_ready=false，不调用全局审批中心，不追加 Merkle Chain，不扣 Trust Score，也不会释放 runtime action。
                </div>
                <JsonViewer title="审批中心桥计划 JSON" value={approvalBridgePlan} rows={12} />
              </Card>
            ) : null}
          </Card>
        </div>
      </div>
    </div>
  );
}
