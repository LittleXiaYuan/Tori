import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ChatMessageList } from "../chat-message-list";
import { api } from "@/lib/api";
import type { ChatMessageListProps } from "../chat-message-list";

vi.mock("@/lib/api", () => ({
  api: {
    previewFile: vi.fn(),
    notifyChannels: vi.fn().mockResolvedValue({ channels: [] }),
  },
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({ t: (key: string) => key }),
}));

vi.mock("@/components/markdown-renderer", () => ({
  default: ({ content }: { content: string }) => <div>{content}</div>,
}));
vi.mock("@/components/execution-trace", () => ({
  ExecutionTrace: () => null,
}));
vi.mock("@/components/browser-connect-card", () => ({
  BrowserConnectCard: () => null,
}));
vi.mock("@/components/skill-growth-panel", () => ({
  SkillGrowthPanel: () => null,
}));
vi.mock("@/components/chat-extras", () => ({
  EmotionBadge: () => null,
  StickerView: () => null,
  SkillTags: () => null,
  AgentActions: () => null,
}));
vi.mock("@/components/cognitive-status-bar", () => ({
  CognitiveStatusBar: () => null,
}));
vi.mock("@/components/chat/thinking-timer", () => ({
  ThinkingTimer: () => null,
}));
vi.mock("@/lib/safe-url", () => ({
  openExternal: vi.fn(),
}));
vi.mock("lucide-react", () => ({
  Pencil: () => <svg data-testid="pencil" />,
  RotateCcw: () => <svg data-testid="retry" />,
  Copy: () => <svg data-testid="copy" />,
  Undo2: () => <svg data-testid="undo" />,
  Check: () => <svg data-testid="check" />,
  Library: () => <svg data-testid="library" />,
  Paperclip: () => <svg data-testid="paperclip" />,
  Volume2: () => <svg data-testid="volume" />,
  VolumeX: () => <svg data-testid="volume-x" />,
  Heart: () => <svg data-testid="heart" />,
  Monitor: () => <svg data-testid="monitor" />,
  Brain: () => <svg data-testid="brain" />,
  Sparkles: () => <svg data-testid="sparkles" />,
  FileDown: () => <svg data-testid="file-down" />,
  BookOpen: () => <svg data-testid="book" />,
  Share2: () => <svg data-testid="share" />,
  Send: () => <svg data-testid="send" />,
  Settings: () => <svg data-testid="settings" />,
  Eye: () => <svg data-testid="eye" />,
  Wand2: () => <svg data-testid="wand" />,
  Cpu: () => <svg data-testid="cpu" />,
  MoreHorizontal: () => <svg data-testid="more" />,
  ArrowRight: () => <svg data-testid="arrow-right" />,
}));

vi.mock("@heroui/react", () => {
  const Avatar = ({ children }: { children: React.ReactNode }) => <div>{children}</div>;
  Avatar.Fallback = ({ children }: { children: React.ReactNode }) => <span>{children}</span>;
  const Popover = ({ children, onOpenChange }: { children: React.ReactNode; onOpenChange?: (open: boolean) => void }) => <div onClick={() => onOpenChange?.(true)}>{children}</div>;
  Popover.Trigger = ({ children }: { children: React.ReactNode }) => <>{children}</>;
  Popover.Content = ({ children }: { children: React.ReactNode }) => <>{children}</>;
  Popover.Dialog = ({ children }: { children: React.ReactNode }) => <div>{children}</div>;
  const Tooltip = ({ children }: { children: React.ReactNode }) => <>{children}</>;
  Tooltip.Content = ({ children }: { children: React.ReactNode }) => <span>{children}</span>;
  return {
    Avatar,
    Popover,
    Tooltip,
    Chip: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
    Spinner: () => <span>loading</span>,
    Button: ({ children, onPress, isIconOnly: _isIconOnly, isDisabled: _isDisabled, ...props }: { children: React.ReactNode; onPress?: () => void; isIconOnly?: boolean; isDisabled?: boolean; [k: string]: unknown }) => (
      <button type="button" onClick={onPress} {...props}>{children}</button>
    ),
  };
});

function props(overrides: Partial<ChatMessageListProps> = {}): ChatMessageListProps {
  return {
    messages: [],
    streaming: false,
    chatMode: "agent",
    currentModel: "demo",
    copiedIdx: null,
    ttsPlaying: null,
    bridgeState: null,
    resumePromptForBrowser: null,
    onCopy: vi.fn(),
    onPlayTTS: vi.fn(),
    onEdit: vi.fn(),
    onRollback: vi.fn(),
    onRetry: vi.fn(),
    onAction: vi.fn(),
    onSlashSelect: vi.fn(),
    onSend: vi.fn(),
    onBrowserRefresh: vi.fn(),
    onBrowserContinue: vi.fn(),
    onShare: vi.fn(),
    ...overrides,
  };
}

describe("ChatMessageList file preview", () => {
  it("renders Pack Studio patch plans as a guarded user task card", () => {
    const prompt = [
      "请以小羽改包方式优化能力包。",
      "",
      "下面是 Pack Studio 已准备好的 Patch Plan。请只把它当作结构化导航和安全约束。",
      "",
      "```json",
      JSON.stringify({
        kind: "yunque.pack_studio.patch_plan.v1",
        pack: { id: "yunque.pack.wasm-plugin", name: "WASM 能力包", version: "0.1.0" },
        goal: "增加结果界面",
        workspace: {
          id: "yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
          path: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
          original_sha256: "a".repeat(64),
        },
        candidates: [
          {
            key: "manifest:C:\\yunque\\packs\\studio\\pack.json",
            label: "manifest 草稿",
            file_path: "C:\\yunque\\packs\\studio\\pack.json",
            risk_level: "low",
            applyable: true,
            gates: ["预览 diff", "内置审计"],
            content_summary: { length: 1200, hash: "abcd1234" },
          },
        ],
      }, null, 2),
      "```",
    ].join("\n");

    render(<ChatMessageList {...props({
      messages: [{ role: "user", content: prompt, id: "u1" }],
    })} />);

    expect(screen.getByText("Pack Studio 改包任务")).toBeInTheDocument();
    expect(screen.getByText(/WASM 能力包/)).toBeInTheDocument();
    expect(screen.getByText("manifest 草稿")).toBeInTheDocument();
    expect(screen.getByText("风险：low")).toBeInTheDocument();
    expect(screen.getByText("摘要：abcd1234")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /回到 Studio/ })).toHaveAttribute("href", "/packs/studio");
    expect(screen.getByText("请以小羽改包方式优化能力包。")).toBeInTheDocument();
    expect(screen.queryByText(/yunque.pack_studio.patch_plan.v1/)).not.toBeInTheDocument();
  });

  it("renders persisted assistant message content without raw parser or runtime diagnostics", () => {
    const onCopy = vi.fn();
    render(<ChatMessageList {...props({
      onCopy,
      messages: [{
        id: "assistant-raw-history",
        role: "assistant",
        content: [
          "已接收附件：[Parsed document: 申请表.docx]",
          "Workspace path: C:\\Code\\AI\\云雀\\tmp\\申请表.docx",
          "公司名称\t云鸢科技",
          'handoff agent "file_exec" execution failed: context deadline exceeded EOF',
        ].join("\n"),
        timestamp: Date.now(),
      }],
    })} />);

    expect(screen.getByText(/附件内容：申请表\.docx/)).toBeInTheDocument();
    expect(screen.getByText(/附件名称：申请表\.docx/)).toBeInTheDocument();
    expect(screen.getByText(/公司名称\s+云鸢科技/)).toBeInTheDocument();
    expect(screen.getByText(/响应暂时超时，已保留现场/)).toBeInTheDocument();
    expect(screen.queryByText(/\[Parsed document:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Workspace path:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/handoff agent/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/execution failed/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/\bEOF\b/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("copy").closest("button")!);
    expect(onCopy).toHaveBeenCalledTimes(1);
    expect(onCopy.mock.calls[0][1]).toContain("附件内容：申请表.docx");
    expect(onCopy.mock.calls[0][1]).not.toContain("[Parsed document:");
    expect(onCopy.mock.calls[0][1]).not.toContain("context deadline exceeded");
  });

  it("shows friendly parse note and status in generated file preview without parser internals", async () => {
    vi.mocked(api.previewFile).mockResolvedValueOnce({
      name: "申请表.pdf",
      path: "申请表.pdf",
      size: 4,
      ext: "pdf",
      kind: "document",
      content_type: "application/pdf",
      preview: "",
      truncated: false,
      editable: false,
      parse: {
        parser: "document",
        backend: "external",
        status: "needs_document_parser",
        note: "附件已添加，但当前本地解析器还不能直接展开 .pdf 正文；配置文档解析后端后会自动提取正文。",
      },
    });

    render(<ChatMessageList {...props({
      messages: [{
        id: "assistant-1",
        role: "assistant",
        content: "已生成文件",
        timestamp: Date.now(),
        traceEvents: [{
          id: "evt-file",
          trace_id: "trace-file",
          ts: new Date().toISOString(),
          domain: "tool",
          type: "tool_result",
          summary: "file created",
          detail: { files: [{ name: "申请表.pdf", path: "申请表.pdf", size: 4 }] },
          meta: {},
        }],
      }],
    })} />);

    const previewButton = screen.getAllByTestId("eye")[0].closest("button");
    expect(previewButton).not.toBeNull();
    fireEvent.click(previewButton!);

    await waitFor(() => expect(api.previewFile).toHaveBeenCalledWith("申请表.pdf"));
    expect(await screen.findByText("等待读取正文")).toBeInTheDocument();
    expect(screen.getByText(/附件已保留，暂时还不能直接展开正文/)).toBeInTheDocument();
    expect(screen.queryByText("document")).not.toBeInTheDocument();
    expect(screen.queryByText(/本地解析器|文档解析后端|MinerU|parser/i)).not.toBeInTheDocument();
  });
});
