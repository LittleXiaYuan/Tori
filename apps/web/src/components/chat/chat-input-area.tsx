"use client";

import { type RefObject } from "react";
import { Button, Tooltip } from "@heroui/react";
import {
  Send,
  Paperclip,
  ImageIcon,
  Mic,
  StopCircle,
  Plug,
  Sparkles,
  Monitor,
} from "lucide-react";
import { SlashCommandMenu } from "@/components/slash-command-menu";
import { ConnectorPopover } from "@/components/connector-popover";
import { ModelSelectorPopup, type ModelOption } from "@/components/model-selector-popup";
import { type PendingFile } from "@/lib/use-chat-media";

export interface ChatInputAreaProps {
  input: string;
  loading: boolean;
  streaming: boolean;
  hasMessages: boolean;
  chatMode: "agent" | "fast" | "chat";
  isDragging: boolean;
  pendingFiles: PendingFile[];
  showSlashMenu: boolean;
  slashQuery: string;
  activeSlashCommand: string | null;
  showConnectors: boolean;
  bridgeConnected: boolean;
  availableModels: ModelOption[];
  currentModel: string;
  currentModelId: string;
  airiAvailable: boolean;
  thinkingLevel: "none" | "auto" | "deep";
  isRecording: boolean;
  inputRef: RefObject<HTMLTextAreaElement | null>;
  fileInputRef: RefObject<HTMLInputElement | null>;
  inputShellRef: RefObject<HTMLDivElement | null>;
  onInputChange: (e: React.ChangeEvent<HTMLTextAreaElement>) => void;
  onKeyDown: (e: React.KeyboardEvent) => void;
  onSlashSelect: (cmd: string) => void;
  onSlashClose: () => void;
  onFileUpload: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onDrop: (e: React.DragEvent) => void;
  onDragOver: (e: React.DragEvent) => void;
  onDragLeave: (e: React.DragEvent) => void;
  onSend: () => void;
  onStop: () => void;
  onRemoveFile: (id: string, preview?: string) => void;
  onConnectorsToggle: (show: boolean) => void;
  onModelSelect: (model: ModelOption) => void;
  onModeChange: (mode: "agent" | "fast" | "chat") => void;
  onThinkingChange: (level: "none" | "auto" | "deep") => void;
  onStartRecording: () => void;
  onStopRecording: () => void;
  onOpenImagePicker: () => void;
}

function attachmentStatusLabel(file: PendingFile): string {
  if (file.note) return file.note;
  if (file.status === "uploading") return "正在添加附件…";
  if (file.status === "parsed") return "已添加，发送后由模型读取";
  if (file.status === "error") return "附件添加失败，请重新添加";
  return file.type === "image" || file.type === "video" ? "已添加" : "附件已添加";
}

export function ChatInputArea(props: ChatInputAreaProps) {
  const {
    input,
    loading,
    hasMessages,
    chatMode,
    isDragging,
    pendingFiles,
    showSlashMenu,
    slashQuery,
    activeSlashCommand,
    showConnectors,
    bridgeConnected,
    availableModels,
    currentModel,
    currentModelId,
    airiAvailable,
    thinkingLevel,
    isRecording,
    inputRef,
    fileInputRef,
    inputShellRef,
    onInputChange,
    onKeyDown,
    onSlashSelect,
    onSlashClose,
    onFileUpload,
    onDrop,
    onDragOver,
    onDragLeave,
    onSend,
    onStop,
    onRemoveFile,
    onConnectorsToggle,
    onModelSelect,
    onModeChange,
    onThinkingChange,
    onStartRecording,
    onStopRecording,
    onOpenImagePicker,
  } = props;

  return (
    <div
      className="px-5 py-2 shrink-0 xl:px-6"
      style={{ borderTop: hasMessages ? "1px solid var(--yunque-border)" : "none" }}
      onDrop={onDrop}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      aria-label="消息输入区"
    >
      <div className="mx-auto" style={{ maxWidth: "min(860px, 92vw)" }}>
        <div
          ref={inputShellRef}
          className="chat-input-wrap chat-composer rounded-[24px] overflow-visible transition-all"
          data-busy={loading ? "true" : "false"}
          style={{
            background:
              "linear-gradient(180deg, rgba(255,255,255,0.025), rgba(255,255,255,0.008)), var(--glass-card, var(--yunque-card))",
            border: isDragging
              ? "1px dashed var(--yunque-accent)"
              : "1px solid var(--glass-edge, var(--yunque-border))",
            boxShadow: isDragging
              ? "0 0 0 1px rgba(59,130,246,0.20), 0 12px 28px rgba(15,23,42,0.20)"
              : "0 8px 22px rgba(15,23,42,0.12), inset 0 1px 0 rgba(255,255,255,0.035)",
            backdropFilter:
              "blur(var(--yunque-glass-blur)) saturate(var(--yunque-glass-saturate))",
            WebkitBackdropFilter:
              "blur(var(--yunque-glass-blur)) saturate(var(--yunque-glass-saturate))",
          }}
        >
          {hasMessages && chatMode !== "chat" && (bridgeConnected || showSlashMenu || activeSlashCommand) && (
            <div
              className="flex items-center justify-between gap-3 rounded-t-[24px] px-4 py-1"
              style={{
                background: "var(--yunque-bg-muted)",
                backdropFilter: "blur(16px) saturate(1.6)",
                WebkitBackdropFilter: "blur(16px) saturate(1.6)",
                borderBottom: "1px solid var(--yunque-border)",
              }}
            >
              <div
                className="text-[11px] truncate"
                style={{ color: "var(--yunque-text-muted)" }}
              >
                {bridgeConnected ? (
                  <span className="flex items-center gap-1.5">
                    <Monitor size={11} />
                    <span className="w-1.5 h-1.5 rounded-full bg-blue-400 inline-block" />
                    浏览器已连接
                  </span>
                ) : (
                  <span className="flex items-center gap-1.5">
                    <Sparkles size={11} />
                    {activeSlashCommand ? `/${activeSlashCommand}` : "命令菜单"}
                  </span>
                )}
              </div>
              <div className="flex items-center gap-1.5">
                <Button
                  size="sm"
                  variant="ghost"
                  className="chat-tool-btn h-7 rounded-full px-2 text-[10px]"
                  data-active={
                    showSlashMenu || activeSlashCommand ? "true" : undefined
                  }
                  onPress={() => onSlashSelect("/")}
                >
                  <Sparkles size={11} /> 命令
                </Button>
              </div>
            </div>
          )}

          {pendingFiles.length > 0 && (
            <div className="flex gap-2 px-5 pt-4 flex-wrap">
              {pendingFiles.map((f) => {
                const statusColor =
                  f.status === "parsed"
                    ? "#4ade80"
                    : f.status === "uploading"
                      ? "#60a5fa"
                      : f.status === "error"
                        ? "#f87171"
                        : "#94a3b8";
                return (
                  <div
                    key={f.id}
                    className="relative group/file flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-xs"
                    style={{
                      background: "var(--yunque-bg-muted)",
                      border: "1px solid var(--yunque-border)",
                    }}
                  >
                    {f.type === "image" && f.preview ? (
                      <img
                        src={f.preview}
                        alt={f.name}
                        className="w-8 h-8 rounded object-cover"
                      />
                    ) : f.type === "video" && f.preview ? (
                      <video
                        src={f.preview}
                        className="w-8 h-8 rounded object-cover"
                        muted
                      />
                    ) : (
                      <Paperclip
                        size={12}
                        style={{ color: "var(--yunque-text-muted)" }}
                      />
                    )}
                    <div className="min-w-0">
                      <div
                        className="truncate max-w-[140px]"
                        style={{ color: "var(--yunque-text-secondary)" }}
                      >
                        {f.name}
                      </div>
                      {(f.note || f.parser || f.status) && (
                        <div
                          className="flex items-center gap-1 text-[10px]"
                          style={{ color: statusColor }}
                        >
                          <span
                            className="inline-block h-1.5 w-1.5 rounded-full"
                            style={{ background: statusColor }}
                          />
                          <span className="truncate max-w-[160px]">
                            {attachmentStatusLabel(f)}
                          </span>
                        </div>
                      )}
                    </div>
                    <button
                      onClick={() => onRemoveFile(f.id, f.preview)}
                      className="ml-1 w-4 h-4 rounded-full flex items-center justify-center text-[10px] opacity-0 group-hover/file:opacity-100 transition-opacity shrink-0"
                      style={{
                        background: "rgba(239,68,68,0.9)",
                        color: "#fff",
                      }}
                    >
                      ×
                    </button>
                  </div>
                );
              })}
            </div>
          )}

          <div className="relative px-4 pt-2.5">
            <SlashCommandMenu
              query={slashQuery}
              visible={showSlashMenu}
              onSelect={onSlashSelect}
              onClose={onSlashClose}
              anchorRef={inputShellRef}
            />
            {(showSlashMenu || activeSlashCommand) && (
              <div
                className="slash-trigger-pill pointer-events-none absolute left-4 top-0 flex items-center gap-1.5 rounded-full px-2 py-1 text-[10px]"
                style={{
                  background: "var(--yunque-accent-soft)",
                  color: "var(--yunque-accent)",
                  boxShadow: "0 8px 20px rgba(59,130,246,0.08)",
                }}
              >
                <span>
                  {showSlashMenu ? "Command menu" : "Slash command"}
                </span>
                {activeSlashCommand && (
                  <span
                    className="rounded-full px-2 py-0.5"
                    style={{
                      background: "rgba(255,255,255,0.12)",
                      color: "var(--yunque-text)",
                    }}
                  >
                    /{activeSlashCommand}
                  </span>
                )}
              </div>
            )}
          </div>

          <textarea
            ref={inputRef}
            value={input}
            onChange={onInputChange}
            onKeyDown={onKeyDown}
            placeholder="描述要交付的工作，或输入 / 打开命令…"
            rows={1}
            className="chat-composer-textarea w-full resize-none bg-transparent px-4 pt-2.5 pb-1.5 text-[14px] outline-none"
            style={{
              color: "var(--yunque-text)",
              minHeight: 36,
              maxHeight: 160,
              lineHeight: 1.65,
            }}
            disabled={loading}
          />

          <div className="flex flex-wrap items-center justify-between gap-3 px-4 pb-3.5 pt-2">
            <div className="flex items-center gap-1.5">
              <input
                type="file"
                ref={fileInputRef}
                className="hidden"
                onChange={onFileUpload}
              />
              <Tooltip delay={0}>
                <Button
                  isIconOnly
                  variant="ghost"
                  size="sm"
                  className="chat-tool-btn compact-tool"
                  onPress={() => fileInputRef.current?.click()}
                >
                  <Paperclip size={14} />
                </Button>
                <Tooltip.Content>添加文件</Tooltip.Content>
              </Tooltip>
              <Tooltip delay={0}>
                <Button
                  isIconOnly
                  variant="ghost"
                  size="sm"
                  className="chat-tool-btn compact-tool"
                  onPress={onOpenImagePicker}
                >
                  <ImageIcon size={14} />
                </Button>
                <Tooltip.Content>添加图片</Tooltip.Content>
              </Tooltip>
              <Tooltip delay={0}>
                <Button
                  isIconOnly
                  variant="ghost"
                  size="sm"
                  className="chat-tool-btn compact-tool"
                  onPress={isRecording ? onStopRecording : onStartRecording}
                  style={isRecording ? { color: "#ef4444" } : {}}
                >
                  {isRecording ? (
                    <StopCircle size={14} className="animate-pulse" />
                  ) : (
                    <Mic size={14} />
                  )}
                </Button>
                <Tooltip.Content>
                  {isRecording ? "停止录音" : "语音输入"}
                </Tooltip.Content>
              </Tooltip>
              <div className="relative">
                <Tooltip delay={0}>
                  <Button
                    isIconOnly
                    variant="ghost"
                    size="sm"
                    className="chat-tool-btn compact-tool"
                    data-active={showConnectors ? "true" : undefined}
                    onPress={() => onConnectorsToggle(!showConnectors)}
                  >
                    <Plug size={14} />
                  </Button>
                  <Tooltip.Content>连接器</Tooltip.Content>
                </Tooltip>
                <ConnectorPopover
                  open={showConnectors}
                  onClose={() => onConnectorsToggle(false)}
                />
              </div>
            </div>
            <ModelSelectorPopup
              models={availableModels}
              currentModelId={currentModelId}
              currentModelLabel={currentModel || "选择模型"}
              onSelect={onModelSelect}
              chatMode={chatMode}
              onModeChange={onModeChange}
              airiAvailable={airiAvailable}
              thinkingLevel={thinkingLevel}
              onThinkingChange={onThinkingChange}
            />
            <div
              className="hidden items-center gap-2 text-[10px] xl:flex"
              style={{ color: "var(--yunque-text-muted)" }}
            >
              <span>Enter 发送</span>
              <span>·</span>
              <span>⇧↵ 换行</span>
            </div>
            {loading ? (
              <Button
                isIconOnly
                aria-label="停止生成"
                size="sm"
                className="rounded-2xl"
                style={{
                  background: "rgba(239,68,68,0.12)",
                  color: "#ef4444",
                }}
                onPress={onStop}
              >
                <StopCircle size={14} />
              </Button>
            ) : (
              <Button
                isIconOnly
                aria-label="发送"
                size="sm"
                className={`chat-send-btn h-10 w-10 rounded-[18px] ${input.trim() ? "chat-send-active" : ""}`}
                data-active={input.trim() ? "true" : "false"}
                isDisabled={!input.trim()}
                style={{
                  background: input.trim()
                    ? "var(--yunque-accent)"
                    : "var(--yunque-bg-muted)",
                  color: input.trim() ? "#fff" : "var(--yunque-text-muted)",
                }}
                onPress={onSend}
              >
                <Send size={14} />
              </Button>
            )}
          </div>
          {!loading && pendingFiles.length > 0 && (
            <div
              className="border-t px-4 py-1.5"
              style={{ borderColor: "var(--yunque-border)" }}
            >
              <span className="text-[10px]" style={{ color: "#4ade80" }}>
                {pendingFiles.length} 个附件待发送
              </span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
