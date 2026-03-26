"use client";

import { useEffect, useState, useCallback } from "react";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  Bot,
  Wifi,
  WifiOff,
  Send,
  ArrowUpRight,
  ArrowDownRight,
  Server,
  Clock,
  Copy,
  Check,
  RefreshCw,
  Terminal,
} from "lucide-react";

interface AiriStatus {
  plugin: string;
  connected: boolean;
  url?: string;
  module_name?: string;
  messages_sent?: number;
  messages_received?: number;
}

const BASE = process.env.NEXT_PUBLIC_API_BASE || "";

function getAuthHeaders(): Record<string, string> {
  const token = typeof window !== "undefined" ? localStorage.getItem("yunque_token") : "";
  if (token) return { Authorization: `Bearer ${token}` };
  return {};
}

async function fetchAiriStatus(): Promise<AiriStatus> {
  const res = await fetch(`${BASE}/v1/ext/airi/status`, {
    headers: { ...getAuthHeaders() },
  });
  if (!res.ok) throw new Error(`${res.status}`);
  return res.json();
}

function StatusBadge({ connected }: { connected: boolean }) {
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 6,
        padding: "4px 12px",
        borderRadius: 999,
        fontSize: 12,
        fontWeight: 600,
        background: connected ? "var(--success-bg)" : "var(--danger-bg)",
        color: connected ? "var(--success)" : "var(--danger)",
      }}
    >
      {connected ? <Wifi size={12} /> : <WifiOff size={12} />}
      {connected ? "已连接" : "未连接"}
    </span>
  );
}

function InfoItem({ icon: Icon, label, value }: { icon: React.ElementType; label: string; value: string | number }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10, padding: "10px 0" }}>
      <div style={{
        width: 32, height: 32, borderRadius: 8,
        background: "var(--bg-hover)",
        display: "flex", alignItems: "center", justifyContent: "center",
      }}>
        <Icon size={14} style={{ color: "var(--text-muted)" }} />
      </div>
      <div>
        <div style={{ fontSize: 11, color: "var(--text-muted)" }}>{label}</div>
        <div style={{ fontSize: 13, fontWeight: 500, fontFamily: "monospace" }}>{value}</div>
      </div>
    </div>
  );
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };
  return (
    <button onClick={copy} style={{
      background: "none", border: "none", cursor: "pointer",
      color: "var(--text-muted)", padding: 4,
    }}>
      {copied ? <Check size={12} style={{ color: "var(--success)" }} /> : <Copy size={12} />}
    </button>
  );
}

export default function AiriPage() {
  const [status, setStatus] = useState<AiriStatus | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const s = await fetchAiriStatus();
      setStatus(s);
      setError("");
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to fetch");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const iv = setInterval(refresh, 5000);
    return () => clearInterval(iv);
  }, [refresh]);

  if (loading) {
    return (
      <div className="animate-in" style={{ display: "flex", justifyContent: "center", alignItems: "center", height: "40vh" }}>
        <RefreshCw size={24} className="breathe" style={{ color: "var(--text-muted)" }} />
      </div>
    );
  }

  if (error) {
    return (
      <div className="animate-in" style={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", height: "40vh", gap: 12 }}>
        <WifiOff size={40} style={{ color: "var(--text-muted)" }} />
        <p style={{ color: "var(--text-muted)", fontSize: 13 }}>无法获取 Airi 桥接状态</p>
        <p style={{ color: "var(--text-muted)", fontSize: 11 }}>{error}</p>
      </div>
    );
  }

  return (
    <div className="animate-in">
      {/* Header */}
      <BlurFade delay={0}>
        <div style={{ marginBottom: 24 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <div style={{
              width: 40, height: 40, borderRadius: 12,
              background: "var(--accent-subtle)",
              display: "flex", alignItems: "center", justifyContent: "center",
            }}>
              <Bot size={22} style={{ color: "var(--accent)" }} />
            </div>
            <div>
              <h1 style={{ fontSize: 22, fontWeight: 700, letterSpacing: "-0.02em" }}>Airi 桥接</h1>
              <p style={{ fontSize: 13, color: "var(--text-muted)", marginTop: 2 }}>
                管理 Airi 桌宠连接状态与消息同步
              </p>
            </div>
          </div>
        </div>
      </BlurFade>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
        {/* Connection Status */}
        <BlurFade delay={0.02}>
          <div className="rounded-xl border card-hover" style={{
            background: "var(--bg-card)", borderColor: "var(--border)", padding: 20,
          }}>
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 20 }}>
              <span style={{ fontSize: 13, fontWeight: 600, color: "var(--text-secondary)" }}>连接状态</span>
              {status && <StatusBadge connected={status.connected} />}
            </div>

            <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <InfoItem icon={Server} label="服务端地址" value={status?.url || "—"} />
              <InfoItem icon={Bot} label="模块名称" value={status?.module_name || "—"} />
            </div>

            {status?.url && (
              <div style={{
                marginTop: 16, padding: "10px 12px", borderRadius: 8,
                background: "var(--bg-hover)", display: "flex",
                alignItems: "center", justifyContent: "space-between",
              }}>
                <code style={{ fontSize: 11, color: "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis" }}>
                  {status.url}
                </code>
                <CopyButton text={status.url} />
              </div>
            )}
          </div>
        </BlurFade>

        {/* Message Stats */}
        <BlurFade delay={0.04}>
          <div className="rounded-xl border card-hover" style={{
            background: "var(--bg-card)", borderColor: "var(--border)", padding: 20,
          }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: "var(--text-secondary)", marginBottom: 20 }}>
              消息统计
            </div>

            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
              <StatCard
                icon={ArrowUpRight}
                label="已发送"
                value={status?.messages_sent ?? 0}
                color="var(--accent)"
              />
              <StatCard
                icon={ArrowDownRight}
                label="已接收"
                value={status?.messages_received ?? 0}
                color="var(--success)"
              />
            </div>
          </div>
        </BlurFade>
      </div>

      {/* API Endpoints */}
      <BlurFade delay={0.06}>
        <div className="rounded-xl border card-hover" style={{
          background: "var(--bg-card)", borderColor: "var(--border)", padding: 20, marginTop: 16,
        }}>
          <div style={{ fontSize: 13, fontWeight: 600, color: "var(--text-secondary)", marginBottom: 16 }}>
            OpenAI 兼容 API
          </div>
          <p style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 16 }}>
            Airi 可以通过以下 OpenAI 兼容接口直接连接到云雀 Agent，无需额外配置。
          </p>

          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            <EndpointRow method="GET" path="/v1/ext/airi/models" desc="可用模型列表" />
            <EndpointRow method="POST" path="/v1/ext/airi/chat/completions" desc="聊天补全 (支持流式)" />
            <EndpointRow method="GET" path="/v1/ext/airi/status" desc="桥接连接状态" />
          </div>
        </div>
      </BlurFade>

      {/* Configuration Guide */}
      <BlurFade delay={0.08}>
        <div className="rounded-xl border card-hover" style={{
          background: "var(--bg-card)", borderColor: "var(--border)", padding: 20, marginTop: 16,
        }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 16 }}>
            <Terminal size={14} style={{ color: "var(--accent)" }} />
            <span style={{ fontSize: 13, fontWeight: 600, color: "var(--text-secondary)" }}>配置指南</span>
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            <ConfigItem
              title="WebSocket 桥接模式"
              desc="设置 Airi 服务端地址和连接令牌"
              envVars={[
                { key: "AIRI_URL", value: "ws://127.0.0.1:6121/ws", desc: "Airi server-runtime WebSocket 地址" },
                { key: "AIRI_TOKEN", value: "", desc: "连接令牌（可选）" },
                { key: "AIRI_MODULE_NAME", value: "yunque-agent", desc: "注册的模块名称" },
              ]}
            />
            <ConfigItem
              title="OpenAI API 直连模式"
              desc="在 Airi 设置中配置 API 地址"
              envVars={[
                { key: "API Base URL", value: `${typeof window !== 'undefined' ? window.location.origin : 'http://localhost:9090'}/v1/ext/airi`, desc: "填入 Airi 的 API Base 设置" },
                { key: "Model", value: "yunque-airi", desc: "选择模型名称" },
              ]}
            />
          </div>
        </div>
      </BlurFade>
    </div>
  );
}

function StatCard({ icon: Icon, label, value, color }: { icon: React.ElementType; label: string; value: number; color: string }) {
  return (
    <div style={{
      padding: 16, borderRadius: 10, background: "var(--bg-hover)",
      display: "flex", flexDirection: "column", alignItems: "center", gap: 4,
    }}>
      <Icon size={18} style={{ color }} />
      <span style={{ fontSize: 24, fontWeight: 700 }} className="count-up">{value}</span>
      <span style={{ fontSize: 11, color: "var(--text-muted)" }}>{label}</span>
    </div>
  );
}

function EndpointRow({ method, path, desc }: { method: string; path: string; desc: string }) {
  return (
    <div style={{
      display: "flex", alignItems: "center", gap: 10, padding: "8px 12px",
      borderRadius: 8, background: "var(--bg-hover)",
    }}>
      <span style={{
        fontSize: 10, fontWeight: 700, padding: "2px 6px", borderRadius: 4,
        background: method === "POST" ? "var(--accent-subtle)" : "var(--success-bg)",
        color: method === "POST" ? "var(--accent)" : "var(--success)",
        fontFamily: "monospace",
      }}>
        {method}
      </span>
      <code style={{ fontSize: 12, color: "var(--text)", flex: 1, fontFamily: "monospace" }}>{path}</code>
      <span style={{ fontSize: 11, color: "var(--text-muted)" }}>{desc}</span>
      <CopyButton text={path} />
    </div>
  );
}

function ConfigItem({ title, desc, envVars }: {
  title: string;
  desc: string;
  envVars: Array<{ key: string; value: string; desc: string }>;
}) {
  return (
    <div style={{ padding: 16, borderRadius: 10, background: "var(--bg-hover)" }}>
      <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4 }}>{title}</div>
      <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 12 }}>{desc}</div>
      <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
        {envVars.map((v) => (
          <div key={v.key} style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12 }}>
            <code style={{
              padding: "2px 8px", borderRadius: 4, background: "rgba(255,255,255,0.05)",
              color: "var(--accent)", fontFamily: "monospace", fontSize: 11,
              whiteSpace: "nowrap",
            }}>
              {v.key}
            </code>
            {v.value && (
              <>
                <span style={{ color: "var(--text-muted)" }}>=</span>
                <code style={{ color: "var(--text-secondary)", fontFamily: "monospace", fontSize: 11 }}>{v.value}</code>
              </>
            )}
            <span style={{ color: "var(--text-muted)", fontSize: 10, marginLeft: "auto" }}>{v.desc}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
