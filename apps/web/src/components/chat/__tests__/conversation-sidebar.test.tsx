import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ConversationSidebar } from "../conversation-sidebar";
import type { ConversationInfo } from "@/lib/api";
import type { ConvState } from "@/lib/conversation-state";

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (key: string) => ({
      "convo.new": "新对话",
      "convo.search": "搜索对话…",
      "convo.modeAria": "对话模式",
      "convo.tab.agent": "智能体",
      "convo.tab.writing": "写作",
      "convo.active": "活跃",
      "convo.archived": "归档",
      "convo.recent": "最近对话",
      "convo.rename": "重命名对话",
      "convo.pin": "置顶对话",
      "convo.unpin": "取消置顶",
      "convo.archive": "归档对话",
      "convo.restore": "恢复对话",
      "convo.delete": "删除对话",
      "convo.pinned": "置顶",
      "convo.noSummary": "暂无摘要",
      "convo.untitled": "新对话",
      "convo.emptyMatch": "没有匹配的对话。",
      "convo.emptyArchived": "没有已归档的对话",
      "convo.emptyActive": "还没有对话，开始第一轮吧",
    }[key] || key),
  }),
}));

vi.mock("lucide-react", () => {
  const Icon = () => <svg aria-hidden="true" />;
  return {
    Archive: Icon,
    ArchiveRestore: Icon,
    Code2: Icon,
    Edit3: Icon,
    MessageCircle: Icon,
    PenLine: Icon,
    Pin: Icon,
    PinOff: Icon,
    Plus: Icon,
    Search: Icon,
    Trash2: Icon,
  };
});

const conversations: ConversationInfo[] = [
  {
    id: "conv-1",
    tenant_id: "default",
    name: "产品方案",
    summary: "整理 Cherry 化主路径",
    pinned: false,
    created_at: "2026-06-21T07:00:00Z",
    updated_at: "2026-06-21T08:00:00Z",
  },
  {
    id: "conv-2",
    tenant_id: "default",
    name: "new-2",
    summary: "第二个会话摘要",
    pinned: true,
    created_at: "2026-06-20T07:00:00Z",
    updated_at: "2026-06-20T08:00:00Z",
  },
];

const baseConv: ConvState = {
  list: conversations,
  activeId: "conv-1",
  showArchived: false,
  searchQuery: "",
  renameId: null,
  renameText: "",
};

function renderSidebar(overrides: Partial<ConvState> = {}) {
  return {
    dispatch: vi.fn(),
    onNew: vi.fn(),
    onSwitch: vi.fn(),
    onManage: vi.fn(),
    onDelete: vi.fn(),
    ...render(
      <ConversationSidebar
        conv={{ ...baseConv, ...overrides }}
        dispatch={vi.fn()}
        conversations={conversations}
        chatMode="agent"
        onModeChange={vi.fn()}
        onNew={vi.fn()}
        onSwitch={vi.fn()}
        onManage={vi.fn()}
        onDelete={vi.fn()}
      />,
    ),
  };
}

describe("ConversationSidebar", () => {
  it("uses native buttons for conversation switching instead of div role=button", () => {
    const onSwitch = vi.fn();
    render(
      <ConversationSidebar
        conv={baseConv}
        dispatch={vi.fn()}
        conversations={conversations}
        chatMode="agent"
        onModeChange={vi.fn()}
        onNew={vi.fn()}
        onSwitch={onSwitch}
        onManage={vi.fn()}
        onDelete={vi.fn()}
      />,
    );

    const conversationButton = screen.getByRole("button", { name: /产品方案/ });
    expect(conversationButton).toHaveAttribute("aria-current", "true");

    fireEvent.click(screen.getByRole("button", { name: /第二个会话摘要/ }));
    expect(onSwitch).toHaveBeenCalledWith("conv-2");
  });

  it("keeps destructive row actions separate from switching the conversation", () => {
    const onSwitch = vi.fn();
    const onDelete = vi.fn();
    render(
      <ConversationSidebar
        conv={baseConv}
        dispatch={vi.fn()}
        conversations={conversations}
        chatMode="agent"
        onModeChange={vi.fn()}
        onNew={vi.fn()}
        onSwitch={onSwitch}
        onManage={vi.fn()}
        onDelete={onDelete}
      />,
    );

    const deleteButtons = screen.getAllByRole("button", { name: "删除对话" });
    fireEvent.click(deleteButtons[0]);

    expect(onDelete).toHaveBeenCalledWith("conv-1");
    expect(onSwitch).not.toHaveBeenCalled();
  });
});
