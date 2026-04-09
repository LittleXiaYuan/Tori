"use client";

import { Button, Chip } from "@heroui/react";
import { AlertTriangle, ExternalLink, Monitor } from "lucide-react";
import { ExecutionTrace, type AgentEvent } from "@/components/execution-trace";
import { browserActionLabel } from "@/lib/browser-action-labels";
import { useI18n } from "@/lib/i18n";

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
  preview?: string;
  suggestedCommand?: string;
  suggestedLabel?: string;
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
  onSuggestCommand?: (command: string) => void;
  className?: string;
  compact?: boolean;
}

export function BrowserSessionCard({
  state,
  pendingAction,
  notice,
  artifact,
  traceEvents,
  onAction,
  onOpenBrowserPage,
  onSuggestCommand,
  className = "",
  compact = false,
}: BrowserSessionCardProps) {
  const { t } = useI18n();
  const session = state?.runtimeSession;
  const connected = Boolean(state?.connected);
  const takeover = Boolean(session?.takeover || state?.takeover);
  const visible = Boolean(session?.id || takeover || connected);

  if (!visible) return null;

  const statusLabel = takeover ? t("browser.takeover") : connected ? session?.status || t("browser.connected") : t("browser.disconnected");
  const statusStyle = takeover
    ? { background: "rgba(245,158,11,0.12)", color: "#f59e0b" }
    : connected
      ? { background: "rgba(59,130,246,0.12)", color: "#60a5fa" }
      : { background: "rgba(248,113,113,0.12)", color: "#f87171" };
  const statusHint = takeover
    ? "You are currently controlling the browser. Resume when you want the agent to continue."
    : connected
      ? "The extension is connected and the agent can continue browsing in your own session."
      : "Connect the browser extension to enable live navigation, clicks, extraction, and resume flows.";

  const noticeStyle = notice?.tone === "error"
    ? { background: "rgba(248,113,113,0.12)", color: "#fca5a5" }
    : notice?.tone === "warning"
      ? { background: "rgba(245,158,11,0.12)", color: "#fbbf24" }
      : notice?.tone === "success"
        ? { background: "rgba(34,197,94,0.12)", color: "#86efac" }
        : { background: "rgba(59,130,246,0.12)", color: "#93c5fd" };

  const updatedLabel = session?.updatedAt ? new Date(session.updatedAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : "";
  const artifactUpdatedLabel = artifact?.updatedAt ? new Date(artifact.updatedAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : "";

  return (
    <div
      className={`browser-session-animated ${compact ? "rounded-[18px] px-2.5 py-2.5" : "rounded-[22px] px-3 py-3"} border ${className}`.trim()}
      style={{
        background: takeover
          ? "linear-gradient(180deg, rgba(245,158,11,0.12), rgba(245,158,11,0.04))"
          : "linear-gradient(180deg, rgba(59,130,246,0.1), rgba(59,130,246,0.03))",
        borderColor: takeover ? "rgba(245,158,11,0.22)" : "rgba(59,130,246,0.18)",
      }}
    >
      <div className={`flex ${compact ? "flex-col gap-2" : "flex-col gap-3 md:flex-row md:items-start md:justify-between"}`}>
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <div
              className="flex h-8 w-8 items-center justify-center rounded-2xl"
              style={{ background: takeover ? "rgba(245,158,11,0.14)" : "rgba(59,130,246,0.14)" }}
            >
              {takeover ? <AlertTriangle size={14} style={{ color: "#f59e0b" }} /> : <Monitor size={14} style={{ color: "#60a5fa" }} />}
            </div>
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{t("browser.runtime")}</div>
            <Chip size="sm" style={{ ...statusStyle, fontSize: "var(--text-2xs)", transition: "all var(--duration-base) ease" }}>{statusLabel}</Chip>
            {typeof state?.sessions === "number" && state.sessions > 0 && (
              <span className="rounded-full px-2 py-1 text-[10px]" style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)" }}>
                {state.sessions} tabs
              </span>
            )}
          </div>

          {(!compact || takeover || !connected) && <div className={`mt-2 text-xs ${compact ? "line-clamp-1" : ""}`} style={{ color: "var(--yunque-text-muted)" }}>{statusHint}</div>}
          {session?.title && <div className="mt-2 truncate text-sm" style={{ color: "var(--yunque-text-secondary)" }}>{session.title}</div>}
          {session?.currentUrl && <div className="mt-1 truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{session.currentUrl}</div>}

          <div className={`mt-2 flex flex-wrap items-center ${compact ? "gap-1.5" : "gap-2"} text-[11px]`} style={{ color: "var(--yunque-text-muted)" }}>
            {session?.lastAction && (
              <span className="rounded-full px-2 py-1" style={{ background: "rgba(255,255,255,0.05)" }}>
                {browserActionLabel(session.lastAction)}
              </span>
            )}
            {updatedLabel && (
              <span className="rounded-full px-2 py-1" style={{ background: "rgba(255,255,255,0.05)" }}>
                {t("browser.updated")} {updatedLabel}
              </span>
            )}
          </div>

          {notice && <div className="mt-2 inline-flex max-w-full items-center rounded-full px-2.5 py-1 text-[11px]" style={noticeStyle}>{notice.text}</div>}

          {artifact && (
            <div className={`animate-content-fade interactive-preview-panel mt-3 border ${compact ? "rounded-[16px] px-2.5 py-2" : "rounded-2xl px-3 py-2.5"}`} style={{ background: "rgba(255,255,255,0.03)", borderColor: "rgba(255,255,255,0.06)" }}>
              <div className="text-[11px] font-semibold uppercase tracking-[0.16em]" style={{ color: "var(--yunque-text-muted)" }}>{t("browser.latest")}</div>
              <div className={`mt-2 flex flex-wrap items-center ${compact ? "gap-1.5" : "gap-2"} text-[11px]`} style={{ color: "var(--yunque-text-secondary)" }}>
                {artifact.action && <span className="rounded-full px-2 py-1" style={{ background: "rgba(59,130,246,0.12)", color: "#93c5fd" }}>{browserActionLabel(artifact.action)}</span>}
                {typeof artifact.elementCount === "number" && <span className="rounded-full px-2 py-1" style={{ background: "rgba(255,255,255,0.05)" }}>{artifact.elementCount} {t("browser.elements")}</span>}
                {!compact && typeof artifact.textLength === "number" && artifact.textLength > 0 && <span className="rounded-full px-2 py-1" style={{ background: "rgba(255,255,255,0.05)" }}>{artifact.textLength} chars</span>}
                {artifact.hasScreenshot && <span className="rounded-full px-2 py-1" style={{ background: "rgba(34,197,94,0.12)", color: "#86efac" }}>{t("browser.screenshot")}</span>}
                {!compact && artifact.tabId && <span className="rounded-full px-2 py-1" style={{ background: "rgba(255,255,255,0.05)" }}>Tab #{artifact.tabId}</span>}
                {artifactUpdatedLabel && <span className="rounded-full px-2 py-1" style={{ background: "rgba(255,255,255,0.05)" }}>{t("browser.updated")} {artifactUpdatedLabel}</span>}
              </div>
              {(artifact.title || artifact.url) && (
                <div className="mt-2 min-w-0">
                  {artifact.title && <div className="truncate text-sm" style={{ color: "var(--yunque-text-secondary)" }}>{artifact.title}</div>}
                  {artifact.url && <div className="mt-1 truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{artifact.url}</div>}
                </div>
              )}
              {!compact && artifact.preview && <div className="mt-2 rounded-2xl px-3 py-2 text-xs leading-6" style={{ background: "rgba(15,23,42,0.35)", color: "var(--yunque-text-secondary)" }}>{artifact.preview}</div>}
              {(artifact.suggestedCommand || artifact.url) && (
                <div className="mt-3 flex flex-wrap items-center gap-2">
                  {artifact.suggestedCommand && onSuggestCommand && (
                    <Button size="sm" variant="ghost" className="rounded-full px-3" onPress={() => onSuggestCommand(artifact.suggestedCommand!)}>
                      {artifact.suggestedLabel || t("browser.next")}
                    </Button>
                  )}
                  {artifact.url && (
                    <Button size="sm" variant="ghost" className="rounded-full px-3" onPress={() => window.open(artifact.url, "_blank", "noopener,noreferrer")}>
                      Open page
                    </Button>
                  )}
                </div>
              )}
            </div>
          )}

          {!compact && traceEvents && traceEvents.length > 0 && (
            <div className="mt-3">
              <ExecutionTrace events={traceEvents} />
            </div>
          )}
        </div>

        <div className={`flex flex-wrap items-center ${compact ? "gap-1.5" : "gap-2"} md:justify-end`}>
          <Button
            size="sm"
            variant="ghost"
            className="rounded-full px-3"
            isDisabled={!session?.currentTabId || !!pendingAction}
            isPending={pendingAction === "bridge/switch-to-tab"}
            onPress={() => onAction("bridge/switch-to-tab", { tabId: session?.currentTabId })}
          >
            <ExternalLink size={14} />
            {t("browser.return")}
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
              {t("browser.resume")}
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
              {t("browser.handoff")}
            </Button>
          )}
          {!connected && onOpenBrowserPage && (
            <Button size="sm" variant="ghost" className="rounded-full px-3" onPress={onOpenBrowserPage}>
              {t("browser.open")}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}

export default BrowserSessionCard;
