import { render, screen } from "@testing-library/react";
import React, { createRef } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ChatInputArea } from "../chat-input-area";
import type { ChatInputAreaProps } from "../chat-input-area";
import { I18nProvider } from "@/lib/i18n";

vi.mock("@heroui/react", () => {
  const Tooltip = ({ children }: { children: React.ReactNode }) => <>{children}</>;
  Tooltip.Trigger = ({ children }: { children: React.ReactNode }) => <>{children}</>;
  Tooltip.Content = ({ children }: { children: React.ReactNode }) => <span>{children}</span>;
  const Dropdown = ({ children }: { children: React.ReactNode }) => <div>{children}</div>;
  Dropdown.Popover = ({ children }: { children: React.ReactNode }) => <div>{children}</div>;
  Dropdown.Menu = ({
    children,
    onAction,
  }: {
    children: React.ReactNode;
    onAction?: (key: string) => void;
  }) => (
    <div>
      {React.Children.map(children, (child) => {
        if (!React.isValidElement<{ id?: string; children?: React.ReactNode }>(child)) return child;
        const id = child.props.id;
        return React.cloneElement(child, {
          onClick: () => id && onAction?.(id),
        } as Partial<React.HTMLAttributes<HTMLElement>>);
      })}
    </div>
  );
  Dropdown.Item = ({
    children,
    onClick,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
  }) => <button type="button" onClick={onClick}>{children}</button>;
  // Popover + toggle primitives are used by the nested AdvancedOptionsPopup.
  // The composer tests don't exercise the popup internals, so pass-through
  // renderers are enough to let the tree mount.
  const Popover = ({ children }: { children: React.ReactNode }) => <div>{children}</div>;
  Popover.Trigger = ({ children }: { children: React.ReactNode }) => <>{children}</>;
  Popover.Content = ({ children }: { children: React.ReactNode }) => <div>{children}</div>;
  Popover.Dialog = ({ children }: { children: React.ReactNode }) => <div>{children}</div>;
  const ToggleButtonGroup = ({ children }: { children: React.ReactNode }) => <div>{children}</div>;
  const ToggleButton = ({ children }: { children: React.ReactNode }) => <button type="button">{children}</button>;
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
    Dropdown,
    Label: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
    Tooltip,
    Popover,
    ToggleButtonGroup,
    ToggleButton,
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
  Plus: () => <svg data-testid="plus-icon" />,
  // Used by the nested AdvancedOptionsPopup rendered inside the input area.
  SlidersHorizontal: () => <svg data-testid="sliders-icon" />,
  Zap: () => <svg data-testid="zap-icon" />,
  MessageCircle: () => <svg data-testid="message-circle-icon" />,
  Cpu: () => <svg data-testid="cpu-icon" />,
  Bird: () => <svg data-testid="bird-icon" />,
  Terminal: () => <svg data-testid="terminal-icon" />,
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
    thinkingLevel: "auto",
    execMode: "xiaoyu",
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
    onExecModeChange: vi.fn(),
    onStartRecording: vi.fn(),
    onStopRecording: vi.fn(),
    onOpenImagePicker: vi.fn(),
  };
}

describe("ChatInputArea attachments", () => {
  beforeEach(() => {
    localStorage.setItem("yunque_locale", "zh");
  });

  it("names the composer textarea and exposes keyboard hints as its description", () => {
    render(
      <I18nProvider>
        <ChatInputArea {...baseProps()} />
      </I18nProvider>,
    );

    const composer = screen.getByRole("textbox", { name: "消息输入区" });
    expect(composer).toBeInTheDocument();
    expect(composer).toHaveAccessibleDescription("Enter 发送·⇧↵ 换行");
  });

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
