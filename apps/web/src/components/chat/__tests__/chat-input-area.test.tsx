import { render, screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";

import { ChatInputArea } from "../chat-input-area";
import type { ChatInputAreaProps } from "../chat-input-area";

vi.mock("@heroui/react", () => {
  const Tooltip = ({ children }: { children: React.ReactNode }) => <>{children}</>;
  Tooltip.Trigger = ({ children }: { children: React.ReactNode }) => <>{children}</>;
  Tooltip.Content = ({ children }: { children: React.ReactNode }) => <span>{children}</span>;
  return {
    Button: ({
      children,
      onPress,
      isIconOnly: _isIconOnly,
      isDisabled,
      isPending: _isPending,
      ...props
    }: {
      children: React.ReactNode;
      onPress?: () => void;
      isIconOnly?: boolean;
      isDisabled?: boolean;
      isPending?: boolean;
      [k: string]: unknown;
    }) => (
      <button type="button" onClick={onPress} disabled={isDisabled} {...props}>{children}</button>
    ),
    Tooltip,
  };
});

vi.mock("lucide-react", () => ({
  Send: () => <svg data-testid="send-icon" />,
  Paperclip: () => <svg data-testid="paperclip-icon" />,
  ImageIcon: () => <svg data-testid="image-icon" />,
  Mic: () => <svg data-testid="mic-icon" />,
  StopCircle: () => <svg data-testid="stop-icon" />,
  Plug: () => <svg data-testid="plug-icon" />,
  Sparkles: () => <svg data-testid="sparkles-icon" />,
  Monitor: () => <svg data-testid="monitor-icon" />,
}));

vi.mock("@/components/slash-command-menu", () => ({
  SlashCommandMenu: () => null,
}));

vi.mock("@/components/connector-popover", () => ({
  ConnectorPopover: () => null,
}));

vi.mock("@/components/model-selector-popup", () => ({
  ModelSelectorPopup: () => null,
}));

function baseProps(): ChatInputAreaProps {
  return {
    input: "",
    loading: false,
    streaming: false,
    hasMessages: false,
    chatMode: "agent",
    isDragging: false,
    pendingFiles: [],
    showSlashMenu: false,
    slashQuery: "",
    activeSlashCommand: null,
    showConnectors: false,
    bridgeConnected: false,
    availableModels: [],
    currentModel: "demo",
    currentModelId: "demo",
    airiAvailable: false,
    thinkingLevel: "auto",
    isRecording: false,
    inputRef: createRef<HTMLTextAreaElement>(),
    fileInputRef: createRef<HTMLInputElement>(),
    inputShellRef: createRef<HTMLDivElement>(),
    onInputChange: vi.fn(),
    onKeyDown: vi.fn(),
    onSlashSelect: vi.fn(),
    onSlashClose: vi.fn(),
    onFileUpload: vi.fn(),
    onDrop: vi.fn(),
    onDragOver: vi.fn(),
    onDragLeave: vi.fn(),
    onSend: vi.fn(),
    onStop: vi.fn(),
    onRemoveFile: vi.fn(),
    onConnectorsToggle: vi.fn(),
    onModelSelect: vi.fn(),
    onModeChange: vi.fn(),
    onThinkingChange: vi.fn(),
    onStartRecording: vi.fn(),
    onStopRecording: vi.fn(),
    onOpenImagePicker: vi.fn(),
  };
}

describe("ChatInputArea attachments", () => {
  it("shows actionable document status without parser implementation badge", () => {
    render(<ChatInputArea {...baseProps()} pendingFiles={[{
      id: "doc",
      name: "申请表.pdf",
      size: 1024,
      type: "binary",
      status: "ready",
      parser: "document",
      note: "附件已保留，等待文档解析后端展开正文",
      workspacePath: "申请表.pdf",
    }]} />);

    expect(screen.getByText("申请表.pdf")).toBeInTheDocument();
    expect(screen.getByText("附件已保留，等待文档解析后端展开正文")).toBeInTheDocument();
    expect(screen.queryByText("document")).not.toBeInTheDocument();
  });

  it("shows model-reading status for parsed Office attachments without exposing parser names", () => {
    render(<ChatInputArea {...baseProps()} pendingFiles={[{
      id: "docx",
      name: "青岛市创业赋能中心OPC入驻申请表.docx",
      size: 2048,
      type: "binary",
      status: "parsed",
      parser: "mineru",
      note: "已添加，发送后由模型读取",
      workspacePath: "uploads/申请表.docx",
      parsedText: "公司名称\t云鸢科技",
    }]} />);

    expect(screen.getByText("青岛市创业赋能中心OPC入驻申请表.docx")).toBeInTheDocument();
    expect(screen.getByText("已添加，发送后由模型读取")).toBeInTheDocument();
    expect(screen.queryByText("mineru")).not.toBeInTheDocument();
  });
});
