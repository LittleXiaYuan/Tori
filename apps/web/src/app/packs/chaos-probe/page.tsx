"use client";

import { useCallback, useEffect, useMemo, useState, type Key } from "react";
import Link from "next/link";
import {
  Button,
  Card,
  Chip,
  Label,
  ListBox,
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
import { JsonViewer } from "@/components/json-viewer";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref } from "@/lib/pack-action-links";
import { PackAbout, PackSectionTitle, PackStepsGrid, type PackBoundaryItem, type PackStep } from "@/components/packs/pack-page-kit";
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

type ChipColor = "danger" | "warning" | "success" | "default";

// Map a health gate status to a semantic Chip color (no hand-rolled rgba).
function gateColor(gate?: string): ChipColor {
  switch (gate) {
    case "fail":
      return "danger";
    case "warn":
      return "warning";
    case "pass":
      return "success";
    default:
      return "default";
  }
}

const userFacingSteps: PackStep[] = [
  { key: "prepare", label: "准备安全探针", detail: "维护只读 health、guardrail 或 runtime 检查项，明确每个探针的安全范围。" },
  { key: "run", label: "运行一次演练", detail: "生成健康报告、失败原因和降级建议，判断系统是否仍能稳住。" },
  { key: "plan", label: "输出运行计划", detail: "生成调度、指标、告警和降级状态引擎的交接计划。" },
];

const boundaryItems: PackBoundaryItem[] = [
  { key: "prod", label: "不破坏生产", detail: "不会破坏生产环境或注入真实故障。" },
  { key: "cron", label: "不建定时任务", detail: "不会创建后台定时任务。" },
  { key: "alert", label: "不发告警", detail: "不会发送告警或发布 Prometheus 指标。" },
  { key: "engine", label: "不写降级引擎", detail: "不会写入真实 runtime degrade-state engine。" },
];

const workflowLoopItems: PackStep[] = [
  { key: "prepare", label: "准备探针", detail: "把 health、guardrail 和关键链路检查写成安全探针，先限定影响范围。" },
  { key: "chat", label: "带回 Chat", detail: "让云雀解释失败原因、降级等级和修复顺序，再拆成可以执行的任务。" },
  { key: "evidence", label: "看证据位置", detail: "报告、调度计划和本地降级状态是演练证据，不会直接改线上状态。" },
  { key: "extend", label: "继续补能力", detail: "如果探针太少或误报，把真实报告交给小羽补检查项和判断规则。" },
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
  const selectedProbeIds = useMemo(
    () => new Set(probeIDs.split(",").map((item) => item.trim()).filter(Boolean)),
    [probeIDs],
  );
  const toneColor = gateColor(selectedReport?.gate_status || reports[0]?.gate_status);

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

  const updateProbeSelection = (keys: "all" | Set<Key>) => {
    if (keys === "all") {
      setProbeIDs(probes.map((probe) => probe.id).join(","));
      return;
    }
    setProbeIDs(Array.from(keys).map(String).join(","));
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

      <PackAbout
        chips={<>
          <Chip size="sm" color="warning">实验中</Chip>
          <Chip size="sm" variant="soft">只跑安全探针</Chip>
          <Chip size="sm" variant="soft">降级只生成计划</Chip>
        </>}
        description="它用于用安全探针检查云雀运行时、护栏和关键链路是否健康，帮助你在真正事故前看到降级建议。当前可以保存 probe definitions、运行 one-shot 检查、查看健康报告并导出证据；真实后台调度、指标发布、告警发送和运行时降级写入仍是计划。"
        boundaries={boundaryItems}
      />

      <Card variant="default">
        <Card.Header className="flex-row flex-wrap items-center justify-between gap-2">
          <PackSectionTitle icon={<Sparkles size={15} />} tone="accent">怎么用</PackSectionTitle>
          <Button size="sm" variant="ghost" onPress={load}>
            <RefreshCw size={14} />
            刷新
          </Button>
        </Card.Header>
        <Card.Content className="flex flex-col gap-4">
          <PackStepsGrid steps={userFacingSteps} columns={3} />
          <div className="flex items-center gap-2">
            <Chip size="sm" color={status?.scheduler_ready ? "success" : "warning"}>
              {status?.scheduler_ready ? "Scheduler ready" : "Pack shell"}
            </Chip>
            <span className="font-mono text-xs text-muted">{status?.pack_id || "yunque.pack.chaos-probe"}</span>
          </div>
          <div className="text-sm leading-6 text-muted">
            当前切片已把安全探针 registry、one-shot
            run、健康评分、降级建议、scheduler/metrics/alert write-back
            计划、pack-local degrade-state store 和证据包放进可选
            Pack。真实后台调度、Prometheus metrics 发布、告警发送和 runtime
            degrade-state engine 后续接入。
          </div>
        </Card.Content>
      </Card>

      <Card variant="default">
        <Card.Header className="flex-row flex-wrap items-start justify-between gap-3">
          <div className="flex flex-col gap-1">
            <PackSectionTitle icon={<ShieldCheck size={15} />} tone="accent">从安全探针到修复任务</PackSectionTitle>
            <span className="text-xs leading-5 text-muted">Chaos Probe 的目标不是制造事故，而是用可控探针提前发现薄弱点，把报告变成任务、证据和后续能力补强。</span>
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
        </Card.Header>
        <Card.Content className="flex flex-col gap-3">
          <PackStepsGrid steps={workflowLoopItems} columns={4} />
          <div className="flex flex-wrap gap-2">
            <Link href="/trace"><Button size="sm" variant="ghost">核对执行轨迹</Button></Link>
            <Link href="/packs/studio?packId=yunque.pack.chaos-probe"><Button size="sm" variant="ghost">让小羽继续改</Button></Link>
          </div>
        </Card.Content>
      </Card>

      {error && (
        <Card variant="secondary">
          <Card.Content className="flex items-center gap-2 text-sm text-danger">
            <AlertTriangle size={16} />
            {error}
          </Card.Content>
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
          <div className="mt-1 text-sm font-medium text-foreground">
            {status?.scheduler_plan_ready ? "已就绪" : "待接通"}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Degrade Store</div>
          <div className={`mt-1 text-sm font-medium ${status?.runtime_degrade_state_ready ? "text-success" : "text-accent"}`}>
            {status?.degrade_state_store_ready ? "已就绪" : "待接通"}
          </div>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[380px_1fr]">
        <Card className="section-card overflow-hidden">
          <div className="flex items-center justify-between border-b px-4 py-3 border-border">
            <div className="flex items-center gap-2 text-sm font-semibold">
              <Sparkles size={16} />
              健康报告
            </div>
            <Chip size="sm">{reports.length}</Chip>
          </div>
          <div className="max-h-[520px] divide-y overflow-auto border-border">
            {reports.length === 0 ? (
              <div className="p-6 text-center text-sm text-muted">
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
                    <Chip size="sm" color={gateColor(item.gate_status)}>
                      {item.gate_status}
                    </Chip>
                  </div>
                  <div className="mt-1 truncate text-xs text-muted">
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
              <Label>Probe definitions JSON</Label>
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
                <div className="mt-1 text-xs text-muted">
                  本阶段不创建真实后台任务；degrade write-back 只写 pack-local
                  store，不写 runtime degrade state。
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <div className="min-w-72">
                  <Label>Probe ID 列表</Label>
                  {probes.length > 0 ? (
                    <ListBox
                      aria-label="选择要运行的 Chaos probes"
                      selectionMode="multiple"
                      selectedKeys={selectedProbeIds}
                      onSelectionChange={updateProbeSelection}
                      className="mt-1 max-h-40 overflow-auto rounded-lg border p-1 border-border"
                    >
                      {probes.map((probe) => (
                        <ListBox.Item key={probe.id} id={probe.id} textValue={probe.name || probe.id}>
                          <div className="flex flex-col">
                            <span className="text-xs font-medium">{probe.name || probe.id}</span>
                            <span className="text-[11px] font-mono text-muted">{probe.id}</span>
                          </div>
                        </ListBox.Item>
                      ))}
                    </ListBox>
                  ) : (
                    <div className="mt-1 rounded-lg border border-dashed p-3 text-xs border-border text-muted">
                      暂无已保存探针。未选择时会让后端运行全部可用探针。
                    </div>
                  )}
                  <div className="mt-1 text-[11px] text-muted">
                    未选择时运行全部探针。
                  </div>
                </div>
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
              <Card className="p-3 bg-surface-secondary">
                <div className="mb-2 flex items-center gap-2 text-sm font-medium">
                  <Chip size="sm" color={toneColor}>
                    {selectedReport.gate_status}
                  </Chip>
                  <span>{selectedReport.id}</span>
                </div>
                <JsonViewer title="运行报告 JSON" value={selectedReport} rows={18} />
              </Card>
            ) : (
              <div className="rounded-xl border border-dashed p-6 text-center text-sm border-border text-muted">
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
              <JsonViewer title="Scheduler 计划 JSON" value={schedulerPlan} rows={12} />
              <div className="mt-2 text-xs text-muted">
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
              <JsonViewer title="本地降级状态 JSON" value={degradeWriteback} rows={12} />
              <div className="mt-2 text-xs text-muted">
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
              <JsonViewer title="Engine handoff JSON" value={enginePlan} rows={12} />
              <div className="mt-2 text-xs text-muted">
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
