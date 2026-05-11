"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import type {
  CogniAlert,
  CogniCheckResult,
  CogniEntryStatus,
  CogniEvolutionResponse,
  CogniExperiencePattern,
  CogniExperienceResponse,
  CogniHealthMetrics,
  CogniTrace,
  CogniVerifyResponse,
  CogniWorkflowDef,
  CogniWorkflowStep,
  CogniExperiment,
} from "@/lib/api-types/cogni";
import { Button, Card, Chip, Switch } from "@heroui/react";
import {
  Activity,
  AlertTriangle,
  Boxes,
  CheckCircle2,
  Download,
  FlaskConical,
  Globe,
  Play,
  RefreshCw,
  Search,
  ShieldCheck,
  Sparkles,
  Target,
  Trash2,
  Upload,
  Workflow,
  XCircle,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";

type HealthMap = Record<string, CogniHealthMetrics>;

// Color for health status chip.
function healthColor(status: string): { bg: string; fg: string } {
  switch (status) {
    case "healthy":
      return { bg: "rgba(23,201,100,0.12)", fg: "#17c964" };
    case "warn":
      return { bg: "rgba(255,170,0,0.12)", fg: "#ffaa00" };
    case "unhealthy":
      return { bg: "rgba(243,18,96,0.12)", fg: "#f31260" };
    default:
      return { bg: "rgba(255,255,255,0.04)", fg: "var(--yunque-text-muted)" };
  }
}

function severityColor(sev: string): { bg: string; fg: string } {
  switch (sev) {
    case "critical":
      return { bg: "rgba(243,18,96,0.15)", fg: "#f31260" };
    case "warn":
      return { bg: "rgba(255,170,0,0.15)", fg: "#ffaa00" };
    default:
      return { bg: "rgba(0,145,255,0.15)", fg: "#0091ff" };
  }
}

const DEMO_COGNI_ID = "code-reviewer";

export default function CognisPage() {
  const [cognis, setCognis] = useState<CogniEntryStatus[]>([]);
  const [health, setHealth] = useState<HealthMap>({});
  const [alerts, setAlerts] = useState<CogniAlert[]>([]);
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(true);
  const [reloading, setReloading] = useState(false);
  const [detailID, setDetailID] = useState<string | null>(null);
  const [detailTraces, setDetailTraces] = useState<CogniTrace[]>([]);
  const [detailTab, setDetailTab] = useState<"traces" | "workflows" | "experience" | "evolution">("traces");
  const [detailWorkflows, setDetailWorkflows] = useState<CogniWorkflowDef[]>([]);
  const [detailExperience, setDetailExperience] = useState<CogniExperienceResponse | null>(null);
  const [detailEvolution, setDetailEvolution] = useState<CogniEvolutionResponse | null>(null);
  const [generateOpen, setGenerateOpen] = useState(false);
  const [generateDesc, setGenerateDesc] = useState("");
  const [generating, setGenerating] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);
  const [demoVerify, setDemoVerify] = useState<CogniVerifyResponse | null>(null);
  const [demoTraces, setDemoTraces] = useState<CogniTrace[]>([]);
  const [verifying, setVerifying] = useState(false);
  const [refreshingTrace, setRefreshingTrace] = useState(false);
  const [refreshingHealth, setRefreshingHealth] = useState(false);

  const load = useCallback(async () => {
    try {
      const [list, alertsRes, demoTracesRes] = await Promise.all([
        api.listCognis(),
        api.getCogniAlerts().catch(() => ({ alerts: [] as CogniAlert[], count: 0 })),
        api.getCogniTracesByID(DEMO_COGNI_ID, 5).catch(() => ({ traces: [] as CogniTrace[] })),
      ]);
      setCognis(list.cognis || []);
      setHealth(list.health || {});
      setAlerts(alertsRes.alerts || []);
      setDemoTraces(demoTracesRes.traces || []);
    } catch (e) {
      showToast(e instanceof Error ? e.message : "加载失败", "error");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const reload = async () => {
    setReloading(true);
    try {
      const r = await api.reloadCognis();
      showToast(
        `已重载：新增 ${r.added} · 更新 ${r.updated} · 移除 ${r.removed}${r.errors?.length ? ` · 错误 ${r.errors.length}` : ""}`,
        r.errors?.length ? "error" : "success",
      );
      await load();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "重载失败", "error");
    } finally {
      setReloading(false);
    }
  };

  const scanAlerts = async () => {
    try {
      const r = await api.scanCogniAlerts();
      setAlerts(r.alerts || []);
      showToast(`扫描完成：${r.count} 条告警`, "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "扫描失败", "error");
    }
  };

  const toggle = async (id: string, enabled: boolean) => {
    try {
      await api.setCogniEnabled(id, enabled);
      setCognis((prev) =>
        prev.map((c) => (c.id === id ? { ...c, enabled } : c)),
      );
    } catch (e) {
      showToast(e instanceof Error ? e.message : "操作失败", "error");
    }
  };

  const remove = async (id: string) => {
    if (!confirm(`删除智体 ${id}？`)) return;
    try {
      await api.removeCogni(id);
      showToast("已删除", "success");
      await load();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "删除失败", "error");
    }
  };

  const exportBundle = () => {
    const base = process.env.NEXT_PUBLIC_API_BASE || "";
    const url = `${base}/v1/cognis/export`;
    window.open(url, "_blank");
  };

  const importBundle = async (file: File) => {
    try {
      const text = await file.text();
      const bundle = JSON.parse(text);
      // Mimic api.fetcher but with parsed body
      const token =
        typeof window !== "undefined"
          ? localStorage.getItem("yunque_token")
          : "";
      const res = await fetch("/v1/cognis/import", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify(bundle),
      });
      if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
      const sum = await res.json();
      showToast(
        `导入：新增 ${sum.added?.length || 0} · 更新 ${sum.updated?.length || 0} · 跳过 ${sum.skipped?.length || 0} · 失败 ${sum.failed?.length || 0}`,
        "success",
      );
      await load();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "导入失败", "error");
    }
  };

  const generateCogni = async () => {
    if (!generateDesc.trim()) return;
    setGenerating(true);
    try {
      const r = await api.generateCogni(generateDesc, true);
      showToast(`智体 "${r.declaration.id}" 已生成并保存`, "success");
      setGenerateOpen(false);
      setGenerateDesc("");
      await load();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "生成失败", "error");
    } finally {
      setGenerating(false);
    }
  };

  const openDetail = async (id: string) => {
    setDetailID(id);
    setDetailTab("traces");
    try {
      const [traces, workflows, experience, evolution] = await Promise.all([
        api.getCogniTracesByID(id, 20).catch(() => ({ traces: [] })),
        api.getCogniWorkflows(id).catch(() => ({ workflows: [] })),
        api.getCogniExperience(id).catch(() => null),
        api.getCogniEvolution(id).catch(() => null),
      ]);
      setDetailTraces(traces.traces || []);
      setDetailWorkflows(workflows.workflows || []);
      setDetailExperience(experience);
      setDetailEvolution(evolution);
    } catch {
      setDetailTraces([]);
    }
  };

  const runDemoVerify = async () => {
    setVerifying(true);
    try {
      const r = await api.verifyCognis();
      setDemoVerify(r);
      const failed = (r.failures || []).length;
      showToast(failed === 0 ? "所有检查通过" : `${failed} 项检查失败`, failed === 0 ? "success" : "error");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "验证失败", "error");
    } finally {
      setVerifying(false);
    }
  };

  const refreshDemoTraces = async () => {
    setRefreshingTrace(true);
    try {
      const r = await api.getCogniTracesByID(DEMO_COGNI_ID, 5);
      setDemoTraces(r.traces || []);
      showToast(`获取 ${(r.traces || []).length} 条 trace`, "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "刷新失败", "error");
    } finally {
      setRefreshingTrace(false);
    }
  };

  const refreshDemoHealth = async () => {
    setRefreshingHealth(true);
    try {
      const r = await api.getCogniHealth();
      const hmap: HealthMap = {};
      for (const h of r.health || []) hmap[h.id] = h;
      setHealth((prev) => ({ ...prev, ...hmap }));
      showToast("健康状态已刷新", "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "刷新失败", "error");
    } finally {
      setRefreshingHealth(false);
    }
  };

  const demoCogni = cognis.find((c) => c.id === DEMO_COGNI_ID);
  const demoHealthData = health[DEMO_COGNI_ID];

  const filteredCognis = cognis.filter(
    (c) =>
      !filter ||
      c.id.toLowerCase().includes(filter.toLowerCase()) ||
      (c.display_name ?? "").toLowerCase().includes(filter.toLowerCase()),
  );

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader
        icon={<Boxes size={20} />}
        title="智体 (Cogni)"
        onRefresh={load}
        actions={
          <div className="flex gap-2">
            <Button
              size="sm"
              onPress={() => setGenerateOpen(true)}
              className="btn-accent"
            >
              <Sparkles size={12} /> 自生成
            </Button>
            <Button
              size="sm"
              onPress={reload}
              isPending={reloading}
              variant="ghost"
            >
              <RefreshCw size={12} /> 热重载
            </Button>
            <Button size="sm" variant="ghost" onPress={exportBundle}>
              <Download size={12} /> 导出
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onPress={() => fileInput.current?.click()}
            >
              <Upload size={12} /> 导入
            </Button>
            <input
              ref={fileInput}
              type="file"
              accept=".json"
              hidden
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) importBundle(f);
                e.target.value = "";
              }}
            />
          </div>
        }
      />

      {/* Demo Evidence Panel */}
      {!loading && (
        <Card className="section-card p-4 border-l-4" style={{ borderLeftColor: "#0091ff" }}>
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Target size={14} style={{ color: "#0091ff" }} />
              <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                演示证据
              </span>
              <span className="text-xs font-mono" style={{ color: "var(--yunque-text-muted)" }}>
                {DEMO_COGNI_ID}
              </span>
            </div>
            <div className="flex gap-1.5">
              <Button size="sm" variant="ghost" onPress={reload} isPending={reloading}>
                <RefreshCw size={10} /> Reload
              </Button>
              <Button size="sm" variant="ghost" onPress={runDemoVerify} isPending={verifying}>
                <ShieldCheck size={10} /> Verify
              </Button>
              <Button size="sm" variant="ghost" onPress={refreshDemoTraces} isPending={refreshingTrace}>
                <Activity size={10} /> Trace
              </Button>
              <Button size="sm" variant="ghost" onPress={refreshDemoHealth} isPending={refreshingHealth}>
                <Globe size={10} /> Health
              </Button>
            </div>
          </div>

          {demoCogni ? (
            <div className="space-y-3">
              <div className="flex flex-wrap items-center gap-2 text-xs">
                <Chip size="sm" style={{ background: "rgba(23,201,100,0.12)", color: "#17c964" }}>
                  已注册
                </Chip>
                <Chip
                  size="sm"
                  style={{
                    background: demoCogni.enabled ? "rgba(23,201,100,0.12)" : "rgba(243,18,96,0.12)",
                    color: demoCogni.enabled ? "#17c964" : "#f31260",
                  }}
                >
                  {demoCogni.enabled ? "已启用" : "已禁用"}
                </Chip>
                {demoHealthData && (
                  <Chip
                    size="sm"
                    style={{
                      background: healthColor(demoHealthData.status).bg,
                      color: healthColor(demoHealthData.status).fg,
                    }}
                  >
                    {demoHealthData.status} · {demoHealthData.score}
                  </Chip>
                )}
              </div>

              {demoHealthData && demoHealthData.evaluations > 0 && (
                <div
                  className="flex flex-wrap gap-x-4 gap-y-1 text-[11px]"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  <span>评估 {demoHealthData.evaluations}</span>
                  <span>激活 {demoHealthData.activations}/{demoHealthData.evaluations}</span>
                  <span>上下文 {demoHealthData.avg_context_bytes}B</span>
                  {demoHealthData.tool_filter_ratio > 0 && (
                    <span>工具过滤 {(demoHealthData.tool_filter_ratio * 100).toFixed(0)}%</span>
                  )}
                  {demoHealthData.avg_duration_ms > 0 && (
                    <span>耗时 {demoHealthData.avg_duration_ms}ms</span>
                  )}
                  {demoHealthData.suppressed > 0 && (
                    <span style={{ color: "#ffaa00" }}>抑制 {demoHealthData.suppressed}</span>
                  )}
                </div>
              )}

              {demoVerify ? (
                <div
                  className="p-2.5 rounded-lg"
                  style={{ background: "rgba(255,255,255,0.02)" }}
                >
                  <div className="flex items-center justify-between mb-1.5">
                    <span className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>
                      Verify 结果
                    </span>
                    {(() => {
                      const checks = demoVerify.results[DEMO_COGNI_ID] || [];
                      const passed = checks.filter((c) => c.passed).length;
                      return (
                        <Chip
                          size="sm"
                          style={{
                            background: passed === checks.length ? "rgba(23,201,100,0.12)" : "rgba(243,18,96,0.12)",
                            color: passed === checks.length ? "#17c964" : "#f31260",
                          }}
                        >
                          {passed}/{checks.length} 通过
                        </Chip>
                      );
                    })()}
                  </div>
                  <div className="space-y-1">
                    {(demoVerify.results[DEMO_COGNI_ID] || []).map((ck, i) => (
                      <div key={i} className="flex items-center gap-2 text-xs">
                        {ck.passed ? (
                          <CheckCircle2 size={12} style={{ color: "#17c964" }} />
                        ) : (
                          <XCircle size={12} style={{ color: "#f31260" }} />
                        )}
                        <span className="font-mono" style={{ color: "var(--yunque-text)" }}>
                          {ck.check_name || `check-${ck.check_index}`}
                        </span>
                        <span style={{ color: "var(--yunque-text-muted)" }}>
                          score={ck.got_score.toFixed(2)} active={String(ck.got_active)}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              ) : (
                <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                  点击 Verify 运行声明式自测
                </div>
              )}

              {demoTraces.length > 0 ? (
                <div>
                  <div className="text-xs font-medium mb-1" style={{ color: "var(--yunque-text)" }}>
                    最近 Trace ({demoTraces.length})
                  </div>
                  <div className="space-y-1.5">
                    {demoTraces.slice(0, 3).map((t, i) => {
                      const own = t.activations?.find((a) => a.id === DEMO_COGNI_ID);
                      return (
                        <div
                          key={i}
                          className="p-2 rounded text-xs"
                          style={{ background: "rgba(255,255,255,0.03)" }}
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <Chip
                              size="sm"
                              style={{
                                background: own?.activated
                                  ? "rgba(23,201,100,0.12)"
                                  : "rgba(255,255,255,0.04)",
                                color: own?.activated ? "#17c964" : "var(--yunque-text-muted)",
                              }}
                            >
                              {own?.activated ? "激活" : "未激活"}
                            </Chip>
                            <span style={{ color: "var(--yunque-text-muted)" }}>
                              {new Date(t.timestamp).toLocaleTimeString()}
                            </span>
                            {t.context && t.context.bytes > 0 && (
                              <span style={{ color: "var(--yunque-text-muted)" }}>
                                上下文 {t.context.bytes}B
                              </span>
                            )}
                            {t.tool_filter?.applied_by?.includes(DEMO_COGNI_ID) && (
                              <span style={{ color: "var(--yunque-text)" }}>
                                工具 {t.tool_filter.before}→{t.tool_filter.after}
                                {t.tool_filter.removed?.length
                                  ? ` 移除 ${t.tool_filter.removed.join(",")}`
                                  : ""}
                              </span>
                            )}
                          </div>
                          {own?.reasons && own.reasons.length > 0 && (
                            <div className="mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                              {own.reasons.join("; ")}
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>
              ) : (
                <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  还没有 trace — 请先在聊天页发送：帮我审查一下这个 PR 的代码
                </div>
              )}
            </div>
          ) : (
            <div
              className="text-xs text-center py-2"
              style={{ color: "var(--yunque-text-muted)" }}
            >
              code-reviewer 未注册 — 将 code-reviewer.cogni.yaml 放入 data/cognis/ 后点击 Reload
            </div>
          )}
        </Card>
      )}

      {/* Alerts banner */}
      {alerts.length > 0 && (
        <Card
          className="section-card p-4 border-l-4"
          style={{ borderLeftColor: "#f31260" }}
        >
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <AlertTriangle size={14} style={{ color: "#f31260" }} />
              <span
                className="text-sm font-medium"
                style={{ color: "var(--yunque-text)" }}
              >
                {alerts.length} 条活跃告警
              </span>
            </div>
            <Button size="sm" variant="ghost" onPress={scanAlerts}>
              立即扫描
            </Button>
          </div>
          <div className="space-y-1.5">
            {alerts.slice(0, 5).map((a) => {
              const sc = severityColor(a.severity);
              return (
                <div
                  key={`${a.cogni_id}|${a.kind}`}
                  className="flex items-start gap-3 text-sm"
                >
                  <Chip
                    size="sm"
                    style={{ background: sc.bg, color: sc.fg }}
                  >
                    {a.severity.toUpperCase()}
                  </Chip>
                  <span
                    className="font-mono text-xs"
                    style={{ color: "var(--yunque-text-muted)" }}
                  >
                    {a.cogni_id}
                  </span>
                  <span style={{ color: "var(--yunque-text)" }}>{a.message}</span>
                  {a.auto_action_taken && (
                    <Chip
                      size="sm"
                      style={{
                        background: "rgba(255,170,0,0.12)",
                        color: "#ffaa00",
                      }}
                    >
                      已{a.auto_action_taken === "disabled" ? "自动禁用" : a.auto_action_taken}
                    </Chip>
                  )}
                </div>
              );
            })}
          </div>
        </Card>
      )}

      {/* Filter */}
      <div className="flex items-center gap-2">
        <div className="relative flex-1 max-w-md">
          <Search
            size={14}
            className="absolute left-3 top-1/2 -translate-y-1/2"
            style={{ color: "var(--yunque-text-muted)" }}
          />
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="搜索智体 ID 或名称…"
            className="w-full pl-9 pr-3 py-1.5 text-sm rounded-md"
            style={{
              background: "rgba(255,255,255,0.04)",
              border: "1px solid rgba(255,255,255,0.08)",
              color: "var(--yunque-text)",
            }}
          />
        </div>
        <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          {filteredCognis.length} / {cognis.length}
        </span>
      </div>

      {/* Cogni list */}
      {loading ? (
        <Card className="section-card p-10 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>
          加载中…
        </Card>
      ) : filteredCognis.length === 0 ? (
        <Card
          className="section-card p-10 text-center text-sm"
          style={{ color: "var(--yunque-text-muted)" }}
        >
          暂无智体。将 <code>*.json</code> 文件放入 <code>data/cognis/</code> 后点击「热重载」。
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {filteredCognis.map((c) => {
            const hm = health[c.id];
            const hc = healthColor(hm?.status ?? "idle");
            return (
              <Card
                key={c.id}
                className="section-card p-4 cursor-pointer"
                onClick={() => openDetail(c.id)}
              >
                <div className="flex items-start justify-between gap-3 mb-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 mb-1">
                      <span
                        className="font-medium truncate"
                        style={{ color: "var(--yunque-text)" }}
                      >
                        {c.display_name ?? c.id}
                      </span>
                      {c.always_on && (
                        <Chip
                          size="sm"
                          style={{
                            background: "rgba(0,145,255,0.12)",
                            color: "#0091ff",
                          }}
                        >
                          always-on
                        </Chip>
                      )}
                      {c.exclusive && (
                        <Chip
                          size="sm"
                          style={{ background: "rgba(255,255,255,0.04)" }}
                        >
                          g:{c.exclusive}
                        </Chip>
                      )}
                    </div>
                    <div
                      className="text-xs font-mono truncate"
                      style={{ color: "var(--yunque-text-muted)" }}
                    >
                      {c.id}
                    </div>
                    {c.description && (
                      <div
                        className="text-sm mt-2"
                        style={{ color: "var(--yunque-text-muted)" }}
                      >
                        {c.description}
                      </div>
                    )}
                  </div>
                  <div
                    className="flex flex-col items-end gap-2"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Chip size="sm" style={{ background: hc.bg, color: hc.fg }}>
                      {hm ? `${hm.status} · ${hm.score}` : "idle"}
                    </Chip>
                    <Switch
                      isSelected={c.enabled}
                      onChange={(v) => toggle(c.id, v)}
                      size="sm"
                    >
                      <Switch.Control>
                        <Switch.Thumb />
                      </Switch.Control>
                    </Switch>
                  </div>
                </div>

                {c.load_error && (
                  <div
                    className="mt-2 text-xs p-2 rounded"
                    style={{
                      background: "rgba(243,18,96,0.1)",
                      color: "#f31260",
                    }}
                  >
                    <ShieldCheck size={10} className="inline mr-1" />
                    {formatErrorMessage(c.load_error, "加载失败")}
                  </div>
                )}

                {hm && hm.activations > 0 && (
                  <div
                    className="mt-2 text-[11px] flex gap-3"
                    style={{ color: "var(--yunque-text-muted)" }}
                  >
                    <span>激活 {hm.activations}/{hm.evaluations}</span>
                    {hm.tool_filter_ratio > 0 && (
                      <span>工具比 {(hm.tool_filter_ratio * 100).toFixed(0)}%</span>
                    )}
                    {hm.avg_duration_ms > 0 && (
                      <span>耗时 {hm.avg_duration_ms}ms</span>
                    )}
                    {hm.template_fallback_rate > 0 && (
                      <span style={{ color: "#ffaa00" }}>
                        模板失败 {(hm.template_fallback_rate * 100).toFixed(0)}%
                      </span>
                    )}
                  </div>
                )}

                <div
                  className="mt-2 flex justify-end"
                  onClick={(e) => e.stopPropagation()}
                >
                  <Button
                    size="sm"
                    variant="ghost"
                    isIconOnly
                    onPress={() => remove(c.id)}
                    aria-label={`删除 ${c.id}`}
                  >
                    <Trash2 size={12} style={{ color: "#f31260" }} />
                  </Button>
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {/* Generate dialog */}
      {generateOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center"
          style={{ background: "rgba(0,0,0,0.5)" }}
          onClick={() => setGenerateOpen(false)}
        >
          <Card
            className="section-card p-6"
            style={{ width: "min(480px, 90%)" }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center gap-2 mb-4" style={{ color: "var(--yunque-text)" }}>
              <Sparkles size={16} />
              <span className="font-medium">自生成智体</span>
            </div>
            <textarea
              value={generateDesc}
              onChange={(e) => setGenerateDesc(e.target.value)}
              placeholder="用自然语言描述你想要的智体，例如：&#10;「我需要一个能自动审查 PR 的智体，关注安全漏洞和代码风格」"
              rows={4}
              className="w-full p-3 text-sm rounded-lg mb-4"
              style={{
                background: "rgba(255,255,255,0.04)",
                border: "1px solid rgba(255,255,255,0.08)",
                color: "var(--yunque-text)",
                resize: "vertical",
              }}
            />
            <div className="flex justify-end gap-2">
              <Button size="sm" variant="ghost" onPress={() => setGenerateOpen(false)}>取消</Button>
              <Button
                size="sm"
                className="btn-accent"
                onPress={generateCogni}
                isPending={generating}
                isDisabled={!generateDesc.trim()}
              >
                <Sparkles size={12} /> 生成并保存
              </Button>
            </div>
          </Card>
        </div>
      )}

      {/* Detail drawer */}
      {detailID && (
        <div
          className="fixed inset-0 z-50 flex justify-end"
          style={{ background: "rgba(0,0,0,0.5)" }}
          onClick={() => setDetailID(null)}
        >
          <Card
            className="h-full overflow-y-auto p-5 section-card"
            style={{ width: "min(560px, 100%)" }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <div className="font-mono text-sm" style={{ color: "var(--yunque-text)" }}>
                {detailID}
              </div>
              <Button size="sm" variant="ghost" onPress={() => setDetailID(null)}>关闭</Button>
            </div>

            {/* Tabs */}
            <div className="flex gap-1 mb-4 border-b" style={{ borderColor: "rgba(255,255,255,0.08)" }}>
              {(["traces", "workflows", "experience", "evolution"] as const).map((tab) => (
                <button
                  key={tab}
                  onClick={() => setDetailTab(tab)}
                  className="px-3 py-1.5 text-xs font-medium rounded-t-md transition-colors"
                  style={{
                    color: detailTab === tab ? "var(--yunque-text)" : "var(--yunque-text-muted)",
                    background: detailTab === tab ? "rgba(255,255,255,0.06)" : "transparent",
                    borderBottom: detailTab === tab ? "2px solid var(--yunque-accent)" : "2px solid transparent",
                  }}
                >
                  {tab === "traces" && "Trace"}
                  {tab === "workflows" && "工作流"}
                  {tab === "experience" && "经验"}
                  {tab === "evolution" && "进化"}
                </button>
              ))}
            </div>

            {/* Traces tab */}
            {detailTab === "traces" && (
              <>
                <div className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>
                  最近 Trace ({detailTraces.length})
                </div>
                {detailTraces.length === 0 ? (
                  <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                    暂无 trace — 该智体尚未参与对话
                  </div>
                ) : (
                  <div className="space-y-2">
                    {detailTraces.map((t, i) => {
                      const ownEntry = t.activations?.find((a) => a.id === detailID);
                      const activated = !!ownEntry?.activated;
                      return (
                        <div key={i} className="p-3 rounded-lg text-xs" style={{ background: "rgba(255,255,255,0.03)" }}>
                          <div className="flex items-center gap-2 mb-1">
                            <Chip size="sm" style={{
                              background: activated ? "rgba(23,201,100,0.12)" : "rgba(255,255,255,0.04)",
                              color: activated ? "#17c964" : "var(--yunque-text-muted)",
                            }}>
                              {activated ? "激活" : "未激活"}
                            </Chip>
                            <span style={{ color: "var(--yunque-text-muted)" }}>
                              {new Date(t.timestamp).toLocaleTimeString()}
                            </span>
                            {t.duration_ms > 0 && <span style={{ color: "var(--yunque-text-muted)" }}>{t.duration_ms}ms</span>}
                          </div>
                          {ownEntry?.reasons && ownEntry.reasons.length > 0 && (
                            <div style={{ color: "var(--yunque-text-muted)" }}>{ownEntry.reasons.join("; ")}</div>
                          )}
                          {ownEntry?.suppressed && <div style={{ color: "#ffaa00" }}>被 {ownEntry.suppressed_by} 排他抑制</div>}
                          {t.tool_filter?.applied_by?.includes(detailID!) && (
                            <div style={{ color: "var(--yunque-text)" }}>
                              工具 {t.tool_filter.before} → {t.tool_filter.after}
                              {t.tool_filter.removed && t.tool_filter.removed.length > 0 &&
                                ` · 移除 ${t.tool_filter.removed.slice(0, 3).join(", ")}${t.tool_filter.removed.length > 3 ? "…" : ""}`}
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
              </>
            )}

            {/* Workflows tab */}
            {detailTab === "workflows" && (
              <>
                <div className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>
                  <Workflow size={12} className="inline mr-1" />
                  工作流 ({detailWorkflows.length})
                </div>
                {detailWorkflows.length === 0 ? (
                  <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                    该智体未定义工作流
                  </div>
                ) : (
                  <div className="space-y-3">
                    {detailWorkflows.map((wf: CogniWorkflowDef) => (
                      <div key={wf.name} className="p-3 rounded-lg" style={{ background: "rgba(255,255,255,0.03)" }}>
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{wf.name}</span>
                          <Button size="sm" variant="ghost" onPress={async () => {
                            try {
                              const r = await api.runCogniWorkflow(detailID!, wf.name);
                              showToast(r.success ? `工作流完成：${r.workflow_name}` : `工作流失败：${r.error}`, r.success ? "success" : "error");
                            } catch (e) {
                              showToast(e instanceof Error ? e.message : "执行失败", "error");
                            }
                          }}>
                            <Play size={10} /> 执行
                          </Button>
                        </div>
                        {wf.description && <div className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>{wf.description}</div>}
                        <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                          {wf.steps?.length || 0} 个步骤
                          {wf.steps?.map((s: CogniWorkflowStep, i: number) => (
                            <span key={i}> · {s.skill}</span>
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </>
            )}

            {/* Experience tab */}
            {detailTab === "experience" && (
              <>
                <div className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>
                  经验累积
                </div>
                {!detailExperience?.enabled ? (
                  <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                    该智体未启用经验引擎
                  </div>
                ) : (
                  <div className="space-y-3">
                    <div className="grid grid-cols-2 gap-2">
                      {Object.entries(detailExperience.stats || {}).map(([k, v]) => (
                        <div key={k} className="p-2 rounded-lg text-center" style={{ background: "rgba(255,255,255,0.03)" }}>
                          <div className="text-lg font-medium" style={{ color: "var(--yunque-text)" }}>{String(v)}</div>
                          <div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{k.replace(/_/g, " ")}</div>
                        </div>
                      ))}
                    </div>
                    {(detailExperience.patterns?.length ?? 0) > 0 && (
                      <div>
                        <div className="text-xs mb-1 font-medium" style={{ color: "var(--yunque-text)" }}>行为模式</div>
                        {detailExperience.patterns!.slice(0, 5).map((p: CogniExperiencePattern, i: number) => (
                          <div key={i} className="text-xs p-2 rounded mb-1" style={{ background: "rgba(255,255,255,0.03)", color: "var(--yunque-text-muted)" }}>
                            {p.trigger} → {p.response} {p.confirmed && "✓"}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </>
            )}

            {/* Evolution tab */}
            {detailTab === "evolution" && (
              <>
                <div className="flex items-center justify-between mb-2">
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                    <FlaskConical size={12} className="inline mr-1" />
                    Skill 进化
                  </div>
                  <Button size="sm" variant="ghost" onPress={async () => {
                    try {
                      await api.triggerCogniEvolution(detailID!);
                      showToast("进化已启动", "success");
                    } catch (e) {
                      showToast(e instanceof Error ? e.message : "启动失败", "error");
                    }
                  }}>
                    <FlaskConical size={10} /> 触发进化
                  </Button>
                </div>
                {detailEvolution?.running && (
                  <Chip size="sm" style={{ background: "rgba(0,145,255,0.12)", color: "#0091ff" }}>进化中…</Chip>
                )}
                {(!detailEvolution?.experiments || detailEvolution.experiments.length === 0) ? (
                  <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                    尚无进化实验记录
                  </div>
                ) : (
                  <div className="space-y-2 mt-2">
                    {detailEvolution.experiments.map((exp: CogniExperiment) => (
                      <div key={exp.id} className="p-3 rounded-lg text-xs" style={{ background: "rgba(255,255,255,0.03)" }}>
                        <div className="flex items-center gap-2 mb-1">
                          <Chip size="sm" style={{
                            background: exp.status === "kept" ? "rgba(23,201,100,0.12)" : "rgba(243,18,96,0.12)",
                            color: exp.status === "kept" ? "#17c964" : "#f31260",
                          }}>
                            {exp.status === "kept" ? "保留" : "回滚"}
                          </Chip>
                          <span style={{ color: "var(--yunque-text-muted)" }}>
                            {new Date(exp.date).toLocaleDateString()}
                          </span>
                          <span style={{ color: exp.delta >= 0 ? "#17c964" : "#f31260" }}>
                            {exp.delta >= 0 ? "+" : ""}{exp.delta.toFixed(1)}%
                          </span>
                        </div>
                        <div style={{ color: "var(--yunque-text-muted)" }}>{exp.change}</div>
                        {exp.reason && <div style={{ color: "var(--yunque-text-muted)" }}>{exp.reason}</div>}
                      </div>
                    ))}
                  </div>
                )}
              </>
            )}
          </Card>
        </div>
      )}
    </div>
  );
}
