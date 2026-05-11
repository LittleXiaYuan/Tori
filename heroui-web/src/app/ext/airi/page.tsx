"use client";

import { useEffect, useState, useCallback } from "react";
import { Card, Button, Spinner, Chip, Tooltip } from "@heroui/react";
import { Bot, Wifi, WifiOff, Server, Clock, Copy, Check, RefreshCw, Terminal, ArrowUpRight, ArrowDownRight } from "lucide-react";
import { getAuthHeaders } from "@/lib/api";
import { usePolling } from "@/lib/use-polling";
import { formatErrorMessage } from "@/lib/error-utils";

const BASE = process.env.NEXT_PUBLIC_API_BASE || "";

interface AiriStatus {
  plugin: string; connected: boolean; url?: string;
  module_name?: string; messages_sent?: number; messages_received?: number;
}

async function fetchAiriStatus(): Promise<AiriStatus> {
  const res = await fetch(`${BASE}/v1/ext/airi/status`, { headers: { ...getAuthHeaders() } });
  if (!res.ok) throw new Error(`${res.status}`);
  return res.json();
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const copy = () => { navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 2000); };
  return <Button isIconOnly aria-label="确认" size="sm" variant="ghost" onPress={copy}>{copied ? <Check size={12} style={{ color: "#22c55e" }} /> : <Copy size={12} />}</Button>;
}

export default function AiriPage() {
  const [status, setStatus] = useState<AiriStatus | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try { const s = await fetchAiriStatus(); setStatus(s); setError(""); }
    catch (e: unknown) { setError(formatErrorMessage(e, "获取 Airi 桥接状态失败")); }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  usePolling(refresh, 5000);

  if (loading) return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-[60vh] gap-3">
        <WifiOff size={40} style={{ color: "var(--yunque-text-muted)" }} />
        <p className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>无法获取 Airi 桥接状态</p>
        <p className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{error}</p>
      </div>
    );
  }

  const endpoints = [
    { method: "GET", path: "/v1/ext/airi/models", desc: "可用模型列表" },
    { method: "POST", path: "/v1/ext/airi/chat/completions", desc: "聊天补全 (支持流式)" },
    { method: "GET", path: "/v1/ext/airi/status", desc: "桥接连接状态" },
  ];

  const configs = [
    { title: "WebSocket 桥接模式", desc: "设置 Airi 服务端地址和连接令牌", vars: [
      { key: "AIRI_URL", value: "ws://127.0.0.1:6121/ws", desc: "Airi server-runtime WebSocket 地址" },
      { key: "AIRI_TOKEN", value: "", desc: "连接令牌（可选）" },
      { key: "AIRI_MODULE_NAME", value: "yunque-agent", desc: "注册的模块名称" },
    ]},
    { title: "OpenAI API 直连模式", desc: "在 Airi 设置中配置 API 地址", vars: [
      { key: "API Base URL", value: `${typeof window !== "undefined" ? window.location.origin : "http://localhost:9090"}/v1/ext/airi`, desc: "填入 Airi 的 API Base 设置" },
      { key: "Model", value: "yunque-airi", desc: "选择模型名称" },
    ]},
  ];

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      {/* Header */}
      <div className="flex items-center gap-3">
        <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: "rgba(0,111,238,0.1)" }}>
          <Bot size={22} style={{ color: "var(--yunque-accent)" }} />
        </div>
        <div>
          <h1 className="page-title">Airi 桥接</h1>
          <p className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>管理 Airi 桌宠连接状态与消息同步</p>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Connection Status */}
        <Card className="section-card p-5">
          <div className="flex items-center justify-between mb-5">
            <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>连接状态</span>
            <Chip size="sm"
              style={{ background: status?.connected ? "#22c55e20" : "#ef444420", color: status?.connected ? "#22c55e" : "#ef4444" }}>
              {status?.connected ? <Wifi size={12} className="inline mr-1" /> : <WifiOff size={12} className="inline mr-1" />}
              {status?.connected ? "已连接" : "未连接"}
            </Chip>
          </div>
          <div className="space-y-3">
            <div className="flex items-center gap-3"><Server size={14} style={{ color: "var(--yunque-text-muted)" }} /><div><div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>服务端地址</div><div className="text-xs font-mono" style={{ color: "var(--yunque-text)" }}>{status?.url || "—"}</div></div></div>
            <div className="flex items-center gap-3"><Bot size={14} style={{ color: "var(--yunque-text-muted)" }} /><div><div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>模块名称</div><div className="text-xs font-mono" style={{ color: "var(--yunque-text)" }}>{status?.module_name || "—"}</div></div></div>
          </div>
          {status?.url && (
            <div className="mt-4 p-2.5 rounded-lg flex items-center justify-between" style={{ background: "var(--yunque-bg)" }}>
              <code className="text-[11px] truncate" style={{ color: "var(--yunque-text-muted)" }}>{status.url}</code>
              <CopyButton text={status.url} />
            </div>
          )}
        </Card>

        {/* Message Stats */}
        <Card className="section-card p-5">
          <div className="text-sm font-semibold mb-5" style={{ color: "var(--yunque-text)" }}>消息统计</div>
          <div className="grid grid-cols-2 gap-4">
            {[
              { icon: ArrowUpRight, label: "已发送", value: status?.messages_sent ?? 0, color: "var(--yunque-accent)" },
              { icon: ArrowDownRight, label: "已接收", value: status?.messages_received ?? 0, color: "#22c55e" },
            ].map(({ icon: Icon, label, value, color }) => (
              <div key={label} className="p-4 rounded-xl text-center" style={{ background: "var(--yunque-bg)" }}>
                <Icon size={18} style={{ color }} className="mx-auto mb-1" />
                <div className="kpi-value">{value}</div>
                <div className="kpi-sub">{label}</div>
              </div>
            ))}
          </div>
        </Card>
      </div>

      {/* API Endpoints */}
      <Card className="section-card p-5">
        <div className="text-sm font-semibold mb-4" style={{ color: "var(--yunque-text)" }}>OpenAI 兼容 API</div>
        <p className="text-xs mb-4" style={{ color: "var(--yunque-text-muted)" }}>Airi 可以通过以下 OpenAI 兼容接口直接连接到云雀 Agent，无需额外配置。</p>
        <div className="space-y-2">
          {endpoints.map((ep) => (
            <div key={ep.path} className="flex items-center gap-3 px-3 py-2 rounded-lg" style={{ background: "var(--yunque-bg)" }}>
              <Chip size="sm" className="font-mono text-[10px] font-bold"
                style={{ background: ep.method === "POST" ? "rgba(0,111,238,0.15)" : "rgba(34,197,94,0.15)", color: ep.method === "POST" ? "var(--yunque-accent)" : "#22c55e" }}>
                {ep.method}
              </Chip>
              <code className="text-xs flex-1 font-mono" style={{ color: "var(--yunque-text)" }}>{ep.path}</code>
              <span className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{ep.desc}</span>
              <CopyButton text={ep.path} />
            </div>
          ))}
        </div>
      </Card>

      {/* Configuration Guide */}
      <Card className="section-card p-5">
        <div className="flex items-center gap-2 mb-4">
          <Terminal size={14} style={{ color: "var(--yunque-accent)" }} />
          <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>配置指南</span>
        </div>
        <div className="space-y-4">
          {configs.map((cfg) => (
            <div key={cfg.title} className="p-4 rounded-xl" style={{ background: "var(--yunque-bg)" }}>
              <div className="text-sm font-semibold mb-1" style={{ color: "var(--yunque-text)" }}>{cfg.title}</div>
              <div className="text-[11px] mb-3" style={{ color: "var(--yunque-text-muted)" }}>{cfg.desc}</div>
              <div className="space-y-1.5">
                {cfg.vars.map((v) => (
                  <div key={v.key} className="flex items-center gap-2 text-xs">
                    <code className="px-2 py-0.5 rounded" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-accent)" }}>{v.key}</code>
                    {v.value && <><span style={{ color: "var(--yunque-text-muted)" }}>=</span><code style={{ color: "var(--yunque-text)" }}>{v.value}</code></>}
                    <span className="ml-auto text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{v.desc}</span>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}
