"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  Button,
  Card,
  Chip,
  Input,
  Spinner,
  TextArea,
  TextField,
} from "@heroui/react";
import {
  Activity,
  AlertTriangle,
  CalendarClock,
  ClipboardList,
  Download,
  Play,
  RefreshCw,
  Send,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref } from "@/lib/pack-action-links";
import {
  createChaosProbePackClient,
  type ChaosProbeDefinition,
  type ChaosProbeDegradeStateEnginePlan,
  type ChaosProbeDegradeStateWriteback,
  type ChaosProbeReport,
  type ChaosProbeReportSummary,
  type ChaosProbeSchedulerPlan,
  type ChaosProbeStatus,
} from "@/lib/chaos-probe-pack-client";

const chaosProbePack = createChaosProbePackClient();

function sampleDefinitions() {
  return JSON.stringify(
    {
      probes: [
        {
          id: "runtime-healthz-probe",
          name: "Runtime healthz probe",
          category: "network",
          description: "Verify local runtime handler responsiveness.",
          safe: true,
          enabled: true,
          interval_seconds: 30,
          weight: 0.2,
          tags: ["healthz", "safe"],
        },
        {
          id: "guardrail-probe",
          name: "Guardrail known-payload probe",
          category: "guard",
          description:
            "Run a known prompt-injection payload through the existing guardrail detector.",
          safe: true,
          enabled: true,
          interval_seconds: 300,
          weight: 0.25,
          tags: ["guardrails", "cognitive"],
        },
      ],
      replace: false,
    },
    null,
    2,
  );
}

function gateTone(gate?: string): { bg: string; fg: string } {
  switch (gate) {
    case "fail":
      return { bg: "rgba(239,68,68,0.16)", fg: "#ef4444" };
    case "warn":
      return { bg: "rgba(250,204,21,0.14)", fg: "#facc15" };
    case "pass":
      return { bg: "rgba(34,197,94,0.12)", fg: "#22c55e" };
    default:
      return { bg: "rgba(56,189,248,0.12)", fg: "#38bdf8" };
  }
}

const userFacingSteps = [
  {
    title: "1. 准备安全探针",
    body: "维护只读 health、guardrail 或 runtime 检查项，明确每个探针的安全范围。",
  },
  {
    title: "2. 运行一次演练",
    body: "生成健康报告、失败原因和降级建议，判断系统是否仍能稳住。",
  },
  {
    title: "3. 输出运行计划",
    body: "生成调度、指标、告警和降级状态引擎的交接计划。",
  },
];

const boundaryItems = [
  "不会破坏生产环境或注入真实故障。",
  "不会创建后台定时任务。",
  "不会发送告警或发布 Prometheus 指标。",
  "不会写入真实 runtime degrade-state engine。",
];

const workflowLoopItems = [
  {
    title: "1. 准备探针",
    body: "把 health、guardrail 和关键链路检查写成安全探针，先限定影响范围。",
  },
  {
    title: "2. 带回 Chat",
    body: "让云雀解释失败原因、降级等级和修复顺序，再拆成可以执行的任务。",
  },
  {
    title: "3. 看证据位置",
    body: "报告、调度计划和本地降级状态是演练证据，不会直接改线上状态。",
  },
  {
    title: "4. 继续补能力",
    body: "如果探针太少或误报，把真实报告交给小羽补检查项和判断规则。",
  },
];

export default function ChaosProbePackPage() {
  const [status, setStatus] = useState<ChaosProbeStatus | null>(null);
  const [reports, setReports] = useState<ChaosProbeReportSummary[]>([]);
  const [probes, setProbes] = useState<ChaosProbeDefinition[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<
    | "probes"
    | "run"
    | "scheduler"
    | "degrade"
    | "engine"
    | "evidence"
    | null
  >(null);
  const [error, setError] = useState<string | null>(null);
  const [definitionJSON, setDefinitionJSON] = useState(sampleDefinitions);
  const [probeIDs, setProbeIDs] = useState(
    "runtime-healthz-probe,guardrail-probe",
  );
  const [report, setReport] = useState<ChaosProbeReport | null>(null);
  const [schedulerPlan, setSchedulerPlan] =
    useState<ChaosProbeSchedulerPlan | null>(null);
  const [degradeWriteback, setDegradeWriteback] =
    useState<ChaosProbeDegradeStateWriteback | null>(null);
  const [enginePlan, setEnginePlan] =
    useState<ChaosProbeDegradeStateEnginePlan | null>(null);

  const selectedReport = useMemo(() => report || null, [report]);
  const tone = gateTone(selectedReport?.gate_status || reports[0]?.gate_status);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [statusRes, probesRes, reportsRes] = await Promise.all([
        chaosProbePack.status(),
        chaosProbePack.probes(),
        chaosProbePack.reports(),
      ]);
      setStatus(statusRes);
      setProbes(probesRes.probes || []);
      setReports(reportsRes.reports || []);
    } catch (e) {
      const msg = formatErrorMessage(e, "加载 Chaos Probe Pack 失败");
      setError(
        msg.includes("pack route is not enabled")
          ? "Chaos Probe Pack 当前未启用。请到「能力包」控制台启用 yunque.pack.chaos-probe 后再使用。"
          : msg,
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const saveProbes = async () => {
    setBusy("probes");
    setError(null);
    try {
      const payload = JSON.parse(definitionJSON);
      const res = await chaosProbePack.saveProbes(payload);
      showToast(
        `Chaos probe definitions 已保存：${res.count} 个 probe`,
        "success",
      );
      await load();
    } catch (e) {
      setError(formatErrorMessage(e, "保存 Chaos probe definitions 失败"));
    } finally {
      setBusy(null);
    }
  };

  const runProbes = async () => {
    setBusy("run");
    setError(null);
    try {
      const ids = probeIDs
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean);
      const res = await chaosProbePack.run({
        probe_ids: ids.length ? ids : undefined,
        persist: true,
        metadata: { source: "web-pack" },
      });
      setReport(res.report);
      setSchedulerPlan(null);
      setDegradeWriteback(null);
      setEnginePlan(null);
      showToast(
        res.report.gate_status === "fail"
          ? "Chaos Probe 发现故障风险，已生成报告"
          : "Chaos Probe 报告已生成",
        res.report.gate_status === "fail" ? "warning" : "success",
      );
      await load();
    } catch (e) {
      setError(formatErrorMessage(e, "运行 Chaos Probe 失败"));
    } finally {
      setBusy(null);
    }
  };

  const exportEvidence = async () => {
    const id = selectedReport?.id || reports[0]?.id;
    if (!id) return;
    setBusy("evidence");
    setError(null);
    try {
      const evidence = await chaosProbePack.evidence(id);
      const blob = new Blob([JSON.stringify(evidence, null, 2)], {
        type: "application/json",
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${id}-chaos-probe-evidence.json`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      showToast("Chaos Probe 证据包已导出", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "导出 Chaos Probe 证据包失败"));
    } finally {
      setBusy(null);
    }
  };

  const planScheduler = async () => {
    setBusy("scheduler");
    setError(null);
    try {
      const res = await chaosProbePack.schedulerPlan({
        report_id: selectedReport?.id || reports[0]?.id,
        interval: "5m",
        requested_by: "pack-console",
      });
      setSchedulerPlan(res.plan);
      showToast(
        "已生成 scheduler / metrics / alert write-back 计划",
        "success",
      );
    } catch (e) {
      setError(formatErrorMessage(e, "生成 Chaos Probe 调度计划失败"));
    } finally {
      setBusy(null);
    }
  };

  const writeDegradeState = async () => {
    const id = selectedReport?.id || reports[0]?.id;
    if (!id) return;
    setBusy("degrade");
    setError(null);
    try {
      const res = await chaosProbePack.writeDegradeState({
        report_id: id,
        requested_by: "pack-console",
        reason:
          "persist pack-local degrade-state summary for later runtime engine handoff",
        metadata: { source: "web-pack" },
      });
      setDegradeWriteback(res.writeback);
      setEnginePlan(null);
      showToast(
        "已写入 pack-local degrade-state store（未写 runtime 状态）",
        "success",
      );
      await load();
    } catch (e) {
      setError(
        formatErrorMessage(e, "写入 pack-local degrade-state store 失败"),
      );
    } finally {
      setBusy(null);
    }
  };

  const planDegradeEngine = async () => {
    const id = selectedReport?.id || reports[0]?.id;
    if (!id) return;
    setBusy("engine");
    setError(null);
    try {
      const res = await chaosProbePack.degradeEnginePlan({
        report_id: id,
        requested_by: "pack-console",
        reason:
          "plan runtime degrade engine handoff from pack-local degrade-state store",
        metadata: { source: "web-pack" },
      });
      setEnginePlan(res.plan);
      showToast(
        "已生成 runtime degrade engine handoff 计划（未写 runtime 状态）",
        "success",
      );
    } catch (e) {
      setError(
        formatErrorMessage(
          e,
          "生成 runtime degrade engine handoff 计划失败。请先写本地降级状态。",
        ),
      );
    } finally {
      setBusy(null);
    }
  };

  if (loading) {
    return (
      <div className="flex h-[60vh] items-center justify-center">
        <Spinner size="lg" />
      </div>
    );
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader icon={<Activity size={20} />} title="Chaos Probe" />

      <Card className="section-card overflow-hidden p-0">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="p-5">
            <div className="flex flex-wrap items-center gap-2">
              <Chip size="sm" style={{ background: "rgba(245,158,11,0.12)", color: "var(--yunque-warning)" }}>实验中</Chip>
              <Chip size="sm" variant="soft">只跑安全探针</Chip>
              <Chip size="sm" variant="soft">降级只生成计划</Chip>
            </div>
            <div className="mt-3 text-base font-semibold" style={{ color: "var(--yunque-text)" }}>
              这个能力包现在适合做什么
            </div>
            <div className="mt-2 max-w-3xl text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
              它用于用安全探针检查云雀运行时、护栏和关键链路是否健康，帮助你在真正事故前看到降级建议。当前可以保存 probe definitions、运行 one-shot 检查、查看健康报告并导出证据；真实后台调度、指标发布、告警发送和运行时降级写入仍是计划。
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
              <Chip
                size="sm"
                style={{
                  background: status?.scheduler_ready
                    ? "rgba(34,197,94,0.12)"
                    : "rgba(250,204,21,0.12)",
                  color: status?.scheduler_ready ? "#22c55e" : "#facc15",
                }}
              >
                {status?.scheduler_ready ? "Scheduler ready" : "Pack shell"}
              </Chip>
              <span
                className="text-xs"
                style={{ color: "var(--yunque-text-muted)" }}
              >
                {status?.pack_id || "yunque.pack.chaos-probe"}
              </span>
            </div>
            <div
              className="text-sm"
              style={{ color: "var(--yunque-text-muted)" }}
            >
              当前切片已把安全探针 registry、one-shot
              run、健康评分、降级建议、scheduler/metrics/alert write-back
              计划、pack-local degrade-state store 和证据包放进可选
              Pack。真实后台调度、Prometheus metrics 发布、告警发送和 runtime
              degrade-state engine 后续接入。
            </div>
          </div>
          <Button size="sm" variant="ghost" onPress={load}>
            <RefreshCw size={14} />
            刷新
          </Button>
        </div>
      </Card>

      <Card className="section-card p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>从安全探针到修复任务</div>
            <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
              Chaos Probe 的目标不是制造事故，而是用可控探针提前发现薄弱点，把报告变成任务、证据和后续能力补强。
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Link href={chatPromptHref("请根据 Chaos Probe 的最新健康报告，解释风险等级、优先修复项和需要观察的指标，并拆成任务。")}>
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
        </div>
        <div className="mt-3 grid gap-2 md:grid-cols-4">
          {workflowLoopItems.map((item) => (
            <div key={item.title} className="rounded-md border p-3" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-surface)" }}>
              <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>{item.title}</div>
              <div className="mt-2 text-[11px] leading-5" style={{ color: "var(--yunque-text-muted)" }}>{item.body}</div>
            </div>
          ))}
        </div>
        <div className="mt-3 flex flex-wrap gap-2 text-xs">
          <Link href="/trace"><Button size="sm" variant="ghost">核对执行轨迹</Button></Link>
          <Link href="/packs/studio?packId=yunque.pack.chaos-probe"><Button size="sm" variant="ghost">让小羽继续改</Button></Link>
        </div>
      </Card>

      {error && (
        <Card className="p-4" style={{ background: "rgba(239,68,68,0.06)" }}>
          <div
            className="flex items-center gap-2 text-sm"
            style={{ color: "var(--yunque-danger)" }}
          >
            <AlertTriangle size={16} />
            {error}
          </div>
        </Card>
      )}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-5">
        <Card className="section-card p-4">
          <div className="kpi-label">Safe probes</div>
          <div className="kpi-value">
            {status?.probe_count ?? probes.length}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Reports</div>
          <div className="kpi-value">
            {status?.report_count ?? reports.length}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Health</div>
          <div className="kpi-value">
            {Math.round(
              selectedReport?.health_score ??
                reports[0]?.health_score ??
                status?.last_report?.health_score ??
                100,
            )}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Scheduler Plan</div>
          <div className="kpi-value text-lg" style={{ color: tone.fg }}>
            {status?.scheduler_plan_ready ? "plan" : "pending"}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Degrade Store</div>
          <div
            className="kpi-value text-lg"
            style={{
              color: status?.runtime_degrade_state_ready
                ? "#22c55e"
                : "#38bdf8",
            }}
          >
            {status?.degrade_state_store_ready ? "local" : "pending"}
          </div>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[380px_1fr]">
        <Card className="section-card overflow-hidden">
          <div
            className="flex items-center justify-between border-b px-4 py-3"
            style={{ borderColor: "var(--yunque-border)" }}
          >
            <div className="flex items-center gap-2 text-sm font-semibold">
              <Sparkles size={16} />
              健康报告
            </div>
            <Chip size="sm">{reports.length}</Chip>
          </div>
          <div
            className="max-h-[520px] divide-y overflow-auto"
            style={{ borderColor: "var(--yunque-border)" }}
          >
            {reports.length === 0 ? (
              <div
                className="p-6 text-center text-sm"
                style={{ color: "var(--yunque-text-muted)" }}
              >
                还没有报告。可以先保存 probe definitions 并运行一次探针。
              </div>
            ) : (
              reports.map((item) => (
                <button
                  key={item.id}
                  onClick={async () =>
                    setReport((await chaosProbePack.report(item.id)).report)
                  }
                  className="block w-full px-4 py-3 text-left hover:bg-white/5"
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="font-medium">{item.id}</div>
                    <Chip
                      size="sm"
                      style={{
                        background: gateTone(item.gate_status).bg,
                        color: gateTone(item.gate_status).fg,
                      }}
                    >
                      {item.gate_status}
                    </Chip>
                  </div>
                  <div
                    className="mt-1 truncate text-xs"
                    style={{ color: "var(--yunque-text-muted)" }}
                  >
                    health {Math.round(item.health_score)} · pass{" "}
                    {item.pass_count} · fail {item.fail_count} · L
                    {item.degrade_level}
                  </div>
                </button>
              ))
            )}
          </div>
        </Card>

        <div className="space-y-4">
          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-sm font-semibold">
                <ShieldCheck size={16} />
                Probe definitions
              </div>
              <Button
                variant="outline"
                isPending={busy === "probes"}
                onPress={saveProbes}
              >
                保存 Definitions
              </Button>
            </div>
            <TextField value={definitionJSON} onChange={setDefinitionJSON}>
              <TextArea
                rows={9}
                aria-label="Chaos probe definitions JSON"
                className="font-mono text-xs"
              />
            </TextField>
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <Play size={16} />
                  Safe probe run
                </div>
                <div
                  className="mt-1 text-xs"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  本阶段不创建真实后台任务；degrade write-back 只写 pack-local
                  store，不写 runtime degrade state。
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <TextField
                  className="min-w-64"
                  value={probeIDs}
                  onChange={setProbeIDs}
                >
                  <Input placeholder="probe ids" />
                </TextField>
                <Button
                  variant="outline"
                  isPending={busy === "scheduler"}
                  onPress={planScheduler}
                >
                  <CalendarClock size={14} />
                  调度计划
                </Button>
                <Button
                  variant="outline"
                  isPending={busy === "degrade"}
                  onPress={writeDegradeState}
                  isDisabled={!selectedReport && reports.length === 0}
                >
                  写本地降级状态
                </Button>
                <Button
                  variant="outline"
                  isPending={busy === "engine"}
                  onPress={planDegradeEngine}
                  isDisabled={!selectedReport && reports.length === 0}
                >
                  Engine handoff 计划
                </Button>
                <Button
                  variant="outline"
                  isPending={busy === "evidence"}
                  onPress={exportEvidence}
                  isDisabled={!selectedReport && reports.length === 0}
                >
                  <Download size={14} />
                  导出证据包
                </Button>
                <Button
                  className="btn-accent"
                  isPending={busy === "run"}
                  onPress={runProbes}
                >
                  运行探针
                </Button>
              </div>
            </div>

            {selectedReport ? (
              <Card
                className="p-3"
                style={{ background: "rgba(255,255,255,0.03)" }}
              >
                <div className="mb-2 flex items-center gap-2 text-sm font-medium">
                  <Chip
                    size="sm"
                    style={{ background: tone.bg, color: tone.fg }}
                  >
                    {selectedReport.gate_status}
                  </Chip>
                  <span>{selectedReport.id}</span>
                </div>
                <TextField
                  value={JSON.stringify(selectedReport, null, 2)}
                  onChange={() => undefined}
                >
                  <TextArea
                    rows={18}
                    aria-label="Chaos probe report JSON"
                    className="font-mono text-xs"
                    readOnly
                  />
                </TextField>
              </Card>
            ) : (
              <div
                className="rounded-xl border border-dashed p-6 text-center text-sm"
                style={{
                  borderColor: "var(--yunque-border)",
                  color: "var(--yunque-text-muted)",
                }}
              >
                运行后会展示 health score / degrade level / remediation 细节。
              </div>
            )}
          </Card>

          {schedulerPlan && (
            <Card className="section-card p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-semibold">
                <CalendarClock size={16} />
                Scheduler / Metrics / Alert write-back 计划
              </div>
              <div className="mb-3 flex flex-wrap gap-2">
                <Chip size="sm">{schedulerPlan.status}</Chip>
                <Chip size="sm">interval: {schedulerPlan.interval}</Chip>
                <Chip size="sm">
                  scheduler_ready: {String(schedulerPlan.scheduler_ready)}
                </Chip>
                <Chip size="sm">
                  prometheus_ready: {String(schedulerPlan.prometheus_ready)}
                </Chip>
              </div>
              <TextField
                value={JSON.stringify(schedulerPlan, null, 2)}
                onChange={() => undefined}
              >
                <TextArea
                  rows={12}
                  aria-label="Chaos Probe Scheduler Plan JSON"
                  className="font-mono text-xs"
                  readOnly
                />
              </TextField>
              <div
                className="mt-2 text-xs"
                style={{ color: "var(--yunque-text-muted)" }}
              >
                非破坏性计划：不会创建 scheduler job、不会发布 Prometheus
                指标、不会发送告警，也不会写入 runtime degrade state。
              </div>
            </Card>
          )}

          {degradeWriteback && (
            <Card className="section-card p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-semibold">
                <AlertTriangle size={16} />
                Pack-local degrade-state write-back
              </div>
              <div className="mb-3 flex flex-wrap gap-2">
                <Chip size="sm">{degradeWriteback.status}</Chip>
                <Chip size="sm">
                  store: {String(degradeWriteback.degrade_state_store_ready)}
                </Chip>
                <Chip size="sm">
                  writes_store:{" "}
                  {String(degradeWriteback.writes_degrade_state_store)}
                </Chip>
                <Chip size="sm">
                  runtime_ready:{" "}
                  {String(degradeWriteback.runtime_degrade_state_ready)}
                </Chip>
              </div>
              <TextField
                value={JSON.stringify(degradeWriteback, null, 2)}
                onChange={() => undefined}
              >
                <TextArea
                  rows={12}
                  aria-label="Chaos Probe Degrade State Writeback JSON"
                  className="font-mono text-xs"
                  readOnly
                />
              </TextField>
              <div
                className="mt-2 text-xs"
                style={{ color: "var(--yunque-text-muted)" }}
              >
                已持久化到 pack-local `degrade-state-store.json`；不会修改
                runtime degrade state、不会触发降级状态机、不会发布 Prometheus
                指标或发送 alert。
              </div>
            </Card>
          )}

          {enginePlan && (
            <Card className="section-card p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-semibold">
                <ShieldCheck size={16} />
                Runtime degrade engine handoff plan
              </div>
              <div className="mb-3 flex flex-wrap gap-2">
                <Chip size="sm">{enginePlan.status}</Chip>
                <Chip size="sm">
                  consumes_store:{" "}
                  {String(enginePlan.consumes_degrade_state_store)}
                </Chip>
                <Chip size="sm">
                  writes_runtime:{" "}
                  {String(enginePlan.writes_runtime_degrade_state)}
                </Chip>
                <Chip size="sm">
                  merkle_append: {String(enginePlan.merkle_append_ready)}
                </Chip>
              </div>
              <TextField
                value={JSON.stringify(enginePlan, null, 2)}
                onChange={() => undefined}
              >
                <TextArea
                  rows={12}
                  aria-label="Chaos Probe Degrade Engine Plan JSON"
                  className="font-mono text-xs"
                  readOnly
                />
              </TextField>
              <div
                className="mt-2 text-xs"
                style={{ color: "var(--yunque-text-muted)" }}
              >
                `degrade-engine-plan.json` 只把 pack-local
                degrade-state-record 映射为后续 runtime engine
                消费契约；`runtime_degrade_state_ready=false`、
                `writes_runtime_degrade_state=false`、`merkle_append_ready=false`
                表示本切片不写 runtime 状态，也不追加 Merkle audit chain。
              </div>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}
