"use client";

import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Card, Chip, Spinner, Tooltip } from "@heroui/react";
import { api } from "@/lib/api";
import { createBrowserIntentPackClient } from "@/lib/browser-intent-pack-client";
import type { ConnectorView } from "@/lib/api-types";
import { connectorRecoveryHint, type ConnectorRecoveryHint } from "@/lib/connector-recovery";
import { formatErrorMessage } from "@/lib/error-utils";
import {
  GitBranch, Mail, Calendar, MessageSquare, Layers, BookOpen,
  ClipboardList, Plug, ArrowRight, Search, CheckCircle2, Globe2,
  AlertTriangle, Link2, Monitor, ShieldAlert, ShieldCheck, ShieldQuestion, RefreshCw,
} from "lucide-react";

const iconMap: Record<string, React.ElementType> = {
  github: GitBranch,
  mail: Mail,
  calendar: Calendar,
  slack: MessageSquare,
  notion: BookOpen,
  linear: Layers,
  jira: ClipboardList,
};

const browserIntentClient = createBrowserIntentPackClient();

type ConnectorSafetyItem = {
  key: string;
  label: string;
  value: string;
  detail: string;
  ok: boolean;
  icon: React.ReactNode;
  actionLabel?: string;
  onAction?: () => void;
};

function connectorAllowlistCount(conn: ConnectorView): number {
  if (typeof conn.allowlist_count === "number") return conn.allowlist_count;
  if (conn.allowed_actions?.length) return conn.allowed_actions.length;
  return conn.supported ? conn.action_count || 0 : 0;
}

function connectorStatusEventLabel(event: ConnectorView["last_event"]): string {
  if (!event) return "暂无事件";
  const target = event.action_id ? ` ${event.action_id}` : "";
  const status = event.status === "ok" ? "成功" : "失败";
  switch (event.kind) {
    case "connect":
    case "oauth2_connect":
      return `连接${status}`;
    case "disconnect":
      return "已断开";
    case "restore":
      return "已从本地凭据恢复";
    case "refresh":
      return `刷新凭据${status}`;
    case "execute":
      return `执行${target}${status}`;
    default:
      return `${event.kind}${target} ${status}`;
  }
}

function connectorEventDetail(event: ConnectorView["last_event"]): string {
  if (!event) return "还没有连接、断开或执行记录。";
  const when = event.at ? new Date(event.at).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }) : "";
  return [when, event.message].filter(Boolean).join(" · ") || "最近事件已记录。";
}

function ConnectorRecoveryCallout({
  hint,
  actionLabel,
  onAction,
}: {
  hint: ConnectorRecoveryHint;
  actionLabel?: string;
  onAction?: () => void;
}) {
  const isDanger = hint.severity === "danger";

  return (
    <div
      className="mb-4 rounded-lg border p-3"
      style={{
        background: isDanger ? "rgba(239,68,68,0.08)" : "rgba(245,158,11,0.08)",
        borderColor: isDanger ? "rgba(239,68,68,0.2)" : "rgba(245,158,11,0.24)",
      }}
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: isDanger ? "#f87171" : "var(--yunque-warning)" }}>
            <ShieldAlert size={15} aria-hidden />
            {hint.title}
          </div>
          <div className="mt-1 text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
            {hint.summary}
          </div>
          <div className="mt-1 break-words text-xs" style={{ color: "var(--yunque-text-muted)" }}>
            {hint.detail}
          </div>
        </div>
        {onAction && actionLabel && (
          <Button className="shrink-0" size="sm" variant={isDanger ? "danger" : "outline"} onPress={onAction}>
            {actionLabel}
          </Button>
        )}
      </div>
    </div>
  );
}

function ConnectorSafetyOverview({
  items,
  onRefresh,
}: {
  items: ConnectorSafetyItem[];
  onRefresh: () => void;
}) {
  const blocked = items.filter((item) => !item.ok);
  const nextAction = blocked.find((item) => item.onAction);

  return (
    <Card className="section-card mb-4 p-4">
      <Card.Header className="flex-row items-start justify-between gap-3 p-0">
        <div>
          <Card.Title className="flex items-center gap-2 text-base font-extrabold" style={{ color: "var(--yunque-text)" }}>
            <ShieldCheck size={17} style={{ color: "var(--yunque-accent)" }} />
            连接器安全状态
          </Card.Title>
          {blocked.length > 0 && (
            <Card.Description className="mt-1 text-xs">
              先修浏览器、异常连接或授权。
            </Card.Description>
          )}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Chip size="sm" style={{
            background: blocked.length === 0 ? "var(--yunque-success-muted)" : "var(--yunque-warning-muted)",
            color: blocked.length === 0 ? "var(--yunque-success)" : "var(--yunque-warning)",
            fontSize: "var(--text-2xs)",
          }}>
            {blocked.length === 0 ? "可信任" : `${blocked.length} 项待处理`}
          </Chip>
          <Tooltip delay={0}>
            <Button isIconOnly aria-label="刷新连接器安全状态" size="sm" variant="ghost" onPress={onRefresh}>
              <RefreshCw size={14} />
            </Button>
            <Tooltip.Content>刷新状态</Tooltip.Content>
          </Tooltip>
        </div>
      </Card.Header>

      <Card.Content className="mt-4 grid gap-2 p-0 sm:grid-cols-2 xl:grid-cols-4">
        {items.map((item) => (
          <div
            key={item.key}
            className="min-h-[88px] rounded-lg border p-3"
            style={{
              background: item.ok ? "var(--yunque-surface-2)" : "rgba(245,158,11,0.08)",
              borderColor: item.ok ? "var(--yunque-border)" : "rgba(245,158,11,0.28)",
            }}
          >
            <div className="flex items-center justify-between gap-2">
              <span className="inline-flex items-center gap-2 text-xs font-bold" style={{ color: "var(--yunque-text-muted)" }}>
                {item.icon}
                {item.label}
              </span>
              {item.ok ? (
                <CheckCircle2 size={14} style={{ color: "var(--yunque-success)" }} />
              ) : (
                <ShieldAlert size={14} style={{ color: "var(--yunque-warning)" }} />
              )}
            </div>
            <div className="mt-3 truncate text-sm font-extrabold" style={{ color: "var(--yunque-text)" }}>
              {item.value}
            </div>
            {!item.ok && (
              <div className="mt-1 line-clamp-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                {item.detail}
              </div>
            )}
            {!item.ok && item.onAction && item.actionLabel && (
              <Button className="mt-3" size="sm" variant="outline" onPress={item.onAction}>
                {item.actionLabel}
              </Button>
            )}
          </div>
        ))}
      </Card.Content>

      {(blocked.length > 0 || (nextAction?.onAction && nextAction.actionLabel)) && (
      <Card.Footer className="mt-3 flex-col items-start gap-2 p-0 sm:flex-row sm:items-center sm:justify-between">
        {blocked.length > 0 && (
          <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
            先处理红黄项，再进入单个应用配置。
          </div>
        )}
        {nextAction?.onAction && nextAction.actionLabel && (
          <Button size="sm" onPress={nextAction.onAction}>
            {nextAction.actionLabel}
            <ArrowRight size={14} />
          </Button>
        )}
      </Card.Footer>
      )}
    </Card>
  );
}

export default function ConnectorsPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const tokenInputRef = useRef<HTMLInputElement>(null);
  const [connectors, setConnectors] = useState<ConnectorView[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [tokenInput, setTokenInput] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [browserConnected, setBrowserConnected] = useState(false);
  const focusedConnectorId = searchParams.get("focus") || searchParams.get("connector");

  const load = useCallback(async () => {
    try {
      const [connectorRes, browserRes] = await Promise.all([
        api.connectorList(),
        browserIntentClient.extensionStatus().catch(() => ({ connected: false })),
      ]);
      setConnectors(connectorRes.connectors || []);
      setBrowserConnected(!!browserRes.connected);
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "加载连接器失败"));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (focusedConnectorId) {
      setSelectedId(focusedConnectorId);
    }
  }, [focusedConnectorId]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return connectors;
    return connectors.filter((conn) =>
      [conn.name, conn.description, conn.category, conn.id]
        .filter(Boolean)
        .some((value) => value.toLowerCase().includes(q))
    );
  }, [connectors, search]);

  const supportedConnectors = connectors.filter((conn) => conn.supported);
  const connectedConnectors = connectors.filter((conn) => conn.status === "connected");
  const needsAttention = connectors.filter((conn) => conn.status === "error" || !!conn.error);
  const connected = filtered.filter((conn) => conn.status === "connected");
  const available = filtered.filter((conn) => conn.status !== "connected");
  const recommendedIds = new Set(["github", "gmail", "google_calendar", "notion", "slack", "linear", "jira"]);
  const recommended = available.filter((conn) => recommendedIds.has(conn.id)).slice(0, 6);
  const selected = selectedId
    ? filtered.find((conn) => conn.id === selectedId) || connectors.find((conn) => conn.id === selectedId) || null
    : needsAttention[0] || null;
  const allowlistSurface = connectors.reduce((sum, conn) => sum + connectorAllowlistCount(conn), 0);
  const connectedAllowlistSurface = connectedConnectors.reduce((sum, conn) => sum + connectorAllowlistCount(conn), 0);

  const handleConnect = async (id: string) => {
    if (!tokenInput.trim()) return;
    setBusy(id);
    setError("");
    try {
      await api.connectorConnect(id, tokenInput.trim());
      setTokenInput("");
      await load();
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "连接失败"));
    } finally {
      setBusy(null);
    }
  };

  const handleDisconnect = async (id: string) => {
    setBusy(id);
    setError("");
    try {
      await api.connectorDisconnect(id);
      await load();
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "断开失败"));
    } finally {
      setBusy(null);
    }
  };

  const selectConnector = (id: string) => {
    setSelectedId(id);
    setTokenInput("");
  };

  const firstAttention = needsAttention[0];
  const firstConnectable = supportedConnectors.find((conn) => conn.status !== "connected");
  const connectorSafetyItems: ConnectorSafetyItem[] = [
    {
      key: "browser",
      label: "浏览器配对",
      value: browserConnected ? "已配对" : "未配对",
      detail: browserConnected ? "真实浏览器可点击/输入/提取。" : "浏览器未在线，网页任务暂停。",
      ok: browserConnected,
      icon: <Monitor size={13} />,
      actionLabel: "打开浏览器包",
      onAction: () => router.push("/packs/browser"),
    },
    {
      key: "apps",
      label: "应用授权",
      value: `${connectedConnectors.length}/${supportedConnectors.length}`,
      detail: connectedConnectors.length > 0 ? "已授权应用可被任务调用。" : "未连接应用，跨工具流不可用。",
      ok: connectedConnectors.length > 0,
      icon: <Plug size={13} />,
      actionLabel: firstConnectable ? "选择应用" : undefined,
      onAction: firstConnectable ? () => selectConnector(firstConnectable.id) : undefined,
    },
    {
      key: "attention",
      label: "异常连接",
      value: `${needsAttention.length} 项`,
      detail: needsAttention.length > 0 ? "连接失败或凭据失效，需重新授权。" : "未发现连接错误。",
      ok: needsAttention.length === 0,
      icon: needsAttention.length > 0 ? <ShieldAlert size={13} /> : <ShieldCheck size={13} />,
      actionLabel: firstAttention ? "查看异常" : undefined,
      onAction: firstAttention ? () => selectConnector(firstAttention.id) : undefined,
    },
    {
      key: "surface",
      label: "Allowlist",
      value: `${allowlistSurface} 个动作`,
      detail: connectedAllowlistSurface > 0 ? `${connectedAllowlistSurface} 个已连接动作。` : `${connectors.length - supportedConnectors.length} 个连接器不可用或预览。`,
      ok: allowlistSurface > 0 || supportedConnectors.length === 0,
      icon: <ShieldQuestion size={13} />,
      actionLabel: firstConnectable ? "连接应用" : undefined,
      onAction: firstConnectable ? () => selectConnector(firstConnectable.id) : undefined,
    },
  ];

  const SelectedIcon = selected ? (iconMap[selected.icon] || Plug) : Plug;
  const selectedSupported = !!selected?.supported;
  const selectedAllowlist = selected?.allowed_actions || [];
  const selectedAllowlistCount = selected ? connectorAllowlistCount(selected) : 0;
  const selectedLastEvent = selected?.last_event;
  const selectedRecovery = selected ? connectorRecoveryHint(selected) : null;
  const selectedOpenedFromRecovery = !!selected && focusedConnectorId === selected.id;
  const showCapabilityBoundary = !!selected && (
    selected.status === "connected"
    || selectedOpenedFromRecovery
    || selectedAllowlist.length > 0
    || !!selectedRecovery
  );
  const showRecentEvent = !!selectedLastEvent;
  const selectedRecoveryAction = selected && selectedRecovery
    ? () => {
        if (selectedRecovery.kind === "browser") {
          router.push(selectedRecovery.href);
          return;
        }
        if (selectedRecovery.kind === "auth") {
          if (selected.status === "connected") {
            void handleDisconnect(selected.id);
            return;
          }
          tokenInputRef.current?.focus();
          return;
        }
        if (selectedRecovery.kind === "rate_limit" || selectedRecovery.kind === "upstream" || selectedRecovery.kind === "generic") {
          void load();
        }
      }
    : undefined;
  const selectedRecoveryActionLabel = selected && selectedRecovery
    ? selectedRecovery.kind === "allowlist"
      ? undefined
      : selectedRecovery.kind === "auth" && selected.status === "connected"
        ? "断开后重新授权"
        : selectedRecovery.actionLabel
    : undefined;

  if (loading) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <Spinner size="lg" />
      </div>
    );
  }

  return (
    <div>
      {error && (
        <div
          className="flex items-center gap-3 rounded-[20px] border px-4 py-3 text-sm"
          style={{ background: "rgba(239,68,68,0.08)", borderColor: "rgba(239,68,68,0.14)", color: "#f87171" }}
        >
          <AlertTriangle size={16} />
          <span className="flex-1">{formatErrorMessage(error, "加载连接器失败")}</span>
          <button type="button" onClick={() => setError("")} className="opacity-70 transition-opacity hover:opacity-100">关闭</button>
        </div>
      )}

      <ConnectorSafetyOverview items={connectorSafetyItems} onRefresh={load} />

      <div className="grid gap-6 lg:grid-cols-[1.25fr_0.9fr]">
        <div className="section-card">
          <div className="mb-5 flex items-center gap-3">
            <div
              className="flex h-12 flex-1 items-center gap-3 rounded-2xl border px-4"
              style={{ background: "rgba(255,255,255,0.03)", borderColor: "var(--yunque-border)" }}
            >
              <Search size={16} style={{ color: "var(--yunque-text-muted)" }} />
              <input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                aria-label="搜索连接器"
                placeholder="搜索连接器"
                className="w-full bg-transparent text-sm outline-none"
                style={{ color: "var(--yunque-text)" }}
              />
            </div>
            <div
              className="flex items-center gap-2 rounded-2xl border px-4 py-3 text-sm"
              style={{ background: "rgba(255,255,255,0.03)", borderColor: "var(--yunque-border)", color: "var(--yunque-text-secondary)" }}
            >
              <span>{connected.length} 已连接</span>
            </div>
          </div>

          <div className="mb-6">
            <div className="section-title">推荐</div>
            <div className="grid gap-3 sm:grid-cols-2">
              <button
                type="button"
                aria-label={browserConnected ? "My Browser 已连接" : "查看 My Browser 连接状态"}
                onClick={() => router.push("/packs/browser")}
                className="rounded-[22px] border p-4 text-left transition-all hover:-translate-y-0.5"
                style={{
                  background: "linear-gradient(180deg, rgba(255,255,255,0.035), rgba(255,255,255,0.015))",
                  borderColor: browserConnected ? "rgba(34,197,94,0.28)" : "var(--yunque-border)",
                }}
              >
                <div className="mb-3 flex items-center justify-between">
                  <div className="flex h-11 w-11 items-center justify-center rounded-2xl" style={{ background: browserConnected ? "rgba(34,197,94,0.14)" : "rgba(255,255,255,0.05)" }}>
                    <Monitor size={19} style={{ color: browserConnected ? "#22c55e" : "var(--yunque-text-secondary)" }} />
                  </div>
                  {browserConnected && <CheckCircle2 size={18} style={{ color: "#22c55e" }} />}
                </div>
                <div className="text-base font-semibold" style={{ color: "var(--yunque-text)" }}>My Browser</div>
                <div className="mt-1 text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
                  在你自己的浏览器中进行点击、输入、标注和结构化提取。
                </div>
              </button>

              {recommended.map((conn) => {
                const Icon = iconMap[conn.icon] || Plug;
                return (
                  <button
                    type="button"
                    key={conn.id}
                    aria-current={selectedId === conn.id ? "true" : undefined}
                    className="rounded-[22px] border p-4 text-left transition-all hover:-translate-y-0.5"
                    style={{
                      background: selectedId === conn.id
                        ? "linear-gradient(180deg, var(--yunque-accent-muted), var(--yunque-accent-soft))"
                        : "linear-gradient(180deg, rgba(255,255,255,0.035), rgba(255,255,255,0.015))",
                      borderColor: selectedId === conn.id ? "var(--yunque-border-accent)" : "var(--yunque-border)",
                    }}
                    onClick={() => selectConnector(conn.id)}
                  >
                    <div className="mb-3 flex items-center justify-between">
                      <div className="flex h-11 w-11 items-center justify-center rounded-2xl" style={{ background: "rgba(255,255,255,0.05)" }}>
                        <Icon size={19} style={{ color: "var(--yunque-text-secondary)" }} />
                      </div>
                      {conn.beta && (
                        <span className="rounded-full px-2 py-1 text-[10px]" style={{ background: "rgba(245,158,11,0.15)", color: "#f59e0b" }}>
                          Beta
                        </span>
                      )}
                      {!conn.supported && (
                        <span className="rounded-full px-2 py-1 text-[10px]" style={{ background: "rgba(148,163,184,0.16)", color: "#cbd5e1" }}>
                          即将开放
                        </span>
                      )}
                    </div>
                    <div className="text-base font-semibold" style={{ color: "var(--yunque-text)" }}>{conn.name}</div>
                    <div className="mt-1 text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
                      {conn.description}
                    </div>
                  </button>
                );
              })}
            </div>
          </div>

          <div>
            <div className="section-title">全部应用</div>
            <div className="grid gap-3 sm:grid-cols-2">
              {filtered.map((conn) => {
                const Icon = iconMap[conn.icon] || Plug;
                const isConnected = conn.status === "connected";
                const isSelected = selectedId === conn.id;
                return (
                  <button
                    type="button"
                    key={conn.id}
                    aria-current={isSelected ? "true" : undefined}
                    className="rounded-[22px] border p-4 text-left transition-all hover:-translate-y-0.5"
                    style={{
                      background: isSelected
                        ? "linear-gradient(180deg, var(--yunque-accent-muted), var(--yunque-accent-soft))"
                        : "rgba(255,255,255,0.02)",
                      borderColor: isConnected
                        ? "rgba(34,197,94,0.26)"
                        : isSelected
                          ? "var(--yunque-border-accent)"
                          : "var(--yunque-border)",
                    }}
                    onClick={() => selectConnector(conn.id)}
                  >
                    <div className="mb-3 flex items-center justify-between">
                      <div className="flex h-11 w-11 items-center justify-center rounded-2xl" style={{ background: isConnected ? "rgba(34,197,94,0.14)" : "rgba(255,255,255,0.05)" }}>
                        <Icon size={19} style={{ color: isConnected ? "#22c55e" : "var(--yunque-text-secondary)" }} />
                      </div>
                      {isConnected ? (
                        <CheckCircle2 size={18} style={{ color: "#22c55e" }} />
                      ) : conn.beta ? (
                        <span className="rounded-full px-2 py-1 text-[10px]" style={{ background: "rgba(245,158,11,0.15)", color: "#f59e0b" }}>
                          Beta
                        </span>
                      ) : !conn.supported ? (
                        <span className="rounded-full px-2 py-1 text-[10px]" style={{ background: "rgba(148,163,184,0.16)", color: "#cbd5e1" }}>
                          即将开放
                        </span>
                      ) : null}
                    </div>
                    <div className="text-base font-semibold" style={{ color: "var(--yunque-text)" }}>{conn.name}</div>
                    <div className="mt-1 text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
                      {conn.description}
                    </div>
                    <div className="mt-3 text-xs" style={{ color: isConnected ? "#22c55e" : "var(--yunque-text-muted)" }}>
                      {isConnected ? (conn.user_info || "已连接") : conn.supported ? "点击查看配置" : "即将开放"}
                    </div>
                  </button>
                );
              })}
            </div>
          </div>
        </div>

        <div className="section-card">
          <div className="section-title">配置面板</div>

          {selected ? (
            <>
              <div className="mb-5 flex items-start gap-4">
                <div className="flex h-14 w-14 items-center justify-center rounded-[20px]" style={{ background: selected.status === "connected" ? "rgba(34,197,94,0.14)" : "rgba(255,255,255,0.05)" }}>
                  <SelectedIcon size={22} style={{ color: selected.status === "connected" ? "#22c55e" : "var(--yunque-text-secondary)" }} />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <div className="text-xl font-semibold" style={{ color: "var(--yunque-text)" }}>{selected.name}</div>
                    {selected.beta && (
                      <span className="rounded-full px-2 py-1 text-[10px]" style={{ background: "rgba(245,158,11,0.15)", color: "#f59e0b" }}>
                        Beta
                      </span>
                    )}
                  </div>
                  <div className="mt-1 text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
                    {selected.description}
                  </div>
                </div>
              </div>

              <div className="mb-4 flex gap-2">
                <span className="rounded-full px-3 py-1 text-xs" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-secondary)" }}>
                  {selected.auth_type === "oauth2" ? "OAuth / Token" : "Token"}
                </span>
                <span className="rounded-full px-3 py-1 text-xs" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-secondary)" }}>
                  {selectedAllowlistCount} 个允许动作
                </span>
                {!selectedSupported && (
                  <span className="rounded-full px-3 py-1 text-xs" style={{ background: "rgba(148,163,184,0.16)", color: "#cbd5e1" }}>
                    即将开放
                  </span>
                )}
              </div>

              {selectedRecovery && (
                <ConnectorRecoveryCallout
                  hint={selectedRecovery}
                  actionLabel={selectedRecoveryActionLabel}
                  onAction={selectedRecoveryActionLabel ? selectedRecoveryAction : undefined}
                />
              )}

              {(showCapabilityBoundary || showRecentEvent) && (
                <div className={`mb-5 grid gap-3 ${showCapabilityBoundary && showRecentEvent ? "sm:grid-cols-2" : ""}`}>
                  {showCapabilityBoundary && (
                    <div className="rounded-[20px] border p-4" style={{ background: "rgba(255,255,255,0.02)", borderColor: "var(--yunque-border)" }}>
                      <div className="mb-2 flex items-center gap-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                        <ShieldCheck size={15} aria-hidden />
                        能力边界
                      </div>
                      <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                        {selectedSupported ? `allowlist 动作：${selectedAllowlistCount}` : "无已启用 handler"}
                      </div>
                      {selectedAllowlist.length > 0 && (
                        <div className="mt-3 flex flex-wrap gap-1.5">
                          {selectedAllowlist.slice(0, 5).map((action) => (
                            <Chip key={action} size="sm" variant="soft">{action}</Chip>
                          ))}
                          {selectedAllowlist.length > 5 && <Chip size="sm" variant="soft">+{selectedAllowlist.length - 5}</Chip>}
                        </div>
                      )}
                    </div>
                  )}
                  {showRecentEvent && (
                    <div className="rounded-[20px] border p-4" style={{ background: "rgba(255,255,255,0.02)", borderColor: "var(--yunque-border)" }}>
                      <div className="mb-2 flex items-center gap-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                        <ShieldQuestion size={15} aria-hidden />
                        最近事件
                      </div>
                      <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>{connectorStatusEventLabel(selectedLastEvent)}</div>
                      <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>{connectorEventDetail(selectedLastEvent)}</div>
                    </div>
                  )}
                </div>
              )}

              {selected.status === "connected" ? (
                <div className="rounded-[20px] border p-4" style={{ background: "rgba(34,197,94,0.08)", borderColor: "rgba(34,197,94,0.2)" }}>
                  <div className="flex items-center gap-2 text-sm font-medium" style={{ color: "#22c55e" }}>
                    <CheckCircle2 size={16} />
                    已连接
                  </div>
                  <div className="mt-2 text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                    {selected.user_info || "聊天可调用。"}
                  </div>
                  <button
                    onClick={() => handleDisconnect(selected.id)}
                    disabled={busy === selected.id}
                    className="mt-4 inline-flex items-center gap-2 rounded-2xl px-4 py-2 text-sm transition-colors"
                    style={{ background: "rgba(239,68,68,0.14)", color: "#f87171" }}
                  >
                    断开连接
                  </button>
                </div>
              ) : selectedSupported ? (
                <>
                  <label htmlFor="connector-token-input" className="mb-2 block text-sm font-medium" style={{ color: "var(--yunque-text-secondary)" }}>
                    {selected.auth_type === "oauth2" ? "Access Token" : "API Token"}
                  </label>
                  <div
                    className="mb-3 flex items-center gap-3 rounded-[20px] border px-4 py-3"
                    style={{ background: "rgba(255,255,255,0.035)", borderColor: "var(--yunque-border)" }}
                  >
                    <Link2 size={16} style={{ color: "var(--yunque-text-muted)" }} />
                    <input
                      ref={tokenInputRef}
                      id="connector-token-input"
                      type="password"
                      className="w-full bg-transparent text-sm outline-none"
                      style={{ color: "var(--yunque-text)" }}
                      placeholder={selected.id === "github" ? "ghp_xxxxxxxxxxxx" : "粘贴你的 Token"}
                      value={tokenInput}
                      onChange={(e) => setTokenInput(e.target.value)}
                      onKeyDown={(e) => e.key === "Enter" && handleConnect(selected.id)}
                    />
                  </div>
                  {selected.error && (
                    <div className="mb-3 rounded-2xl border px-4 py-3 text-sm" style={{ background: "rgba(239,68,68,0.08)", borderColor: "rgba(239,68,68,0.16)", color: "#f87171" }}>
                      {formatErrorMessage(selected.error, "连接失败")}
                    </div>
                  )}
                  <button
                    className="flex w-full items-center justify-center gap-2 rounded-[20px] px-4 py-3 text-sm font-medium transition-all"
                    style={{
                      background: tokenInput.trim() ? "var(--yunque-accent)" : "rgba(255,255,255,0.05)",
                      color: tokenInput.trim() ? "#fff" : "var(--yunque-text-muted)",
                    }}
                    disabled={!tokenInput.trim() || busy === selected.id}
                    onClick={() => handleConnect(selected.id)}
                  >
                    <span>{busy === selected.id ? "连接中..." : "连接此应用"}</span>
                    {busy === selected.id ? <Spinner size="sm" /> : <ArrowRight size={15} />}
                  </button>
                </>
              ) : (
                <div className="rounded-[20px] border p-4" style={{ background: "rgba(148,163,184,0.08)", borderColor: "rgba(148,163,184,0.16)" }}>
                  <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>后端尚未接入</div>
                  <div className="mt-2 text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
                    无服务端 handler；coming soon，不能连接。
                  </div>
                </div>
              )}

              {selected.status === "connected" && (
                <div className="mt-5 rounded-[20px] border p-4" style={{ background: "rgba(255,255,255,0.02)", borderColor: "var(--yunque-border)" }}>
                  <div className="mb-2 flex items-center gap-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                    <Globe2 size={15} />
                    使用提示
                  </div>
                  <div className="text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
                    连接后直接说：列 GitHub 仓库、看今日日程。
                  </div>
                </div>
              )}
            </>
          ) : (
            <div className="rounded-[26px] border px-6 py-12 text-center" style={{ background: "rgba(255,255,255,0.02)", borderColor: "var(--yunque-border)" }}>
              <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-[20px]" style={{ background: "rgba(255,255,255,0.05)" }}>
                <Plug size={22} style={{ color: "var(--yunque-text-secondary)" }} />
              </div>
              <div className="text-base font-semibold" style={{ color: "var(--yunque-text)" }}>选择一个连接器</div>
              <div className="mt-2 text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                选应用后配置授权。
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
