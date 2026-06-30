import { Button, Tooltip } from "@heroui/react";
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
import { useI18n } from "@/lib/i18n";
import type { ConvDispatch, ConvState } from "@/lib/conversation-state";

function convDisplayTitle(c: ConversationInfo, untitled: string): string {
  const name = (c.name || "").trim();
  if (name && name !== c.id && !name.startsWith("new-")) {
    return name;
  }
  const summary = (c.summary || "").trim();
  if (summary) {
    const runes = [...summary];
    return runes.length > 24 ? runes.slice(0, 24).join("") + "…" : summary;
  }
  if (c.id.startsWith("new-")) {
    return untitled;
  }
  return name || c.id;
}

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
  chatMode: "agent" | "fast" | "chat";
  onModeChange: (mode: "agent" | "fast" | "chat") => void;
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
  chatMode,
  onModeChange,
  onNew,
  onSwitch,
  onManage,
  onDelete,
}: ConversationSidebarProps) {
  const { t } = useI18n();

  return (
    <div
      className="conv-sidebar flex flex-col h-full w-full min-w-0"
      style={{
        background: "var(--glass-sidebar, var(--yunque-sidebar))",
        backdropFilter: "blur(var(--yunque-glass-blur)) saturate(var(--yunque-glass-saturate))",
        WebkitBackdropFilter: "blur(var(--yunque-glass-blur)) saturate(var(--yunque-glass-saturate))",
      }}
    >
      {/* Sidebar Header */}
      <div className="p-3 space-y-2.5">
        <Button
          className="w-full justify-start gap-2 rounded-[14px] text-[13px]"
          size="sm"
          variant="ghost"
          onPress={onNew}
          style={{
            background: "var(--yunque-bg-muted)",
            color: "var(--yunque-text)",
            border: "1px solid var(--yunque-border)",
          }}
        >
          <Plus size={14} /> {t("convo.new")}
        </Button>
        <div
          className="flex items-center gap-2 rounded-[14px] px-2.5 py-1.5 text-[11px]"
          style={{ background: "var(--yunque-bg-muted)", color: "var(--yunque-text-muted)" }}
        >
          <Search size={12} />
          <input
            placeholder={t("convo.search")}
            value={conv.searchQuery}
            onChange={(e) => dispatch({ type: "SET_SEARCH", query: e.target.value })}
            className="bg-transparent outline-none text-xs flex-1"
            style={{ color: "var(--yunque-text)" }}
          />
        </div>
      </div>

      {/* 对话/写作模式切换已统一到输入框的「选择模型」弹窗（Agent/Fast/Chat），
          此处不再重复展示，避免与列表过滤混淆。 */}

      {/* Conversation List */}
      <div
        className="flex-1 overflow-y-auto px-2 pb-2"
        style={{ overscrollBehavior: "contain", WebkitOverflowScrolling: "touch" }}
      >
        <div className="conv-sidebar__list-head">
          <span>
            {conv.showArchived ? t("convo.archived") : t("convo.recent")} · {conversations.length}
          </span>
          <Tooltip delay={0}>
            <Button
              isIconOnly
              aria-label={conv.showArchived ? t("convo.active") : t("convo.archived")}
              variant="ghost"
              size="sm"
              className="conv-sidebar__archive-toggle"
              onPress={() => dispatch({ type: "SET_ARCHIVED", show: !conv.showArchived })}
            >
              {conv.showArchived ? <MessageCircle size={13} /> : <Archive size={13} />}
            </Button>
            <Tooltip.Content>{conv.showArchived ? t("convo.active") : t("convo.archived")}</Tooltip.Content>
          </Tooltip>
        </div>
        <div className="chat-thread-list space-y-1">
          {conversations.map((c) => (
            <article
              key={c.id}
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
                  aria-hidden="true"
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
                      e.preventDefault();
                      e.currentTarget.blur();
                    }
                    if (e.key === "Escape") {
                      e.preventDefault();
                      dispatch({ type: "CANCEL_RENAME" });
                    }
                  }}
                  className="text-xs font-medium bg-transparent outline-none w-full px-1 py-0.5 rounded"
                  style={{
                    color: "var(--yunque-text)",
                    background: "var(--yunque-bg-muted)",
                    border: "1px solid var(--yunque-accent)",
                  }}
                  onClick={(e) => e.stopPropagation()}
                />
              ) : (
                <button
                  type="button"
                  className="conv-thread-main w-full text-left"
                  aria-current={conv.activeId === c.id ? "true" : undefined}
                  onClick={() => onSwitch(c.id)}
                >
                  <span className="block text-[12px] font-medium truncate pr-4">{convDisplayTitle(c, t("convo.untitled"))}</span>
                  <span
                    className="mt-0.5 block truncate text-[10px]"
                    style={{ color: "var(--yunque-text-muted)" }}
                  >
                    {(c.summary || "").trim() || t("convo.noSummary")}
                  </span>
                </button>
              )}
              <div className="mt-1.5 flex items-center justify-between">
                <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
                  {new Date(c.updated_at).toLocaleDateString([], {
                    month: "numeric",
                    day: "numeric",
                  })}
                  {c.pinned ? ` · ${t("convo.pinned")}` : ""}
                </span>
                <div className="chat-thread-actions flex items-center gap-0.5">
                  <Tooltip delay={0}>
                    <Button
                      isIconOnly
                      aria-label={t("convo.rename")}
                      variant="ghost"
                      size="sm"
                      onPress={() =>
                        dispatch({ type: "START_RENAME", id: c.id, text: c.name || c.id })
                      }
                    >
                      <Edit3 size={11} style={{ color: "var(--yunque-text-muted)" }} />
                    </Button>
                    <Tooltip.Content>{t("convo.rename")}</Tooltip.Content>
                  </Tooltip>
                  <Tooltip delay={0}>
                    <Button
                      isIconOnly
                      aria-label={c.pinned ? t("convo.unpin") : t("convo.pin")}
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
                    <Tooltip.Content>{c.pinned ? t("convo.unpin") : t("convo.pin")}</Tooltip.Content>
                  </Tooltip>
                  <Tooltip delay={0}>
                    <Button
                      isIconOnly
                      aria-label={conv.showArchived ? t("convo.restore") : t("convo.archive")}
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
                    <Tooltip.Content>{conv.showArchived ? t("convo.restore") : t("convo.archive")}</Tooltip.Content>
                  </Tooltip>
                  <Tooltip delay={0}>
                    <Button
                      isIconOnly
                      aria-label={t("convo.delete")}
                      variant="ghost"
                      size="sm"
                      onPress={() => onDelete(c.id)}
                    >
                      <Trash2 size={11} style={{ color: "#ef4444" }} />
                    </Button>
                    <Tooltip.Content>{t("convo.delete")}</Tooltip.Content>
                  </Tooltip>
                </div>
              </div>
            </article>
          ))}
          {conversations.length === 0 && (
            <div
              className="text-center py-8 text-xs"
              style={{ color: "var(--yunque-text-muted)" }}
            >
              {conv.searchQuery
                ? t("convo.emptyMatch")
                : conv.showArchived
                ? t("convo.emptyArchived")
                : t("convo.emptyActive")}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
