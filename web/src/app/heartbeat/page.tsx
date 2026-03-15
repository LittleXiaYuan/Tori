"use client";

import { useEffect, useState } from "react";
import { api, type HeartbeatLog, type InboxItem } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { AnimatedList } from "@/components/ui/animated-list";
import { HeartPulse, Play, Circle, Mail, CheckCheck } from "lucide-react";

export default function HeartbeatPage() {
  const [running, setRunning] = useState(false);
  const [logs, setLogs] = useState<HeartbeatLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [triggering, setTriggering] = useState(false);
  // Inbox state (merged)
  const [inboxItems, setInboxItems] = useState<InboxItem[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);

  const load = async () => {
    try {
      const [hb, hbLogs, inbox] = await Promise.all([
        api.getHeartbeat(),
        api.getHeartbeatLogs(20),
        api.getInbox().catch(() => ({ items: [], count: { unread: 0, total: 0 } })),
      ]);
      setRunning(hb.running);
      setLogs(Array.isArray(hbLogs) ? hbLogs : []);
      setInboxItems(Array.isArray(inbox.items) ? inbox.items.slice(0, 20) : []);
      setUnreadCount(inbox.count?.unread || 0);
    } catch {
      /* offline */
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const markAllRead = async () => {
    try {
      await api.markAllInboxRead();
      setInboxItems((prev) => prev.map((i) => ({ ...i, is_read: true })));
      setUnreadCount(0);
    } catch { /* */ }
  };

  const trigger = async () => {
    setTriggering(true);
    try {
      await api.triggerHeartbeat();
      await load();
    } finally {
      setTriggering(false);
    }
  };

  const toggle = async (enabled: boolean) => {
    await api.updateHeartbeat(enabled);
    load();
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin" style={{ borderColor: "var(--text-muted)", borderTopColor: "transparent" }} />
      </div>
    );
  }

  return (
    <div className="max-w-4xl">
      <BlurFade delay={0}>
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-3">
            <HeartPulse size={20} />
            <h1 className="text-xl font-semibold tracking-tight">Heartbeat</h1>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={trigger}
              disabled={triggering}
              className="flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-medium transition-colors cursor-pointer"
              style={{ background: "var(--text)", color: "var(--bg)" }}
            >
              <Play size={12} />
              {triggering ? "Running..." : "Trigger Now"}
            </button>
          </div>
        </div>
      </BlurFade>

      <BlurFade delay={0.05}>
        <div className="rounded-xl border p-5 mb-6" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="relative">
                <Circle size={10} fill={running ? "var(--text)" : "var(--text-muted)"} style={{ color: running ? "var(--text)" : "var(--text-muted)" }} />
                {running && (
                  <div className="absolute inset-0 rounded-full animate-ping" style={{ background: "var(--text)", opacity: 0.3 }} />
                )}
              </div>
              <span className="text-sm font-medium">{running ? "Running" : "Stopped"}</span>
            </div>
            <button
              onClick={() => toggle(!running)}
              className="px-4 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer border"
              style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}
            >
              {running ? "Stop" : "Start"}
            </button>
          </div>
        </div>
      </BlurFade>

      <BlurFade delay={0.1}>
        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="text-xs font-medium uppercase tracking-wider mb-4" style={{ color: "var(--text-muted)" }}>
            Logs ({logs.length})
          </div>
          {logs.length === 0 ? (
            <div className="text-sm text-center py-12" style={{ color: "var(--text-muted)" }}>
              No heartbeat logs yet
            </div>
          ) : (
            <AnimatedList>
              {logs.map((log) => (
                <div key={log.id} className="flex items-start gap-3 p-3 rounded-lg" style={{ background: "var(--bg-hover)" }}>
                  <div className="mt-1 shrink-0">
                    <Circle
                      size={8}
                      fill={log.status === "ok" ? "var(--text)" : "var(--text-muted)"}
                      style={{ color: log.status === "ok" ? "var(--text)" : "var(--text-muted)" }}
                    />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-xs font-mono font-medium">{log.status?.toUpperCase()}</span>
                      {log.duration && (
                        <span className="text-xs" style={{ color: "var(--text-muted)" }}>{log.duration}</span>
                      )}
                    </div>
                    {log.result && <div className="text-sm truncate">{log.result}</div>}
                    {log.error && <div className="text-sm truncate" style={{ color: "var(--text-muted)" }}>{log.error}</div>}
                    <div className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>
                      {new Date(log.started_at).toLocaleString()}
                    </div>
                  </div>
                </div>
              ))}
            </AnimatedList>
          )}
        </div>
      </BlurFade>

      {/* Inbox section (merged) */}
      <BlurFade delay={0.15}>
        <div className="rounded-xl border p-5 mt-6" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <Mail size={14} style={{ color: "var(--text-muted)" }} />
              <span className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>
                Inbox {unreadCount > 0 && `(${unreadCount} unread)`}
              </span>
            </div>
            {unreadCount > 0 && (
              <button
                onClick={markAllRead}
                className="flex items-center gap-1.5 px-3 py-1 rounded-full text-[11px] cursor-pointer border transition-colors"
                style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}
              >
                <CheckCheck size={10} />
                Mark all read
              </button>
            )}
          </div>
          {inboxItems.length === 0 ? (
            <div className="text-sm text-center py-8" style={{ color: "var(--text-muted)" }}>
              No messages
            </div>
          ) : (
            <div className="space-y-1.5">
              {inboxItems.map((item) => (
                <div key={item.id} className="flex items-start gap-3 p-3 rounded-lg" style={{ background: "var(--bg-hover)" }}>
                  <div className="mt-1.5 shrink-0">
                    <Circle
                      size={6}
                      fill={item.is_read ? "var(--text-muted)" : "var(--text)"}
                      style={{ color: item.is_read ? "var(--text-muted)" : "var(--text)" }}
                    />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 mb-0.5">
                      <span className="text-[10px] px-1.5 py-0.5 rounded-full" style={{ background: "var(--bg)", color: "var(--text-muted)" }}>
                        {item.source}
                      </span>
                      <span className="text-[10px]" style={{ color: "var(--text-muted)" }}>
                        {new Date(item.created_at).toLocaleString()}
                      </span>
                    </div>
                    <div className="text-sm truncate">{item.content}</div>
                    {item.action && (
                      <div className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>{item.action}</div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </BlurFade>
    </div>
  );
}
