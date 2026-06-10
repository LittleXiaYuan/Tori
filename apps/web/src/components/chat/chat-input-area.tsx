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
import { useI18n } from "@/lib/i18n";

type Translate = (key: string) => string;

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

function attachmentStatusLabel(file: PendingFile, t: Translate): string {
  if (file.note) return file.note;
  if (file.status === "uploading") return t("composer.attach.uploading");
  if (file.status === "parsed") return t("composer.attach.parsed");
  if (file.status === "error") return t("composer.attach.error");
  return file.type === "image" || file.type === "video"
    ? t("composer.attach.addedMedia")
    : t("composer.attach.addedFile");
}

export function ChatInputArea(props: ChatInputAreaProps) {
  const { t } = useI18n();
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

  const emptyComposer = !hasMessages;

  return (
    <div
      className={`shrink-0 ${emptyComposer ? "chat-composer-shell--empty px-0 py-0" : "px-5 py-2 xl:px-6"}`}
      style={{ borderTop: hasMessages ? "1px solid var(--yunque-border)" : "none" }}
      onDrop={onDrop}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      aria-label={t("composer.aria")}
    >
      <div className="mx-auto" style={{ maxWidth: emptyComposer ? "min(720px, 92vw)" : "min(860px, 92vw)" }}>
        <div
          ref={inputShellRef}
          className={`chat-input-wrap chat-composer overflow-visible transition-all ${emptyComposer ? "chat-composer--empty rounded-[20px]" : "rounded-[24px]"}`}
          data-busy={loading ? "true" : "false"}
          data-empty={emptyComposer ? "true" : undefined}
          style={{
            background:
              "linear-gradient(180deg, rgba(255,255,255,0.025), rgba(255,255,255,0.008)), var(--glass-card, var(--yunque-card))",
            border: isDragging
              ? "1px dashed var(--yunque-accent)"
              : "1px solid var(--glass-edge, var(--yunque-border))",
            boxShadow: isDragging
              ? "0 0 0 1px rgba(59,130,246,0.20), 0 12px 28px rgba(15,23,42,0.20)"
              : emptyComposer
                ? "0 4px 16px rgba(15,23,42,0.08), inset 0 1px 0 rgba(255,255,255,0.04)"
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
                    {t("composer.browserConnected")}
                  </span>
                ) : (
                  <span className="flex items-center gap-1.5">
                    <Sparkles size={11} />
                    {activeSlashCommand ? `/${activeSlashCommand}` : t("composer.commandMenu")}
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
                  <Sparkles size={11} /> {t("composer.command")}
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
                            {attachmentStatusLabel(f, t)}
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
            placeholder={emptyComposer ? t("composer.placeholderEmpty") : t("composer.placeholder")}
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
                <Tooltip.Content>{t("composer.addFile")}</Tooltip.Content>
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
                <Tooltip.Content>{t("composer.addImage")}</Tooltip.Content>
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
                  {isRecording ? t("composer.stopRec") : t("composer.voice")}
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
                  <Tooltip.Content>{t("composer.connector")}</Tooltip.Content>
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
              currentModelLabel={currentModel || t("composer.selectModel")}
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
              <span>{t("composer.enterSend")}</span>
              <span>·</span>
              <span>{t("composer.shiftNewline")}</span>
            </div>
            {loading ? (
              <Button
                isIconOnly
                aria-label={t("composer.stop")}
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
                aria-label={t("composer.send")}
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
                {t("composer.pendingFiles").replace("{n}", String(pendingFiles.length))}
              </span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
