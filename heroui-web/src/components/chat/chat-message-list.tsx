import { Avatar, Button, Spinner, Tooltip, Chip } from "@heroui/react";
import {
  Pencil, RotateCcw, Copy, Undo2, Check, Library,
  Paperclip, Volume2, VolumeX, Heart, Monitor,
  Brain, Sparkles, FileDown, BookOpen,
} from "lucide-react";
import { api } from "@/lib/api";
import MarkdownRenderer from "@/components/markdown-renderer";
import { ExecutionTrace, type AgentEvent } from "@/components/execution-trace";
import { BrowserConnectCard } from "@/components/browser-connect-card";
import { SkillGrowthPanel } from "@/components/skill-growth-panel";
import { EmotionBadge, StickerView, SkillTags, AgentActions, type AgentAction } from "@/components/chat-extras";
import { ThinkingTimer } from "@/components/chat/thinking-timer";
import { openExternal } from "@/lib/safe-url";
import { browserActionLabel } from "@/lib/browser-action-labels";
import type { Message } from "@/lib/chat-types";
import { collectGeneratedFiles, summarizeAssistantWork } from "@/lib/chat-utils";
import type { BrowserBridgeState, BrowserSessionNotice } from "@/components/browser-session-card";

export interface ChatMessageListProps {
  messages: Message[];
  streaming: boolean;
  chatMode: "agent" | "fast" | "chat";
  currentModel: string;
  copiedIdx: string | null;
  ttsPlaying: string | null;
  bridgeState: BrowserBridgeState | null;
  resumePromptForBrowser: string | null;
  onCopy: (id: string, content: string) => void;
  onPlayTTS: (id: string, text: string) => void;
  onEdit: (id: string) => void;
  onRollback: (id: string) => void;
  onRetry: (id: string) => void;
  onAction: (action: AgentAction) => void;
  onSlashSelect: (cmd: string) => void;
  onSend: (text: string) => void;
  onBrowserRefresh: () => void;
  onBrowserContinue: (prompt: string) => void;
}

export function ChatMessageList({
  messages, streaming, chatMode, currentModel,
  copiedIdx, ttsPlaying, bridgeState, resumePromptForBrowser,
  onCopy, onPlayTTS, onEdit, onRollback, onRetry,
  onAction, onSlashSelect, onSend, onBrowserRefresh, onBrowserContinue,
}: ChatMessageListProps) {
  const isBubble = chatMode === "chat";
  return (
    <div className="mx-auto space-y-5" style={{ maxWidth: "min(900px, 70%)" }}>
      {messages.map((msg, idx) => (
        <div key={msg.id} className={`group chat-message-row flex gap-2.5 ${isBubble && msg.role === "user" ? "justify-end" : ""}`}>
          {(!isBubble || msg.role === "assistant") && (
            <Avatar size="sm" className="chat-message-avatar shrink-0 mt-1" style={{ background: msg.role === "assistant" ? "var(--yunque-accent)" : "#374151" }}>
              <Avatar.Fallback className="text-white text-xs font-bold">{msg.role === "assistant" ? "Y" : "U"}</Avatar.Fallback>
            </Avatar>
          )}
          <div className={`chat-message-stack ${isBubble ? `max-w-[74%] xl:max-w-[72%] ${msg.role === "user" ? "flex flex-col items-end" : ""}` : "flex-1 min-w-0"}`}>
            {!isBubble && (
              <div className="flex items-center gap-2 mb-1">
                <span className="text-[13px] font-semibold" style={{ color: msg.role === "assistant" ? "var(--yunque-accent)" : "var(--yunque-text)" }}>
                  {msg.role === "assistant" ? (currentModel || "Yunque Agent") : "用户"}
                </span>
                {msg.timestamp && (
                  <span className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                    {new Date(msg.timestamp).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })}
                  </span>
                )}
              </div>
            )}
            {/* Step summary */}
            {msg.role === "assistant" && msg.traceEvents && msg.traceEvents.length > 0 && (() => {
              const isLive = streaming && msg.id === messages[messages.length - 1]?.id;
              const toolEvents = msg.traceEvents.filter(e => e.type === "tool_start" || e.type === "tool_result");
              const warnEvents = msg.traceEvents.filter((e) => {
                const summary = (e.summary || "").toLowerCase();
                return e.type === "plan" && (summary.includes("warning") || summary.includes("risk") || summary.includes("blocked") || summary.includes("needs review"));
              });
              return (
                <>
                  <div className="chat-inline-panel mb-1.5 rounded-xl border px-2 py-2" style={{ background: "var(--yunque-bg-muted)", borderColor: "var(--yunque-border)" }}>
                    <div className="mb-2 flex flex-wrap items-center gap-2">
                      <span className="rounded-full px-2.5 py-1 text-[10px]" style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent)" }}>
                        {isLive ? "运行中" : "已完成"}
                      </span>
                      {(() => {
                        const summary = summarizeAssistantWork(msg);
                        return (
                          <>
                            {summary.primarySkill && (
                              <span className="rounded-full px-2.5 py-1 text-[10px]" style={{ background: "var(--yunque-bg-muted)", color: "var(--yunque-text-secondary)" }}>
                                {summary.primarySkill}
                              </span>
                            )}
                            {summary.toolCount > 0 && (
                              <span className="rounded-full px-2.5 py-1 text-[10px]" style={{ background: "var(--yunque-bg-muted)", color: "var(--yunque-text-secondary)" }}>
                                {summary.toolCount} tool events
                              </span>
                            )}
                            {summary.fileCount > 0 && (
                              <span className="rounded-full px-2.5 py-1 text-[10px]" style={{ background: "rgba(34,197,94,0.1)", color: "#4ade80" }}>
                                {summary.fileCount} files
                              </span>
                            )}
                          </>
                        );
                      })()}
                    </div>
                    <div className="text-xs leading-6" style={{ color: "var(--yunque-text-muted)" }}>
                      {isLive ? "The agent is still executing this request. Watch the live trace and computer panel for progress." : "This response includes structured execution steps, generated changes, and follow-up actions."}
                    </div>
                  </div>
                  {warnEvents.length > 0 && (
                    <div className="mb-2 rounded-lg px-3 py-1.5 text-[11px]" style={{ background: "rgba(245,158,11,0.08)", color: "#f59e0b", border: "1px solid rgba(245,158,11,0.15)" }}>
                      {warnEvents.map((w, wi) => <div key={wi}>{w.summary}</div>)}
                    </div>
                  )}
                  {toolEvents.length > 0 && (() => {
                    const uniqueSkills = [...new Set(toolEvents.map(e => e.meta?.skill).filter(Boolean))];
                    const lastSkill = toolEvents[toolEvents.length - 1]?.meta?.skill || "";
                    return (
                      <div className="chat-inline-panel mb-1.5 flex items-center gap-2 rounded-xl px-2 py-1 text-[10px]" style={{ background: "rgba(59,130,246,0.06)", color: "var(--yunque-text-muted)" }}>
                        {isLive && <span className="w-1.5 h-1.5 rounded-full animate-pulse" style={{ background: "var(--yunque-accent)" }} />}
                        <span>
                          {isLive ? `Working with ${lastSkill || "tools"}…` : `Used ${uniqueSkills.length} tools`}
                          {uniqueSkills.length > 0 && !isLive && (
                            <span style={{ color: "var(--yunque-text-muted)", marginLeft: 4 }}>
                              ({uniqueSkills.slice(0, 3).join(", ")}{uniqueSkills.length > 3 ? "…" : ""})
                            </span>
                          )}
                        </span>
                      </div>
                    );
                  })()}
                </>
              );
            })()}
            {/* Message content */}
            <div
              className={`chat-message-card text-[14px] leading-7 whitespace-pre-wrap ${isBubble ? `px-3.5 py-2.5 rounded-[18px] ${msg.role === "assistant" ? "assistant-message-shell chat-message-card--assistant" : "chat-message-card--user"}` : "py-1"}`}
              style={isBubble ? {
                background: msg.role === "user" ? "var(--neutral-strong-bg)" : "var(--yunque-card)",
                color: msg.role === "user" ? "var(--neutral-strong-fg)" : "var(--yunque-text)",
                border: msg.role === "assistant" ? "1px solid var(--yunque-border)" : "1px solid transparent",
                borderBottomRightRadius: msg.role === "user" ? "8px" : undefined,
                borderBottomLeftRadius: msg.role === "assistant" ? "8px" : undefined,
                boxShadow: "var(--shadow-sm)",
              } : { color: "var(--yunque-text)" }}
            >
              {msg.role === "user" && msg.images && msg.images.length > 0 && (
                <div className="flex gap-2 flex-wrap mb-2">
                  {msg.images.map((src, i) => (
                    <img key={i} src={src} alt="" className="max-w-[200px] max-h-[200px] rounded-lg object-cover cursor-pointer hover:opacity-90 transition-opacity" onClick={() => openExternal(src)} />
                  ))}
                </div>
              )}
              {msg.role === "assistant" && msg.reasoning && (
                <details className="mb-2" open={false} style={{ fontSize: "var(--text-sm)" }}>
                  <summary style={{ cursor: "pointer", color: "var(--yunque-text-muted)", fontStyle: "italic", display: "flex", alignItems: "center", gap: 4 }}>
                    <span style={{ fontSize: "var(--text-xs)", background: "rgba(245,158,11,0.12)", color: "#f59e0b", padding: "1px 6px", borderRadius: 4 }}>
                      {streaming && idx === messages.length - 1 ? "推理中…" : "已深度思考"}
                    </span>
                    <ThinkingTimer startMs={msg.reasoningStartMs} endMs={msg.reasoningEndMs} isStreaming={streaming && idx === messages.length - 1} />
                  </summary>
                  <div style={{ marginTop: 6, padding: "8px 12px", borderRadius: 8, background: "rgba(245,158,11,0.04)", border: "1px solid rgba(245,158,11,0.12)", whiteSpace: "pre-wrap", color: "var(--yunque-text-secondary)", fontSize: "var(--text-xs)", maxHeight: 300, overflow: "auto" }}>
                    {msg.reasoning}
                  </div>
                </details>
              )}
              {msg.content ? (
                msg.role === "assistant" ? <MarkdownRenderer content={msg.content} /> : (msg.content.replace(/\[(Uploaded file|File):\s*[^\]]+\]\s*/g, "").trim() || (msg.images?.length ? null : msg.content))
              ) : (
                !msg.images?.length && (
                  <div className="flex items-center gap-1.5">
                    <Spinner size="sm" color="current" /> Thinking…
                  </div>
                )
              )}
            </div>
            {/* Emotion + Sticker + Airi */}
            {msg.role === "assistant" && (msg.emotion || msg.sticker || msg.stickers || msg.airiSynced) && (
              <div className="flex items-center gap-2 mt-1.5 flex-wrap">
                {msg.emotion && <EmotionBadge emotion={msg.emotion} />}
                {msg.sticker && <StickerView sticker={msg.sticker} />}
                {msg.stickers && Object.values(msg.stickers).map((s, i) => <StickerView key={i} sticker={s} />)}
                {msg.airiSynced && (
                  <span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium" style={{ background: "linear-gradient(135deg, rgba(236,72,153,0.15), rgba(139,92,246,0.15))", color: "#d946ef", border: "1px solid rgba(217,70,239,0.2)" }}>
                    <Heart size={10} fill="#d946ef" /> Airi {msg.airiEmotion && msg.airiEmotion !== "neutral" ? `· ${msg.airiEmotion}` : ""}
                  </span>
                )}
              </div>
            )}
            {msg.role === "assistant" && msg.skills_used && msg.skills_used.length > 0 && <SkillTags skills={msg.skills_used} />}
            {/* Context layers */}
            {msg.role === "assistant" && msg.contextLayers && msg.contextLayers.length > 0 && (
              <div className="mt-2 rounded-xl px-3 py-2" style={{ background: "rgba(139,92,246,0.06)", border: "1px solid rgba(139,92,246,0.12)" }}>
                <div className="flex flex-wrap items-center gap-2">
                  {msg.contextLayers.includes("memory") && (
                    <span className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(139,92,246,0.12)", color: "#a78bfa" }}>
                      <Brain size={11} /> 调用了你的记忆
                    </span>
                  )}
                  {(msg.contextLayers.includes("graph") || msg.contextLayers.includes("code")) && (
                    <span className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(6,182,212,0.12)", color: "#22d3ee" }}>
                      <Library size={11} /> 参考了知识库
                    </span>
                  )}
                  {msg.contextLayers.includes("emotion") && (
                    <span className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(236,72,153,0.12)", color: "#f472b6" }}>
                      <Heart size={11} /> 感知了你的情绪
                    </span>
                  )}
                  {msg.contextLayers.includes("strategy") && (
                    <span className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(245,158,11,0.12)", color: "#fbbf24" }}>
                      <Sparkles size={11} /> 运用了积累的经验
                    </span>
                  )}
                </div>
                <div className="mt-1.5 text-[10px] leading-4" style={{ color: "var(--yunque-text-muted)" }}>
                  {msg.contextLayers.includes("memory") ? "这条回复参考了你过去的对话和偏好。" :
                   msg.contextLayers.includes("graph") || msg.contextLayers.includes("code") ? "这条回复引用了知识库中的相关内容。" :
                   "Agent 利用了积累的上下文来提升回复质量。"}
                </div>
              </div>
            )}
            {/* Actions */}
            {msg.role === "assistant" && msg.actions && msg.actions.length > 0 && (
              <div className="chat-inline-panel mt-2 rounded-xl border p-2" style={{ background: "var(--yunque-bg-muted)", borderColor: "var(--yunque-border)" }}>
                <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>Suggested actions</div>
                <AgentActions actions={msg.actions} onAction={onAction} />
              </div>
            )}
            {/* Browser summary */}
            {msg.role === "assistant" && msg.browserSummary && (
              <div className="chat-inline-panel mt-2 rounded-xl border p-2" style={{ background: "var(--yunque-bg-muted)", borderColor: "var(--yunque-border)" }}>
                <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>Browser artifact</div>
                <div className="flex flex-wrap items-center gap-2 text-[11px]" style={{ color: "var(--yunque-text-secondary)" }}>
                  {msg.browserSummary.action && <span className="rounded-full px-2.5 py-1" style={{ background: "rgba(59,130,246,0.12)", color: "#93c5fd" }}>{browserActionLabel(msg.browserSummary.action)}</span>}
                  {typeof msg.browserSummary.elementCount === "number" && <span className="rounded-full px-2.5 py-1" style={{ background: "var(--yunque-bg-muted)", color: "var(--yunque-text-muted)" }}>{msg.browserSummary.elementCount} elements</span>}
                  {msg.browserSummary.hasScreenshot && <span className="rounded-full px-2.5 py-1" style={{ background: "var(--yunque-success-muted)", color: "var(--yunque-success)" }}>screenshot ready</span>}
                  {typeof msg.browserSummary.textLength === "number" && msg.browserSummary.textLength > 0 && <span className="rounded-full px-2.5 py-1" style={{ background: "var(--yunque-bg-muted)", color: "var(--yunque-text-muted)" }}>{msg.browserSummary.textLength} chars</span>}
                </div>
                {(msg.browserSummary.title || msg.browserSummary.url) && (
                  <div className="mt-2 min-w-0">
                    {msg.browserSummary.title && <div className="truncate text-sm" style={{ color: "var(--yunque-text-secondary)" }}>{msg.browserSummary.title}</div>}
                    {msg.browserSummary.url && <div className="mt-1 truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{msg.browserSummary.url}</div>}
                  </div>
                )}
                {msg.browserSummary.preview && (
                  <div className="mt-2 rounded-2xl px-3 py-2 text-xs leading-6" style={{ background: "rgba(15,23,42,0.35)", color: "var(--yunque-text-secondary)" }}>{msg.browserSummary.preview}</div>
                )}
                {(msg.browserSummary.suggestedCommand || msg.browserSummary.url) && (
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    {msg.browserSummary.suggestedCommand && <Button size="sm" variant="ghost" className="rounded-full px-3" onPress={() => onSlashSelect(msg.browserSummary?.suggestedCommand || "/")}>{msg.browserSummary.suggestedLabel || "Use next command"}</Button>}
                    {msg.browserSummary.url && <Button size="sm" variant="ghost" className="rounded-full px-3" onPress={() => openExternal(msg.browserSummary?.url)}>Open page</Button>}
                  </div>
                )}
              </div>
            )}
            {/* E2B Sandbox */}
            {msg.role === "assistant" && msg.sandbox && (
              <div className="chat-inline-panel mt-2 rounded-xl border p-3" style={{ background: "linear-gradient(135deg, rgba(34,197,94,0.06), rgba(59,130,246,0.06))", borderColor: "rgba(34,197,94,0.2)" }}>
                <div className="flex items-center gap-2 mb-2">
                  <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: "rgba(34,197,94,0.15)" }}><Monitor size={16} style={{ color: "#22c55e" }} /></div>
                  <div>
                    <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>E2B Desktop</div>
                    <div className="text-[11px] font-mono" style={{ color: "var(--yunque-text-muted)" }}>{msg.sandbox.sandbox_id}</div>
                  </div>
                  <Chip size="sm" style={{ marginLeft: "auto", background: "rgba(34,197,94,0.12)", color: "#22c55e", fontSize: "10px" }}>LIVE</Chip>
                </div>
                {msg.sandbox.stream_url && (
                  <Button size="sm" className="w-full mt-1" onPress={() => openExternal(msg.sandbox?.stream_url)} style={{ background: "rgba(34,197,94,0.15)", color: "#22c55e", border: "1px solid rgba(34,197,94,0.25)" }}>
                    <Monitor size={14} className="mr-2" /> Open Desktop
                  </Button>
                )}
              </div>
            )}
            {/* Generated files */}
            {msg.role === "assistant" && msg.traceEvents && (() => {
              const files = collectGeneratedFiles(msg.traceEvents);
              if (files.length === 0) return null;
              return (
                <div className="chat-inline-panel mt-2 rounded-xl border p-2" style={{ background: "var(--yunque-bg-muted)", borderColor: "var(--yunque-border)" }}>
                  <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>Generated files</div>
                  <div className="space-y-2">
                    {files.map((f, i) => {
                      const ext = (f.name || f.path).split(".").pop()?.toLowerCase() || "";
                      const isDoc = ["pdf", "docx", "xlsx", "pptx", "doc", "xls", "ppt"].includes(ext);
                      return (
                        <a key={i} href={`/api/files/download?path=${encodeURIComponent(f.path)}`} download={f.name || f.path}
                          className="flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-medium transition-all hover:scale-[1.01]"
                          style={{ background: isDoc ? "var(--yunque-accent-muted)" : "var(--yunque-bg-muted)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}>
                          <div className="w-10 h-10 rounded-lg flex items-center justify-center shrink-0" style={{ background: isDoc ? "var(--yunque-accent-muted)" : "var(--yunque-bg-muted)" }}><Paperclip size={18} /></div>
                          <div className="flex-1 min-w-0">
                            <div className="truncate font-semibold">{f.name || f.path.split("/").pop() || f.path}</div>
                            <div className="text-[11px] mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>
                              {ext.toUpperCase()} {f.size != null && f.size > 0 ? `  ${f.size > 1024 * 1024 ? `${(f.size / 1024 / 1024).toFixed(1)} MB` : `${(f.size / 1024).toFixed(1)} KB`}` : ""}
                            </div>
                          </div>
                          <div className="w-8 h-8 rounded-full flex items-center justify-center shrink-0" style={{ background: "var(--yunque-accent-muted)" }}><span style={{ color: "var(--yunque-accent)", fontSize: 16 }}>↗</span></div>
                        </a>
                      );
                    })}
                  </div>
                </div>
              );
            })()}
            {/* Suggestions */}
            {msg.role === "assistant" && msg.suggestions && msg.suggestions.length > 0 && !streaming && (
              <details className="mt-3">
                <summary className="cursor-pointer text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>Next moves</summary>
                <div className="chat-inline-panel mt-2 rounded-xl border p-2" style={{ background: "var(--yunque-bg-muted)", borderColor: "var(--yunque-border)" }}>
                  <div className="flex flex-wrap gap-2">
                    {msg.suggestions.map((s, i) => (
                      <button key={i} onClick={() => {
                        if (s.label === "存入知识库") onSend("/save_knowledge Save the above response to knowledge base.");
                        else if (s.type === "save_skill") onSend("Turn this workflow into a reusable skill and save it for later.");
                        else onSend(s.label);
                      }}
                        className="chat-followup-chip px-3 py-1.5 rounded-full text-xs font-medium cursor-pointer"
                        style={{ background: s.type === "save_skill" ? "rgba(139,92,246,0.12)" : "rgba(59,130,246,0.08)", border: `1px solid ${s.type === "save_skill" ? "rgba(139,92,246,0.3)" : "rgba(59,130,246,0.15)"}`, color: s.type === "save_skill" ? "#a78bfa" : "#93c5fd" }}>
                        {s.type === "save_skill" ? "Save " : "→ "}{s.label}
                      </button>
                    ))}
                  </div>
                </div>
              </details>
            )}
            {/* Browser requirement */}
            {msg.role === "assistant" && msg.browserRequirement?.required && (
              <BrowserConnectCard
                requirement={msg.browserRequirement}
                connected={Boolean(bridgeState?.connected)}
                onOpenSetup={() => window.open(msg.browserRequirement?.install_path || "/browser", "_blank", "noopener,noreferrer")}
                onRefresh={onBrowserRefresh}
                onContinue={bridgeState?.connected ? () => {
                  const prevPrompt = messages[idx - 1]?.role === "user" ? messages[idx - 1]?.content : resumePromptForBrowser;
                  if (prevPrompt) onBrowserContinue(prevPrompt);
                } : undefined}
                continueLabel="Continue blocked task"
              />
            )}
            {/* Skill growth */}
            {msg.role === "assistant" && msg.skillSuggestions && msg.skillSuggestions.length > 0 && (
              <div className="mt-2 rounded-xl px-3 py-2.5" style={{ background: "rgba(34,197,94,0.06)", border: "1px solid rgba(34,197,94,0.15)" }}>
                <div className="flex items-center gap-2 mb-2">
                  <Sparkles size={13} style={{ color: "#4ade80" }} />
                  <span className="text-[11px] font-semibold" style={{ color: "#4ade80" }}>Agent 学到了新技能</span>
                </div>
                <SkillGrowthPanel suggestions={msg.skillSuggestions} onSave={(s) => onSend(`Turn this into a reusable skill.\n\nName: ${s.name}\nDescription: ${s.description}\nTrigger: ${s.trigger}`)} />
              </div>
            )}
            {/* Execution trace */}
            {msg.role === "assistant" && msg.traceEvents && msg.traceEvents.length > 0 && (
              <details className="mt-3">
                <summary className="cursor-pointer text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>Execution trace</summary>
                <div className="mt-2"><ExecutionTrace events={msg.traceEvents} isLive={streaming && idx === messages.length - 1} /></div>
              </details>
            )}
            {/* Quick actions card for substantial responses */}
            {msg.role === "assistant" && msg.content && msg.content.length > 400 && !streaming && (
              <div className="mt-2 flex items-center gap-2">
                <button
                  onClick={() => onSend("/save_knowledge Save the above response to knowledge base.")}
                  className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[11px] font-medium transition-all hover:scale-[1.02]"
                  style={{ background: "rgba(139,92,246,0.1)", border: "1px solid rgba(139,92,246,0.2)", color: "#a78bfa" }}
                >
                  <BookOpen size={11} /> 存入知识库
                </button>
                <button
                  onClick={() => onSend("/report Generate a structured report from the above conversation.")}
                  className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[11px] font-medium transition-all hover:scale-[1.02]"
                  style={{ background: "rgba(59,130,246,0.08)", border: "1px solid rgba(59,130,246,0.15)", color: "#93c5fd" }}
                >
                  <FileDown size={11} /> 导出报告
                </button>
              </div>
            )}
            {/* Message tools */}
            {msg.content && (
              <div className={`chat-message-tools flex gap-0.5 mt-1 ${!isBubble ? "justify-end" : ""}`} style={isBubble ? { justifyContent: msg.role === "user" ? "flex-end" : "flex-start" } : undefined}>
                {msg.role === "user" && (
                  <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={() => onEdit(msg.id)}><Pencil size={11} /></Button><Tooltip.Content>编辑</Tooltip.Content></Tooltip>
                )}
                {msg.role === "assistant" && (
                  <>
                    <Tooltip delay={0}>
                      <Button isIconOnly variant="ghost" size="sm" onPress={() => onCopy(msg.id, msg.content)}>
                        {copiedIdx === msg.id ? <Check size={11} className="text-green-400" /> : <Copy size={11} />}
                      </Button>
                      <Tooltip.Content>{copiedIdx === msg.id ? "已复制" : "复制"}</Tooltip.Content>
                    </Tooltip>
                    <Tooltip delay={0}>
                      <Button isIconOnly variant="ghost" size="sm" onPress={() => onPlayTTS(msg.id, msg.content)}>
                        {ttsPlaying === msg.id ? <VolumeX size={11} style={{ color: "var(--yunque-accent)" }} /> : <Volume2 size={11} />}
                      </Button>
                      <Tooltip.Content>{ttsPlaying === msg.id ? "停止播放" : "播放语音"}</Tooltip.Content>
                    </Tooltip>
                    <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={() => onRollback(msg.id)}><Undo2 size={11} /></Button><Tooltip.Content>回滚到此</Tooltip.Content></Tooltip>
                    <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={() => onSend(`/save_knowledge Save the above response to knowledge base.`)}><BookOpen size={11} /></Button><Tooltip.Content>存入知识库</Tooltip.Content></Tooltip>
                    <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={() => onSend(`/report Generate a structured report from the above conversation.`)}><FileDown size={11} /></Button><Tooltip.Content>导出报告</Tooltip.Content></Tooltip>
                  </>
                )}
                <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={() => onRetry(msg.id)}><RotateCcw size={11} /></Button><Tooltip.Content>重新发送</Tooltip.Content></Tooltip>
              </div>
            )}
            {!isBubble && <div className="mt-3" style={{ borderBottom: "1px solid var(--yunque-border)" }} />}
          </div>
          {isBubble && msg.role === "user" && (
            <Avatar size="sm" className="shrink-0 mt-1" style={{ background: "#374151" }}>
              <Avatar.Fallback className="text-white text-xs">U</Avatar.Fallback>
            </Avatar>
          )}
        </div>
      ))}
    </div>
  );
}
