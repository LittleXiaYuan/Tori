"use client";

import { useState, useEffect, useCallback } from "react";
import { api, type WorkerInfo } from "@/lib/api";
import { Card, Button, Spinner, Chip } from "@heroui/react";
import {
  Cpu, RefreshCw, Trash2, Copy, Plus,
  CheckCircle2, AlertTriangle, Clock,
  Monitor, Terminal, Globe,
} from "lucide-react";
import { showToast } from "@/components/toast-provider";
import EmptyState from "@/components/empty-state";

type ConfigTab = "cursor" | "claude_code" | "windsurf";

const STATUS_STYLE: Record<string, { bg: string; color: string }> = {
  online: { bg: "rgba(34,197,94,0.1)", color: "#22c55e" },
  busy: { bg: "rgba(245,158,11,0.1)", color: "#f59e0b" },
  offline: { bg: "rgba(239,68,68,0.1)", color: "#ef4444" },
};

const TYPE_ICON: Record<string, React.ReactNode> = {
  cursor: <Monitor size={16} />,
  claude_code: <Terminal size={16} />,
  windsurf: <Globe size={16} />,
  custom: <Cpu size={16} />,
};

export default function WorkersPage() {
  const [workers, setWorkers] = useState<WorkerInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [showConfig, setShowConfig] = useState(false);
  const [configTab, setConfigTab] = useState<ConfigTab>("cursor");
  const [configData, setConfigData] = useState<{ mcp_config: string; instructions: string; server_url: string } | null>(null);
  const [configLoading, setConfigLoading] = useState(false);

  const loadWorkers = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.listWorkers();
      setWorkers(res.workers || []);
    } catch {
      showToast("加载 Worker 列表失败", "error");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { loadWorkers(); }, [loadWorkers]);

  const handleRemove = async (id: string) => {
    try {
      await api.removeWorker(id);
      showToast("Worker 已移除", "success");
      loadWorkers();
    } catch {
      showToast("移除失败", "error");
    }
  };

  const loadConfig = async (type: ConfigTab) => {
    setConfigTab(type);
    setConfigLoading(true);
    try {
      const res = await api.getWorkerConfig(type);
      setConfigData(res);
    } catch {
      showToast("加载配置失败", "error");
    } finally {
      setConfigLoading(false);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    showToast("已复制到剪贴板", "success");
  };

  const timeSince = (dateStr: string) => {
    const d = new Date(dateStr);
    const now = new Date();
    const sec = Math.floor((now.getTime() - d.getTime()) / 1000);
    if (sec < 60) return `${sec}秒前`;
    const min = Math.floor(sec / 60);
    if (min < 60) return `${min}分钟前`;
    const hr = Math.floor(min / 60);
    if (hr < 24) return `${hr}小时前`;
    return `${Math.floor(hr / 24)}天前`;
  };

  return (
    <div className="flex flex-col h-full p-4 sm:p-6 overflow-auto" style={{ background: "var(--yunque-bg)" }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6 flex-wrap gap-3">
        <div>
          <h1 className="text-xl font-bold flex items-center gap-2" style={{ color: "var(--yunque-fg)" }}>
            <Cpu size={22} /> Worker 管理
          </h1>
          <p className="text-sm mt-1" style={{ color: "var(--yunque-fg-muted)" }}>
            管理已连接的外部 Worker（Cursor、Claude Code、Windsurf 等）
          </p>
        </div>
        <div className="flex gap-2">
          <Button size="sm" variant="ghost" onPress={loadWorkers}><RefreshCw size={14} /> 刷新</Button>
          <Button size="sm" variant="primary" onPress={() => { setShowConfig(true); loadConfig("cursor"); }}>
            <Plus size={14} /> 连接 Worker
          </Button>
        </div>
      </div>

      {/* Worker List */}
      {loading ? (
        <div className="flex justify-center py-20"><Spinner size="lg" /></div>
      ) : workers.length === 0 ? (
        <EmptyState
          icon={<Cpu size={40} />}
          title="暂无 Worker"
          description="点击「连接 Worker」获取配置说明，将 Cursor/Claude Code/Windsurf 连接到云雀"
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {workers.map((w) => {
            const sty = STATUS_STYLE[w.status] || STATUS_STYLE.offline;
            return (
              <Card key={w.id} className="p-4" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center gap-2">
                    <span className="p-1.5 rounded-lg" style={{ background: "var(--yunque-bg)" }}>
                      {TYPE_ICON[w.type] || TYPE_ICON.custom}
                    </span>
                    <div>
                      <div className="font-medium text-sm" style={{ color: "var(--yunque-fg)" }}>{w.name}</div>
                      <div className="text-xs" style={{ color: "var(--yunque-fg-muted)" }}>{w.type} · {w.id}</div>
                    </div>
                  </div>
                  <Chip size="sm" style={{ background: sty.bg, color: sty.color, fontSize: "var(--text-2xs)" }}>
                    {w.status === "online" && <CheckCircle2 size={10} />}
                    {w.status === "busy" && <Clock size={10} />}
                    {w.status === "offline" && <AlertTriangle size={10} />}
                    {" "}{w.status}
                  </Chip>
                </div>

                <div className="space-y-2 text-xs" style={{ color: "var(--yunque-fg-muted)" }}>
                  <div className="flex justify-between">
                    <span>能力标签</span>
                    <div className="flex gap-1 flex-wrap justify-end">
                      {w.capabilities.map((c) => (
                        <Chip key={c} size="sm" style={{ background: "rgba(255,255,255,0.05)", fontSize: "var(--text-2xs)" }}>{c}</Chip>
                      ))}
                    </div>
                  </div>
                  <div className="flex justify-between">
                    <span>活跃任务</span>
                    <span>{w.active_tasks} / {w.max_concurrency}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>最后心跳</span>
                    <span>{timeSince(w.last_heartbeat)}</span>
                  </div>
                </div>

                <div className="flex justify-end mt-3">
                  <Button size="sm" variant="ghost" onPress={() => handleRemove(w.id)} style={{ color: "#ef4444" }}>
                    <Trash2 size={12} /> 移除
                  </Button>
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {/* Config Modal */}
      {showConfig && (
        <div className="fixed inset-0 z-50 flex items-center justify-center" style={{ background: "rgba(0,0,0,0.5)" }}>
          <div className="w-full max-w-2xl mx-4 rounded-xl overflow-hidden" style={{ background: "var(--yunque-bg)", border: "1px solid var(--yunque-border)" }}>
            <div className="flex items-center justify-between p-4 border-b" style={{ borderColor: "var(--yunque-border)" }}>
              <h2 className="text-lg font-bold" style={{ color: "var(--yunque-fg)" }}>连接 Worker</h2>
              <Button size="sm" variant="ghost" isIconOnly onPress={() => setShowConfig(false)}>✕</Button>
            </div>

            <div className="p-4">
              <div className="flex gap-2 mb-4">
                {(["cursor", "claude_code", "windsurf"] as ConfigTab[]).map((t) => (
                  <Button
                    key={t}
                    size="sm"
                    variant={configTab === t ? "primary" : "ghost"}
                    onPress={() => loadConfig(t)}
                  >
                    {TYPE_ICON[t]}{" "}
                    {t === "cursor" ? "Cursor" : t === "claude_code" ? "Claude Code" : "Windsurf"}
                  </Button>
                ))}
              </div>

              {configLoading ? (
                <div className="flex justify-center py-10"><Spinner /></div>
              ) : configData ? (
                <div className="space-y-4">
                  <div>
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-sm font-medium" style={{ color: "var(--yunque-fg)" }}>MCP 配置</span>
                      <Button size="sm" variant="ghost" onPress={() => copyToClipboard(configData.mcp_config)}>
                        <Copy size={12} /> 复制
                      </Button>
                    </div>
                    <p className="text-xs mb-2" style={{ color: "var(--yunque-fg-muted)" }}>
                      {configTab === "cursor"
                        ? "添加到 Cursor Settings → MCP 或项目 .cursor/mcp.json"
                        : configTab === "claude_code"
                        ? "添加到 Claude Code 的 MCP 设置中"
                        : "添加到 Windsurf 的 MCP 配置中"}
                    </p>
                    <pre
                      className="p-3 rounded-lg text-xs overflow-auto max-h-48"
                      style={{ background: "var(--yunque-card)", color: "var(--yunque-fg)", border: "1px solid var(--yunque-border)" }}
                    >
                      {configData.mcp_config}
                    </pre>
                  </div>

                  <div>
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-sm font-medium" style={{ color: "var(--yunque-fg)" }}>Worker 指令</span>
                      <Button size="sm" variant="ghost" onPress={() => copyToClipboard(configData.instructions)}>
                        <Copy size={12} /> 复制
                      </Button>
                    </div>
                    <p className="text-xs mb-2" style={{ color: "var(--yunque-fg-muted)" }}>
                      添加为 Cursor Rules / Claude Code 系统提示，让 AI 自动轮询和执行任务
                    </p>
                    <pre
                      className="p-3 rounded-lg text-xs overflow-auto max-h-48"
                      style={{ background: "var(--yunque-card)", color: "var(--yunque-fg)", border: "1px solid var(--yunque-border)" }}
                    >
                      {configData.instructions}
                    </pre>
                  </div>

                  <div className="p-3 rounded-lg text-xs" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
                    <p className="font-medium mb-1" style={{ color: "var(--yunque-fg)" }}>连接步骤：</p>
                    <ol className="list-decimal list-inside space-y-1" style={{ color: "var(--yunque-fg-muted)" }}>
                      <li>复制上面的 MCP 配置到目标工具的 MCP 设置中</li>
                      <li>复制 Worker 指令到工具的系统提示 / Rules 文件中</li>
                      <li>在目标工具中发送消息「开始工作」以触发注册</li>
                      <li>Worker 将自动出现在上方的列表中</li>
                    </ol>
                  </div>
                </div>
              ) : null}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
