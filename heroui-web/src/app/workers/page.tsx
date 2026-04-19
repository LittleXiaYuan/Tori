"use client";

import { useState, useEffect, useCallback } from "react";
import { api, type WorkerInfo } from "@/lib/api";
import { Card, Button, Spinner, Chip } from "@heroui/react";
import {
  Cpu, RefreshCw, Trash2, Copy, Plus,
  CheckCircle2, AlertTriangle, Clock,
  Monitor, Terminal, Globe,
  Play, Square, Activity,
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

  const [orchRunning, setOrchRunning] = useState(false);
  const [orchAdapters, setOrchAdapters] = useState<string[]>([]);
  const [orchSessions, setOrchSessions] = useState<Array<{ session_id: string; adapter: string; task_id: string; started_at: string }>>([]);
  const [orchLoading, setOrchLoading] = useState(false);
  const [detectedIDEs, setDetectedIDEs] = useState<Array<{ name: string; binary: string; available: boolean; path?: string }>>([]);

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

  const loadOrchStatus = useCallback(async () => {
    try {
      const [status, sess, detect] = await Promise.all([
        api.orchestratorStatus(),
        api.orchestratorSessions(),
        api.detectIDEs(),
      ]);
      setOrchRunning(status.running);
      setOrchAdapters(status.adapters || []);
      setOrchSessions(sess.sessions || []);
      setDetectedIDEs(detect.ides || []);
    } catch { /* ignore */ }
  }, []);

  const toggleOrch = async () => {
    setOrchLoading(true);
    try {
      await api.orchestratorToggle(orchRunning ? "stop" : "start");
      showToast(orchRunning ? "守护进程已停止" : "守护进程已启动", "success");
      await loadOrchStatus();
    } catch (e) {
      showToast(`操作失败: ${(e as Error).message}`, "error");
    }
    setOrchLoading(false);
  };

  useEffect(() => { loadWorkers(); loadOrchStatus(); }, [loadWorkers, loadOrchStatus]);

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

      {/* Orchestrator Daemon Control */}
      <div style={{ borderTop: "1px solid var(--yunque-border)", marginTop: 24, paddingTop: 24 }}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-base font-semibold flex items-center gap-2" style={{ color: "var(--yunque-text)" }}>
            <Activity size={18} /> 编排守护进程
          </h2>
          <div className="flex items-center gap-3">
            <span className="text-xs px-2 py-1 rounded" style={{
              background: orchRunning ? "rgba(34,197,94,0.1)" : "rgba(239,68,68,0.1)",
              color: orchRunning ? "#22c55e" : "#ef4444",
            }}>
              {orchRunning ? "运行中" : "已停止"}
            </span>
            <Button size="sm" isDisabled={orchLoading} onPress={toggleOrch}
              style={{ background: orchRunning ? "rgba(239,68,68,0.1)" : "rgba(34,197,94,0.1)", color: orchRunning ? "#ef4444" : "#22c55e" }}>
              {orchRunning ? <><Square size={14} /> 停止</> : <><Play size={14} /> 启动</>}
            </Button>
          </div>
        </div>

        <div className="grid grid-cols-3 gap-3 mb-4">
          <Card style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
            <div className="p-3 text-center">
              <div className="text-2xl font-bold" style={{ color: "var(--yunque-accent)" }}>{orchAdapters.length}</div>
              <div className="text-xs" style={{ color: "var(--yunque-muted)" }}>可用适配器</div>
              {orchAdapters.length > 0 && (
                <div className="flex gap-1 justify-center mt-1 flex-wrap">
                  {orchAdapters.map((a) => (
                    <span key={a} className="text-[10px] px-1.5 py-0.5 rounded"
                      style={{ background: "rgba(99,102,241,0.15)", color: "#818cf8" }}>{a}</span>
                  ))}
                </div>
              )}
            </div>
          </Card>
          <Card style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
            <div className="p-3 text-center">
              <div className="text-2xl font-bold" style={{ color: "#f59e0b" }}>{orchSessions.length}</div>
              <div className="text-xs" style={{ color: "var(--yunque-muted)" }}>活跃会话</div>
            </div>
          </Card>
          <Card style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
            <div className="p-3 text-center">
              <div className="text-2xl font-bold" style={{ color: "#22c55e" }}>{workers.length}</div>
              <div className="text-xs" style={{ color: "var(--yunque-muted)" }}>已连接 Worker</div>
            </div>
          </Card>
        </div>

        {detectedIDEs.length > 0 && (
          <div className="mb-4">
            <h3 className="text-sm font-medium mb-2" style={{ color: "var(--yunque-muted)" }}>IDE 环境检测</h3>
            <div className="flex gap-2 flex-wrap">
              {detectedIDEs.map((ide) => (
                <span key={ide.binary} className="inline-flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-lg"
                  style={{
                    background: ide.available ? "rgba(34,197,94,0.1)" : "rgba(100,100,100,0.1)",
                    color: ide.available ? "#22c55e" : "var(--yunque-muted)",
                  }}>
                  {ide.available ? <CheckCircle2 size={12} /> : <AlertTriangle size={12} />}
                  {ide.name}
                  {ide.available && ide.path && <span className="opacity-50 ml-1 hidden sm:inline">({ide.path})</span>}
                </span>
              ))}
            </div>
          </div>
        )}

        {orchSessions.length > 0 && (
          <div className="space-y-2">
            <h3 className="text-sm font-medium" style={{ color: "var(--yunque-muted)" }}>活跃会话</h3>
            {orchSessions.map((s) => (
              <Card key={s.session_id} style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
                <div className="p-3 flex items-center justify-between">
                  <div>
                    <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{s.session_id}</div>
                    <div className="text-xs" style={{ color: "var(--yunque-muted)" }}>
                      适配器: {s.adapter} | 任务: {s.task_id} | {new Date(s.started_at).toLocaleString()}
                    </div>
                  </div>
                  <Chip style={{ background: "rgba(34,197,94,0.1)", color: "#22c55e" }}>运行中</Chip>
                </div>
              </Card>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
