import { useMemo, useState } from "react";
import { Avatar, Button, Spinner, Tooltip, Chip, Popover } from "@heroui/react";
import {
  Pencil, RotateCcw, Copy, Undo2, Check, Library,
  Paperclip, Volume2, VolumeX, Heart, Monitor,
  Brain, Sparkles, FileDown, BookOpen, Share2, Send, Settings, Eye, Wand2, Cpu, MoreHorizontal,
  ArrowRight,
} from "lucide-react";
import { api, type FilePreviewResponse, type NotifyChannel } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import MarkdownRenderer from "@/components/markdown-renderer";
import { ExecutionTrace, type AgentEvent } from "@/components/execution-trace";
import { BrowserConnectCard } from "@/components/browser-connect-card";
import { SkillGrowthPanel } from "@/components/skill-growth-panel";
import { EmotionBadge, StickerView, SkillTags, AgentActions, type AgentAction } from "@/components/chat-extras";
import { CognitiveStatusBar } from "@/components/cognitive-status-bar";
import { ThinkingTimer } from "@/components/chat/thinking-timer";
import { TaskPlanCard } from "@/components/chat/task-plan-card";
import { openExternal } from "@/lib/safe-url";
import { browserActionLabel } from "@/lib/browser-action-labels";
import type { ChatSharePayload, Message } from "@/lib/chat-types";
import { collectGeneratedFiles } from "@/lib/chat-utils";
import { formatErrorMessage } from "@/lib/error-utils";
import {
  parsePackStudioBatchDraftRequestPrompt,
  parsePackStudioPatchDraftRequestPrompt,
  parsePackStudioPatchDraftPrompt,
  parsePackStudioPatchPlanPrompt,
  type PackStudioBatchDraftRequest,
  type PackStudioPatchDraft,
  type PackStudioPatchDraftRequest,
  type PackStudioPatchPlanSummary,
} from "@/lib/pack-studio-chat";
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
  onShare: (messageId: string, channel: NotifyChannel, payload: ChatSharePayload) => Promise<void>;
}

function filePreviewStatusLabel(status?: string): string {
  switch (status) {
    case "parsed": return "已可预览";
    case "needs_document_parser": return "等待读取正文";
    case "ready": return "已添加";
    case "error": return "需要处理";
    default: return status || "";
  }
}

function filePreviewNoteLabel(note?: string): string {
  const raw = (note || "").trim();
  if (!raw) return "";
  const normalized = raw.toLowerCase();
  if (
    raw.includes("本地解析器") ||
    raw.includes("文档解析后端") ||
    normalized.includes("mineru") ||
    normalized.includes("parser")
  ) {
    if (raw.includes("不能直接展开") || raw.includes("文档解析") || normalized.includes("needs_document_parser")) {
      return "附件已保留，暂时还不能直接展开正文；可稍后继续或补充文档读取能力后重试。";
    }
    return "附件状态已更新，发送后由模型继续读取。";
  }
  return formatErrorMessage(raw, raw);
}

function basenameFromAttachmentPath(path: string): string {
  const normalized = path.trim().replace(/\\/g, "/");
  return normalized.split("/").filter(Boolean).pop() || normalized || "附件";
}

function maskParsedAttachmentMarker(line: string): string {
  return line
    .replace(/\[Parsed document:\s*([^\]]+)\]/gi, (_match, file: string) => `附件内容：${file.trim()}`)
    .replace(/^Workspace path:\s*(.+)$/i, (_match, file: string) => `附件名称：${basenameFromAttachmentPath(file)}`)
    .replace(/^Parser:\s*(.+)$/i, (_match, parser: string) => `附件解析器：${parser.trim()}`)
    .replace(/^Status:\s*(.+)$/i, (_match, status: string) => `附件状态：${status.trim()}`)
    .replace(/^Note:\s*(.+)$/i, (_match, note: string) => `附件说明：${note.trim()}`);
}

function displayChatText(text?: string): string {
  const raw = (text || "").trim();
  if (!raw) return "";
  return raw
    .split(/\r?\n/)
    .map((line) => formatErrorMessage(line, line))
    .map(maskParsedAttachmentMarker)
    .join("\n")
    .trim();
}

function displayMessageContent(msg: Message): string {
  if (msg.role === "user") return msg.content;
  return displayChatText(msg.content);
}

function packStudioToolSummary(content: string): string | null {
  const batchRequest = parsePackStudioBatchDraftRequestPrompt(content);
  if (batchRequest) {
    const packLines = batchRequest.packs.slice(0, 5).map((pack, index) => {
      const missing = pack.missing.length ? `缺口：${pack.missing.join(" / ")}` : "缺口：未标注";
      return `${index + 1}. ${pack.name || pack.id} ${pack.version || ""} · ${pack.readiness || "待评估"} · ${missing}`.trim();
    });
    return [
      batchRequest.displayText,
      `Pack Studio 批量补肉任务: ${batchRequest.packs.length} 个能力包`,
      batchRequest.batch?.total ? `队列批次：第 ${batchRequest.batch.page || 1} / ${batchRequest.batch.pageCount || 1} 批；总计 ${batchRequest.batch.total} 个待补肉` : "",
      batchRequest.goal ? `目标：${batchRequest.goal}` : "",
      batchRequest.rules.length ? `规则：${batchRequest.rules.join(" / ")}` : "",
      ...packLines,
      batchRequest.packs.length > packLines.length ? `还有 ${batchRequest.packs.length - packLines.length} 个能力包未展开。` : "",
      "请逐包生成 Draft Request；不要跳过 Studio 的 diff / audit / repack。",
    ].filter(Boolean).join("\n");
  }
  const request = parsePackStudioPatchDraftRequestPrompt(content);
  if (request) {
    return [
      request.displayText,
      `Pack Studio Draft Request: ${request.pack.name || request.pack.id} ${request.pack.version || ""}`.trim(),
      `目标文件：${request.target.filePath}`,
      `风险：${request.target.riskLevel || "未标注"}`,
      `starter 内容长度：${request.starterContentLength.toLocaleString()} chars`,
      request.target.gates.length ? `门禁：${request.target.gates.join(" / ")}` : "",
      request.target.reason ? `原因：${request.target.reason}` : "",
      "请让小羽只返回 yunque.pack_studio.patch_draft.v1 JSON；不要跳过 Studio 的 diff / audit / repack。",
    ].filter(Boolean).join("\n");
  }
  const draft = parsePackStudioPatchDraftPrompt(content);
  if (draft) {
    return [
      draft.displayText,
      `Pack Studio Patch Draft: ${draft.pack.name || draft.pack.id} ${draft.pack.version || ""}`.trim(),
      `文件：${draft.filePath}`,
      `风险：${draft.riskLevel || "未标注"}`,
      `内容长度：${draft.content.length.toLocaleString()} chars`,
      draft.gates.length ? `门禁：${draft.gates.join(" / ")}` : "",
      draft.reason ? `原因：${draft.reason}` : "",
      "完整文件内容已隐藏；请回到 /packs/studio 导入后预览 diff、运行审计、重新打包。",
    ].filter(Boolean).join("\n");
  }
  const plan = parsePackStudioPatchPlanPrompt(content);
  if (plan) {
    return [
      plan.displayText,
      `Pack Studio Patch Plan: ${plan.pack.name || plan.pack.id} ${plan.pack.version || ""}`.trim(),
      `工作区：${plan.workspace.id || plan.workspace.path}`,
      `候选改动：${plan.candidates.length}`,
      "完整文件内容未包含在计划里；请回到 /packs/studio 载入候选并走 diff / audit / repack。",
    ].filter(Boolean).join("\n");
  }
  return null;
}

function displayMessageContentForTools(msg: Message): string {
  const structured = packStudioToolSummary(msg.content);
  if (structured) return msg.role === "assistant" ? displayChatText(structured) : structured;
  return displayMessageContent(msg);
}

function packStudioHandoffHref(pack: { id?: string }, goal: string | undefined, hash: string): string {
  const params = new URLSearchParams();
  if (pack.id) params.set("packId", pack.id);
  if (goal) params.set("goal", goal);
  const query = params.toString();
  return `/packs/studio${query ? `?${query}` : ""}${hash}`;
}

function packDetailHref(pack: { id?: string }): string | null {
  return pack.id ? `/packs/detail?id=${encodeURIComponent(pack.id)}` : null;
}

function packCenterHref(pack: { id?: string }): string {
  return pack.id ? `/packs?q=${encodeURIComponent(pack.id)}` : "/packs";
}

function renderPackGovernanceLinks(pack: { id?: string }, detailLabel = "查看详情", centerLabel = "回中心") {
  const detailHref = packDetailHref(pack);
  return (
    <div className="mt-2 flex flex-wrap items-center gap-2">
      {detailHref && (
        <a href={detailHref} className="inline-flex items-center gap-1 text-[11px] font-medium" style={{ color: "var(--yunque-accent)" }}>
          {detailLabel} <ArrowRight size={10} />
        </a>
      )}
      <a href={packCenterHref(pack)} className="inline-flex items-center gap-1 text-[11px] font-medium" style={{ color: "var(--yunque-text-muted)" }}>
        {centerLabel} <ArrowRight size={10} />
      </a>
    </div>
  );
}

function packStudioBatchHandoffHref(request: PackStudioBatchDraftRequest): string {
  const payload = {
    kind: "yunque.pack_studio.batch_draft_request.v1",
    goal: request.goal,
    ...(request.batch ? {
      batch: {
        page: request.batch.page,
        page_count: request.batch.pageCount,
        total: request.batch.total,
        page_size: request.batch.pageSize,
      },
    } : {}),
    rules: request.rules,
    packs: request.packs.map((pack) => ({
      id: pack.id,
      name: pack.name,
      version: pack.version,
      status: pack.status,
      source: pack.source,
      missing: pack.missing,
      readiness: pack.readiness,
      studio_url: pack.studioUrl,
      package_url: pack.packageUrl,
      sha256: pack.sha256,
    })),
  };
  const batchText = [
    "小羽收到批量补肉任务。",
    "",
    "```json",
    JSON.stringify(payload, null, 2),
    "```",
  ].join("\n");
  return `/packs/studio?batch=${encodeURIComponent(batchText)}`;
}

function renderPackStudioPlan(plan: PackStudioPatchPlanSummary) {
  return (
    <div
      className="mb-3 rounded-xl border p-3 text-xs"
      style={{
        background: "rgba(59,130,246,0.08)",
        borderColor: "rgba(59,130,246,0.22)",
        color: "var(--yunque-text)",
      }}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2 font-semibold">
            <Sparkles size={14} style={{ color: "var(--yunque-accent)" }} />
            <span>Pack Studio 改包任务</span>
          </div>
          <div className="mt-1 truncate" style={{ color: "var(--yunque-text-muted)" }}>
            {plan.pack.name || plan.pack.id} · {plan.pack.version || "unknown"} · {plan.candidates.length} 个候选改动
          </div>
        </div>
        <a
          href={packStudioHandoffHref(plan.pack, plan.goal, "#import-plan")}
          className="inline-flex shrink-0 items-center gap-1 rounded-full px-2.5 py-1.5 text-[11px] font-medium"
          style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent)" }}
        >
          导入 Plan <ArrowRight size={11} />
        </a>
      </div>
      <div className="mt-2 break-all font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
        工作区：{plan.workspace.id || plan.workspace.path}
      </div>
      <div className="mt-2 grid gap-1.5">
        {plan.candidates.slice(0, 3).map((candidate) => (
          <div key={candidate.key || candidate.filePath} className="rounded-lg px-2 py-1.5" style={{ background: "var(--yunque-bg-muted)" }}>
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="font-medium">{candidate.label || "候选改动"}</span>
              {candidate.riskLevel && <Chip size="sm" variant="soft">风险：{candidate.riskLevel}</Chip>}
              {candidate.contentSummary && <Chip size="sm" variant="soft">摘要：{candidate.contentSummary.hash}</Chip>}
            </div>
            <div className="mt-1 truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
              {candidate.filePath}
            </div>
          </div>
        ))}
      </div>
      <div className="mt-2 leading-5" style={{ color: "var(--yunque-text-muted)" }}>
        这张卡只展示结构化计划，不包含完整文件内容。请在 Studio 导入 Plan、载入草稿、预览 diff、运行审计、重新打包并复检 SHA 后再安装或回滚。
      </div>
      {renderPackGovernanceLinks(plan.pack, "查看能力包详情", "回中心定位")}
    </div>
  );
}

function renderPackStudioDraft(draft: PackStudioPatchDraft) {
  return (
    <div
      className="mb-3 rounded-xl border p-3 text-xs"
      style={{
        background: "rgba(16,185,129,0.08)",
        borderColor: "rgba(16,185,129,0.22)",
        color: "var(--yunque-text)",
      }}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2 font-semibold">
            <Wand2 size={14} style={{ color: "var(--yunque-success)" }} />
            <span>Pack Studio Patch Draft</span>
          </div>
          <div className="mt-1 truncate" style={{ color: "var(--yunque-text-muted)" }}>
            {draft.pack.name || draft.pack.id} · {draft.pack.version || "unknown"} · 单文件草稿
          </div>
        </div>
        <a
          href={packStudioHandoffHref(draft.pack, draft.goal, "#import-draft")}
          className="inline-flex shrink-0 items-center gap-1 rounded-full px-2.5 py-1.5 text-[11px] font-medium"
          style={{ background: "var(--yunque-success-muted)", color: "var(--yunque-success)" }}
        >
          导入 Draft <ArrowRight size={11} />
        </a>
      </div>
      <div className="mt-2 break-all font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
        文件：{draft.filePath}
      </div>
      <div className="mt-2 flex flex-wrap gap-1.5">
        {draft.riskLevel && <Chip size="sm" variant="soft">风险：{draft.riskLevel}</Chip>}
        <Chip size="sm" variant="soft">{draft.content.length.toLocaleString()} chars</Chip>
        {draft.gates.map((gate) => (
          <Chip key={`draft:${gate}`} size="sm" variant="soft">{gate}</Chip>
        ))}
      </div>
      {draft.reason && (
        <div className="mt-2 leading-5" style={{ color: "var(--yunque-text-muted)" }}>
          原因：{draft.reason}
        </div>
      )}
      <div className="mt-2 leading-5" style={{ color: "var(--yunque-text-muted)" }}>
        完整文件内容已隐藏。请把这条消息粘贴到 Studio 的 Patch Draft 导入区，确认工作区匹配后预览 diff、运行审计，再应用和重新打包。
      </div>
      {renderPackGovernanceLinks(draft.pack, "查看能力包详情", "回中心定位")}
    </div>
  );
}

function renderPackStudioBatchDraftRequest(request: PackStudioBatchDraftRequest) {
  return (
    <div
      className="mb-3 rounded-xl border p-3 text-xs"
      style={{
        background: "rgba(14,165,233,0.08)",
        borderColor: "rgba(14,165,233,0.22)",
        color: "var(--yunque-text)",
      }}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2 font-semibold">
            <Sparkles size={14} style={{ color: "var(--yunque-accent)" }} />
            <span>Pack Studio 批量补肉任务</span>
          </div>
          <div className="mt-1 truncate" style={{ color: "var(--yunque-text-muted)" }}>
            {request.batch?.total
              ? `第 ${request.batch.page || 1} / ${request.batch.pageCount || 1} 批 · 本批 ${request.packs.length} 个 · 总计 ${request.batch.total} 个待补肉`
              : `${request.packs.length} 个能力包 · 小羽逐包生成 Draft Request`}
          </div>
        </div>
        <a
          href={packStudioBatchHandoffHref(request)}
          className="inline-flex shrink-0 items-center gap-1 rounded-full px-2.5 py-1.5 text-[11px] font-medium"
          style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent)" }}
        >
          导入 Studio 逐包处理 <ArrowRight size={11} />
        </a>
      </div>
      {request.goal && (
        <div className="mt-2 leading-5" style={{ color: "var(--yunque-text-muted)" }}>
          目标：{request.goal}
        </div>
      )}
      <div className="mt-2 grid gap-1.5">
        {request.packs.slice(0, 3).map((pack) => (
          <div key={pack.id || pack.name} className="rounded-lg px-2 py-1.5" style={{ background: "var(--yunque-bg-muted)" }}>
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="min-w-0">
                <div className="truncate font-medium">{pack.name || pack.id}</div>
                <div className="mt-0.5 truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                  {pack.id} · {pack.version || "unknown"} · {pack.source || "来源未标注"}
                </div>
              </div>
              {pack.studioUrl && (
                <a
                  href={pack.studioUrl}
                  className="inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-1 text-[11px] font-medium"
                  style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent)" }}
                >
                  打开 Studio <ArrowRight size={10} />
                </a>
              )}
            </div>
            {renderPackGovernanceLinks(pack, "详情", "中心")}
            <div className="mt-1.5 flex flex-wrap gap-1.5">
              {pack.readiness && <Chip size="sm" variant="soft">{pack.readiness}</Chip>}
              {pack.status && <Chip size="sm" variant="soft">稳定性：{pack.status}</Chip>}
              {pack.missing.slice(0, 4).map((gap) => (
                <Chip key={`${pack.id}:${gap}`} size="sm" variant="soft">{gap}</Chip>
              ))}
              {pack.missing.length > 4 && <Chip size="sm" variant="soft">+{pack.missing.length - 4}</Chip>}
            </div>
          </div>
        ))}
      </div>
      {request.packs.length > 3 && (
        <div className="mt-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
          还有 {request.packs.length - 3} 个能力包在批量请求里，复制后交给小羽逐包处理。
        </div>
      )}
      {request.rules.length > 0 && (
        <div className="mt-2 leading-5" style={{ color: "var(--yunque-text-muted)" }}>
          规则：{request.rules.slice(0, 3).join("；")}
        </div>
      )}
      <div className="mt-2 leading-5" style={{ color: "var(--yunque-text-muted)" }}>
        这只是批量生成请求，不会自动应用改动。每个包都要回到 Studio 预览 diff、运行 audit、重新打包并复检 SHA 后再安装或回滚。
      </div>
      <div className="mt-2">
        <a href="/packs#readiness-queue" className="inline-flex items-center gap-1 text-[11px] font-medium" style={{ color: "var(--yunque-accent)" }}>
          返回能力包中心队列 <ArrowRight size={10} />
        </a>
      </div>
    </div>
  );
}

function renderPackStudioDraftRequest(request: PackStudioPatchDraftRequest) {
  return (
    <div
      className="mb-3 rounded-xl border p-3 text-xs"
      style={{
        background: "rgba(168,85,247,0.08)",
        borderColor: "rgba(168,85,247,0.22)",
        color: "var(--yunque-text)",
      }}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2 font-semibold">
            <Sparkles size={14} style={{ color: "var(--yunque-accent)" }} />
            <span>Pack Studio Draft 请求</span>
          </div>
          <div className="mt-1 truncate" style={{ color: "var(--yunque-text-muted)" }}>
            {request.pack.name || request.pack.id} · {request.pack.version || "unknown"} · 让小羽生成单文件 Draft
          </div>
        </div>
        <a
          href={packStudioHandoffHref(request.pack, request.goal, "#draft-queue")}
          className="inline-flex shrink-0 items-center gap-1 rounded-full px-2.5 py-1.5 text-[11px] font-medium"
          style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent)" }}
        >
          查看草稿队列 <ArrowRight size={11} />
        </a>
      </div>
      <div className="mt-2 break-all font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
        目标文件：{request.target.filePath}
      </div>
      <div className="mt-2 flex flex-wrap gap-1.5">
        {request.target.riskLevel && <Chip size="sm" variant="soft">风险：{request.target.riskLevel}</Chip>}
        <Chip size="sm" variant="soft">starter {request.starterContentLength.toLocaleString()} chars</Chip>
        {request.target.contentSummary && <Chip size="sm" variant="soft">摘要：{request.target.contentSummary.hash}</Chip>}
        {request.target.gates.map((gate) => (
          <Chip key={`draft-request:${gate}`} size="sm" variant="soft">{gate}</Chip>
        ))}
      </div>
      {request.target.reason && (
        <div className="mt-2 leading-5" style={{ color: "var(--yunque-text-muted)" }}>
          原因：{request.target.reason}
        </div>
      )}
      <div className="mt-2 leading-5" style={{ color: "var(--yunque-text-muted)" }}>
        这是一条生成请求，不是已应用改动。小羽应只返回 {request.expectedKind || "yunque.pack_studio.patch_draft.v1"}，用户再回到 Studio 导入、预览 diff、审计、重新打包。
      </div>
      {renderPackGovernanceLinks(request.pack, "查看能力包详情", "回中心定位")}
    </div>
  );
}

function structuredPackStudioMessage(content: string) {
  const batchRequest = parsePackStudioBatchDraftRequestPrompt(content);
  if (batchRequest) {
    return {
      card: renderPackStudioBatchDraftRequest(batchRequest),
      text: batchRequest.displayText,
    };
  }
  const request = parsePackStudioPatchDraftRequestPrompt(content);
  if (request) {
    return {
      card: renderPackStudioDraftRequest(request),
      text: request.displayText,
    };
  }
  const draft = parsePackStudioPatchDraftPrompt(content);
  if (draft) {
    return {
      card: renderPackStudioDraft(draft),
      text: draft.displayText,
    };
  }
  const plan = parsePackStudioPatchPlanPrompt(content);
  if (plan) {
    return {
      card: renderPackStudioPlan(plan),
      text: plan.displayText,
    };
  }
  return null;
}

export function ChatMessageList({
  messages, streaming, chatMode, currentModel,
  copiedIdx, ttsPlaying, bridgeState, resumePromptForBrowser,
  onCopy, onPlayTTS, onEdit, onRollback, onRetry,
  onAction, onSlashSelect, onSend, onBrowserRefresh, onBrowserContinue, onShare,
}: ChatMessageListProps) {
  const { t } = useI18n();
  const isBubble = chatMode === "chat";
  const [shareOpenKey, setShareOpenKey] = useState<string | null>(null);
  const [shareChannels, setShareChannels] = useState<NotifyChannel[]>([]);
  const [shareChannelsLoaded, setShareChannelsLoaded] = useState(false);
  const [shareChannelsLoading, setShareChannelsLoading] = useState(false);
  const [sharePendingKey, setSharePendingKey] = useState<string | null>(null);
  const [previewOpenKey, setPreviewOpenKey] = useState<string | null>(null);
  const [previewData, setPreviewData] = useState<Record<string, FilePreviewResponse>>({});
  const [previewLoadingKey, setPreviewLoadingKey] = useState<string | null>(null);
  const [previewError, setPreviewError] = useState<Record<string, string>>({});
  const enabledShareChannels = useMemo(() => shareChannels.filter((ch) => ch.enabled), [shareChannels]);

  async function ensureShareChannels() {
    if (shareChannelsLoaded || shareChannelsLoading) return;
    setShareChannelsLoading(true);
    try {
      const data = await api.notifyChannels();
      setShareChannels(data.channels || []);
      setShareChannelsLoaded(true);
    } catch {
      setShareChannels([]);
      setShareChannelsLoaded(true);
    } finally {
      setShareChannelsLoading(false);
    }
  }

  function shareMessagePayload(msg: Message): ChatSharePayload {
    const files = collectGeneratedFiles(msg.traceEvents);
    const content = displayMessageContentForTools(msg).trim();
    return {
      title: files.length > 0 ? `云雀任务结果 · ${files.length} 个产物` : "云雀任务结果",
      message: content.length > 6000 ? `${content.slice(0, 6000)}\n\n...已截断` : content,
      files,
    };
  }

  function shareFilePayload(msg: Message, file: { path: string; name: string; size?: number }): ChatSharePayload {
    const name = file.name || file.path.split("/").pop() || file.path;
    const summary = displayMessageContentForTools(msg).trim();
    return {
      title: `云雀产物：${name}`,
      message: summary ? `云雀生成了产物文件：${name}\n\n${summary.slice(0, 1200)}${summary.length > 1200 ? "\n\n...已截断" : ""}` : `云雀生成了产物文件：${name}`,
      files: [file],
    };
  }

  async function ensureFilePreview(key: string, path: string) {
    if (previewData[key] || previewLoadingKey === key) return;
    setPreviewLoadingKey(key);
    setPreviewError((prev) => ({ ...prev, [key]: "" }));
    try {
      const preview = await api.previewFile(path);
      setPreviewData((prev) => ({ ...prev, [key]: preview }));
    } catch (e) {
      setPreviewError((prev) => ({ ...prev, [key]: formatErrorMessage(e, "预览失败") }));
    } finally {
      setPreviewLoadingKey(null);
    }
  }

  function continueFilePrompt(file: { path: string; name: string }) {
    const name = file.name || file.path.split("/").pop() || file.path;
    return `请继续处理这个产物文件：${name}\n路径：${file.path}\n\n请先分析文件内容，然后给出可执行的下一步编辑、优化或转换方案。`;
  }

  function dispatchToAIIDEPrompt(msg: Message, file?: { path: string; name: string; size?: number }) {
    const files = file ? [file] : collectGeneratedFiles(msg.traceEvents);
    const content = displayMessageContentForTools(msg).trim();
    const context = content.length > 4000 ? `${content.slice(0, 4000)}\n\n...已截断` : content;
    const fileLines = files.length > 0
      ? files.map((f) => `- ${f.name || f.path}: ${f.path}${f.size ? ` (${f.size} bytes)` : ""}`).join("\n")
      : "无";
    return [
      "请把下面需求派给 AI IDE 执行（Cursor / Claude Code / Windsurf 均可）。",
      "请优先使用 orchestrate_task 创建外部执行任务，让 AI IDE 领取并真实修改/验证；过程中需要我确认时回到 Chat/IM 询问。",
      "",
      "任务来源：当前 Chat 消息",
      "",
      "关联文件：",
      fileLines,
      "",
      "需求/上下文：",
      context || "请基于当前对话上下文继续完成这个开发任务。",
      "",
      "验收要求：",
      "1. 说明改了哪些文件。",
      "2. 运行必要的测试或类型检查。",
      "3. 产物进入 Workspace 后可预览/下载。",
    ].join("\n");
  }

  function renderFilePreview(key: string, file: { path: string; name: string; size?: number }) {
    const preview = previewData[key];
    const error = previewError[key];
    const loading = previewLoadingKey === key;
    return (
      <Popover
        isOpen={previewOpenKey === key}
        onOpenChange={(open) => {
          setPreviewOpenKey(open ? key : null);
          if (open) void ensureFilePreview(key, file.path);
        }}
      >
        <Popover.Trigger>
          <Button isIconOnly variant="ghost" size="sm">
            <Eye size={11} />
          </Button>
        </Popover.Trigger>
        <Popover.Content placement="bottom end" offset={6}>
          <Popover.Dialog className="w-[420px] max-w-[calc(100vw-32px)] p-3" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)", borderRadius: 16 }}>
            <div className="mb-2 flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="truncate text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{file.name || file.path}</div>
                <div className="mt-1 flex flex-wrap items-center gap-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                  <span>{preview ? `${preview.kind} · ${preview.ext.toUpperCase() || "FILE"}` : "产物预览"}</span>
                  {preview?.parse?.status && preview.parse.status !== "parsed" && (
                    <span className="rounded-full px-1.5 py-0.5" style={{ background: "rgba(251,191,36,0.1)", color: "#fbbf24" }}>
                      {filePreviewStatusLabel(preview.parse.status)}
                    </span>
                  )}
                </div>
              </div>
              <button
                type="button"
                onClick={() => {
                  setPreviewOpenKey(null);
                  onSend(continueFilePrompt(file));
                }}
                className="flex shrink-0 items-center gap-1 rounded-full px-2.5 py-1.5 text-[11px] font-medium"
                style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent)" }}
              >
                <Wand2 size={11} /> 继续处理
              </button>
            </div>
            {loading && (
              <div className="flex items-center gap-2 rounded-xl px-3 py-6 text-xs" style={{ color: "var(--yunque-text-muted)", background: "var(--yunque-bg-muted)" }}>
                <Spinner size="sm" /> 正在生成预览
              </div>
            )}
            {!loading && error && (
              <div className="rounded-xl px-3 py-4 text-xs leading-5" style={{ color: "#f87171", background: "rgba(248,113,113,0.08)", border: "1px solid rgba(248,113,113,0.18)" }}>
                {error}
              </div>
            )}
            {!loading && preview?.parse?.note && (
              <div className="mb-2 rounded-xl px-3 py-2 text-xs leading-5" style={{ color: "#fbbf24", background: "rgba(251,191,36,0.08)", border: "1px solid rgba(251,191,36,0.18)" }}>
                {filePreviewNoteLabel(preview.parse.note)}
              </div>
            )}
            {!loading && preview && (
              <div className="max-h-[360px] overflow-auto rounded-xl border p-3 text-xs leading-5" style={{ background: "var(--yunque-bg-muted)", borderColor: "var(--yunque-border)", color: "var(--yunque-text)" }}>
                <pre className="whitespace-pre-wrap break-words font-mono">{preview.preview || preview.parse?.preview || "暂无可预览内容"}</pre>
              </div>
            )}
            {preview?.truncated && (
              <div className="mt-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>预览已截断，可下载完整文件或让云雀继续处理。</div>
            )}
          </Popover.Dialog>
        </Popover.Content>
      </Popover>
    );
  }

  function renderShareMenu(key: string, msg: Message, payload: ChatSharePayload, compact = false) {
    const isOpen = shareOpenKey === key;
    return (
      <Popover
        isOpen={isOpen}
        onOpenChange={(open) => {
          setShareOpenKey(open ? key : null);
          if (open) void ensureShareChannels();
        }}
      >
        <Popover.Trigger>
          {compact ? (
            <Button isIconOnly variant="ghost" size="sm">
              <Share2 size={11} />
            </Button>
          ) : (
            <button
              type="button"
              className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[11px] font-medium transition-all hover:scale-[1.02]"
              style={{ background: "rgba(14,165,233,0.1)", border: "1px solid rgba(14,165,233,0.22)", color: "#7dd3fc" }}
            >
              <Share2 size={11} /> 协作同步
            </button>
          )}
        </Popover.Trigger>
        <Popover.Content placement="bottom end" offset={6}>
          <Popover.Dialog className="w-[280px] p-3" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)", borderRadius: 16 }}>
            <div className="mb-2">
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>同步到协作应用</div>
              <div className="mt-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>发送后会生成协作码，可在 IM 中用 `/yq 协作码 内容` 回流到同一会话。</div>
            </div>
            {shareChannelsLoading && (
              <div className="flex items-center gap-2 rounded-lg px-2 py-3 text-xs" style={{ color: "var(--yunque-text-muted)", background: "var(--yunque-bg-muted)" }}>
                <Spinner size="sm" /> 正在加载渠道
              </div>
            )}
            {!shareChannelsLoading && enabledShareChannels.length === 0 && (
              <div className="space-y-2">
                <div className="rounded-lg px-2 py-3 text-xs leading-5" style={{ color: "var(--yunque-text-muted)", background: "var(--yunque-bg-muted)" }}>
                  还没有可用的协作同步渠道。
                </div>
                <a href="/settings/notifications" className="flex items-center justify-center gap-1.5 rounded-lg px-3 py-2 text-xs font-medium" style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent)" }}>
                  <Settings size={12} /> 去配置通知渠道
                </a>
              </div>
            )}
            {!shareChannelsLoading && enabledShareChannels.length > 0 && (
              <div className="space-y-1.5">
                {enabledShareChannels.map((ch) => {
                  const pendingKey = `${key}:${ch.id}`;
                  return (
                    <button
                      key={ch.id}
                      type="button"
                      disabled={sharePendingKey === pendingKey}
                      className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs transition-all hover:scale-[1.01] disabled:opacity-60"
                      style={{ background: "var(--yunque-bg-muted)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                      onClick={async () => {
                        setSharePendingKey(pendingKey);
                        try {
                          await onShare(msg.id, ch, payload);
                          setShareOpenKey(null);
                        } finally {
                          setSharePendingKey(null);
                        }
                      }}
                    >
                      <span className="flex h-7 w-7 items-center justify-center rounded-lg" style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent)" }}>
                        {sharePendingKey === pendingKey ? <Spinner size="sm" /> : <Send size={13} />}
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-semibold">{ch.name}</span>
                        <span className="block truncate text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{ch.type}</span>
                      </span>
                    </button>
                  );
                })}
              </div>
            )}
          </Popover.Dialog>
        </Popover.Content>
      </Popover>
    );
  }

  function renderMoreActions(key: string, msg: Message) {
    return (
      <Popover>
        <Popover.Trigger>
          <Button isIconOnly variant="ghost" size="sm">
            <MoreHorizontal size={11} />
          </Button>
        </Popover.Trigger>
        <Popover.Content placement="bottom end" offset={6}>
          <Popover.Dialog className="w-[220px] p-2" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)", borderRadius: 14 }}>
            <div className="space-y-1">
              <button
                type="button"
                onClick={() => onPlayTTS(msg.id, displayMessageContentForTools(msg))}
                className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs"
                style={{ color: "var(--yunque-text)", background: "transparent" }}
              >
                {ttsPlaying === msg.id ? <VolumeX size={12} /> : <Volume2 size={12} />}
                {ttsPlaying === msg.id ? t("chat.stopTTS") : t("chat.playTTS")}
              </button>
              <button
                type="button"
                onClick={() => onRollback(msg.id)}
                className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs"
                style={{ color: "var(--yunque-text)", background: "transparent" }}
              >
                <Undo2 size={12} /> {t("chat.rollback")}
              </button>
              <button
                type="button"
                onClick={() => onSend(`/save_knowledge Save the above response to knowledge base.`)}
                className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs"
                style={{ color: "var(--yunque-text)", background: "transparent" }}
              >
                <BookOpen size={12} /> {t("chat.saveToKnowledge")}
              </button>
              <button
                type="button"
                onClick={() => onSend("把这次对话里值得长期记住的偏好或事实，整理后存进我的记忆。")}
                className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs"
                style={{ color: "var(--yunque-text)", background: "transparent" }}
              >
                <Brain size={12} /> {t("chat.saveToMemory")}
              </button>
              <button
                type="button"
                onClick={() => onSend(`/report Generate a structured report from the above conversation.`)}
                className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs"
                style={{ color: "var(--yunque-text)", background: "transparent" }}
              >
                <FileDown size={12} /> {t("chat.exportReport")}
              </button>
              {renderShareMenu(`more-share:${key}`, msg, shareMessagePayload(msg), false)}
            </div>
          </Popover.Dialog>
        </Popover.Content>
      </Popover>
    );
  }
  return (
    <div className="mx-auto space-y-5" style={{ maxWidth: "min(860px, 92vw)" }} aria-live="polite" aria-relevant="additions" role="log">
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
                  {msg.role === "assistant" ? (msg.model || currentModel || "Yunque Agent") : t("chat.user")}
                </span>
                {msg.timestamp && (
                  <span className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                    {new Date(msg.timestamp).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })}
                  </span>
                )}
              </div>
            )}
            {/* Task plan card (Qwen-style step list) */}
            {msg.role === "assistant" && msg.traceEvents && msg.traceEvents.length > 0 && (
              <TaskPlanCard
                events={msg.traceEvents}
                isLive={streaming && msg.id === messages[messages.length - 1]?.id}
              />
            )}
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
                    <img key={i} src={src} alt={`${t("chat.uploadedImage")} ${i + 1}`} className="max-w-[200px] max-h-[200px] rounded-lg object-cover cursor-pointer hover:opacity-90 transition-opacity" onClick={() => openExternal(src)} />
                  ))}
                </div>
              )}
              {msg.role === "user" && msg.files && msg.files.length > 0 && (
                <div className="mb-2 flex flex-wrap gap-2">
                  {msg.files.map((file, i) => (
                    <span
                      key={`${file.path}:${i}`}
                      className="inline-flex max-w-[260px] items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px]"
                      style={{
                        background: "rgba(255,255,255,0.16)",
                        border: "1px solid rgba(255,255,255,0.18)",
                        color: msg.role === "user" ? "var(--neutral-strong-fg)" : "var(--yunque-text-secondary)",
                      }}
                    >
                      <Paperclip size={11} />
                      <span className="truncate">{file.name || file.path}</span>
                      {typeof file.size === "number" && file.size > 0 && (
                        <span className="opacity-70">
                          {file.size > 1024 * 1024 ? `${(file.size / 1024 / 1024).toFixed(1)} MB` : `${(file.size / 1024).toFixed(1)} KB`}
                        </span>
                      )}
                    </span>
                  ))}
                </div>
              )}
              {msg.role === "assistant" && msg.reasoning && (
                <details className="mb-2" open={false} style={{ fontSize: "var(--text-sm)" }}>
                  <summary style={{ cursor: "pointer", color: "var(--yunque-text-muted)", fontStyle: "italic", display: "flex", alignItems: "center", gap: 4 }}>
                    <span style={{ fontSize: "var(--text-xs)", background: "rgba(245,158,11,0.12)", color: "#f59e0b", padding: "1px 6px", borderRadius: 4 }}>
                      {streaming && idx === messages.length - 1 ? t("chat.reasoning") : t("chat.reasoned")}
                    </span>
                    <ThinkingTimer startMs={msg.reasoningStartMs} endMs={msg.reasoningEndMs} isStreaming={streaming && idx === messages.length - 1} />
                  </summary>
                  <div style={{ marginTop: 6, padding: "8px 12px", borderRadius: 8, background: "rgba(245,158,11,0.04)", border: "1px solid rgba(245,158,11,0.12)", whiteSpace: "pre-wrap", color: "var(--yunque-text-secondary)", fontSize: "var(--text-xs)", maxHeight: 300, overflow: "auto" }}>
                    {msg.reasoning}
                  </div>
                </details>
              )}
              {msg.content ? (
                msg.role === "assistant" ? (() => {
                  const structured = structuredPackStudioMessage(msg.content);
                  if (!structured) return <MarkdownRenderer content={displayMessageContent(msg)} />;
                  return (
                    <>
                      {structured.card}
                      {structured.text && <MarkdownRenderer content={displayChatText(structured.text)} />}
                    </>
                  );
                })() : (() => {
                  const structured = structuredPackStudioMessage(msg.content);
                  const userContent = structured
                    ? structured.text
                    : msg.content.replace(/\[(Uploaded file|File):\s*[^\]]+\]\s*/g, "").trim();
                  return (
                    <>
                      {structured?.card}
                      {userContent || (structured || msg.images?.length ? null : msg.content)}
                    </>
                  );
                })()
              ) : (
                !msg.images?.length && (
                  <div className="flex items-center gap-1.5">
                    <Spinner size="sm" color="current" /> {t("chat.thinking")}
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
            {/* Context layers — only surface concrete signals (memory / knowledge
                / emotion). The generic "strategy / 运用了积累的经验" badge was
                noise: it appeared on essentially every reply, so it's dropped. */}
            {msg.role === "assistant" && (() => {
              const layers = msg.contextLayers || [];
              const hasMemory = layers.includes("memory");
              const hasKnowledge = layers.includes("graph") || layers.includes("code");
              const hasEmotion = layers.includes("emotion");
              if (!hasMemory && !hasKnowledge && !hasEmotion) return null;
              return (
                <div className="mt-2 flex flex-wrap items-center gap-2">
                  {hasMemory && (
                    <span className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(139,92,246,0.12)", color: "#a78bfa" }}>
                      <Brain size={11} /> 调用了你的记忆
                    </span>
                  )}
                  {hasKnowledge && (
                    <span className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(6,182,212,0.12)", color: "#22d3ee" }}>
                      <Library size={11} /> 参考了知识库
                    </span>
                  )}
                  {hasEmotion && (
                    <span className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(236,72,153,0.12)", color: "#f472b6" }}>
                      <Heart size={11} /> 感知了你的情绪
                    </span>
                  )}
                </div>
              );
            })()}
            {/* Cognitive status bar */}
            {msg.role === "assistant" && (msg.cognitiveMemories?.length || msg.cognitiveReflections?.length || msg.cognitiveContextLayers?.length || msg.skills_used?.length) && (
              <CognitiveStatusBar
                memories={msg.cognitiveMemories}
                reflections={msg.cognitiveReflections}
                contextLayers={msg.cognitiveContextLayers}
                activeSkills={msg.skills_used}
                isLive={streaming && msg.id === messages[messages.length - 1]?.id}
              />
            )}
            {/* Actions */}
            {msg.role === "assistant" && msg.actions && msg.actions.length > 0 && (
              <div className="chat-inline-panel mt-2 rounded-xl border p-2" style={{ background: "var(--yunque-bg-muted)", borderColor: "var(--yunque-border)" }}>
                <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>{t("chat.suggestedActions")}</div>
                <AgentActions actions={msg.actions} onAction={onAction} />
              </div>
            )}
            {/* Browser summary */}
            {msg.role === "assistant" && msg.browserSummary && (
              <div className="chat-inline-panel mt-2 rounded-xl border p-2" style={{ background: "var(--yunque-bg-muted)", borderColor: "var(--yunque-border)" }}>
                <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>{t("chat.browserArtifact")}</div>
                <div className="flex flex-wrap items-center gap-2 text-[11px]" style={{ color: "var(--yunque-text-secondary)" }}>
                  {msg.browserSummary.action && <span className="rounded-full px-2.5 py-1" style={{ background: "rgba(59,130,246,0.12)", color: "#93c5fd" }}>{browserActionLabel(msg.browserSummary.action)}</span>}
                  {typeof msg.browserSummary.elementCount === "number" && <span className="rounded-full px-2.5 py-1" style={{ background: "var(--yunque-bg-muted)", color: "var(--yunque-text-muted)" }}>{msg.browserSummary.elementCount} {t("chat.elements")}</span>}
                  {msg.browserSummary.hasScreenshot && <span className="rounded-full px-2.5 py-1" style={{ background: "var(--yunque-success-muted)", color: "var(--yunque-success)" }}>{t("chat.screenshotReady")}</span>}
                  {typeof msg.browserSummary.textLength === "number" && msg.browserSummary.textLength > 0 && <span className="rounded-full px-2.5 py-1" style={{ background: "var(--yunque-bg-muted)", color: "var(--yunque-text-muted)" }}>{msg.browserSummary.textLength} {t("chat.chars")}</span>}
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
                    {msg.browserSummary.suggestedCommand && <Button size="sm" variant="ghost" className="rounded-full px-3" onPress={() => onSlashSelect(msg.browserSummary?.suggestedCommand || "/")}>{msg.browserSummary.suggestedLabel || t("chat.useNextCommand")}</Button>}
                    {msg.browserSummary.url && <Button size="sm" variant="ghost" className="rounded-full px-3" onPress={() => openExternal(msg.browserSummary?.url)}>{t("chat.openPage")}</Button>}
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
                    <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>云电脑</div>
                    <div className="text-[11px] font-mono" style={{ color: "var(--yunque-text-muted)" }}>{msg.sandbox.sandbox_id}</div>
                  </div>
                  <Chip size="sm" style={{ marginLeft: "auto", background: "rgba(34,197,94,0.12)", color: "#22c55e", fontSize: "10px" }}>LIVE</Chip>
                </div>
                {msg.sandbox.stream_url && (
                  <Button size="sm" className="w-full mt-1" onPress={() => openExternal(msg.sandbox?.stream_url)} style={{ background: "rgba(34,197,94,0.15)", color: "#22c55e", border: "1px solid rgba(34,197,94,0.25)" }}>
                    <Monitor size={14} className="mr-2" /> 打开云电脑
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
                  <div className="mb-1 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>{t("chat.generatedFiles")}</div>
                  <div className="mb-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{t("chat.generatedFilesHint")}</div>
                  <div className="space-y-2">
                    {files.map((f, i) => {
                      const ext = (f.name || f.path).split(".").pop()?.toLowerCase() || "";
                      const isDoc = ["pdf", "docx", "xlsx", "pptx", "doc", "xls", "ppt"].includes(ext);
                      return (
                        <div key={i}
                          className="flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-medium transition-all hover:scale-[1.01]"
                          style={{ background: isDoc ? "var(--yunque-accent-muted)" : "var(--yunque-bg-muted)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}>
                          <div className="w-10 h-10 rounded-lg flex items-center justify-center shrink-0" style={{ background: isDoc ? "var(--yunque-accent-muted)" : "var(--yunque-bg-muted)" }}><Paperclip size={18} /></div>
                          <div className="flex-1 min-w-0">
                            <div className="truncate font-semibold">{f.name || f.path.split("/").pop() || f.path}</div>
                            <div className="text-[11px] mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>
                              {ext.toUpperCase()} {f.size != null && f.size > 0 ? `  ${f.size > 1024 * 1024 ? `${(f.size / 1024 / 1024).toFixed(1)} MB` : `${(f.size / 1024).toFixed(1)} KB`}` : ""}
                            </div>
                          </div>
                          <div className="flex items-center gap-1.5 shrink-0">
                            {renderFilePreview(`preview:${msg.id}:${i}`, f)}
                            <Button isIconOnly variant="ghost" size="sm" onPress={() => onSend(continueFilePrompt(f))}>
                              <Wand2 size={11} />
                            </Button>
                            <Button isIconOnly variant="ghost" size="sm" onPress={() => onSend(dispatchToAIIDEPrompt(msg, f))}>
                              <Cpu size={11} />
                            </Button>
                            {renderShareMenu(`file:${msg.id}:${i}`, msg, shareFilePayload(msg, f), true)}
                            <a
                              href={`/api/files/download?path=${encodeURIComponent(f.path)}`}
                              download={f.name || f.path}
                              className="w-8 h-8 rounded-full flex items-center justify-center"
                              style={{ background: "var(--yunque-accent-muted)" }}
                            >
                              <span style={{ color: "var(--yunque-accent)", fontSize: 16 }}>↗</span>
                            </a>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              );
            })()}
            {msg.role === "assistant" && msg.shareReceipts && msg.shareReceipts.length > 0 && (
              <div className="mt-2 rounded-xl border px-3 py-2" style={{ background: "rgba(14,165,233,0.06)", borderColor: "rgba(14,165,233,0.18)" }}>
                <div className="mb-1 flex items-center gap-1.5 text-[11px] font-semibold" style={{ color: "#7dd3fc" }}>
                  <Share2 size={12} /> 协作同步记录
                </div>
                <div className="space-y-1">
                  {msg.shareReceipts.slice(-3).map((receipt) => (
                    <div key={receipt.id} className="flex items-center gap-2 text-[11px]" style={{ color: receipt.status === "sent" ? "var(--yunque-text-muted)" : "#f87171" }}>
                      <span className="h-1.5 w-1.5 rounded-full" style={{ background: receipt.status === "sent" ? "#22c55e" : "#f87171" }} />
                      <span className="min-w-0 flex-1 truncate">
                        {receipt.status === "sent" ? "已发送到" : "发送失败"} {receipt.channelName}
                        {receipt.shareCode ? ` · ${receipt.shareCode}` : ""}
                        {receipt.error ? `：${formatErrorMessage(receipt.error, "同步失败")}` : ""}
                      </span>
                      <span className="shrink-0 font-mono opacity-70">
                        {new Date(receipt.sentAt).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}
            {/* Suggestions */}
            {msg.role === "assistant" && msg.suggestions && msg.suggestions.length > 0 && !streaming && (
              <details className="mt-3">
                <summary className="cursor-pointer text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--yunque-text-muted)" }}>{t("chat.nextMoves")}</summary>
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
                onOpenSetup={() => window.open(msg.browserRequirement?.install_path || "/packs/browser", "_blank", "noopener,noreferrer")}
                onRefresh={onBrowserRefresh}
                onContinue={bridgeState?.connected ? () => {
                  const prevPrompt = messages[idx - 1]?.role === "user" ? messages[idx - 1]?.content : resumePromptForBrowser;
                  if (prevPrompt) onBrowserContinue(prevPrompt);
                } : undefined}
                continueLabel={t("chat.continueBlocked")}
              />
            )}
            {/* Skill growth */}
            {msg.role === "assistant" && msg.skillSuggestions && msg.skillSuggestions.length > 0 && (
              <div className="mt-2 rounded-xl px-3 py-2.5" style={{ background: "rgba(34,197,94,0.06)", border: "1px solid rgba(34,197,94,0.15)" }}>
                <div className="flex items-center gap-2 mb-2">
                  <Sparkles size={13} style={{ color: "#4ade80" }} />
                  <span className="text-[11px] font-semibold" style={{ color: "#4ade80" }}>{t("chat.skillLearned")}</span>
                </div>
                <SkillGrowthPanel suggestions={msg.skillSuggestions} onSave={(s) => onSend(`Turn this into a reusable skill.\n\nName: ${s.name}\nDescription: ${s.description}\nTrigger: ${s.trigger}`)} />
              </div>
            )}
            {/* Execution trace */}
            {msg.role === "assistant" && msg.traceEvents && msg.traceEvents.length > 0 && (
              <details className="mt-3">
                <summary className="cursor-pointer text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{t("chat.executionTrace")}</summary>
                <div className="mt-2"><ExecutionTrace events={msg.traceEvents} isLive={streaming && idx === messages.length - 1} onRecoveryPrompt={onSend} /></div>
              </details>
            )}
            {/* Quick actions card for substantial responses */}
            {msg.role === "assistant" && msg.content && msg.content.length > 400 && !streaming && (
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <button
                  onClick={() => onSend(dispatchToAIIDEPrompt(msg))}
                  className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[11px] font-medium transition-all hover:scale-[1.02]"
                  style={{ background: "rgba(34,197,94,0.08)", border: "1px solid rgba(34,197,94,0.18)", color: "#86efac" }}
                >
                  <Cpu size={11} /> 派给 AI IDE
                </button>
                {renderShareMenu(`quick:${msg.id}`, msg, shareMessagePayload(msg))}
              </div>
            )}
            {/* Message tools */}
            {msg.content && (
              <div className={`chat-message-tools flex gap-0.5 mt-1 ${!isBubble ? "justify-end" : ""}`} style={isBubble ? { justifyContent: msg.role === "user" ? "flex-end" : "flex-start" } : undefined}>
                {msg.role === "user" && (
                  <>
                    <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={() => onEdit(msg.id)}><Pencil size={11} /></Button><Tooltip.Content>{t("chat.edit")}</Tooltip.Content></Tooltip>
                    <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={() => onSend(dispatchToAIIDEPrompt(msg))}><Cpu size={11} /></Button><Tooltip.Content>派给 AI IDE</Tooltip.Content></Tooltip>
                  </>
                )}
                {msg.role === "assistant" && (
                  <>
                    <Tooltip delay={0}>
                      <Button isIconOnly variant="ghost" size="sm" onPress={() => onCopy(msg.id, displayMessageContentForTools(msg))}>
                        {copiedIdx === msg.id ? <Check size={11} className="text-green-400" /> : <Copy size={11} />}
                      </Button>
                      <Tooltip.Content>{copiedIdx === msg.id ? t("chat.copied") : t("chat.copy")}</Tooltip.Content>
                    </Tooltip>
                    <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={() => onSend(dispatchToAIIDEPrompt(msg))}><Cpu size={11} /></Button><Tooltip.Content>派给 AI IDE</Tooltip.Content></Tooltip>
                    {renderMoreActions(`tools:${msg.id}`, msg)}
                  </>
                )}
                <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={() => onRetry(msg.id)}><RotateCcw size={11} /></Button><Tooltip.Content>{t("chat.retry")}</Tooltip.Content></Tooltip>
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
