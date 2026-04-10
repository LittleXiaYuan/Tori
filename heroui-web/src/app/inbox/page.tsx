"use client";

import { useEffect, useState } from "react";
import { Card, Button, Spinner, Chip, Tooltip, TextField, Input, Label, TextArea, Avatar } from "@heroui/react";
import { api } from "@/lib/api";
import EmptyState from "@/components/empty-state";
import { formatDate } from "@/lib/constants";
import {
  Inbox, CheckCheck, Mail, MailOpen, Send, X, Star, StarOff,
  Search, Plus, Archive, Trash2, Reply, Forward, MoreVertical,
  FileText, Flag, PenLine,
} from "lucide-react";

interface InboxItem {
  id: string;
  source: string;
  content: string;
  action?: string;
  is_read: boolean;
  created_at: string;
  starred?: boolean;
}

const categories = [
  { key: "inbox", label: "收件箱", icon: <Inbox size={14} /> },
  { key: "sent", label: "已发送", icon: <Send size={14} /> },
  { key: "starred", label: "已标记", icon: <Star size={14} /> },
  { key: "drafts", label: "草稿", icon: <FileText size={14} /> },
  { key: "flagged", label: "标旗", icon: <Flag size={14} /> },
];

function getInitials(source: string) {
  return source.charAt(0).toUpperCase();
}

function getAvatarColor(source: string) {
  const colors = ["#006fee", "#8b5cf6", "#f59e0b", "#22c55e", "#ef4444", "#06b6d4"];
  let hash = 0;
  for (let i = 0; i < source.length; i++) hash = source.charCodeAt(i) + ((hash << 5) - hash);
  return colors[Math.abs(hash) % colors.length];
}

export default function InboxPage() {
  const [items, setItems] = useState<InboxItem[]>([]);
  const [unread, setUnread] = useState(0);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [showCompose, setShowCompose] = useState(false);
  const [compose, setCompose] = useState({ source: "", content: "", action: "" });
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [activeCategory, setActiveCategory] = useState("inbox");
  const [searchQuery, setSearchQuery] = useState("");

  const load = async () => {
    try {
      const res = await api.getInbox();
      const list = (res.items || []).map((item) => ({ ...item, starred: false }));
      setItems(list);
      setUnread(res.count?.unread || 0);
      setTotal(res.count?.total || 0);
      if (list.length > 0 && !selectedId) setSelectedId(list[0].id);
    } catch { /* offline */ }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, []);

  const markAllRead = async () => {
    await api.markAllInboxRead();
    load();
  };

  const pushMessage = async () => {
    if (!compose.content) return;
    await api.pushInbox(compose.source || "manual", compose.content, compose.action || "none");
    setCompose({ source: "", content: "", action: "" });
    setShowCompose(false);
    load();
  };

  const toggleStar = (id: string) => {
    setItems((prev) => prev.map((item) => item.id === id ? { ...item, starred: !item.starred } : item));
  };

  const selectedItem = items.find((i) => i.id === selectedId);

  const filteredItems = items.filter((item) => {
    if (searchQuery && !item.content.toLowerCase().includes(searchQuery.toLowerCase()) && !item.source.toLowerCase().includes(searchQuery.toLowerCase())) return false;
    if (activeCategory === "starred") return item.starred;
    return true;
  });

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="flex h-screen overflow-hidden animate-fade-in-up">
      {/* Left Sidebar - Categories */}
      <div className="w-52 flex flex-col shrink-0 p-3" style={{ borderRight: "1px solid var(--yunque-border)" }}>
        {/* Search */}
        <div
          className="flex items-center gap-2 px-2.5 py-2 rounded-lg text-xs mb-3"
          style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-muted)" }}
        >
          <Search size={13} />
          <input
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="搜索消息..."
            className="bg-transparent outline-none text-xs flex-1"
            style={{ color: "var(--yunque-text)" }}
          />
        </div>

        {/* Categories */}
        <div className="space-y-0.5 flex-1">
          {categories.map((cat) => (
            <button
              key={cat.key}
              onClick={() => setActiveCategory(cat.key)}
              className="inbox-cat"
              data-active={activeCategory === cat.key}
            >
              {activeCategory === cat.key && (
                <div className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-4 rounded-r-full" style={{ background: "var(--yunque-accent)" }} />
              )}
              {cat.icon}
              <span className="flex-1 text-left">{cat.label}</span>
              {cat.key === "inbox" && unread > 0 && (
                <span className="text-[10px] font-bold px-1.5 py-0.5 rounded-md" style={{ background: "rgba(0,111,238,0.15)", color: "var(--yunque-accent)" }}>
                  {unread}
                </span>
              )}
            </button>
          ))}
        </div>

        {/* New Email button */}
        <Button
          className="btn-accent w-full gap-2 rounded-lg text-sm mt-3"
          size="sm"
          onPress={() => setShowCompose(true)}
        >
          <PenLine size={14} /> 新建消息
        </Button>
      </div>

      {/* Middle - Message List */}
      <div className="w-80 flex flex-col shrink-0" style={{ borderRight: "1px solid var(--yunque-border)" }}>
        {/* List Header */}
        <div className="flex items-center justify-between px-4 py-3" style={{ borderBottom: "1px solid var(--yunque-border)" }}>
          <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
            {categories.find(c => c.key === activeCategory)?.label}
            <span className="text-xs font-normal ml-1.5" style={{ color: "var(--yunque-text-muted)" }}>({filteredItems.length})</span>
          </span>
          <div className="flex items-center gap-0.5">
            {unread > 0 && (
              <Tooltip delay={0}>
                <Button isIconOnly variant="ghost" size="sm" onPress={markAllRead} style={{ color: "var(--yunque-text-muted)" }}>
                  <CheckCheck size={14} />
                </Button>
                <Tooltip.Content>全部已读</Tooltip.Content>
              </Tooltip>
            )}
          </div>
        </div>

        {/* Message List */}
        <div className="flex-1 overflow-y-auto custom-scrollbar">
          {filteredItems.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full">
              <EmptyState icon={<Mail size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无消息" description="Agent 在执行任务时会通过收件箱与你沟通；你也可以在对话中直接给 Agent 留言。" />
            </div>
          ) : (
            <div>
              {filteredItems.map((item) => (
                <button
                  key={item.id}
                  onClick={() => setSelectedId(item.id)}
                  className="inbox-msg group"
                  data-active={selectedId === item.id}
                >
                  <div className="flex items-start gap-3">
                    {/* Unread indicator */}
                    <div className="flex flex-col items-center gap-1 pt-1 shrink-0">
                      {!item.is_read && (
                        <div className="w-2 h-2 rounded-full" style={{ background: "var(--yunque-accent)" }} />
                      )}
                      {item.is_read && <div className="w-2 h-2" />}
                    </div>

                    {/* Avatar */}
                    <Avatar size="sm" className="shrink-0" style={{ background: getAvatarColor(item.source || "system") }}>
                      <Avatar.Fallback className="text-[10px] text-white font-bold">
                        {getInitials(item.source || "S")}
                      </Avatar.Fallback>
                    </Avatar>

                    {/* Content */}
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center justify-between gap-2">
                        <span className="text-xs font-semibold truncate" style={{ color: item.is_read ? "var(--yunque-text-secondary)" : "var(--yunque-text)" }}>
                          {item.source || "System"}
                        </span>
                        <span className="text-[10px] shrink-0" style={{ color: "var(--yunque-text-muted)" }}>
                          {formatDate(item.created_at)}
                        </span>
                      </div>
                      <div className="text-[11px] truncate mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>
                        {item.content}
                      </div>
                      {item.action && item.action !== "none" && (
                        <Chip size="sm" className="mt-1" style={{ background: "rgba(34,197,94,0.1)", color: "#22c55e", fontSize: 9 }}>
                          {item.action}
                        </Chip>
                      )}
                    </div>

                    {/* Star */}
                    <button
                      onClick={(e) => { e.stopPropagation(); toggleStar(item.id); }}
                      className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity pt-0.5"
                    >
                      {item.starred ? (
                        <Star size={13} fill="#f59e0b" style={{ color: "#f59e0b" }} />
                      ) : (
                        <StarOff size={13} style={{ color: "var(--yunque-text-muted)" }} />
                      )}
                    </button>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Right - Message Detail */}
      <div className="flex-1 flex flex-col min-w-0">
        {showCompose ? (
          /* Compose Form */
          <div className="flex-1 p-6 animate-fade-in-up">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-lg font-semibold" style={{ color: "var(--yunque-text)" }}>新建消息</h2>
              <Button isIconOnly aria-label="关闭" variant="ghost" size="sm" onPress={() => setShowCompose(false)}>
                <X size={16} />
              </Button>
            </div>
            <div className="space-y-4 max-w-lg">
              <TextField value={compose.source} onChange={(v) => setCompose({ ...compose, source: v })}>
                <Label>来源</Label>
                <Input placeholder="e.g. telegram, email, webhook" />
              </TextField>
              <TextField value={compose.action} onChange={(v) => setCompose({ ...compose, action: v })}>
                <Label>动作</Label>
                <Input placeholder="可选动作标签" />
              </TextField>
              <TextField value={compose.content} onChange={(v) => setCompose({ ...compose, content: v })}>
                <Label>内容</Label>
                <TextArea placeholder="消息内容..." rows={6} />
              </TextField>
              <div className="flex justify-end gap-2 pt-2">
                <Button variant="ghost" size="sm" onPress={() => setShowCompose(false)}>取消</Button>
                <Button size="sm" onPress={pushMessage} className="btn-accent">
                  <Send size={13} /> 发送                </Button>
              </div>
            </div>
          </div>
        ) : selectedItem ? (
          /* Message Detail */
          <div className="flex-1 flex flex-col">
            {/* Detail Header */}
            <div className="flex items-center justify-between px-6 py-3" style={{ borderBottom: "1px solid var(--yunque-border)" }}>
              <div className="flex items-center gap-3">
                <Avatar size="sm" style={{ background: getAvatarColor(selectedItem.source || "system") }}>
                  <Avatar.Fallback className="text-[10px] text-white font-bold">
                    {getInitials(selectedItem.source || "S")}
                  </Avatar.Fallback>
                </Avatar>
                <div>
                  <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{selectedItem.source || "System"}</div>
                  <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                    {selectedItem.created_at ? new Date(selectedItem.created_at).toLocaleString("zh-CN") : ""}
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-1">
                <Tooltip delay={0}>
                  <Button isIconOnly variant="ghost" size="sm"><Reply size={14} /></Button>
                  <Tooltip.Content>回复</Tooltip.Content>
                </Tooltip>
                <Tooltip delay={0}>
                  <Button isIconOnly variant="ghost" size="sm"><Forward size={14} /></Button>
                  <Tooltip.Content>转发</Tooltip.Content>
                </Tooltip>
                <Tooltip delay={0}>
                  <Button isIconOnly variant="ghost" size="sm"><Archive size={14} /></Button>
                  <Tooltip.Content>归档</Tooltip.Content>
                </Tooltip>
                <Tooltip delay={0}>
                  <Button isIconOnly variant="ghost" size="sm" style={{ color: "#ef4444" }}><Trash2 size={14} /></Button>
                  <Tooltip.Content>删除</Tooltip.Content>
                </Tooltip>
              </div>
            </div>

            {/* Message Content */}
            <div className="flex-1 overflow-y-auto px-6 py-5 custom-scrollbar">
              <div className="flex items-center gap-2 mb-4">
                {selectedItem.action && selectedItem.action !== "none" && (
                  <Chip size="sm" style={{ background: "rgba(34,197,94,0.12)", color: "#22c55e" }}>{selectedItem.action}</Chip>
                )}
                {!selectedItem.is_read && (
                  <Chip size="sm" style={{ background: "rgba(0,111,238,0.12)", color: "var(--yunque-accent)" }}>未读</Chip>
                )}
              </div>
              <div className="text-sm leading-relaxed whitespace-pre-wrap" style={{ color: "var(--yunque-text)" }}>
                {selectedItem.content}
              </div>
            </div>
          </div>
        ) : (
          /* Empty state */
          <div className="flex-1 flex flex-col items-center justify-center">
            <Mail size={40} style={{ color: "var(--yunque-text-muted)", opacity: 0.2 }} />
            <span className="text-sm mt-3" style={{ color: "var(--yunque-text-muted)" }}>选择一条消息查看详情</span>
          </div>
        )}
      </div>
    </div>
  );
}
