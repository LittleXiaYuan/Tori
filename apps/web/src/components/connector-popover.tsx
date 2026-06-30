"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { createPortal } from "react-dom";
import { Button, Input, ListBox, Label, Description, Spinner, Tooltip } from "@heroui/react";
import { api } from "@/lib/api";
import { createBrowserIntentPackClient } from "@/lib/browser-intent-pack-client";
import type { ConnectorView, ConnectorDef } from "@/lib/api-types";
import { useI18n } from "@/lib/i18n";
import {
  GitBranch,
  Mail,
  Calendar,
  MessageSquare,
  Layers,
  BookOpen,
  ClipboardList,
  Plug,
  Search,
  CheckCircle2,
  Monitor,
  Link2,
  ShieldCheck,
  X,
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

interface Props {
  open: boolean;
  onClose: () => void;
  browserConnected?: boolean;
}

function statusTone(status: ConnectorView["status"]) {
  switch (status) {
    case "connected":
      return { bg: "rgba(34,197,94,0.12)", border: "rgba(34,197,94,0.24)", text: "#22c55e", label: "Connected" };
    case "connecting":
      return { bg: "var(--yunque-accent-muted)", border: "var(--yunque-border-accent)", text: "var(--yunque-accent-strong)", label: "Connecting" };
    case "error":
      return { bg: "rgba(239,68,68,0.12)", border: "rgba(239,68,68,0.24)", text: "#f87171", label: "Needs attention" };
    default:
      return { bg: "rgba(255,255,255,0.04)", border: "rgba(255,255,255,0.08)", text: "var(--yunque-text-muted)", label: "Disconnected" };
  }
}

export function ConnectorPopover({ open, onClose, browserConnected }: Props) {
  const { t } = useI18n();
  const [connectors, setConnectors] = useState<ConnectorView[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<ConnectorDef | null>(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [tokenInput, setTokenInput] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [browserState, setBrowserState] = useState<boolean>(!!browserConnected);

  const load = useCallback(async () => {
    try {
      const res = await api.connectorList();
      const next = res.connectors || [];
      setConnectors(next);
      setSelectedId((curr) => curr || next[0]?.id || null);
    } catch {
      setConnectors([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    load();
    browserIntentClient.extensionStatus()
      .then((res) => setBrowserState(!!res.connected))
      .catch(() => setBrowserState(!!browserConnected));
  }, [open, load, browserConnected]);

  useEffect(() => {
    if (!open || !selectedId) return;
    setDetailLoading(true);
    api.connectorDetail(selectedId)
      .then((res) => setDetail(res.connector || null))
      .catch(() => setDetail(null))
      .finally(() => setDetailLoading(false));
  }, [open, selectedId]);

  const filteredConnectors = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return connectors;
    return connectors.filter((conn) =>
      [conn.name, conn.description, conn.category, conn.id]
        .filter(Boolean)
        .some((value) => value.toLowerCase().includes(q))
    );
  }, [connectors, search]);

  const selected =
    filteredConnectors.find((conn) => conn.id === selectedId) ||
    connectors.find((conn) => conn.id === selectedId) ||
    null;

  const handleConnect = async (id: string) => {
    if (!tokenInput.trim()) return;
    setBusy(id);
    try {
      await api.connectorConnect(id, tokenInput.trim());
      setTokenInput("");
      await load();
    } finally {
      setBusy(null);
    }
  };

  const handleDisconnect = async (id: string) => {
    setBusy(id);
    try {
      await api.connectorDisconnect(id);
      await load();
    } finally {
      setBusy(null);
    }
  };

  const selectedActions = detail?.actions || [];
  const SelectedIcon = selected ? (iconMap[selected.icon] || Plug) : Plug;
  const tone = selected ? statusTone(selected.status) : statusTone("disconnected");

  useEffect(() => {
    if (!open || typeof document === "undefined") return;
    const prevOverflow = document.body.style.overflow;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", onKeyDown);
    return () => {
      document.body.style.overflow = prevOverflow;
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [open, onClose]);

  if (!open || typeof document === "undefined") {
    return null;
  }

  return createPortal(
    <div className="fixed inset-0 z-[140]">
      <button
        type="button"
        aria-label="关闭连接器面板"
        className="absolute inset-0 bg-black/55 backdrop-blur-sm"
        onClick={onClose}
      />
      <div className="absolute inset-0 flex items-start justify-end p-3 md:p-5">
        <div
          className="animate-command-panel mt-12 flex h-[min(78vh,760px)] w-full max-w-[620px] flex-col overflow-hidden rounded-[24px] border border-white/10 shadow-2xl"
          style={{ background: "linear-gradient(180deg, rgba(17,24,39,0.96), rgba(9,9,11,0.98))", backdropFilter: "blur(18px)" }}
        >
          <div className="flex items-start justify-between gap-4 border-b border-white/8 px-4 py-3.5">
            <div className="min-w-0">
              <h2 className="text-lg font-semibold">{t("connector.title")}</h2>
              <p className="mt-1 text-[13px]" style={{ color: "var(--yunque-text-secondary)" }}>
                {t("connector.subtitle")}
              </p>
            </div>
            <Tooltip delay={0}>
              <Button isIconOnly variant="ghost" aria-label="关闭连接器面板" onPress={onClose}>
                <X size={18} />
              </Button>
              <Tooltip.Content>关闭</Tooltip.Content>
            </Tooltip>
          </div>

          <div className="min-h-0 flex-1 overflow-hidden p-0">
            {loading ? (
              <div className="flex h-full items-center justify-center">
                <Spinner size="lg" />
              </div>
            ) : (
              <div className="grid h-full min-h-0 grid-cols-[240px_1fr]">
                <div className="min-w-0 border-r border-white/8 p-3">
                  <div className="mb-3 flex items-center gap-2 rounded-[16px] border border-white/8 bg-white/4 px-3 py-2">
                    <Search size={15} style={{ color: "var(--yunque-text-muted)" }} />
                    <Input
                      aria-label={t("connector.search")}
                      className="w-full"
                      placeholder={t("connector.search")}
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                      variant="secondary"
                    />
                  </div>

                  <button
                    type="button"
                    onClick={() => setSelectedId(null)}
                    aria-current={selectedId === null ? "true" : undefined}
                    className="interactive-list-item mb-3 flex w-full items-center gap-3 rounded-[16px] border px-3 py-2.5 text-left transition-colors"
                    style={{
                      background: browserState ? "rgba(34,197,94,0.08)" : "rgba(255,255,255,0.03)",
                      borderColor: browserState ? "rgba(34,197,94,0.22)" : "rgba(255,255,255,0.08)",
                    }}
                  >
                    <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-white/6">
                      <Monitor size={18} style={{ color: browserState ? "#22c55e" : "var(--yunque-text-secondary)" }} />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="text-sm font-medium">{t("connector.browser")}</div>
                      <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                        {browserState ? t("connector.browserConnected") : t("connector.browserDisconnected")}
                      </div>
                    </div>
                  </button>

                  <ListBox aria-label="Connector list" selectionMode="single" selectionBehavior="replace" selectedKeys={selectedId ? new Set([selectedId]) : new Set()}>
                    {filteredConnectors.map((conn) => {
                      const Icon = iconMap[conn.icon] || Plug;
                      const connTone = statusTone(conn.status);
                      return (
                        <ListBox.Item key={conn.id} id={conn.id} textValue={conn.name} onAction={() => { setSelectedId(conn.id); setTokenInput(""); }}>
                          <div className="flex items-start gap-3">
                            <div className="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl" style={{ background: connTone.bg, color: connTone.text }}>
                              <Icon size={18} />
                            </div>
                            <div className="min-w-0 flex-1">
                              <Label className="block text-sm font-medium leading-6 break-words">{conn.name}</Label>
                              <Description className="mt-1 block text-xs leading-5 break-words">{conn.description}</Description>
                              <div className="mt-2 flex items-center gap-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                                <span className="rounded-full px-2 py-0.5" style={{ background: connTone.bg, color: connTone.text }}>
                                  {connTone.label}
                                </span>
                                <span>{conn.action_count} actions</span>
                              </div>
                            </div>
                            {conn.status === "connected" ? <CheckCircle2 size={16} style={{ color: "#22c55e" }} /> : null}
                          </div>
                        </ListBox.Item>
                      );
                    })}
                  </ListBox>
                </div>

                <div className="animate-content-fade min-h-0 overflow-y-auto p-4">
                  {selected ? (
                    <div className="space-y-3.5">
                      <div className="flex items-start gap-3">
                        <div className="flex h-11 w-11 items-center justify-center rounded-[16px]" style={{ background: tone.bg, color: tone.text }}>
                          <SelectedIcon size={20} />
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <h3 className="text-[16px] font-semibold">{selected.name}</h3>
                            <span className="rounded-full px-2 py-0.5 text-[11px]" style={{ background: tone.bg, color: tone.text }}>
                              {tone.label}
                            </span>
                          </div>
                          <p className="mt-1 text-sm break-words" style={{ color: "var(--yunque-text-secondary)" }}>
                            {selected.description}
                          </p>
                        </div>
                      </div>

                      <div className="rounded-[18px] border border-white/8 bg-white/3 p-3.5">
                        <div className="mb-2 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>
                          <ShieldCheck size={12} />
                          Connection status
                        </div>
                        <div className="text-sm" style={{ color: selected.status === "connected" ? "#86efac" : "var(--yunque-text-secondary)" }}>
                          {selected.status === "connected"
                            ? (selected.user_info || "This connector is ready to use from chat.")
                            : "Once connected, the agent can call this connector directly from the current thread."}
                        </div>

                        {selected.status === "connected" ? (
                          <Button className="mt-4" variant="danger" onPress={() => handleDisconnect(selected.id)}>
                            {busy === selected.id ? "Disconnecting..." : t("connector.disconnect")}
                          </Button>
                        ) : (
                          <div className="mt-3 flex items-end gap-2.5">
                            <div className="flex-1">
                              <Input
                                type="password"
                                placeholder={selected.auth_type === "oauth2" ? "Paste OAuth token" : "Paste API token"}
                                value={tokenInput}
                                onChange={(e) => setTokenInput(e.target.value)}
                                variant="secondary"
                              />
                            </div>
                            <Button isDisabled={!tokenInput.trim() || busy === selected.id} onPress={() => handleConnect(selected.id)}>
                              {busy === selected.id ? "Connecting..." : t("connector.connect")}
                            </Button>
                          </div>
                        )}
                      </div>

                      <div className="rounded-[18px] border border-white/8 bg-white/3 p-3.5">
                        <div className="mb-2 flex items-center justify-between gap-3">
                          <div className="text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>
                            {t("connector.actions")}
                          </div>
                          <span className="rounded-full bg-white/6 px-2 py-0.5 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                            {selected.action_count}
                          </span>
                        </div>

                        {detailLoading ? (
                          <div className="flex items-center gap-2 text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                            <Spinner size="sm" />
                            Loading actions...
                          </div>
                        ) : selectedActions.length > 0 ? (
                          <div className="grid gap-2">
                            {selectedActions.slice(0, 6).map((action) => (
                              <div key={action.id} className="rounded-[16px] border border-white/8 bg-white/2 p-2.5">
                                <div className="text-sm font-medium break-words">{action.name}</div>
                                <div className="mt-1 text-xs leading-5 break-words" style={{ color: "var(--yunque-text-secondary)" }}>
                                  {action.description}
                                </div>
                                <div className="mt-2 inline-flex items-center gap-2 rounded-full bg-white/5 px-2 py-1 text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
                                  <Link2 size={10} />
                                  /{selected.id}.{action.id}
                                </div>
                              </div>
                            ))}
                          </div>
                        ) : (
                          <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                            No action metadata yet, but you can still connect it and call it from chat.
                          </div>
                        )}
                      </div>
                    </div>
                  ) : (
                    <div className="rounded-[18px] border border-white/8 bg-white/3 p-4">
                      <div className="flex items-center gap-3">
                        <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-white/6">
                          <Monitor size={20} style={{ color: browserState ? "#22c55e" : "var(--yunque-text-secondary)" }} />
                        </div>
                        <div>
                          <div className="text-lg font-semibold">{t("connector.browser")}</div>
                          <div className="mt-1 text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                            {browserState
                              ? "The browser extension is connected and ready for navigation, marking, clicking, and extraction."
                              : "The browser extension is not connected yet."}
                          </div>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
