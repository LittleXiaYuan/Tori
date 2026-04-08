"use client";

import { Button, Chip } from "@heroui/react";
import { AlertTriangle, ExternalLink, Monitor } from "lucide-react";
import { ExecutionTrace, type AgentEvent } from "@/components/execution-trace";

export interface BrowserRuntimeSession {
  id?: string | null;
  status?: string;
  currentTabId?: number | null;
  currentUrl?: string;
  title?: string;
  lastAction?: string;
  takeover?: boolean;
  updatedAt?: number;
}

export interface BrowserBridgeState {
  connected?: boolean;
  wsUrl?: string;
  sessions?: number;
  takeover?: boolean;
  runtimeSession?: BrowserRuntimeSession;
}

export interface BrowserSessionNotice {
  tone: "info" | "success" | "warning" | "error";
  text: string;
}

export interface BrowserActionArtifactSummary {
  action?: string;
  url?: string;
  title?: string;
  elementCount?: number;
  tabId?: number | null;
  hasScreenshot?: boolean;
  textLength?: number;
  updatedAt?: number;
}

interface BrowserSessionCardProps {
  state: BrowserBridgeState | null;
  pendingAction?: string | null;
  notice?: BrowserSessionNotice | null;
  artifact?: BrowserActionArtifactSummary | null;
  traceEvents?: AgentEvent[];
  onAction: (type: string, extra?: Record<string, unknown>) => void;
  onOpenBrowserPage?: () => void;
  className?: string;
}

export function BrowserSessionCard({
  state,
  pendingAction,
  notice,
  artifact,
  traceEvents,
  onAction,
  onOpenBrowserPage,
  className = "",
}: BrowserSessionCardProps) {
  const session = state?.runtimeSession;
  const connected = Boolean(state?.connected);
  const takeover = Boolean(session?.takeover || state?.takeover);
  const visible = Boolean(session?.id || takeover || connected);

  if (!visible) return null;

  const statusLabel = takeover
    ? "????"
    : connected
      ? (session?.status || "???")
      : "?????";

  const statusStyle = takeover
    ? { background: "rgba(245,158,11,0.12)", color: "#f59e0b" }
    : connected
      ? { background: "rgba(59,130,246,0.12)", color: "#60a5fa" }
      : { background: "rgba(248,113,113,0.12)", color: "#f87171" };

  const statusHint = takeover
    ? "?????????Agent ???????????"
    : connected
      ? "?????????????????????????"
      : "??????????????????????????";

  const noticeStyle = notice?.tone === "error"
    ? { background: "rgba(248,113,113,0.12)", color: "#fca5a5" }
    : notice?.tone === "warning"
      ? { background: "rgba(245,158,11,0.12)", color: "#fbbf24" }
      : notice?.tone === "success"
        ? { background: "rgba(34,197,94,0.12)", color: "#86efac" }
        : { background: "rgba(59,130,246,0.12)", color: "#93c5fd" };

  const updatedLabel = session?.updatedAt
    ? new Date(session.updatedAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    : "";

  const artifactUpdatedLabel = artifact?.updatedAt
    ? new Date(artifact.updatedAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    : "";

  return (
    <div
      className={`rounded-[22px] border px-3 py-3 ${className}`.trim()}
      style={{
        background: takeover
          ? "linear-gradient(180deg, rgba(245,158,11,0.12), rgba(245,158,11,0.04))"
          : "linear-gradient(180deg, rgba(59,130,246,0.1), rgba(59,130,246,0.03))",
        borderColor: takeover ? "rgba(245,158,11,0.22)" : "rgba(59,130,246,0.18)",
      }}
    >
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <div
              className="flex h-8 w-8 items-center justify-center rounded-2xl"
              style={{ background: takeover ? "rgba(245,158,11,0.14)" : "rgba(59,130,246,0.14)" }}
            >
              {takeover ? <AlertTriangle size={14} style={{ color: "#f59e0b" }} /> : <Monitor size={14} style={{ color: "#60a5fa" }} />}
            </div>
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>?????</div>
            <Chip size="sm" style={{ ...statusStyle, fontSize: "var(--text-2xs)" }}>
              {statusLabel}
            </Chip>
            {typeof state?.sessions === "number" && state.sessions > 0 && (
              <span className="rounded-full px-2 py-1 text-[10px]" style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)" }}>
                {state.sessions} tabs
              </span>
            )}
          </div>
          <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
            {statusHint}
          </div>
          {session?.title && (
            <div className="mt-2 truncate text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
              {session.title}
            </div>
          )}
          {session?.currentUrl && (
            <div className="mt-1 truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
              {session.currentUrl}
            </div>
          )}
          <div className="mt-2 flex flex-wrap items-center gap-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
            {session?.lastAction && (
              <span className="rounded-full px-2 py-1" style={{ background: "rgba(255,255,255,0.05)" }}>
                ?????{session.lastAction}
              </span>
            )}
            {updatedLabel && (
              <span className="rounded-full px-2 py-1" style={{ background: "rgba(255,255,255,0.05)" }}>
                ??? {updatedLabel}
              </span>
            )}
          </div>
          {notice && (
            <div className="mt-2 inline-flex max-w-full items-center rounded-full px-2.5 py-1 text-[11px]" style={noticeStyle}>
              {notice.text}
            </div>
          )}
          {artifact && (
            <div className="mt-3 rounded-2xl border px-3 py-2.5" style={{ background: "rgba(255,255,255,0.03)", borderColor: "rgba(255,255,255,0.06)" }}>
              <div className="text-[11px] font-semibold uppercase tracking-[0.16em]" style={{ color: "var(--yunque-text-muted)" }}>
                ????
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2 text-[11px]" style={{ color: "var(--yunque-text-secondary)" }}>
                {artifact.action && (
                  <span className="rounded-full px-2 py-1" style={{ background: "rgba(59,130,246,0.12)", color: "#93c5fd" }}>
                    {artifact.action.replace("bridge/", "")}
                  </span>
                )}
                {typeof artifact.elementCount === "number" && (
                  <span className="rounded-full px-2 py-1" style={{ background: "rgba(255,255,255,0.05)" }}>
                    {artifact.elementCount} ???
                  </span>
                )}
                {typeof artifact.textLength === "number" && artifact.textLength > 0 && (
                  <span className="rounded-full px-2 py-1" style={{ background: "rgba(255,255,255,0.05)" }}>
                    {artifact.textLength} chars
                  </span>
                )}
                {artifact.hasScreenshot && (
                  <span className="rounded-full px-2 py-1" style={{ background: "rgba(34,197,94,0.12)", color: "#86efac" }}>
                    ?????
                  </span>
                )}
                {artifact.tabId && (
                  <span className="rounded-full px-2 py-1" style={{ background: "rgba(255,255,255,0.05)" }}>
                    Tab #{artifact.tabId}
                  </span>
                )}
                {artifactUpdatedLabel && (
                  <span className="rounded-full px-2 py-1" style={{ background: "rgba(255,255,255,0.05)" }}>
                    ??? {artifactUpdatedLabel}
                  </span>
                )}
              </div>
              {(artifact.title || artifact.url) && (
                <div className="mt-2 min-w-0">
                  {artifact.title && (
                    <div className="truncate text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                      {artifact.title}
                    </div>
                  )}
                  {artifact.url && (
                    <div className="mt-1 truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                      {artifact.url}
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
          {traceEvents && traceEvents.length > 0 && (
            <div className="mt-3">
              <ExecutionTrace events={traceEvents} />
            </div>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-2 md:justify-end">
          <Button
            size="sm"
            variant="ghost"
            className="rounded-full px-3"
            isDisabled={!session?.currentTabId || !!pendingAction}
            isPending={pendingAction === "bridge/switch-to-tab"}
            onPress={() => onAction("bridge/switch-to-tab", { tabId: session?.currentTabId })}
          >
            <ExternalLink size={14} />
            ?????
          </Button>
          {takeover ? (
            <Button
              size="sm"
              className="rounded-full px-3"
              style={{ background: "rgba(59,130,246,0.14)", color: "#93c5fd" }}
              isDisabled={!!pendingAction}
              isPending={pendingAction === "bridge/resume"}
              onPress={() => onAction("bridge/resume")}
            >
              ??????
            </Button>
          ) : (
            <Button
              size="sm"
              variant="ghost"
              className="rounded-full px-3"
              isDisabled={!connected || !!pendingAction}
              isPending={pendingAction === "bridge/takeover"}
              onPress={() => onAction("bridge/takeover", { reason: "User takeover from workspace" })}
            >
              ?????
            </Button>
          )}
          {!connected && onOpenBrowserPage && (
            <Button
              size="sm"
              variant="ghost"
              className="rounded-full px-3"
              onPress={onOpenBrowserPage}
            >
              ??????
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}

export default BrowserSessionCard;
