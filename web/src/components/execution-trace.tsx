"use client";

import { useState } from "react";
import {
  Brain, Wrench, CheckCircle2, XCircle,
  RefreshCw, MessageCircle, Settings2, ClipboardList,
  Lock, Unlock, Ban, MapPin,
  ChevronDown, ChevronRight, Plus, Minus,
  type LucideIcon,
} from "lucide-react";
import "./execution-trace.css";

/** Mirrors observe.AgentEvent from the Go backend. */
export interface AgentEvent {
  id: string;
  trace_id: string;
  span_id?: string;
  ts: string;
  domain: string;   // "planner" | "workflow" | "approval" | "agent"
  type: string;     // "thinking" | "tool_start" | "tool_result" | "reflect" | "node_start" | etc.
  summary: string;
  detail?: unknown;
  meta: {
    tenant_id?: string;
    session_id?: string;
    task_id?: string;
    skill?: string;
    node_id?: string;
    node_name?: string;
    instance_id?: string;
  };
}

function domainIcon(domain: string, type: string): LucideIcon {
  if (domain === "planner") {
    switch (type) {
      case "thinking": return Brain;
      case "tool_start": return Wrench;
      case "tool_result": return type.includes("fail") ? XCircle : CheckCircle2;
      case "reflect": return RefreshCw;
      default: return MessageCircle;
    }
  }
  if (domain === "workflow") {
    switch (type) {
      case "node_start": return Settings2;
      case "node_done": return CheckCircle2;
      case "node_failed": return XCircle;
      default: return ClipboardList;
    }
  }
  if (domain === "approval") {
    switch (type) {
      case "request": return Lock;
      case "approved": return Unlock;
      case "denied": return Ban;
      default: return ClipboardList;
    }
  }
  return MapPin;
}

function domainColor(domain: string, type: string): string {
  if (domain === "planner") {
    switch (type) {
      case "thinking": return "var(--trace-blue, #60a5fa)";
      case "tool_start": return "var(--trace-amber, #fbbf24)";
      case "tool_result":
        return type.includes("fail") ? "var(--trace-red, #f87171)" : "var(--trace-green, #34d399)";
      case "reflect": return "var(--trace-purple, #a78bfa)";
      default: return "var(--trace-blue, #60a5fa)";
    }
  }
  if (domain === "workflow") {
    if (type === "node_failed") return "var(--trace-red, #f87171)";
    if (type === "node_done") return "var(--trace-green, #34d399)";
    return "var(--trace-cyan, #22d3ee)";
  }
  if (domain === "approval") {
    if (type === "denied") return "var(--trace-red, #f87171)";
    if (type === "approved") return "var(--trace-green, #34d399)";
    return "var(--trace-orange, #fb923c)";
  }
  return "var(--text-muted)";
}

function formatDuration(startTs: string, evtTs: string): string {
  const diff = new Date(evtTs).getTime() - new Date(startTs).getTime();
  if (diff < 1000) return `${diff}ms`;
  return `${(diff / 1000).toFixed(1)}s`;
}

interface ExecutionTraceProps {
  events: AgentEvent[];
  isLive?: boolean;
}

export function ExecutionTrace({ events, isLive }: ExecutionTraceProps) {
  const [expanded, setExpanded] = useState(false);

  if (events.length === 0) return null;

  const firstTs = events[0]?.ts;
  const lastTs = events[events.length - 1]?.ts;
  const totalDuration = firstTs && lastTs ? formatDuration(firstTs, lastTs) : "—";
  const traceId = events[0]?.trace_id ?? "";
  const shortTrace = traceId.length > 12 ? traceId.slice(0, 12) + "…" : traceId;

  return (
    <div className="execution-trace">
      <button
        className="execution-trace__header"
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
      >
        <span className="execution-trace__toggle">
          {expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        </span>
        <span className="execution-trace__title">
          执行过程 ({events.length} 步, {totalDuration})
        </span>
        {isLive && <span className="execution-trace__live-badge">● LIVE</span>}
        <span className="execution-trace__trace-id">{shortTrace}</span>
      </button>

      {expanded && (
        <div className="execution-trace__timeline">
          {events.map((evt) => (
            <TraceItem key={evt.id} event={evt} startTs={firstTs} />
          ))}
        </div>
      )}
    </div>
  );
}

function TraceItem({ event, startTs }: { event: AgentEvent; startTs: string }) {
  const [detailOpen, setDetailOpen] = useState(false);
  const Icon = domainIcon(event.domain, event.type);
  const color = domainColor(event.domain, event.type);
  const offset = formatDuration(startTs, event.ts);
  const hasDetail = event.detail !== null && event.detail !== undefined;

  return (
    <div className="trace-item" style={{ "--trace-color": color } as React.CSSProperties}>
      <div className="trace-item__line" />
      <div className="trace-item__dot" />
      <div className="trace-item__content">
        <div
          className="trace-item__header"
          onClick={() => hasDetail && setDetailOpen(!detailOpen)}
          style={{ cursor: hasDetail ? "pointer" : "default" }}
        >
          <span className="trace-item__icon"><Icon size={14} /></span>
          <span className="trace-item__label">{event.domain}.{event.type}</span>
          {event.meta.skill && (
            <span className="trace-item__skill">{event.meta.skill}</span>
          )}
          {event.meta.node_name && (
            <span className="trace-item__skill">{event.meta.node_name}</span>
          )}
          <span className="trace-item__offset">+{offset}</span>
          {hasDetail && (
            <span className="trace-item__expand">{detailOpen ? <Minus size={10} /> : <Plus size={10} />}</span>
          )}
        </div>
        <div className="trace-item__summary">{event.summary}</div>
        {detailOpen && hasDetail && (
          <pre className="trace-item__detail">
            {JSON.stringify(event.detail, null, 2)}
          </pre>
        )}
      </div>
    </div>
  );
}

export default ExecutionTrace;
