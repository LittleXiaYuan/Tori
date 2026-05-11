"use client";

import { useEffect, useState } from "react";
import { api, type HeartbeatLog, type InboxItem } from "@/lib/api";
import { Card, Button, Switch, Chip } from "@heroui/react";
import { HeartPulse, Play, Circle, Mail, CheckCheck } from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";

export default function HeartbeatPage() {
  const [running, setRunning] = useState(false);
  const [logs, setLogs] = useState<HeartbeatLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [triggering, setTriggering] = useState(false);
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
    } catch { /* offline */ }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, []);

  const markAllRead = async () => {
    try {
      await api.markAllInboxRead();
      setInboxItems((prev) => prev.map((i) => ({ ...i, is_read: true })));
      setUnreadCount(0);
    } catch (e) { showToast(e instanceof Error ? e.message : "操作失败", "error"); }
  };

  const trigger = async () => {
    setTriggering(true);
    try { await api.triggerHeartbeat(); await load(); }
    catch (e) { showToast(e instanceof Error ? e.message : "触发失败", "error"); }
    finally { setTriggering(false); }
  };

  const toggle = async (enabled: boolean) => {
    try { await api.updateHeartbeat(enabled); load(); }
    catch (e) { showToast(e instanceof Error ? e.message : "操作失败", "error"); }
  };

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader
        icon={<HeartPulse size={20} />}
        title="心跳"
        onRefresh={() => load()}
        actions={
          <Button size="sm" onPress={trigger} isPending={triggering} className="btn-accent">
            <Play size={12} /> 立即触发
          </Button>
        }
      />

      {/* Status Card */}
      <Card className="section-card p-5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="relative">
              <Circle size={10} fill={running ? "#17c964" : "var(--yunque-text-muted)"} style={{ color: running ? "#17c964" : "var(--yunque-text-muted)" }} />
              {running && <div className="absolute inset-0 rounded-full animate-ping" style={{ background: "#17c964", opacity: 0.3 }} />}
            </div>
            <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{running ? "运行中" : "已停止"}</span>
          </div>
          <Switch isSelected={running} onChange={(val) => toggle(val)} size="sm">
            <Switch.Control><Switch.Thumb /></Switch.Control>
          </Switch>
        </div>
      </Card>

      {/* Logs + Inbox side by side */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">

      {/* Logs */}
      <Card className="section-card p-5">
        <div className="text-xs font-medium uppercase tracking-wider mb-4" style={{ color: "var(--yunque-text-muted)" }}>
          日志 ({logs.length})
        </div>
        {logs.length === 0 ? (
          <div className="text-sm text-center py-12" style={{ color: "var(--yunque-text-muted)" }}>暂无心跳日志</div>
        ) : (
          <div className="space-y-2">
            {logs.map((log) => (
              <div key={log.id} className="flex items-start gap-3 p-3 rounded-lg" style={{ background: "rgba(255,255,255,0.02)" }}>
                <div className="mt-1 shrink-0">
                  <Circle size={8} fill={log.status === "ok" ? "#17c964" : "var(--yunque-text-muted)"} style={{ color: log.status === "ok" ? "#17c964" : "var(--yunque-text-muted)" }} />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 mb-1">
                    <Chip size="sm" style={{ background: log.status === "ok" ? "rgba(23,201,100,0.1)" : "rgba(255,255,255,0.04)", color: log.status === "ok" ? "#17c964" : "var(--yunque-text-muted)" }}>
                      {log.status?.toUpperCase()}
                    </Chip>
                    {log.duration && <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{log.duration}</span>}
                  </div>
                  {log.result && <div className="text-sm truncate" style={{ color: "var(--yunque-text)" }}>{log.result}</div>}
                  {log.error && <div className="text-sm truncate" style={{ color: "#f31260" }}>{formatErrorMessage(log.error, "心跳失败")}</div>}
                  <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>{new Date(log.started_at).toLocaleString()}</div>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* Inbox */}
      <Card className="section-card p-5">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Mail size={14} style={{ color: "var(--yunque-text-muted)" }} />
            <span className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--yunque-text-muted)" }}>
              收件箱 {unreadCount > 0 && `(${unreadCount} 未读)`}
            </span>
          </div>
          {unreadCount > 0 && (
            <Button size="sm" variant="ghost" onPress={markAllRead}>
              <CheckCheck size={12} /> 全部已读
            </Button>
          )}
        </div>
        {inboxItems.length === 0 ? (
          <div className="text-sm text-center py-8" style={{ color: "var(--yunque-text-muted)" }}>暂无消息</div>
        ) : (
          <div className="space-y-1.5">
            {inboxItems.map((item) => (
              <div key={item.id} className="flex items-start gap-3 p-3 rounded-lg" style={{ background: "rgba(255,255,255,0.02)" }}>
                <div className="mt-1.5 shrink-0">
                  <Circle size={6} fill={item.is_read ? "var(--yunque-text-muted)" : "var(--yunque-accent)"} style={{ color: item.is_read ? "var(--yunque-text-muted)" : "var(--yunque-accent)" }} />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 mb-0.5">
                    <Chip size="sm" style={{ background: "rgba(255,255,255,0.04)" }}>{item.source}</Chip>
                    <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{new Date(item.created_at).toLocaleString()}</span>
                  </div>
                  <div className="text-sm truncate" style={{ color: "var(--yunque-text)" }}>{item.content}</div>
                  {item.action && <div className="text-xs mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>{item.action}</div>}
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>

      </div>{/* end grid */}
    </div>
  );
}
