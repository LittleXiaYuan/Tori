"use client";

import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import type { NotifyChannel } from "@/lib/api-types";
import { showToast } from "@/components/toast-provider";
import { Card, Button, Switch, Spinner, TextField, Input, Label, Select, ListBox } from "@heroui/react";
import {
  Bell, Plus, Trash2, Send, MessageSquare, AlertTriangle,
  ChevronDown,
} from "lucide-react";

const channelTypes = [
  { id: "webhook", name: "通用 Webhook", desc: "POST JSON 到任意URL" },
  { id: "dingtalk", name: "钉钉机器人", desc: "通过 Webhook 推送到钉钉群" },
  { id: "feishu", name: "飞书机器人", desc: "通过 Webhook 推送到飞书群" },
  { id: "wechat_work", name: "企业微信机器人", desc: "通过 Webhook 推送到企微群" },
];

export default function NotificationsPage() {
  const [channels, setChannels] = useState<NotifyChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showAdd, setShowAdd] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);

  const [newType, setNewType] = useState("webhook");
  const [newName, setNewName] = useState("");
  const [newURL, setNewURL] = useState("");

  const load = useCallback(async () => {
    try {
      const res = await api.notifyChannels();
      setChannels(res.channels || []);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleAdd = async () => {
    if (!newName.trim() || !newURL.trim()) return;
    setBusy("add");
    setError("");
    try {
      const id = `${newType}_${Date.now()}`;
      await api.notifyAdd({ id, type: newType, name: newName.trim(), url: newURL.trim(), enabled: true });
      setNewName("");
      setNewURL("");
      setShowAdd(false);
      await load();
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(null);
    }
  };

  const handleRemove = async (id: string) => {
    setBusy(id);
    try {
      await api.notifyRemove(id);
      await load();
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(null);
    }
  };

  const handleToggle = async (id: string, enabled: boolean) => {
    try {
      await api.notifyToggle(id, enabled);
      setChannels(prev => prev.map(ch => ch.id === id ? { ...ch, enabled } : ch));
    } catch (e: any) {
      setError(e.message);
    }
  };

  const handleTest = async (id: string) => {
    setBusy(`test_${id}`);
    try {
      await api.notifyTest(id);
      showToast("测试通知已发送", "success");
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(null);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Spinner size="lg" />
      </div>
    );
  }

  const typeInfo = channelTypes.find(t => t.id === newType);

  return (
    <div className="max-w-xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-semibold mb-1">通知渠道</h2>
          <p className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>
            任务完成、研究报告生成等事件自动推送到你的消息平台
          </p>
        </div>
        <Button
          size="sm"
          onPress={() => setShowAdd(!showAdd)}
        >
          <Plus size={14} /> 添加
        </Button>
      </div>

      {error && (
        <div className="flex items-center gap-2 mb-4 p-3 rounded-lg text-sm"
          style={{ background: "rgba(239,68,68,0.1)", color: "#ef4444" }}>
          <AlertTriangle size={14} />
          <span className="flex-1">{error}</span>
          <button onClick={() => setError("")} className="opacity-60 hover:opacity-100">×</button>
        </div>
      )}

      {/* Add form */}
      {showAdd && (
        <Card className="mb-4 p-4" style={{ background: "var(--yunque-bg-surface)" }}>
          <div className="space-y-3">
            <div>
              <label className="text-xs font-medium mb-1 block" style={{ color: "var(--yunque-text-secondary)" }}>类型</label>
              <div className="flex gap-2 flex-wrap">
                {channelTypes.map(t => (
                  <button
                    key={t.id}
                    className="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
                    style={{
                      background: newType === t.id ? "var(--yunque-accent)" : "var(--yunque-bg-muted)",
                      color: newType === t.id ? "#fff" : "var(--yunque-text-secondary)",
                    }}
                    onClick={() => setNewType(t.id)}
                  >
                    {t.name}
                  </button>
                ))}
              </div>
              {typeInfo && (
                <p className="text-[10px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>{typeInfo.desc}</p>
              )}
            </div>
            <TextField value={newName} onChange={setNewName}>
              <Label className="text-xs">名称</Label>
              <Input placeholder="例如：产品群通知" />
            </TextField>
            <TextField value={newURL} onChange={setNewURL}>
              <Label className="text-xs">Webhook URL</Label>
              <Input placeholder="https://..." />
            </TextField>
            <div className="flex justify-end gap-2">
              <Button size="sm" variant="ghost" onPress={() => setShowAdd(false)}>取消</Button>
              <Button
                size="sm"
                isPending={busy === "add"}
                isDisabled={!newName.trim() || !newURL.trim()}
                onPress={handleAdd}
              >
                确认添加
              </Button>
            </div>
          </div>
        </Card>
      )}

      {/* Channel list */}
      {channels.length === 0 ? (
        <Card className="p-8 text-center" style={{ background: "var(--yunque-bg-surface)" }}>
          <Bell size={32} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)", opacity: 0.4 }} />
          <p className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>
            暂无通知渠道
          </p>
          <p className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
            添加钉钉、飞书或企微机器人的 Webhook URL 即可开始接收通知
          </p>
        </Card>
      ) : (
        <Card className="overflow-hidden" style={{ background: "var(--yunque-bg-surface)" }}>
          {channels.map((ch, i) => {
            const typeLabel = channelTypes.find(t => t.id === ch.type)?.name || ch.type;
            return (
              <div key={ch.id}>
                {i > 0 && <div style={{ borderTop: "1px solid var(--yunque-border)" }} />}
                <div className="flex items-center gap-3 px-4 py-3">
                  <div className="w-8 h-8 rounded-lg flex items-center justify-center"
                    style={{ background: ch.enabled ? "rgba(34,197,94,0.1)" : "var(--yunque-bg-muted)" }}>
                    <MessageSquare size={16} style={{ color: ch.enabled ? "#22c55e" : "var(--yunque-text-muted)" }} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-sm">{ch.name}</div>
                    <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                      {typeLabel} · {ch.url}
                    </div>
                  </div>
                  <Button
                    isIconOnly
                    variant="ghost"
                    size="sm"
                    isPending={busy === `test_${ch.id}`}
                    onPress={() => handleTest(ch.id)}
                  >
                    <Send size={13} />
                  </Button>
                  <Switch
                    isSelected={ch.enabled}
                    onChange={(v) => handleToggle(ch.id, v)}
                    size="sm"
                  />
                  <Button
                    isIconOnly
                    variant="ghost"
                    size="sm"
                    isPending={busy === ch.id}
                    onPress={() => handleRemove(ch.id)}
                  >
                    <Trash2 size={13} style={{ color: "#ef4444" }} />
                  </Button>
                </div>
              </div>
            );
          })}
        </Card>
      )}

      <div className="mt-6 text-center">
        <p className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          支持事件: 任务完成 · 研究报告生成 · 审批请求 · 错误告警
        </p>
      </div>
    </div>
  );
}
