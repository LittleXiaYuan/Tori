"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Card, Chip, Input, Spinner, TextArea, TextField } from "@heroui/react";
import { Activity, AlertTriangle, ClipboardCheck, Download, Radar, RefreshCw, ShieldAlert } from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import { createSkillAnomalyPackClient, type SkillAnomalyAuditHookPlan, type SkillAnomalyProfileSummary, type SkillAnomalyResult, type SkillAnomalyStatus } from "@/lib/skill-anomaly-pack-client";

const skillAnomalyPack = createSkillAnomalyPackClient();

function severityTone(severity?: string): { bg: string; fg: string } {
  switch (severity) {
    case "block": return { bg: "rgba(239,68,68,0.16)", fg: "#ef4444" };
    case "needs_approval": return { bg: "rgba(250,204,21,0.14)", fg: "#facc15" };
    case "learning": return { bg: "rgba(56,189,248,0.12)", fg: "#38bdf8" };
    default: return { bg: "rgba(34,197,94,0.12)", fg: "#22c55e" };
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

export default function SkillAnomalyPackPage() {
  const [status, setStatus] = useState<SkillAnomalyStatus | null>(null);
  const [profiles, setProfiles] = useState<SkillAnomalyProfileSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"observe" | "detect" | "audit-plan" | "evidence" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [skillSlug, setSkillSlug] = useState("text_processing");
  const [normalJSON, setNormalJSON] = useState(() => sampleEvent("text_processing"));
  const [candidateJSON, setCandidateJSON] = useState(() => sampleEvent("text_processing", true));
  const [result, setResult] = useState<SkillAnomalyResult | null>(null);
  const [auditPlan, setAuditPlan] = useState<SkillAnomalyAuditHookPlan | null>(null);

  const selectedProfile = useMemo(() => profiles.find((profile) => profile.skill_slug === skillSlug) || profiles[0] || null, [profiles, skillSlug]);
  const tone = severityTone(result?.severity);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [statusRes, profilesRes] = await Promise.all([skillAnomalyPack.status(), skillAnomalyPack.profiles()]);
      setStatus(statusRes);
      setProfiles(profilesRes.profiles || []);
      if (!skillSlug && profilesRes.profiles?.[0]?.skill_slug) setSkillSlug(profilesRes.profiles[0].skill_slug);
    } catch (e) {
      const msg = formatErrorMessage(e, "加载 Skill Anomaly Pack 失败");
      setError(msg.includes("pack route is not enabled") ? "Skill Anomaly Pack 当前未启用。请到「增量包」控制台启用 yunque.pack.skill-anomaly 后再使用。" : msg);
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
      showToast(res.plan.approval_required ? "已生成 audit hook / Trust mutation 审批计划" : "当前候选不需要写回计划", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 audit hook / Trust mutation 计划失败"));
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

      <Card className="section-card p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="mb-1 flex items-center gap-2">
              <Chip size="sm" style={{ background: status?.audit_hook_ready ? "rgba(34,197,94,0.12)" : "rgba(250,204,21,0.12)", color: status?.audit_hook_ready ? "#22c55e" : "#facc15" }}>
                {status?.audit_hook_ready ? "Audit hook ready" : status?.audit_hook_plan_ready ? "Audit plan ready" : "Pack shell"}
              </Chip>
              <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{status?.pack_id || "yunque.pack.skill-anomaly"}</span>
            </div>
            <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>
              当前切片先把 skill 行为基线、滑动窗口异常评分、NeedsApproval 计划、audit hook / Trust mutation 审批计划和证据包放进可选 Pack。Merkle Chain 直连 append、Trust Score 真正扣分和审批队列写回仍保持后续接入。
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
        <Card className="section-card p-4"><div className="kpi-label">画像数量</div><div className="kpi-value">{status?.profile_count ?? profiles.length}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">活跃画像</div><div className="kpi-value">{status?.active_profiles ?? profiles.filter((p) => p.observed > 0).length}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">异常次数</div><div className="kpi-value">{status?.anomaly_count ?? profiles.reduce((sum, p) => sum + (p.anomaly_count || 0), 0)}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">阶段</div><div className="kpi-value text-lg">{status?.stage || "pack-shell"}</div></Card>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <Card className="section-card p-4">
          <div className="kpi-label">Audit hook 计划</div>
          <div className="mt-2"><Chip size="sm" style={{ background: status?.audit_hook_plan_ready ? "rgba(34,197,94,0.12)" : "rgba(148,163,184,0.12)", color: status?.audit_hook_plan_ready ? "#22c55e" : "var(--yunque-text-muted)" }}>{status?.audit_hook_plan_ready ? "plan ready" : "not ready"}</Chip></div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Trust mutation 计划</div>
          <div className="mt-2"><Chip size="sm" style={{ background: status?.trust_mutation_plan_ready ? "rgba(34,197,94,0.12)" : "rgba(148,163,184,0.12)", color: status?.trust_mutation_plan_ready ? "#22c55e" : "var(--yunque-text-muted)" }}>{status?.trust_mutation_plan_ready ? "plan ready" : "not ready"}</Chip></div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">真实写回</div>
          <div className="mt-2"><Chip size="sm" style={{ background: status?.approval_writeback_ready ? "rgba(34,197,94,0.12)" : "rgba(250,204,21,0.12)", color: status?.approval_writeback_ready ? "#22c55e" : "#facc15" }}>{status?.approval_writeback_ready ? "write-back ready" : "plan-only"}</Chip></div>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_1fr]">
        <Card className="section-card overflow-hidden">
          <div className="flex items-center justify-between border-b px-4 py-3" style={{ borderColor: "var(--yunque-border)" }}>
            <div className="flex items-center gap-2 text-sm font-semibold"><Activity size={16} />Skill 行为画像</div>
            <Chip size="sm">{profiles.length}</Chip>
          </div>
          <div className="max-h-[520px] divide-y overflow-auto" style={{ borderColor: "var(--yunque-border)" }}>
            {profiles.length === 0 ? <div className="p-6 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>还没有画像。可以先写入几条正常行为样本。</div> : profiles.map((profile) => (
              <button key={profile.skill_slug} onClick={() => setSkillSlug(profile.skill_slug)} className="block w-full px-4 py-3 text-left hover:bg-white/5">
                <div className="flex items-center justify-between gap-2"><div className="font-medium">{profile.skill_slug}</div><Chip size="sm">{profile.observed} obs</Chip></div>
                <div className="mt-1 truncate text-xs" style={{ color: "var(--yunque-text-muted)" }}>success {Math.round((profile.success_rate || 0) * 100)}% · anomalies {profile.anomaly_count || 0}</div>
              </button>
            ))}
          </div>
        </Card>

        <div className="space-y-4">
          <Card className="section-card p-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-sm font-semibold"><Activity size={16} />写入基线事件</div>
              <TextField className="w-56" value={skillSlug} onChange={(value) => { setSkillSlug(value); setNormalJSON(sampleEvent(value)); setCandidateJSON(sampleEvent(value, true)); }}><Input placeholder="skill slug" /></TextField>
            </div>
            <TextField value={normalJSON} onChange={setNormalJSON}>
              <TextArea rows={8} aria-label="Skill anomaly observation JSON" className="font-mono text-xs" />
            </TextField>
            <div className="mt-3 flex justify-end"><Button className="btn-accent" isPending={busy === "observe"} onPress={observeEvent}>写入 / 校验事件</Button></div>
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold"><ShieldAlert size={16} />异常检测计划</div>
                <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>目标画像：{selectedProfile?.skill_slug || skillSlug}</div>
              </div>
              <div className="flex gap-2">
                <Button variant="outline" isPending={busy === "evidence"} onPress={exportEvidence} isDisabled={!selectedProfile && !skillSlug}><Download size={14} />导出证据包</Button>
                <Button variant="outline" isPending={busy === "audit-plan"} onPress={planAuditHook}><ClipboardCheck size={14} />写回计划</Button>
                <Button className="btn-accent" isPending={busy === "detect"} onPress={detectCandidate}>Dry-run 检测</Button>
              </div>
            </div>
            <TextField value={candidateJSON} onChange={setCandidateJSON}>
              <TextArea rows={7} aria-label="Skill anomaly candidate JSON" className="font-mono text-xs" />
            </TextField>
            {result ? (
              <Card className="mt-3 p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
                <div className="mb-2 flex items-center gap-2 text-sm font-medium"><Chip size="sm" style={{ background: tone.bg, color: tone.fg }}>{result.severity}</Chip><span>score {result.score}</span></div>
                <TextField value={JSON.stringify(result, null, 2)} onChange={() => undefined}>
                  <TextArea rows={12} aria-label="Skill anomaly result JSON" className="font-mono text-xs" readOnly />
                </TextField>
              </Card>
            ) : (
              <div className="mt-3 rounded-xl border border-dashed p-6 text-center text-sm" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>基线达到最小观测数后，可以生成候选行为的 NeedsApproval / block 计划。</div>
            )}
            {auditPlan ? (
              <Card className="mt-3 p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
                <div className="mb-2 flex items-center gap-2 text-sm font-medium">
                  <ClipboardCheck size={15} />
                  <span>Audit hook / Trust mutation 计划</span>
                  <Chip size="sm" style={{ background: auditPlan.approval_required ? "rgba(250,204,21,0.14)" : "rgba(34,197,94,0.12)", color: auditPlan.approval_required ? "#facc15" : "#22c55e" }}>{auditPlan.status}</Chip>
                </div>
                <div className="mb-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  非破坏性预览：不会写 Merkle Chain，不会扣 Trust Score，不会写审批队列；这里只固定后续写回契约。
                </div>
                <TextField value={JSON.stringify(auditPlan, null, 2)} onChange={() => undefined}>
                  <TextArea rows={12} aria-label="Skill anomaly audit hook plan JSON" className="font-mono text-xs" readOnly />
                </TextField>
              </Card>
            ) : null}
          </Card>
        </div>
      </div>
    </div>
  );
}
