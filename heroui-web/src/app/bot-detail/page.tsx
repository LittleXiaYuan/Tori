"use client";

import { useEffect, useState, useCallback } from "react";
import { Card, Button, Spinner, Chip, Tooltip, Tabs, Switch } from "@heroui/react";
import { api, type PersonaMemoryBlock, type HeartbeatLog, type InboxItem, type BotInfo } from "@/lib/api";
import { Blocks, ArrowLeft, Settings2, ScanFace, Database, Radar, MailWarning, RefreshCw, Play, Circle, CheckCheck, Mail } from "lucide-react";
import EmptyState from "@/components/empty-state";
import Link from "next/link";
import { Suspense } from "react";
import { useSearchParams } from "next/navigation";

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="p-3 rounded-lg" style={{ background: "var(--yunque-bg)" }}>
      <div className="text-[10px] font-medium uppercase tracking-wider mb-1" style={{ color: "var(--yunque-text-muted)" }}>{label}</div>
      <div className={`text-sm ${mono ? "font-mono" : ""} truncate`} style={{ color: "var(--yunque-text)" }}>{value}</div>
    </div>
  );
}

function OverviewTab({ bot }: { bot: BotInfo }) {
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <Field label="ID" value={bot.id} mono />
        <Field label="Status" value={bot.status || "idle"} />
        <Field label="Active" value={bot.is_active ? "是" : "否"} />
        <Field label="创建时间" value={new Date(bot.created_at).toLocaleString()} />
      </div>
      {bot.description && (
        <div>
          <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--yunque-text-muted)" }}>描述</div>
          <div className="text-sm p-3 rounded-lg" style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)" }}>{bot.description}</div>
        </div>
      )}
      {bot.config && Object.keys(bot.config).length > 0 && (
        <div>
          <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--yunque-text-muted)" }}>配置</div>
          <pre className="text-xs p-3 rounded-lg overflow-auto font-mono" style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)" }}>
            {JSON.stringify(bot.config, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}

function PersonaTab() {
  const [identity, setIdentity] = useState("");
  const [soul, setSoul] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getPersona().then((r) => { setIdentity(r.identity || ""); setSoul(r.soul || ""); }).catch(() => {}).finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="flex items-center justify-center py-12"><Spinner size="sm" /></div>;

  return (
    <div className="space-y-4">
      <div>
        <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--yunque-text-muted)" }}>身份设定</div>
        <div className="text-sm p-3 rounded-lg whitespace-pre-wrap" style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)" }}>
          {identity || "未设置"}
        </div>
      </div>
      <div>
        <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--yunque-text-muted)" }}>灵魂设定</div>
        <div className="text-sm p-3 rounded-lg whitespace-pre-wrap" style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)" }}>
          {soul || "未设置"}
        </div>
      </div>
    </div>
  );
}

function MemoryTab() {
  const [blocks, setBlocks] = useState<PersonaMemoryBlock[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getMemoryPersona().then(setBlocks).catch(() => {}).finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="flex items-center justify-center py-12"><Spinner size="sm" /></div>;

  return (
    <div className="space-y-3">
      {blocks.length === 0 ? (
        <EmptyState icon={<Database size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无记忆块" description="Bot 运行后的记忆会在此展示" />
      ) : blocks.map((b) => (
        <Card key={b.id} className="p-4" style={{ background: "var(--yunque-bg)" }}>
          <div className="flex items-center gap-2 mb-2">
            <Chip size="sm" style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)", fontSize: 10 }}>{b.label}</Chip>
            {b.read_only && <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)", fontSize: 10 }}>只读</Chip>}
            <span className="text-[10px] ml-auto" style={{ color: "var(--yunque-text-muted)" }}>v{b.version}</span>
          </div>
          <div className="text-sm whitespace-pre-wrap" style={{ color: "var(--yunque-text)" }}>{b.content}</div>
        </Card>
      ))}
    </div>
  );
}

function HeartbeatTab() {
  const [running, setRunning] = useState(false);
  const [logs, setLogs] = useState<HeartbeatLog[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      api.getHeartbeat().then((r) => setRunning(r.running)),
      api.getHeartbeatLogs(10).then((r) => setLogs(Array.isArray(r) ? r : [])),
    ]).catch(() => {}).finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="flex items-center justify-center py-12"><Spinner size="sm" /></div>;

  return (
    <div className="space-y-4">
      <Card className="p-4" style={{ background: "var(--yunque-bg)" }}>
        <div className="flex items-center gap-3">
          <Circle size={10} fill={running ? "#17c964" : "var(--yunque-text-muted)"} style={{ color: running ? "#17c964" : "var(--yunque-text-muted)" }} />
          <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{running ? "心跳运行中" : "心跳已停止"}</span>
        </div>
      </Card>
      {logs.length === 0 ? (
        <EmptyState icon={<Radar size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无心跳日志" description="启用心跳后日志将在此显示" />
      ) : (
        <div className="space-y-2">
          {logs.map((log) => (
            <div key={log.id} className="flex items-start gap-3 p-3 rounded-lg" style={{ background: "var(--yunque-bg)" }}>
              <Circle size={8} className="mt-1.5 shrink-0" fill={log.status === "ok" ? "#17c964" : "var(--yunque-text-muted)"} style={{ color: log.status === "ok" ? "#17c964" : "var(--yunque-text-muted)" }} />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <Chip size="sm" style={{ background: log.status === "ok" ? "rgba(23,201,100,0.1)" : "rgba(255,255,255,0.04)", color: log.status === "ok" ? "#17c964" : "var(--yunque-text-muted)" }}>{log.status?.toUpperCase()}</Chip>
                  {log.duration && <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{log.duration}</span>}
                </div>
                {log.result && <div className="text-sm truncate mt-1" style={{ color: "var(--yunque-text)" }}>{log.result}</div>}
                {log.error && <div className="text-sm truncate mt-1" style={{ color: "#f31260" }}>{log.error}</div>}
                <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>{new Date(log.started_at).toLocaleString()}</div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function InboxTab() {
  const [items, setItems] = useState<InboxItem[]>([]);
  const [unread, setUnread] = useState(0);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getInbox().then((r) => { setItems(r.items || []); setUnread(r.count?.unread || 0); }).catch(() => {}).finally(() => setLoading(false));
  }, []);

  const markAllRead = async () => {
    await api.markAllInboxRead();
    setItems((prev) => prev.map((i) => ({ ...i, is_read: true })));
    setUnread(0);
  };

  if (loading) return <div className="flex items-center justify-center py-12"><Spinner size="sm" /></div>;

  return (
    <div className="space-y-3">
      {unread > 0 && (
        <div className="flex justify-end">
          <Button size="sm" variant="ghost" onPress={markAllRead}><CheckCheck size={12} /> 全部已读</Button>
        </div>
      )}
      {items.length === 0 ? (
        <EmptyState icon={<Mail size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无消息" description="收到的内部消息将在此展示" />
      ) : items.slice(0, 20).map((item) => (
        <div key={item.id} className="flex items-start gap-3 p-3 rounded-lg" style={{ background: "var(--yunque-bg)" }}>
          <Circle size={6} className="mt-2 shrink-0" fill={item.is_read ? "var(--yunque-text-muted)" : "var(--yunque-accent)"} style={{ color: item.is_read ? "var(--yunque-text-muted)" : "var(--yunque-accent)" }} />
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 mb-0.5">
              <Chip size="sm" style={{ background: "rgba(255,255,255,0.04)" }}>{item.source}</Chip>
              <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{new Date(item.created_at).toLocaleString()}</span>
            </div>
            <div className="text-sm truncate" style={{ color: "var(--yunque-text)" }}>{item.content}</div>
          </div>
        </div>
      ))}
    </div>
  );
}

function BotDetailContent() {
  const searchParams = useSearchParams();
  const id = searchParams.get("id") || "";
  const [bot, setBot] = useState<BotInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState("overview");

  useEffect(() => {
    if (!id) return;
    api.getBot(id).then(setBot).catch(() => {}).finally(() => setLoading(false));
  }, [id]);

  if (loading) return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;

  if (!bot) {
    return (
      <div className="text-center py-16" style={{ color: "var(--yunque-text-muted)" }}>
        <Blocks size={40} className="mx-auto mb-3 opacity-30" />
        <p>Bot 未找到</p>
        <Link href="/bots"><Button className="mt-4" size="sm">返回列表</Button></Link>
      </div>
    );
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <div className="flex items-center gap-3">
        <Link href="/bots"><Button isIconOnly aria-label="返回" variant="ghost" size="sm"><ArrowLeft size={18} /></Button></Link>
        <div className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold text-white" style={{ background: "var(--yunque-accent)" }}>
          {bot.name?.charAt(0) || "B"}
        </div>
        <div className="flex-1 min-w-0">
          <h1 className="text-xl font-bold truncate" style={{ color: "var(--yunque-text)" }}>{bot.name}</h1>
          <Chip size="sm" className="mt-0.5" style={{ background: bot.is_active ? "#22c55e20" : "#9ca3af20", color: bot.is_active ? "#22c55e" : "#9ca3af" }}>
            {bot.is_active ? "活跃" : "非活跃"}
          </Chip>
        </div>
      </div>

      <Tabs selectedKey={activeTab} onSelectionChange={(k) => setActiveTab(k as string)}>
        <Tabs.ListContainer>
          <Tabs.List aria-label="Bot详情">
            <Tabs.Tab id="overview"><Settings2 size={14} /> 概览<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="persona"><Tabs.Separator /><ScanFace size={14} /> Persona<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="memory"><Tabs.Separator /><Database size={14} /> Memory<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="heartbeat"><Tabs.Separator /><Radar size={14} /> Heartbeat<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="inbox"><Tabs.Separator /><MailWarning size={14} /> Inbox<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>
        <Tabs.Panel id="overview">{bot && <OverviewTab bot={bot} />}</Tabs.Panel>
        <Tabs.Panel id="persona"><PersonaTab /></Tabs.Panel>
        <Tabs.Panel id="memory"><MemoryTab /></Tabs.Panel>
        <Tabs.Panel id="heartbeat"><HeartbeatTab /></Tabs.Panel>
        <Tabs.Panel id="inbox"><InboxTab /></Tabs.Panel>
      </Tabs>
    </div>
  );
}

export default function BotDetailPage() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>}>
      <BotDetailContent />
    </Suspense>
  );
}
