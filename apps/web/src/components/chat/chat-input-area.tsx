"use client";

import { type RefObject } from "react";
import { Button, Dropdown, Label, Tooltip } from "@heroui/react";
import {
  Send,
  Paperclip,
  ImageIcon,
  Mic,
  StopCircle,
  Plug,
  Sparkles,
  Monitor,
  Plus,
} from "lucide-react";
import { SlashCommandMenu } from "@/components/slash-command-menu";
import { ConnectorPopover } from "@/components/connector-popover";
import { ModelSelectorPopup, type ModelOption } from "@/components/model-selector-popup";
import { AdvancedOptionsPopup } from "@/components/chat/advanced-options-popup";
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

  // Mirror the send guard in chat/page.tsx: a message can be sent when there is
  // text OR at least one attachment carrying content (parsed text, a workspace
  // path, or inline base64). Without this the send button stayed dead when the
  // user only attached a file.
  const hasPendingFileContext = pendingFiles.some(
    (f) => f.parsedText || f.workspacePath || f.base64,
  );
  const canSend = input.trim().length > 0 || hasPendingFileContext;

  return (
    <div
      className={`shrink-0 ${emptyComposer ? "chat-composer-shell--empty px-0 py-0" : "px-5 py-4 xl:px-6 mb-2"}`}
      style={{ borderTop: "none" }}
      onDrop={onDrop}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      aria-label={t("composer.aria")}
    >
      <div className="mx-auto" style={{ maxWidth: emptyComposer ? "min(720px, 92vw)" : "min(860px, 92vw)" }}>
        <div
          ref={inputShellRef}
          className={`chat-input-wrap chat-composer overflow-visible transition-all ${emptyComposer ? "chat-composer--empty rounded-[28px]" : "rounded-[28px]"}`}
          data-busy={loading ? "true" : "false"}
          data-empty={emptyComposer ? "true" : undefined}
          style={{
            background: "var(--yunque-elevated)",
            border: isDragging
              ? "1px dashed var(--yunque-accent)"
              : "1px solid var(--glass-edge, var(--yunque-border))",
            boxShadow: "var(--shadow-lg, 0 12px 32px rgba(0,0,0,0.4)), var(--glass-inner-highlight)",
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
                    <span className="w-1.5 h-1.5 rounded-full inline-block" style={{ background: "var(--yunque-accent)" }} />
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
                      ? "var(--yunque-accent)"
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
                      aria-label={`${t("composer.removeFile")}: ${f.name}`}
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
          </div>

          <textarea
            ref={inputRef}
            value={input}
            onChange={onInputChange}
            onKeyDown={onKeyDown}
            aria-label={t("composer.aria")}
            aria-describedby="chat-composer-hint"
            placeholder={emptyComposer ? "今天帮你做些什么？@引用对话文件，/调用技能与指令" : t("composer.placeholder")}
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
          {/* Keyboard hint, referenced by the textarea's aria-describedby so
              screen readers announce send/newline keys. Visually hidden. */}
          <span id="chat-composer-hint" className="sr-only">Enter 发送·⇧↵ 换行</span>

          <div className="flex flex-wrap items-center justify-between gap-3 px-4 pb-3 pt-2 border-t border-transparent">
            {/* Left Tools */}
            <div className="flex items-center gap-1 overflow-x-auto no-scrollbar">
              <Button
                variant="ghost"
                size="sm"

                className="text-xs font-medium px-2 min-w-0"
                style={{ color: "var(--yunque-text-secondary)" }}
                onPress={() => onSlashSelect("/")}
              >
                <Sparkles size={14} className="mr-1" />
                技能
              </Button>
              <Button
                variant="ghost"
                size="sm"
               
                className="text-xs font-medium px-2 min-w-0"
                style={{ color: "var(--yunque-text-secondary)" }}
                onPress={() => onConnectorsToggle(!showConnectors)}
              >
                <Plug size={14} className="mr-1" />
                连应用
              </Button>
              <ConnectorPopover
                open={showConnectors}
                onClose={() => onConnectorsToggle(false)}
              />
              <AdvancedOptionsPopup
                chatMode={chatMode}
                onModeChange={onModeChange}
                thinkingLevel={thinkingLevel}
                onThinkingChange={onThinkingChange}
              />
              <input
                type="file"
                ref={fileInputRef}
                className="hidden"
                multiple
                onChange={onFileUpload}
              />
            </div>

            {/* Right Tools */}
            <div className="flex items-center gap-1.5 shrink-0">
              <ModelSelectorPopup
                models={availableModels}
                currentModelId={currentModelId}
                currentModelLabel={currentModel || t("composer.selectModel")}
                onSelect={onModelSelect}
              />
              <Dropdown>
                <Button
                  isIconOnly
                  variant="ghost"
                  size="sm"
                 
                  aria-label={t("composer.add")}
                  style={{ color: "var(--yunque-text-secondary)" }}
                >
                  <Plus size={16} />
                </Button>
                <Dropdown.Popover className="min-w-[220px]">
                  <Dropdown.Menu
                    onAction={(key) => {
                      const action = String(key);
                      if (action === "file") fileInputRef.current?.click();
                      if (action === "image") onOpenImagePicker();
                      if (action === "voice") {
                        if (isRecording) onStopRecording();
                        else onStartRecording();
                      }
                      if (action === "connectors") onConnectorsToggle(!showConnectors);
                    }}
                  >
                    <Dropdown.Item id="file" textValue={t("composer.addFile")}>
                      <Paperclip size={14} />
                      <Label>{t("composer.addFile")}</Label>
                    </Dropdown.Item>
                    <Dropdown.Item id="image" textValue={t("composer.addImage")}>
                      <ImageIcon size={14} />
                      <Label>{t("composer.addImage")}</Label>
                    </Dropdown.Item>
                  </Dropdown.Menu>
                </Dropdown.Popover>
              </Dropdown>

              <Button
                isIconOnly
                variant="ghost"
                size="sm"
               
                aria-label={isRecording ? t("composer.stopRec") : t("composer.voice")}
                style={{ color: isRecording ? "#ef4444" : "var(--yunque-text-secondary)" }}
                onPress={() => isRecording ? onStopRecording() : onStartRecording()}
              >
                {isRecording ? <StopCircle size={16} /> : <Mic size={16} />}
              </Button>

              {loading ? (
                <Button
                  isIconOnly
                  aria-label={t("composer.stop")}
                  size="sm"
                 
                  style={{ background: "rgba(239,68,68,0.12)", color: "#ef4444" }}
                  onPress={onStop}
                >
                  <StopCircle size={14} />
                </Button>
              ) : (
                <Button
                  isIconOnly
                  aria-label={t("composer.send")}
                  size="sm"
                 
                  isDisabled={!canSend}
                  style={{
                    background: canSend ? "var(--yunque-text)" : "var(--yunque-bg-muted)",
                    color: canSend ? "var(--yunque-bg)" : "var(--yunque-text-muted)",
                  }}
                  onPress={onSend}
                >
                  <Send size={14} />
                </Button>
              )}
            </div>
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
