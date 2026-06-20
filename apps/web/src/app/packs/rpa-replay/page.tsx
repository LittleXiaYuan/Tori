"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Button, Card, Chip, Input, Spinner, TextArea, TextField } from "@heroui/react";
import { AlertTriangle, ClipboardList, Download, Play, Plus, Radio, RefreshCw, Route, Send, ShieldCheck, Workflow } from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref } from "@/lib/pack-action-links";
import { createRPAReplayPackClient, type RPAReplayExecutorPlan, type RPAReplayResult, type RPAReplayStatus, type RPAReplayTraceSummary } from "@/lib/rpa-replay-pack-client";

const rpaReplayPack = createRPAReplayPackClient();

function statusTone(status: RPAReplayStatus | null): { bg: string; fg: string } {
  if (!status) return { bg: "rgba(255,255,255,0.06)", fg: "var(--yunque-text-muted)" };
  if (status.executor_ready) return { bg: "rgba(34,197,94,0.12)", fg: "#22c55e" };
  return { bg: "rgba(250,204,21,0.12)", fg: "#facc15" };
}

const userFacingSteps = [
  {
    title: "1. 保存流程轨迹",
    body: "把重复网页流程写成带参数的步骤，例如打开报表、选择月份、准备导出。",
  },
  {
    title: "2. 生成回放计划",
    body: "用参数替换模板，先得到 dry-run 计划和阻塞原因，不直接操作浏览器。",
  },
  {
    title: "3. 导出交接证据",
    body: "把轨迹、参数、Browser Intent/ActionTracer 交接计划打包给后续执行器审核。",
  },
];

const boundaryItems = [
  "不会点击网页、输入表单或下载文件。",
  "不会消费 Browser Intent 会话或写浏览器状态。",
  "不会访问外部目标站点或执行真实网络动作。",
  "不会把 plan-only 轨迹当成已完成的自动化任务。",
];

const workflowLoopItems = [
  {
    title: "1. 保存轨迹",
    body: "把网页流程、参数和断言保存成可复用记录，先形成可检查的自动化意图。",
  },
  {
    title: "2. 带回 Chat",
    body: "让云雀解释回放计划、补齐缺失步骤，或拆成需要人工确认的任务。",
  },
  {
    title: "3. 看证据位置",
    body: "dry-run 结果和 executor handoff 是验证材料，不等于已经执行成功。",
  },
  {
    title: "4. 继续补能力",
    body: "如果还缺点击、截图或站点适配，把真实问题交给小羽继续改包。",
  },
];

function sampleTrace(slug: string) {
  return JSON.stringify({
    slug,
    name: "导出月度报表",
    description: "打开报表页面并按月份导出文件的 RPA 回放轨迹样例。",
    target_url: "https://erp.example.com/reports",
    parameters: {
      month: { type: "string", description: "目标月份，格式 YYYY-MM", required: true },
      format: { type: "string", description: "导出格式", default: "xlsx" },
    },
    steps: [
      { action: "navigate", value: "https://erp.example.com/reports?month={{month}}", assertion: { type: "url_matches", expected: "{{month}}" } },
      { action: "click", selector: "button[data-action=export]" },
      { action: "select", selector: "select[name=format]", value: "{{format}}" },
    ],
  }, null, 2);
}

export default function RPAReplayPackPage() {
  const [status, setStatus] = useState<RPAReplayStatus | null>(null);
  const [traces, setTraces] = useState<RPAReplayTraceSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"create" | "replay" | "executorPlan" | "evidence" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [slug, setSlug] = useState("export-report");
  const [traceJSON, setTraceJSON] = useState(() => sampleTrace("export-report"));
  const [paramsJSON, setParamsJSON] = useState(() => JSON.stringify({ month: "2026-05", format: "xlsx" }, null, 2));
  const [result, setResult] = useState<RPAReplayResult | null>(null);
  const [executorPlan, setExecutorPlan] = useState<RPAReplayExecutorPlan | null>(null);
  const tone = statusTone(status);

  const selectedTrace = useMemo(() => traces.find((trace) => trace.slug === slug) || traces[0] || null, [slug, traces]);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [statusRes, tracesRes] = await Promise.all([rpaReplayPack.status(), rpaReplayPack.traces()]);
      setStatus(statusRes);
      setTraces(tracesRes.traces || []);
      if (!slug && tracesRes.traces?.[0]?.slug) setSlug(tracesRes.traces[0].slug);
    } catch (e) {
      const msg = formatErrorMessage(e, "加载 RPA Replay Pack 失败");
      setError(msg);
      if (msg.includes("pack route is not enabled")) {
        setError("RPA Replay Pack 当前未启用。请到「能力包」控制台启用 yunque.pack.rpa-replay 后再使用。");
      }
    } finally {
      setLoading(false);
    }
  }, [slug]);

  useEffect(() => { load(); }, [load]);

  const createTrace = async () => {
    setBusy("create");
    setError(null);
    try {
      const payload = JSON.parse(traceJSON);
      await rpaReplayPack.createTrace(payload);
      setSlug(payload.slug || slug);
      showToast("RPA 轨迹已保存", "success");
      await load();
    } catch (e) {
      setError(formatErrorMessage(e, "保存轨迹失败"));
    } finally {
      setBusy(null);
    }
  };

  const replay = async () => {
    setBusy("replay");
    setError(null);
    try {
      const params = JSON.parse(paramsJSON || "{}");
      const res = await rpaReplayPack.replay({ slug, params, dry_run: true });
      setResult(res.result);
      showToast("已生成 dry-run 回放计划", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "回放计划生成失败"));
    } finally {
      setBusy(null);
    }
  };

  const planExecutorHandoff = async () => {
    setBusy("executorPlan");
    setError(null);
    try {
      const params = JSON.parse(paramsJSON || "{}");
      const res = await rpaReplayPack.executorPlan({
        slug,
        params,
        dry_run: true,
        requested_by: "pack-console",
        reason: "operator Browser Intent / ActionTracer handoff review",
      });
      setExecutorPlan(res.plan);
      showToast("已生成 executor handoff plan（plan-only，未执行浏览器动作）", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 executor handoff plan 失败"));
    } finally {
      setBusy(null);
    }
  };

  const exportEvidence = async () => {
    setBusy("evidence");
    setError(null);
    try {
      const evidence = await rpaReplayPack.evidence(slug);
      const blob = new Blob([JSON.stringify(evidence, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${slug || "rpa-trace"}-evidence.json`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      showToast("证据包已导出", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "导出证据包失败"));
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
      <PageHeader icon={<Workflow size={20} />} title="RPA 录制回放" />

      <Card className="section-card overflow-hidden p-0">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="p-5">
            <div className="flex flex-wrap items-center gap-2">
              <Chip size="sm" style={{ background: "rgba(245,158,11,0.12)", color: "var(--yunque-warning)" }}>实验中</Chip>
              <Chip size="sm" variant="soft">只生成计划</Chip>
              <Chip size="sm" variant="soft">可导出证据</Chip>
            </div>
            <div className="mt-3 text-base font-semibold" style={{ color: "var(--yunque-text)" }}>
              这个能力包现在适合做什么
            </div>
            <div className="mt-2 max-w-3xl text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
              它用于把重复网页流程整理成可复用轨迹，并预演“如果交给未来执行器会怎么做”。当前阶段主要帮助你保存步骤、替换参数、生成 dry-run 回放计划和证据包；它还不是能直接接管浏览器的完整 RPA。
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
              <Chip size="sm" style={{ background: tone.bg, color: tone.fg }}>
                {status?.executor_ready ? "Executor ready" : "Pack shell"}
              </Chip>
              <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{status?.pack_id || "yunque.pack.rpa-replay"}</span>
            </div>
            <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>
              当前切片完成 manifest、route gate、trace store、dry-run replay plan、executor handoff plan、Browser Intent gate plan、ActionTracer handoff plan 和证据包导出。该入口只生成未来执行器契约，不消费 Browser Intent、不执行浏览器动作、不写浏览器状态/文件、不访问网络。
            </div>
          </div>
          <Button size="sm" variant="ghost" onPress={load}><RefreshCw size={14} />刷新</Button>
        </div>
      </Card>

      <Card className="section-card p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>从回放计划到可验证自动化</div>
            <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
              RPA Replay 的价值不是假装已经接管浏览器，而是把重复流程沉淀成可审计计划，再交给 Chat、任务中心和小羽逐步补成真能力。
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Link href={chatPromptHref("请检查 RPA Replay 当前轨迹和 dry-run 计划，指出哪些步骤可以安全执行、哪些还需要人工确认，并把下一步拆成任务。")}>
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
          <Link href="/packs/studio?packId=yunque.pack.rpa-replay"><Button size="sm" variant="ghost">让小羽继续改</Button></Link>
        </div>
      </Card>

      {error && (
        <Card className="p-4" style={{ background: "rgba(239,68,68,0.06)" }}>
          <div className="flex items-center gap-2 text-sm" style={{ color: "var(--yunque-danger)" }}>
            <AlertTriangle size={16} />{error}
          </div>
        </Card>
      )}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <Card className="section-card p-4"><div className="kpi-label">轨迹数量</div><div className="kpi-value">{status?.trace_count ?? traces.length}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">录制会话</div><div className="kpi-value">{status?.active_recordings ?? 0}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">能力数</div><div className="kpi-value">{status?.capabilities?.length ?? 0}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">阶段</div><div className="kpi-value text-lg">{status?.stage || "pack-shell"}</div></Card>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_1fr]">
        <Card className="section-card overflow-hidden">
          <div className="flex items-center justify-between border-b px-4 py-3" style={{ borderColor: "var(--yunque-border)" }}>
            <div className="flex items-center gap-2 text-sm font-semibold"><ClipboardList size={16} />已保存轨迹</div>
            <Chip size="sm">{traces.length}</Chip>
          </div>
          <div className="max-h-[520px] divide-y overflow-auto" style={{ borderColor: "var(--yunque-border)" }}>
            {traces.length === 0 ? (
              <div className="p-6 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>还没有轨迹。可以先保存右侧示例。</div>
            ) : traces.map((trace) => (
              <button key={trace.slug} onClick={() => setSlug(trace.slug)} className="block w-full px-4 py-3 text-left hover:bg-white/5">
                <div className="flex items-center justify-between gap-2">
                  <div className="font-medium">{trace.name || trace.slug}</div>
                  <Chip size="sm">{trace.step_count} steps</Chip>
                </div>
                <div className="mt-1 truncate text-xs" style={{ color: "var(--yunque-text-muted)" }}>{trace.slug}</div>
              </button>
            ))}
          </div>
        </Card>

        <div className="space-y-4">
          <Card className="section-card p-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-sm font-semibold"><Plus size={16} />创建 / 导入轨迹</div>
              <TextField aria-label="RPA trace slug" className="w-56" value={slug} onChange={setSlug}>
                <Input placeholder="trace slug" />
              </TextField>
            </div>
            <div className="mb-2 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>轨迹定义 JSON</div>
            <TextField aria-label="RPA trace JSON" value={traceJSON} onChange={setTraceJSON}>
              <TextArea rows={10} aria-label="Trace JSON" className="font-mono text-xs" />
            </TextField>
            <div className="mt-2 text-[11px] leading-5" style={{ color: "var(--yunque-text-muted)" }}>
              这里保存的是可审计轨迹，不会自动打开网页或执行点击；后续回放仍是 dry-run 计划。
            </div>
            <div className="mt-3 flex justify-end">
              <Button className="btn-accent" isPending={busy === "create"} onPress={createTrace}><Plus size={14} />保存轨迹</Button>
            </div>
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold"><Route size={16} />Dry-run 回放计划</div>
                <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>目标轨迹：{selectedTrace?.slug || slug}</div>
              </div>
              <div className="flex gap-2">
                <Button variant="outline" isPending={busy === "evidence"} onPress={exportEvidence} isDisabled={!slug}><Download size={14} />导出证据包</Button>
                <Button variant="outline" isPending={busy === "executorPlan"} onPress={planExecutorHandoff} isDisabled={!slug}><ShieldCheck size={14} />Executor handoff</Button>
                <Button className="btn-accent" isPending={busy === "replay"} onPress={replay} isDisabled={!slug}><Play size={14} />生成回放计划</Button>
              </div>
            </div>
            <div className="mb-2 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>回放参数 JSON</div>
            <TextField aria-label="RPA replay params JSON" value={paramsJSON} onChange={setParamsJSON}>
              <TextArea rows={4} aria-label="Replay params JSON" className="font-mono text-xs" />
            </TextField>
            <div className="mt-2 text-[11px] leading-5" style={{ color: "var(--yunque-text-muted)" }}>
              参数只用于生成计划和证据包；当前不会消费 Browser Intent、写浏览器状态或访问目标站点。
            </div>
            {result && (
              <Card className="mt-3 p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
                <div className="mb-2 flex items-center gap-2 text-sm font-medium"><Radio size={15} />{result.output || "回放计划"}</div>
                <pre className="max-h-72 overflow-auto whitespace-pre-wrap text-xs" style={{ color: "var(--yunque-text-muted)" }}>{JSON.stringify(result, null, 2)}</pre>
              </Card>
            )}
            {executorPlan && (
              <Card className="mt-3 p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
                <div className="mb-2 flex flex-wrap items-center gap-2 text-sm font-medium">
                  <ShieldCheck size={15} />Browser Intent / ActionTracer handoff
                  <Chip size="sm">executor_plan_ready {String(executorPlan.executor_plan_ready)}</Chip>
                  <Chip size="sm" color={executorPlan.executor_ready ? "success" : "warning"}>executor_ready {String(executorPlan.executor_ready)}</Chip>
                  <Chip size="sm">browser_intent_gate_plan_ready {String(executorPlan.browser_intent_gate_plan_ready)}</Chip>
                  <Chip size="sm">executes_browser_actions {String(executorPlan.executes_browser_actions)}</Chip>
                  <Chip size="sm">writes_browser_state {String(executorPlan.writes_browser_state)}</Chip>
                  <Chip size="sm">network_access {String(executorPlan.network_access)}</Chip>
                </div>
                <div className="mb-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  这是 plan-only 的执行器交接预览：只把参数替换后的 RPA steps 映射为未来 Browser Intent + ActionTracer 输入契约；当前 executor_ready=false，不会捕获 runtime trace、不会访问外部目标、不会写入本地文件。
                </div>
                <pre className="max-h-72 overflow-auto whitespace-pre-wrap text-xs" style={{ color: "var(--yunque-text-muted)" }}>{JSON.stringify(executorPlan, null, 2)}</pre>
              </Card>
            )}
          </Card>
        </div>
      </div>
    </div>
  );
}
