"use client";

import { useState, useRef, useEffect, type Dispatch, type RefObject } from "react";
import { Avatar, Tooltip, Spinner } from "@heroui/react";
import { Send, Plus, MessageCircle, ChevronDown, Paperclip, Settings, Cpu, Sparkles, Search, X, ArrowUp, ArrowDown, MoreHorizontal } from "lucide-react";
import MarkdownRenderer from "@/components/markdown-renderer";
import { ModelSelectorPopup, type ModelOption, type ChatMode } from "@/components/model-selector-popup";

interface SimpleMessage {
  role: "user" | "assistant";
  content: string;
  id: string;
  reasoning?: string;
  reasoningStartMs?: number;
  reasoningEndMs?: number;
}

interface SimpleChatState {
  messages: SimpleMessage[];
  input: string;
  loading: boolean;
  streaming: boolean;
}

interface ConvItem {
  id: string;
  title: string;
  lastMessage?: string;
  updatedAt?: string;
  pinned?: boolean;
  archived?: boolean;
}

export interface SimpleChatViewProps {
  chat: SimpleChatState;
  chatDispatch: Dispatch<{ type: string; value?: string }>;
  conversations: ConvItem[];
  activeConvId: string;
  onSelectConv: (id: string) => void;
  onNewConversation: () => void;
  onDeleteConversation: (id: string) => void;
  onSend: (text: string) => void;
  scrollRef: RefObject<HTMLDivElement | null>;
  inputRef: RefObject<HTMLTextAreaElement | null>;
  availableModels: ModelOption[];
  currentModel: string;
  currentModelId: string;
  onSelectModel: (m: ModelOption) => void;
  chatMode: ChatMode;
  onModeChange: (mode: ChatMode) => void;
  airiAvailable: boolean;
  setupNeeded: boolean;
}

function ThinkingTimer({ startMs, endMs, isStreaming }: { startMs?: number; endMs?: number; isStreaming: boolean }) {
  const [elapsed, setElapsed] = useState(0);
  useEffect(() => {
    if (!startMs) return;
    if (endMs) { setElapsed((endMs - startMs) / 1000); return; }
    if (!isStreaming) return;
    const tick = () => setElapsed((Date.now() - startMs) / 1000);
    tick();
    const id = setInterval(tick, 100);
    return () => clearInterval(id);
  }, [startMs, endMs, isStreaming]);
  if (!startMs || elapsed <= 0) return null;
  return <span className="simple-thinking-time">（用时 {elapsed.toFixed(1)} 秒）</span>;
}

export function SimpleChatView({
  chat, chatDispatch, conversations, activeConvId, onSelectConv,
  onNewConversation, onDeleteConversation, onSend, scrollRef, inputRef,
  availableModels, currentModel, currentModelId, onSelectModel,
  chatMode, onModeChange, airiAvailable, setupNeeded,
}: SimpleChatViewProps) {
  const [showConvList, setShowConvList] = useState(false);
  const [convSearch, setConvSearch] = useState("");
  const convRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!showConvList) return;
    const handler = (e: MouseEvent) => {
      if (convRef.current && !convRef.current.contains(e.target as Node)) setShowConvList(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [showConvList]);

  const filteredConvs = convSearch
    ? conversations.filter(c => c.title?.toLowerCase().includes(convSearch.toLowerCase()))
    : conversations;

  const activeTitle = conversations.find(c => c.id === activeConvId)?.title || "新对话";

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      if (chat.input.trim() && !chat.loading) onSend(chat.input);
    }
  };

  useEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [chat.messages, scrollRef]);

  return (
    <div className="simple-chat-root">
      {/* Floating conversations panel */}
      {showConvList && <div className="simple-conv-overlay" onClick={() => setShowConvList(false)} />}
      {showConvList && (
        <div ref={convRef} className="simple-conv-panel">
          <div className="simple-conv-header">
            <span className="simple-conv-title">对话列表</span>
            <button onClick={onNewConversation} className="simple-conv-new-btn">
              <Plus size={16} />
            </button>
          </div>
          <div className="simple-conv-search">
            <Search size={13} />
            <input
              value={convSearch}
              onChange={e => setConvSearch(e.target.value)}
              placeholder="搜索对话…"
            />
            {convSearch && <button onClick={() => setConvSearch("")}><X size={12} /></button>}
          </div>
          <div className="simple-conv-list">
            {filteredConvs.map(c => (
              <button
                key={c.id}
                className={`simple-conv-item ${c.id === activeConvId ? "active" : ""}`}
                onClick={() => { onSelectConv(c.id); setShowConvList(false); }}
              >
                <MessageCircle size={14} />
                <span className="truncate">{c.title || "新对话"}</span>
              </button>
            ))}
            {filteredConvs.length === 0 && (
              <div className="simple-conv-empty">没有对话</div>
            )}
          </div>
        </div>
      )}

      {/* Top Bar — minimal Cherry style */}
      <header className="simple-topbar">
        <div className="simple-topbar-left">
          <button className="simple-topbar-btn" onClick={() => setShowConvList(!showConvList)}>
            <MessageCircle size={17} />
          </button>
          <ModelSelectorPopup
            models={availableModels}
            currentModelId={currentModelId}
            currentModelLabel={currentModel || "选择模型"}
            onSelect={onSelectModel}
            chatMode={chatMode}
            onModeChange={onModeChange}
            airiAvailable={airiAvailable}
          />
        </div>
        <div className="simple-topbar-center">
          <span className="simple-topbar-title">{activeTitle}</span>
        </div>
        <div className="simple-topbar-right">
          {chat.streaming && <span className="simple-streaming-dot" />}
          <button className="simple-topbar-btn" onClick={onNewConversation}>
            <Plus size={17} />
          </button>
        </div>
      </header>

      {/* Messages */}
      <div ref={scrollRef} className="simple-messages">
        {chat.messages.length === 0 ? (
          <div className="simple-empty-state">
            <div className="simple-empty-icon">
              <Sparkles size={32} />
            </div>
            <h3>开始新的对话</h3>
            <p>输入你的问题，AI 会为你解答</p>
            {setupNeeded && (
              <a href="/settings/providers" className="simple-setup-link">
                <Settings size={14} /> 配置模型提供商
              </a>
            )}
          </div>
        ) : (
          <div className="simple-message-list">
            {chat.messages.map((msg, idx) => (
              <div key={msg.id} className={`simple-msg ${msg.role}`}>
                {msg.role === "assistant" && (
                  <div className="simple-msg-avatar">
                    <Avatar size="sm" style={{ background: "var(--yunque-accent)", width: 30, height: 30 }}>
                      <Avatar.Fallback className="text-white text-[10px] font-bold">Y</Avatar.Fallback>
                    </Avatar>
                  </div>
                )}
                <div className={`simple-msg-bubble ${msg.role}`}>
                  {msg.role === "assistant" && msg.reasoning && (
                    <details className="simple-thinking-block">
                      <summary>
                        <Sparkles size={12} />
                        <span>{chat.streaming && idx === chat.messages.length - 1 ? "思考中…" : "已深度思考"}</span>
                        <ThinkingTimer
                          startMs={msg.reasoningStartMs}
                          endMs={msg.reasoningEndMs}
                          isStreaming={chat.streaming && idx === chat.messages.length - 1}
                        />
                      </summary>
                      <div className="simple-thinking-content">{msg.reasoning}</div>
                    </details>
                  )}
                  {msg.role === "assistant" ? (
                    <MarkdownRenderer content={msg.content || (chat.streaming && idx === chat.messages.length - 1 ? "" : "…")} />
                  ) : (
                    <div className="whitespace-pre-wrap">{msg.content}</div>
                  )}
                  {msg.role === "assistant" && chat.streaming && idx === chat.messages.length - 1 && !msg.content && (
                    <div className="simple-typing">
                      <span /><span /><span />
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Input Area */}
      <div className="simple-input-area">
        <div className="simple-input-box">
          <textarea
            ref={inputRef}
            value={chat.input}
            onChange={e => chatDispatch({ type: "SET_INPUT", value: e.target.value })}
            onKeyDown={handleKeyDown}
            placeholder="输入消息… (Shift+Enter 换行)"
            rows={1}
            disabled={chat.loading}
          />
          <div className="simple-input-actions">
            <button
              className="simple-send-btn"
              onClick={() => { if (chat.input.trim() && !chat.loading) onSend(chat.input); }}
              disabled={!chat.input.trim() || chat.loading}
            >
              {chat.loading ? <Spinner size="sm" /> : <Send size={16} />}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
