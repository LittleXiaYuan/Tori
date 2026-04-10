"use client";

import { useState } from "react";
import { Card, Button, Spinner, Chip, Tooltip, TextField, Input, Label, TextArea } from "@heroui/react";
import { api, type BotInfo } from "@/lib/api";
import { Blocks, Plus, Trash2, Edit3, Save, X, Bot } from "lucide-react";
import PageHeader from "@/components/page-header";
import { useApiData } from "@/lib/use-api-data";

export default function BotsPage() {
  const { data: bots, loading, refresh } = useApiData(
    async () => { const res = await api.getBots(); return res.bots || []; },
    [] as BotInfo[],
  );
  const [showCreate, setShowCreate] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState({ name: "", description: "", system_prompt: "" });

  const createBot = async () => {
    if (!form.name) return;
    await api.createBot(form.name, form.description);
    setForm({ name: "", description: "", system_prompt: "" });
    setShowCreate(false);
    refresh();
  };

  const deleteBot = async (id: string) => {
    await api.deleteBot(id);
    refresh();
  };

  const updateBot = async (id: string) => {
    await api.updateBot(id, form);
    setEditingId(null);
    setForm({ name: "", description: "", system_prompt: "" });
    refresh();
  };

  const startEdit = (bot: BotInfo) => {
    setEditingId(bot.id);
    setForm({ name: bot.name || "", description: bot.description || "", system_prompt: bot.system_prompt || "" });
  };

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader
        icon={<Blocks size={20} />}
        title={`Bot 管理`}
        actions={
          <Button size="sm" onPress={() => setShowCreate(!showCreate)} className="btn-accent">
            <Plus size={14} /> {"创建 Bot"}
          </Button>
        }
      />

      {/* Create form */}
      {showCreate && (
        <Card className="section-card p-5 space-y-3 animate-scale-in">
          <TextField value={form.name} onChange={(v) => setForm({ ...form, name: v })}>
            <Label>{"名称"}</Label>
            <Input placeholder="Bot 名称" />
          </TextField>
          <TextField value={form.description} onChange={(v) => setForm({ ...form, description: v })}>
            <Label>{"描述"}</Label>
            <Input placeholder={"简短描述"} />
          </TextField>
          <TextField value={form.system_prompt} onChange={(v) => setForm({ ...form, system_prompt: v })}>
            <Label>{"系统提示词"}</Label>
            <TextArea rows={4} placeholder={"定义 Bot 的行为..."} />
          </TextField>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" size="sm" onPress={() => setShowCreate(false)}>{"取消"}</Button>
            <Button size="sm" isPending={false} onPress={createBot}
              className="btn-accent">{"创建"}</Button>
          </div>
        </Card>
      )}

      {/* Bot list */}
      {bots.length === 0 ? (
        <Card className="section-card p-12 text-center">
          <Bot size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
          <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{"暂无 Bot，点击上方按钮创建"}</div>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4 stagger-children">
          {bots.map((bot) => (
            <Card key={bot.id} className="section-card p-5 hover-lift h-full">
              {editingId === bot.id ? (
                <div className="space-y-3">
                  <TextField value={form.name} onChange={(v) => setForm({ ...form, name: v })}>
                    <Label>{"名称"}</Label>
                    <Input />
                  </TextField>
                  <TextField value={form.description} onChange={(v) => setForm({ ...form, description: v })}>
                    <Label>{"描述"}</Label>
                    <Input />
                  </TextField>
                  <TextField value={form.system_prompt} onChange={(v) => setForm({ ...form, system_prompt: v })}>
                    <Label>{"系统提示词"}</Label>
                    <TextArea rows={3} />
                  </TextField>
                  <div className="flex justify-end gap-2">
                    <Button variant="ghost" size="sm" onPress={() => setEditingId(null)}><X size={14} /></Button>
                    <Button size="sm" onPress={() => updateBot(bot.id)} className="btn-accent">
                      <Save size={14} /> {"保存"}
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="flex items-start justify-between">
                  <div className="flex items-start gap-3 min-w-0">
                    <div className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
                      style={{ background: "rgba(0,111,238,0.1)" }}>
                      <Bot size={18} style={{ color: "var(--yunque-accent)" }} />
                    </div>
                    <div className="min-w-0">
                      <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{bot.name}</div>
                      <div className="text-xs mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>{bot.description || "无描述"}</div>
                      {bot.system_prompt && (
                        <div className="text-xs mt-2 p-2 rounded-lg line-clamp-2"
                          style={{ background: "rgba(255,255,255,0.03)", color: "var(--yunque-text-secondary)" }}>
                          {bot.system_prompt}
                        </div>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    <Tooltip delay={0}>
                      <Button isIconOnly variant="ghost" size="sm" onPress={() => startEdit(bot)}
                        style={{ color: "var(--yunque-text-muted)" }}>
                        <Edit3 size={14} />
                      </Button>
                      <Tooltip.Content>{"编辑"}</Tooltip.Content>
                    </Tooltip>
                    <Tooltip delay={0}>
                      <Button isIconOnly variant="ghost" size="sm" onPress={() => deleteBot(bot.id)}
                        style={{ color: "var(--yunque-danger)" }}>
                        <Trash2 size={14} />
                      </Button>
                      <Tooltip.Content>{"删除"}</Tooltip.Content>
                    </Tooltip>
                  </div>
                </div>
              )}
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
