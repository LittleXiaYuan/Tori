import { Button } from "@heroui/react";
import {
  Archive,
  ArchiveRestore,
  Edit3,
  MessageCircle,
  Pin,
  PinOff,
  Plus,
  Search,
  Trash2,
} from "lucide-react";

import type { ConversationInfo } from "@/lib/api";
import type { ConvDispatch, ConvState } from "@/lib/conversation-state";

/**
 * ManageConversationOpts is the shape the chat page's own
 * `manageConversation` callback takes. We duplicate it here (rather than
 * reach back into chat/page.tsx) so the sidebar can be extracted without
 * pulling page-scope types with it; the chat page just satisfies this
 * structural contract when wiring the prop.
 */
export interface ManageConversationOpts {
  name?: string;
  pinned?: boolean;
  archive?: boolean;
}

export interface ConversationSidebarProps {
  conv: ConvState;
  dispatch: ConvDispatch;
  conversations: ConversationInfo[];
  onNew: () => void;
  onSwitch: (id: string) => void;
  onManage: (
    id: string,
    opts: ManageConversationOpts,
  ) => void | Promise<void>;
  onDelete: (id: string) => void | Promise<void>;
}

/**
 * ConversationSidebar renders the 228–244px left rail of chat/page.tsx:
 * header (count + 新对话 + search), archive toggle, and the scrollable
 * conversation list with inline rename / pin / archive / delete actions.
 *
 * This component is a pure view over `conv` + pre-filtered conversations.
 * All stateful effects (loading, activation, rename persistence) stay in
 * chat/page.tsx so the sidebar can be tested and re-skinned in isolation.
 *
 * Extracted as part of the PR3 step of TECH-DEBT-2026-04-18.md §7
 * (chat/page.tsx 拆分方案). Previously lived inline in chat/page.tsx at
 * lines ~967–1103.
 */
export function ConversationSidebar({
  conv,
  dispatch,
  conversations,
  onNew,
  onSwitch,
  onManage,
  onDelete,
}: ConversationSidebarProps) {
  return (
    <div
      className="flex flex-col h-full animate-slide-in-left w-[228px] xl:w-[244px] shrink-0"
      style={{
        background: "var(--yunque-sidebar)",
        borderRight: "1px solid var(--yunque-border)",
        transition: "width 0.2s ease",
      }}
    >
      {/* Sidebar Header */}
      <div className="p-2.5 space-y-2">
        <div className="flex items-center justify-between px-1 pt-0.5">
          <span className="text-xs font-semibold" style={{ color: "var(--yunque-text-muted)" }}>
            {conv.showArchived ? "归档" : "对话"} · {conversations.length}
          </span>
        </div>
        <Button
          className="w-full justify-start gap-2 rounded-[14px] text-[13px] btn-accent"
          size="sm"
          onPress={onNew}
        >
          <Plus size={14} /> 新对话
        </Button>
        <div
          className="flex items-center gap-2 rounded-[14px] px-2.5 py-1.5 text-[11px]"
          style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-muted)" }}
        >
          <Search size={12} />
          <input
            placeholder="搜索对话…"
            value={conv.searchQuery}
            onChange={(e) => dispatch({ type: "SET_SEARCH", query: e.target.value })}
            className="bg-transparent outline-none text-xs flex-1"
            style={{ color: "var(--yunque-text)" }}
          />
        </div>
      </div>

      {/* Archive toggle */}
      <div className="px-2.5 pb-2 flex gap-1">
        <button
          onClick={() => dispatch({ type: "SET_ARCHIVED", show: false })}
          className="flex items-center gap-1.5 rounded-[12px] px-2 py-1.5 text-[10px] transition-colors flex-1 justify-center"
          style={{
            color: !conv.showArchived ? "var(--yunque-accent)" : "var(--yunque-text-muted)",
            background: !conv.showArchived ? "rgba(0,111,238,0.1)" : "rgba(255,255,255,0.03)",
          }}
        >
          <MessageCircle size={13} /> 活跃
        </button>
        <button
          onClick={() => dispatch({ type: "SET_ARCHIVED", show: true })}
          className="flex items-center gap-1.5 rounded-[12px] px-2 py-1.5 text-[10px] transition-colors flex-1 justify-center"
          style={{
            color: conv.showArchived ? "var(--yunque-accent)" : "var(--yunque-text-muted)",
            background: conv.showArchived ? "rgba(0,111,238,0.1)" : "rgba(255,255,255,0.03)",
          }}
        >
          <Archive size={13} /> 归档
        </button>
      </div>

      {/* Conversation List */}
      <div
        className="flex-1 overflow-y-auto px-2 pb-2"
        style={{ overscrollBehavior: "contain", WebkitOverflowScrolling: "touch" }}
      >
        <div
          className="px-2 py-2 text-[10px] font-semibold uppercase tracking-[0.22em]"
          style={{ color: "var(--yunque-text-muted)" }}
        >
          {conv.showArchived ? "归档对话" : "最近对话"} ({conversations.length})
        </div>
        <div className="chat-thread-list space-y-1">
          {conversations.map((c) => (
            <div
              key={c.id}
              onClick={() => {
                if (conv.renameId !== c.id) onSwitch(c.id);
              }}
              className="conv-item chat-thread-item w-full text-left px-3 py-2.5 rounded-[16px] group relative"
              data-active={conv.activeId === c.id || undefined}
              style={{
                color:
                  conv.activeId === c.id
                    ? "var(--yunque-accent)"
                    : "var(--yunque-text-secondary)",
              }}
            >
              <div className="chat-thread-indicator" aria-hidden="true" />
              {c.pinned && (
                <Pin
                  size={10}
                  className="absolute right-2 top-2"
                  style={{ color: "var(--yunque-accent)", opacity: 0.6 }}
                />
              )}
              {conv.renameId === c.id ? (
                <input
                  autoFocus
                  value={conv.renameText}
                  onChange={(e) => dispatch({ type: "SET_RENAME_TEXT", text: e.target.value })}
                  onBlur={() => {
                    if (conv.renameText.trim())
                      onManage(c.id, { name: conv.renameText.trim() });
                    dispatch({ type: "CANCEL_RENAME" });
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      if (conv.renameText.trim())
                        onManage(c.id, { name: conv.renameText.trim() });
                      dispatch({ type: "CANCEL_RENAME" });
                    }
                    if (e.key === "Escape") dispatch({ type: "CANCEL_RENAME" });
                  }}
                  className="text-xs font-medium bg-transparent outline-none w-full px-1 py-0.5 rounded"
                  style={{
                    color: "var(--yunque-text)",
                    background: "rgba(255,255,255,0.08)",
                    border: "1px solid var(--yunque-accent)",
                  }}
                  onClick={(e) => e.stopPropagation()}
                />
              ) : (
                <div className="text-[12px] font-medium truncate pr-4">{c.name || c.id}</div>
              )}
              <div
                className="mt-0.5 truncate text-[10px]"
                style={{ color: "var(--yunque-text-muted)" }}
              >
                {c.summary || "暂无摘要"}
              </div>
              <div className="mt-1.5 flex items-center justify-between">
                <div className="flex items-center gap-1.5">
                  <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
                    {new Date(c.updated_at).toLocaleDateString([], {
                      month: "numeric",
                      day: "numeric",
                    })}
                  </span>
                  {c.pinned && (
                    <span
                      className="rounded-full px-2 py-0.5 text-[10px]"
                      style={{
                        background: "rgba(59,130,246,0.1)",
                        color: "var(--yunque-accent)",
                      }}
                    >
                      置顶
                    </span>
                  )}
                </div>
                <div className="chat-thread-actions flex items-center gap-0.5">
                  <Button
                    isIconOnly
                    aria-label="重命名对话"
                    variant="ghost"
                    size="sm"
                    onPress={() =>
                      dispatch({ type: "START_RENAME", id: c.id, text: c.name || c.id })
                    }
                  >
                    <Edit3 size={11} style={{ color: "var(--yunque-text-muted)" }} />
                  </Button>
                  <Button
                    isIconOnly
                    aria-label="置顶对话"
                    variant="ghost"
                    size="sm"
                    onPress={() => onManage(c.id, { pinned: !c.pinned })}
                  >
                    {c.pinned ? (
                      <PinOff size={11} style={{ color: "var(--yunque-accent)" }} />
                    ) : (
                      <Pin size={11} style={{ color: "var(--yunque-text-muted)" }} />
                    )}
                  </Button>
                  <Button
                    isIconOnly
                    aria-label={conv.showArchived ? "恢复对话" : "归档对话"}
                    variant="ghost"
                    size="sm"
                    onPress={() => onManage(c.id, { archive: !conv.showArchived })}
                  >
                    {conv.showArchived ? (
                      <ArchiveRestore size={11} style={{ color: "var(--yunque-text-muted)" }} />
                    ) : (
                      <Archive size={11} style={{ color: "var(--yunque-text-muted)" }} />
                    )}
                  </Button>
                  <Button
                    isIconOnly
                    aria-label="Delete conversation"
                    variant="ghost"
                    size="sm"
                    onPress={() => onDelete(c.id)}
                  >
                    <Trash2 size={11} style={{ color: "#ef4444" }} />
                  </Button>
                </div>
              </div>
            </div>
          ))}
          {conversations.length === 0 && (
            <div
              className="text-center py-8 text-xs"
              style={{ color: "var(--yunque-text-muted)" }}
            >
              {conv.searchQuery
                ? "没有匹配的对话。"
                : conv.showArchived
                ? "暂时没有归档对话。"
                : "还没有对话，开始新建一个吧。"}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
