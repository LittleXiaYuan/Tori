"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { EvolutionState, LoRAStatus, TrainingRecord, TrainingSummary } from "@/lib/api-types/lora";
import type { LoRAConfig } from "@/lib/api-types/lora";
import { Button, Card, Chip, Input, Label, Spinner, Table, TextField, Tooltip } from "@heroui/react";
import {
  Activity,
  BrainCircuit,
  Cpu,
  Layers,
  Play,
  RefreshCw,
  RotateCcw,
  Save,
  Settings,
  Target,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { fmtTime, relTime } from "@/lib/constants";

function fmtNs(ns?: number): string {
  if (ns == null || !Number.isFinite(ns) || ns <= 0) return "—";
  const s = ns / 1e9;
  if (s < 60) return `${s.toFixed(1)}s`;
  if (s < 3600) return `${(s / 60).toFixed(1)}min`;
  return `${(s / 3600).toFixed(1)}h`;
}

function fmtNsDuration(ns?: number): string {
  if (!ns || ns <= 0) return "24h";
  const hours = ns / 3.6e12;
  if (hours >= 1) return `${hours}h`;
  const mins = ns / 6e10;
  return `${mins}m`;
}

function pct(n: number, d: number): string {
  if (d <= 0) return "—";
  return `${Math.round((n / d) * 100)}%`;
}

export default function LoRAPage() {
  const [status, setStatus] = useState<LoRAStatus | null>(null);
  const [records, setRecords] = useState<TrainingRecord[]>([]);
  const [summary, setSummary] = useState<TrainingSummary | null>(null);
  const [evolution, setEvolution] = useState<EvolutionState | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"trigger" | "rollback" | "save-config" | null>(null);
  const [disabledReason, setDisabledReason] = useState<string | null>(null);
  const [config, setConfig] = useState<LoRAConfig | null>(null);
  const [configDraft, setConfigDraft] = useState<Record<string, string>>({});
  const [showConfig, setShowConfig] = useState(false);

  const load = useCallback(async () => {
    setDisabledReason(null);
    const [stR, histR, sumR, evoR, cfgR] = await Promise.allSettled([
      api.getLoRAStatus(),
      api.getLoRAHistory(),
      api.getLoRASummary(),
      api.getEvolutionState(),
      api.getLoRAConfig(),
    ]);

    if (stR.status === "fulfilled") {
      setStatus(stR.value);
    } else {
      setStatus(null);
      const msg = stR.reason instanceof Error ? stR.reason.message : String(stR.reason);
      if (msg.includes("LoRA scheduler not configured") || msg.includes("not configured")) {
        setDisabledReason("当前进程未启用 LoRA / 进化组件（需 LocalBrain 与 Ledger，参见 init_intelligence）。");
      } else {
        showToast(msg, "error");
      }
    }

    if (histR.status === "fulfilled") {
      setRecords(histR.value.records || []);
    } else {
      setRecords([]);
    }

    if (sumR.status === "fulfilled") {
      setSummary(sumR.value.summary ?? null);
    } else {
      setSummary(null);
    }

    if (evoR.status === "fulfilled") {
      setEvolution(evoR.value.state ?? null);
    } else {
      setEvolution(null);
    }

    if (cfgR.status === "fulfilled" && cfgR.value.config) {
      const c = cfgR.value.config;
      setConfig(c);
      setConfigDraft({
        min_samples: String(c.min_samples || 200),
        min_interval: fmtNsDuration(c.min_interval),
        eval_min_score: String(c.eval_min_score || 0.7),
        max_adapters: String(c.max_adapters || 3),
        base_model: c.base_model || "",
        ab_test_duration: fmtNsDuration(c.ab_test_duration),
      });
    }

    setLoading(false);
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const onTrigger = async () => {
    setBusy("trigger");
    try {
      await api.triggerLoRATraining();
      showToast("已触发训练检查与流水线", "success");
      await load();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "触发失败", "error");
    } finally {
      setBusy(null);
    }
  };

  const onSaveConfig = async () => {
    setBusy("save-config");
    try {
      const patch: Record<string, unknown> = {};
      const ms = parseInt(configDraft.min_samples);
      if (!isNaN(ms) && ms > 0) patch.min_samples = ms;
      if (configDraft.min_interval) patch.min_interval = configDraft.min_interval;
      const ems = parseFloat(configDraft.eval_min_score);
      if (!isNaN(ems) && ems > 0) patch.eval_min_score = ems;
      const ma = parseInt(configDraft.max_adapters);
      if (!isNaN(ma) && ma > 0) patch.max_adapters = ma;
      if (configDraft.base_model) patch.base_model = configDraft.base_model;
      if (configDraft.ab_test_duration) patch.ab_test_duration = configDraft.ab_test_duration;
      await api.updateLoRAConfig(patch as any);
      showToast("配置已保存", "success");
      await load();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "保存失败", "error");
    } finally {
      setBusy(null);
    }
  };

  const onRollback = async () => {
    if (!confirm("确定要回滚到上一适配器吗？")) return;
    setBusy("rollback");
    try {
      await api.rollbackLoRA();
      showToast("已执行回滚", "success");
      await load();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "回滚失败", "error");
    } finally {
      setBusy(null);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Spinner size="lg" />
      </div>
    );
  }

  const sch = status?.scheduler;
  const ab = sch?.ab_test_metrics;
  const rollRate =
    status?.rolling_success_rate ??
    evolution?.rolling_success_rate ??
    0;

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader
        icon={<Cpu size={22} />}
        iconColor="#a78bfa"
        title="LoRA 训练"
        description="小模型适配器调度、训练历史与多层进化状态"
        onRefresh={load}
        actions={
          <div className="flex items-center gap-2">
            <Tooltip delay={0}>
              <Button
                size="sm"
                className="gap-1.5 rounded-lg"
                isPending={busy === "trigger"}
                isDisabled={!!disabledReason}
                onPress={onTrigger}
              >
                <Play size={14} /> 手动触发训练
              </Button>
              <Tooltip.Content>调用调度器 CheckAndTrigger（受样本量与间隔限制）</Tooltip.Content>
            </Tooltip>
            <Tooltip delay={0}>
              <Button
                size="sm"
                variant="ghost"
                className="gap-1.5"
                isPending={busy === "rollback"}
                isDisabled={!!disabledReason}
                onPress={onRollback}
              >
                <RotateCcw size={14} /> 手动回滚
              </Button>
              <Tooltip.Content>回滚到上一适配器</Tooltip.Content>
            </Tooltip>
          </div>
        }
      />

      {disabledReason && (
        <Card className="section-card p-4" style={{ borderColor: "rgba(255,170,0,0.35)" }}>
          <p className="text-sm" style={{ color: "var(--yunque-warning)" }}>
            {disabledReason}
          </p>
        </Card>
      )}

      {/* 状态概览 */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 stagger-children">
        <Card className="section-card p-4">
          <div className="flex items-center gap-1.5 kpi-label mb-1">
            <Cpu size={13} /> 当前适配器
          </div>
          <div className="kpi-value text-sm font-mono truncate" title={sch?.current_adapter || status?.active_model}>
            {sch?.current_adapter || "—"}
          </div>
          <div className="text-[11px] mt-1 truncate" style={{ color: "var(--yunque-text-muted)" }}>
            推理模型：{status?.active_model || "—"}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="flex items-center gap-1.5 kpi-label mb-1">
            <Activity size={13} /> A/B 测试
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            <Chip
              size="sm"
              style={{
                background: sch?.ab_test_active ? "rgba(0,145,255,0.15)" : "rgba(255,255,255,0.06)",
                color: sch?.ab_test_active ? "#0091ff" : "var(--yunque-text-muted)",
              }}
            >
              {sch?.ab_test_active ? "进行中" : "未激活"}
            </Chip>
            {sch?.ab_test_active && sch.ab_test_start && (
              <span className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                始于 {relTime(sch.ab_test_start)}
              </span>
            )}
          </div>
          <div className="text-[11px] mt-2 space-y-0.5" style={{ color: "var(--yunque-text-secondary)" }}>
            <div>新适配器：{ab?.new_adapter_queries ?? 0} 次 · 均分 {(ab?.new_adapter_score ?? 0).toFixed(3)}</div>
            <div>旧适配器：{ab?.old_adapter_queries ?? 0} 次 · 均分 {(ab?.old_adapter_score ?? 0).toFixed(3)}</div>
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="flex items-center gap-1.5 kpi-label mb-1">
            <Target size={13} /> 滚动成功率
          </div>
          <div className="kpi-value">{(rollRate * 100).toFixed(1)}%</div>
          <div className="text-[11px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>
            来自进化协调器窗口（近 {evolution?.recent_window?.length ?? 50} 条任务）
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="flex items-center gap-1.5 kpi-label mb-1">
            <RefreshCw size={13} /> 上次训练
          </div>
          <div className="kpi-value text-sm">{sch?.last_train_time ? fmtTime(sch.last_train_time) : "—"}</div>
          <div className="text-[11px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>
            累计训练 {sch?.total_trains ?? 0} · 回滚 {sch?.total_rollbacks ?? 0}
          </div>
        </Card>
      </div>

      {/* 聚合统计 */}
      <Card className="section-card p-4">
        <div className="flex items-center gap-2 mb-3">
          <Layers size={16} style={{ color: "var(--yunque-accent)" }} />
          <h2 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
            训练聚合统计
          </h2>
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <div>
            <div className="text-[11px] kpi-label">总训练次数</div>
            <div className="text-lg font-semibold">{summary?.total_runs ?? 0}</div>
          </div>
          <div>
            <div className="text-[11px] kpi-label">成功率</div>
            <div className="text-lg font-semibold">
              {pct(summary?.success_count ?? 0, summary?.total_runs ?? 0)}
            </div>
          </div>
          <div>
            <div className="text-[11px] kpi-label">平均 loss</div>
            <div className="text-lg font-semibold">
              {summary && summary.total_runs > 0 ? summary.avg_loss.toFixed(4) : "—"}
            </div>
          </div>
          <div>
            <div className="text-[11px] kpi-label">平均时长</div>
            <div className="text-lg font-semibold">{fmtNs(summary?.avg_duration)}</div>
          </div>
        </div>
      </Card>

      {/* 进化三层状态 */}
      <Card className="section-card p-4">
        <div className="flex items-center gap-2 mb-3">
          <BrainCircuit size={16} style={{ color: "#c084fc" }} />
          <h2 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
            进化协调（记忆 → 策略 → 权重）
          </h2>
        </div>
        {evolution ? (
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-sm">
            <div className="rounded-lg p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
              <div className="text-[11px] kpi-label mb-1">记忆层</div>
              <div style={{ color: "var(--yunque-text-secondary)" }}>
                累计任务 <strong>{evolution.total_tasks}</strong> · 成功{" "}
                <strong>{evolution.success_tasks}</strong>
              </div>
            </div>
            <div className="rounded-lg p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
              <div className="text-[11px] kpi-label mb-1">策略层</div>
              <div style={{ color: "var(--yunque-text-secondary)" }}>
                距上次更新后任务 <strong>{evolution.tasks_since_strategy}</strong> · 策略更新次数{" "}
                <strong>{evolution.strategy_updates}</strong>
              </div>
              <div className="text-[11px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                上次：{evolution.last_strategy_update ? fmtTime(evolution.last_strategy_update) : "—"}
              </div>
            </div>
            <div className="rounded-lg p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
              <div className="text-[11px] kpi-label mb-1">权重层（LoRA）</div>
              <div style={{ color: "var(--yunque-text-secondary)" }}>
                距上次触发任务 <strong>{evolution.tasks_since_weights}</strong> · 权重触发{" "}
                <strong>{evolution.weight_triggers}</strong>
              </div>
              <div className="text-[11px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                上次：{evolution.last_weight_trigger ? fmtTime(evolution.last_weight_trigger) : "—"}
              </div>
            </div>
            <div className="sm:col-span-3 rounded-lg p-3" style={{ background: "rgba(139,92,246,0.08)" }}>
              <div className="text-[11px] kpi-label mb-1">滚动窗口成功率</div>
              <div className="text-xl font-semibold" style={{ color: "#c084fc" }}>
                {(evolution.rolling_success_rate * 100).toFixed(1)}%
              </div>
            </div>
          </div>
        ) : (
          <p className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>
            暂无进化状态数据
          </p>
        )}
      </Card>

      {/* 配置面板 */}
      <Card className="section-card p-4">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <Settings size={16} style={{ color: "var(--yunque-accent)" }} />
            <h2 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              调度器配置
            </h2>
          </div>
          <div className="flex gap-2">
            <Button size="sm" variant="ghost" onPress={() => setShowConfig(!showConfig)}>
              {showConfig ? "收起" : "展开"}
            </Button>
            {showConfig && (
              <Button
                size="sm"
                className="gap-1"
                isPending={busy === "save-config"}
                isDisabled={!!disabledReason}
                onPress={onSaveConfig}
              >
                <Save size={12} /> 保存
              </Button>
            )}
          </div>
        </div>
        {showConfig && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            <TextField value={configDraft.min_samples ?? ""} onChange={(v) => setConfigDraft({ ...configDraft, min_samples: v })}>
              <Label>最小样本数</Label>
              <Input />
            </TextField>
            <TextField value={configDraft.min_interval ?? ""} onChange={(v) => setConfigDraft({ ...configDraft, min_interval: v })}>
              <Label>最小间隔 (如 24h, 12h)</Label>
              <Input />
            </TextField>
            <TextField value={configDraft.eval_min_score ?? ""} onChange={(v) => setConfigDraft({ ...configDraft, eval_min_score: v })}>
              <Label>评估最低分 (0-1)</Label>
              <Input />
            </TextField>
            <TextField value={configDraft.max_adapters ?? ""} onChange={(v) => setConfigDraft({ ...configDraft, max_adapters: v })}>
              <Label>最大适配器数</Label>
              <Input />
            </TextField>
            <TextField value={configDraft.base_model ?? ""} onChange={(v) => setConfigDraft({ ...configDraft, base_model: v })}>
              <Label>基础模型</Label>
              <Input />
            </TextField>
            <TextField value={configDraft.ab_test_duration ?? ""} onChange={(v) => setConfigDraft({ ...configDraft, ab_test_duration: v })}>
              <Label>A/B 测试时长 (如 1h, 30m)</Label>
              <Input />
            </TextField>
          </div>
        )}
      </Card>

      {/* 训练历史 */}
      <Card>
        <div className="p-4 pb-0 flex items-center justify-between">
          <h2 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
            训练历史（最近 50 条）
          </h2>
        </div>
        {records.length === 0 ? (
          <div className="p-8 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>
            暂无训练记录
          </div>
        ) : (
          <Table>
            <Table.ScrollContainer>
              <Table.Content aria-label="训练历史" className="min-w-[900px]">
                <Table.Header>
                  <Table.Column isRowHeader>时间</Table.Column>
                  <Table.Column>适配器</Table.Column>
                  <Table.Column>loss</Table.Column>
                  <Table.Column>样本数</Table.Column>
                  <Table.Column>评估分</Table.Column>
                  <Table.Column>部署状态</Table.Column>
                </Table.Header>
                <Table.Body>
                  {records.map((r) => (
                    <Table.Row key={r.id}>
                      <Table.Cell>
                        <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                          {fmtTime(r.end_time || r.start_time)}
                        </span>
                      </Table.Cell>
                      <Table.Cell>
                        <span className="text-xs font-mono truncate max-w-[180px] block">{r.adapter_name}</span>
                      </Table.Cell>
                      <Table.Cell>
                        <span className="text-xs font-mono">{r.final_loss?.toFixed?.(4) ?? "—"}</span>
                      </Table.Cell>
                      <Table.Cell>
                        <span className="text-xs">{r.samples}</span>
                      </Table.Cell>
                      <Table.Cell>
                        <span className="text-xs font-mono">
                          {r.eval_score != null ? r.eval_score.toFixed(3) : "—"}
                        </span>
                      </Table.Cell>
                      <Table.Cell>
                        <div className="flex flex-wrap gap-1">
                          {r.deployed && (
                            <Chip size="sm" style={{ background: "rgba(23,201,100,0.15)", color: "#17c964" }}>
                              已部署
                            </Chip>
                          )}
                          {r.rolled_back && (
                            <Chip size="sm" style={{ background: "rgba(243,18,96,0.12)", color: "#f31260" }}>
                              已回滚
                            </Chip>
                          )}
                          {!r.deployed && !r.rolled_back && (
                            <Chip size="sm" style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)" }}>
                              —
                            </Chip>
                          )}
                          {!r.success && (
                            <Chip size="sm" style={{ background: "rgba(255,170,0,0.12)", color: "#ffaa00" }}>
                              失败
                            </Chip>
                          )}
                        </div>
                      </Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table>
        )}
      </Card>
    </div>
  );
}
