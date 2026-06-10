import { Button } from "@heroui/react";
import {
  Archive,
  ArchiveRestore,
  Code2,
  Edit3,
  MessageCircle,
  PenLine,
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
  const writingMode = chatMode === "chat";

  return (
    <div
      className="conv-sidebar flex flex-col h-full w-full min-w-0"
      style={{
        background: "var(--glass-sidebar, var(--yunque-sidebar))",
        backdropFilter: "blur(var(--yunque-glass-blur)) saturate(var(--yunque-glass-saturate))",
        WebkitBackdropFilter: "blur(var(--yunque-glass-blur)) saturate(var(--yunque-glass-saturate))",
      }}
    >
      {/* Mode toggle — DeepSeek-style pill switch */}
      <div className="conv-sidebar__modes px-2.5 pt-3 pb-1">
        <div className="conv-sidebar__mode-track" role="tablist" aria-label={t("convo.modeAria")}>
          <button
            type="button"
            role="tab"
            aria-selected={!writingMode}
            className="conv-sidebar__mode-btn"
            data-active={!writingMode || undefined}
            onClick={() => onModeChange("agent")}
          >
            <Code2 size={14} /> {t("convo.tab.agent")}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={writingMode}
            className="conv-sidebar__mode-btn"
            data-active={writingMode || undefined}
            onClick={() => onModeChange("chat")}
          >
            <PenLine size={14} /> {t("convo.tab.writing")}
          </button>
        </div>
      </div>

      {/* Sidebar Header */}
      <div className="p-2.5 space-y-2">
        <Button
          className="w-full justify-start gap-2 rounded-[14px] text-[13px] btn-accent"
          size="sm"
          onPress={onNew}
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

      {/* Active / archived filter — accent pill, distinct from mode toggle above */}
      <div className="px-2.5 pb-2">
        <div className="conv-sidebar__filter-track" role="tablist" aria-label={t("convo.filterAria")}>
          <button
            type="button"
            role="tab"
            aria-selected={!conv.showArchived}
            className="conv-sidebar__filter-btn"
            data-active={!conv.showArchived || undefined}
            onClick={() => dispatch({ type: "SET_ARCHIVED", show: false })}
          >
            <MessageCircle size={13} /> {t("convo.active")}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={conv.showArchived}
            className="conv-sidebar__filter-btn"
            data-active={conv.showArchived || undefined}
            onClick={() => dispatch({ type: "SET_ARCHIVED", show: true })}
          >
            <Archive size={13} /> {t("convo.archived")}
          </button>
        </div>
      </div>

      {/* Conversation List */}
      <div
        className="flex-1 overflow-y-auto px-2 pb-2"
        style={{ overscrollBehavior: "contain", WebkitOverflowScrolling: "touch" }}
      >
        <div
          className="px-2 py-2 text-[11px] font-semibold"
          style={{ color: "var(--yunque-text-muted)" }}
        >
          {conv.showArchived ? t("convo.archived") : t("convo.recent")} · {conversations.length}
        </div>
        <div className="chat-thread-list space-y-1">
          {conversations.map((c) => (
            <div
              key={c.id}
              role="button"
              tabIndex={0}
              onClick={() => {
                if (conv.renameId !== c.id) onSwitch(c.id);
              }}
              onKeyDown={(e) => {
                if ((e.key === "Enter" || e.key === " ") && conv.renameId !== c.id) {
                  e.preventDefault(); onSwitch(c.id);
                }
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
                    background: "var(--yunque-bg-muted)",
                    border: "1px solid var(--yunque-accent)",
                  }}
                  onClick={(e) => e.stopPropagation()}
                />
              ) : (
                <div className="text-[12px] font-medium truncate pr-4">{convDisplayTitle(c, t("convo.untitled"))}</div>
              )}
              <div
                className="mt-0.5 truncate text-[10px]"
                style={{ color: "var(--yunque-text-muted)" }}
              >
                {(c.summary || "").trim() || t("convo.noSummary")}
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
                      {t("convo.pinned")}
                    </span>
                  )}
                </div>
                <div className="chat-thread-actions flex items-center gap-0.5">
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
                  <Button
                    isIconOnly
                    aria-label={t("convo.delete")}
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
