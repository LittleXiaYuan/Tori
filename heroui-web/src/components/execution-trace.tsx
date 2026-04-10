"use client";

import { useState } from "react";
import {
  Brain, Wrench, CheckCircle2, XCircle, RefreshCw,
  MessageCircle, Settings2, ClipboardList, Lock, LockOpen,
  Ban, MapPin, ChevronDown, ChevronRight, Plus, Minus,
} from "lucide-react";

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

function domainIcon(domain: string, type: string) {
  if (domain === "planner") {
    switch (type) {
      case "thinking": return <Brain size={14} />;
      case "tool_start": return <Wrench size={14} />;
      case "tool_result": return type.includes("fail") ? <XCircle size={14} /> : <CheckCircle2 size={14} />;
      case "reflect": return <RefreshCw size={14} />;
      default: return <MessageCircle size={14} />;
    }
  }
  if (domain === "workflow") {
    switch (type) {
      case "node_start": return <Settings2 size={14} />;
      case "node_done": return <CheckCircle2 size={14} />;
      case "node_failed": return <XCircle size={14} />;
      default: return <ClipboardList size={14} />;
    }
  }
  if (domain === "approval") {
    switch (type) {
      case "request": return <Lock size={14} />;
      case "approved": return <LockOpen size={14} />;
      case "denied": return <Ban size={14} />;
      default: return <ClipboardList size={14} />;
    }
  }
  return <MapPin size={14} />;
}

function domainColor(domain: string, type: string): string {
  if (domain === "planner") {
    switch (type) {
      case "thinking": return "#60a5fa";
      case "tool_start": return "#fbbf24";
      case "tool_result":
        return type.includes("fail") ? "#f87171" : "#34d399";
      case "reflect": return "#a78bfa";
      default: return "#60a5fa";
    }
  }
  if (domain === "workflow") {
    if (type === "node_failed") return "#f87171";
    if (type === "node_done") return "#34d399";
    return "#22d3ee";
  }
  if (domain === "approval") {
    if (type === "denied") return "#f87171";
    if (type === "approved") return "#34d399";
    return "#fb923c";
  }
  return "#6b7280";
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
    <div className="rounded-xl overflow-hidden text-[13px]" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
      <button
        className="flex items-center gap-2 w-full px-3.5 py-2.5 text-left transition-colors"
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        style={{ color: "var(--yunque-text)" }}
        onMouseEnter={(e) => e.currentTarget.style.background = "rgba(255,255,255,0.04)"}
        onMouseLeave={(e) => e.currentTarget.style.background = "transparent"}
      >
        <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
          {expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        </span>
        <span className="font-medium text-[13px]">
          执行过程 ({events.length} 步, {totalDuration})
        </span>
        {isLive && (
          <span className="text-[10px] font-semibold animate-pulse" style={{ color: "#34d399" }}>
            ● LIVE
          </span>
        )}
        <span className="ml-auto font-mono text-[11px] opacity-60" style={{ color: "var(--yunque-text-muted)" }}>
          {shortTrace}
        </span>
      </button>

      {expanded && (
        <div className="px-3.5 pb-3.5 pt-1">
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
  const icon = domainIcon(event.domain, event.type);
  const color = domainColor(event.domain, event.type);
  const offset = formatDuration(startTs, event.ts);
  const hasDetail = event.detail !== null && event.detail !== undefined;

  return (
    <div className="relative pl-7 pb-1.5">
      {/* Vertical line */}
      <div className="absolute left-[7px] top-0 bottom-0 w-0.5" style={{ background: "var(--yunque-border)" }} />
      {/* Dot */}
      <div
        className="absolute left-[3px] top-1.5 w-2.5 h-2.5 rounded-full"
        style={{ background: color, boxShadow: `0 0 6px ${color}66` }}
      />
      <div>
        <div
          className="flex items-center gap-1.5 leading-relaxed"
          onClick={() => hasDetail && setDetailOpen(!detailOpen)}
          style={{ cursor: hasDetail ? "pointer" : "default" }}
        >
          <span style={{ color }}>{icon}</span>
          <span className="font-medium text-xs whitespace-nowrap" style={{ color }}>
            {event.domain}.{event.type}
          </span>
          {event.meta.skill && (
            <span
              className="text-[11px] px-1.5 py-0.5 rounded max-w-[150px] truncate"
              style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)" }}
            >
              {event.meta.skill}
            </span>
          )}
          {event.meta.node_name && (
            <span
              className="text-[11px] px-1.5 py-0.5 rounded max-w-[150px] truncate"
              style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)" }}
            >
              {event.meta.node_name}
            </span>
          )}
          <span className="ml-auto font-mono text-[11px] opacity-60 shrink-0" style={{ color: "var(--yunque-text-muted)" }}>
            +{offset}
          </span>
          {hasDetail && (
            <span className="text-xs shrink-0" style={{ color: "var(--yunque-text-muted)" }}>
              {detailOpen ? <Minus size={12} /> : <Plus size={12} />}
            </span>
          )}
        </div>
        <div className="text-xs mt-0.5 leading-relaxed" style={{ color: "var(--yunque-text-muted)" }}>
          {event.summary}
        </div>
        {detailOpen && hasDetail && (
          <pre
            className="mt-1.5 p-2.5 rounded-lg font-mono text-[11px] whitespace-pre-wrap break-all max-h-[200px] overflow-y-auto"
            style={{ background: "rgba(0,0,0,0.2)", color: "var(--yunque-text-muted)", border: "1px solid var(--yunque-border)" }}
          >
            {JSON.stringify(event.detail, null, 2)}
          </pre>
        )}
      </div>
    </div>
  );
}

export default ExecutionTrace;
