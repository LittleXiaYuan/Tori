"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { Spinner } from "@heroui/react";
import { api } from "@/lib/api";
import type { ConnectorView } from "@/lib/api-types";
import {
  GitBranch, Mail, Calendar, MessageSquare, Layers, BookOpen,
  ClipboardList, Plug, ArrowRight, Search, CheckCircle2, Globe2,
  AlertTriangle, Link2,
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

export default function ConnectorsPage() {
  const [connectors, setConnectors] = useState<ConnectorView[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [tokenInput, setTokenInput] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [browserConnected, setBrowserConnected] = useState(false);

  const load = useCallback(async () => {
    try {
      const [connectorRes, browserRes] = await Promise.all([
        api.connectorList(),
        api.browserExtStatus().catch(() => ({ connected: false })),
      ]);
      setConnectors(connectorRes.connectors || []);
      setBrowserConnected(!!browserRes.connected);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "加载连接器失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return connectors;
    return connectors.filter((conn) =>
      [conn.name, conn.description, conn.category, conn.id]
        .filter(Boolean)
        .some((value) => value.toLowerCase().includes(q))
    );
  }, [connectors, search]);

  const connected = filtered.filter((conn) => conn.status === "connected");
  const available = filtered.filter((conn) => conn.status !== "connected");
  const recommendedIds = new Set(["github", "gmail", "google_calendar", "notion", "slack", "linear", "jira"]);
  const recommended = available.filter((conn) => recommendedIds.has(conn.id)).slice(0, 6);
  const selected = filtered.find((conn) => conn.id === selectedId) || connectors.find((conn) => conn.id === selectedId) || available[0] || null;

  const handleConnect = async (id: string) => {
    if (!tokenInput.trim()) return;
    setBusy(id);
    setError("");
    try {
      await api.connectorConnect(id, tokenInput.trim());
      setTokenInput("");
      await load();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "连接失败");
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
      setError(e instanceof Error ? e.message : "断开失败");
    } finally {
      setBusy(null);
    }
  };

  const selectConnector = (id: string) => {
    setSelectedId(id);
    setTokenInput("");
  };

  const SelectedIcon = selected ? (iconMap[selected.icon] || Plug) : Plug;
  const selectedSupported = !!selected?.supported;

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
          <span className="flex-1">{error}</span>
          <button onClick={() => setError("")} className="opacity-70 transition-opacity hover:opacity-100">关闭</button>
        </div>
      )}

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
                    key={conn.id}
                    className="rounded-[22px] border p-4 text-left transition-all hover:-translate-y-0.5"
                    style={{
                      background: selectedId === conn.id
                        ? "linear-gradient(180deg, rgba(59,130,246,0.12), rgba(59,130,246,0.05))"
                        : "linear-gradient(180deg, rgba(255,255,255,0.035), rgba(255,255,255,0.015))",
                      borderColor: selectedId === conn.id ? "rgba(59,130,246,0.35)" : "var(--yunque-border)",
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
                    key={conn.id}
                    className="rounded-[22px] border p-4 text-left transition-all hover:-translate-y-0.5"
                    style={{
                      background: isSelected
                        ? "linear-gradient(180deg, rgba(59,130,246,0.12), rgba(59,130,246,0.04))"
                        : "rgba(255,255,255,0.02)",
                      borderColor: isConnected
                        ? "rgba(34,197,94,0.26)"
                        : isSelected
                          ? "rgba(59,130,246,0.35)"
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
                  {selected.action_count} 个动作
                </span>
                {!selectedSupported && (
                  <span className="rounded-full px-3 py-1 text-xs" style={{ background: "rgba(148,163,184,0.16)", color: "#cbd5e1" }}>
                    即将开放
                  </span>
                )}
              </div>

              {selected.status === "connected" ? (
                <div className="rounded-[20px] border p-4" style={{ background: "rgba(34,197,94,0.08)", borderColor: "rgba(34,197,94,0.2)" }}>
                  <div className="flex items-center gap-2 text-sm font-medium" style={{ color: "#22c55e" }}>
                    <CheckCircle2 size={16} />
                    已连接
                  </div>
                  <div className="mt-2 text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                    {selected.user_info || "这个连接器已经准备好在聊天中调用。"}
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
                  <label className="mb-2 block text-sm font-medium" style={{ color: "var(--yunque-text-secondary)" }}>
                    {selected.auth_type === "oauth2" ? "Access Token" : "API Token"}
                  </label>
                  <div
                    className="mb-3 flex items-center gap-3 rounded-[20px] border px-4 py-3"
                    style={{ background: "rgba(255,255,255,0.035)", borderColor: "var(--yunque-border)" }}
                  >
                    <Link2 size={16} style={{ color: "var(--yunque-text-muted)" }} />
                    <input
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
                      {selected.error}
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
                    这个连接器的卡片已经展示在产品里，但当前版本还没有可用的服务端 handler。先明确标记为 coming soon，避免用户误以为它已经可以连接。
                  </div>
                </div>
              )}

              <div className="mt-5 rounded-[20px] border p-4" style={{ background: "rgba(255,255,255,0.02)", borderColor: "var(--yunque-border)" }}>
                <div className="mb-2 flex items-center gap-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                  <Globe2 size={15} />
                  使用提示
                </div>
                <div className="text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
                  连接后，可以直接在聊天中使用自然语言调用，例如“列出我的 GitHub 仓库”或“查看今天的日程安排”。
                </div>
              </div>
            </>
          ) : (
            <div className="rounded-[26px] border px-6 py-12 text-center" style={{ background: "rgba(255,255,255,0.02)", borderColor: "var(--yunque-border)" }}>
              <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-[20px]" style={{ background: "rgba(255,255,255,0.05)" }}>
                <Plug size={22} style={{ color: "var(--yunque-text-secondary)" }} />
              </div>
              <div className="text-base font-semibold" style={{ color: "var(--yunque-text)" }}>选择一个连接器</div>
              <div className="mt-2 text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                从左侧选择应用后，这里会显示连接状态和配置入口。
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function Monitor(props: { size: number; style?: React.CSSProperties }) {
  return (
    <svg width={props.size} height={props.size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" style={props.style}>
      <rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
      <line x1="8" y1="21" x2="16" y2="21" />
      <line x1="12" y1="17" x2="12" y2="21" />
    </svg>
  );
}
