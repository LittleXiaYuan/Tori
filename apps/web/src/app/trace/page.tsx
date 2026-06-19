"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Card, Chip, Input, Label, Spinner, TextField } from "@heroui/react";
import { Activity, ListFilter, RefreshCw, Search } from "lucide-react";
import EmptyState from "@/components/empty-state";
import ExecutionTrace from "@/components/execution-trace";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";
import { createTracePackClient, type TraceEventsResponse } from "@/lib/trace-pack-client";

const traceClient = createTracePackClient();

type TraceMode = "recent" | "task" | "trace";

const modeOptions: Array<{ key: TraceMode; label: string; placeholder: string }> = [
  { key: "recent", label: "最近轨迹", placeholder: "最近 50 条" },
  { key: "task", label: "按任务 ID", placeholder: "输入 task id" },
  { key: "trace", label: "按 Trace ID", placeholder: "输入 trace id" },
];

export default function TracePage() {
  const [mode, setMode] = useState<TraceMode>("recent");
  const [query, setQuery] = useState("");
  const [data, setData] = useState<TraceEventsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async (nextMode = mode, nextQuery = query) => {
    setLoading(true);
    setError("");
    try {
      const trimmed = nextQuery.trim();
      let res: TraceEventsResponse;
      if (nextMode === "task") {
        if (!trimmed) throw new Error("请输入任务 ID");
        res = await traceClient.byTask(trimmed);
      } else if (nextMode === "trace") {
        if (!trimmed) throw new Error("请输入 Trace ID");
        res = await traceClient.byTrace(trimmed);
      } else {
        res = await traceClient.recent(50);
      }
      setData(res);
    } catch (e) {
      setError(formatErrorMessage(e, "加载执行轨迹失败"));
    } finally {
      setLoading(false);
    }
  }, [mode, query]);

  useEffect(() => {
    void load("recent", "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const currentMode = useMemo(() => modeOptions.find((item) => item.key === mode) || modeOptions[0], [mode]);
  const events = data?.events || [];
  const distinctTasks = new Set(events.map((event) => event.meta?.task_id).filter(Boolean)).size;
  const distinctTraces = new Set(events.map((event) => event.trace_id).filter(Boolean)).size;

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader
        icon={<Activity size={20} />}
        title="执行轨迹"
        description="查看云雀最近的执行过程、按任务定位步骤，或用 trace id 回放一次执行。"
        onRefresh={() => load()}
      />

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <Card className="section-card p-4">
          <div className="kpi-label">事件</div>
          <div className="kpi-value">{data?.count ?? 0}</div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">任务</div>
          <div className="kpi-value">{distinctTasks || "-"}</div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Trace</div>
          <div className="kpi-value">{distinctTraces || "-"}</div>
        </Card>
      </div>

      <Card className="section-card p-4">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end">
          <div className="flex flex-wrap gap-2">
            {modeOptions.map((option) => (
              <button
                key={option.key}
                type="button"
                className="filter-pill"
                data-active={mode === option.key}
                onClick={() => {
                  setMode(option.key);
                  setError("");
                  if (option.key === "recent") void load("recent", "");
                }}
              >
                <ListFilter size={13} /> {option.label}
              </button>
            ))}
          </div>
          <div className="flex-1" />
          {mode !== "recent" && (
            <TextField value={query} onChange={setQuery} className="min-w-[280px]">
              <Label>{currentMode.label}</Label>
              <Input
                placeholder={currentMode.placeholder}
                onKeyDown={(e) => {
                  if (e.key === "Enter") void load();
                }}
              />
            </TextField>
          )}
          <Button className="btn-accent" isPending={loading} onPress={() => load()}>
            {mode === "recent" ? <RefreshCw size={14} /> : <Search size={14} />}
            {mode === "recent" ? "刷新" : "查询"}
          </Button>
        </div>
        {error && (
          <div className="mt-3 rounded-lg px-3 py-2 text-sm" style={{ background: "rgba(239,68,68,0.1)", color: "#ef4444" }}>
            {error}
          </div>
        )}
      </Card>

      <Card className="section-card p-4">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div>
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>轨迹事件</div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              默认展示用户安全文案；原始审计事件仍由后端 raw 模式保留。
            </div>
          </div>
          {data && (
            <div className="flex flex-wrap gap-1.5">
              <Chip size="sm" variant="soft">{data.raw ? "Raw" : "安全文案"}</Chip>
              {data.task_id && <Chip size="sm" variant="soft">任务 {data.task_id}</Chip>}
              {data.trace_id && <Chip size="sm" variant="soft">Trace {data.trace_id}</Chip>}
            </div>
          )}
        </div>
        {loading ? (
          <div className="flex h-40 items-center justify-center">
            <Spinner size="lg" />
          </div>
        ) : events.length === 0 ? (
          <EmptyState
            icon={<Activity size={24} style={{ color: "var(--yunque-accent)" }} />}
            title="暂无执行轨迹"
            description="运行一次对话、任务或工作流后，执行事件会显示在这里。"
          />
        ) : (
          <ExecutionTrace events={events} />
        )}
      </Card>
    </div>
  );
}
