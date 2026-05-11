import {
  BookOpen, Search, Zap, Package, Sparkles,
  AlertTriangle, ArrowRight, Cpu, FolderOpen, MessageCircle,
} from "lucide-react";
import type { SkillInfo } from "@/lib/api";
import type { ChatDispatch } from "@/lib/chat-state";

interface ChatEmptyStateProps {
  setupNeeded: boolean;
  heroSkills: SkillInfo[];
  chatD: ChatDispatch;
  inputRef: React.RefObject<HTMLTextAreaElement | null>;
  onSend?: (text: string) => void;
}

export function ChatEmptyState({ setupNeeded, heroSkills, chatD, inputRef, onSend }: ChatEmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center h-full gap-6 animate-fade-in-up">
      {setupNeeded && (
        <div className="w-full max-w-md p-4 rounded-xl border-l-4" style={{ background: "rgba(245,158,11,0.06)", borderColor: "var(--yunque-border)", borderLeftColor: "#f59e0b" }}>
          <div className="flex items-center gap-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
            <AlertTriangle size={16} style={{ color: "#f59e0b" }} /> 先完成模型配置
          </div>
          <p className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
            请先在设置中添加模型提供商 API Key，再开始第一轮对话。
          </p>
          <a href="/settings/providers" className="inline-flex items-center gap-1 text-xs mt-2 font-medium" style={{ color: "#f59e0b" }}>前往配置提供商 →</a>
        </div>
      )}
      <div className="w-14 h-14 rounded-2xl flex items-center justify-center chat-hero-icon" style={{ background: "linear-gradient(135deg, rgba(59,130,246,0.15), rgba(168,85,247,0.10))" }}>
        <Sparkles size={28} style={{ color: "var(--yunque-accent)" }} />
      </div>
      <div className="max-w-xl text-center space-y-2">
        <h1 className="text-[28px] font-bold tracking-tight" style={{ color: "var(--yunque-text)" }}>你好，我是云雀</h1>
        <p className="text-sm leading-relaxed" style={{ color: "var(--yunque-text-muted)", maxWidth: 540, margin: "0 auto" }}>
          你的全能 AI 助手。可以聊天、研究、写代码、处理文件、调度 AI IDE；复杂能力会被收进清晰的工作路径里。
        </p>
      </div>

      <div className="grid w-full max-w-[620px] grid-cols-3 gap-2">
        {[
          { icon: <MessageCircle size={13} />, title: "直接对话", desc: "先说需求，不用先理解工具。" },
          { icon: <Cpu size={13} />, title: "AI IDE 执行", desc: "复杂代码任务可交给外部 IDE。" },
          { icon: <FolderOpen size={13} />, title: "Workspace 验收", desc: "文件产物可预览、下载、继续改。" },
        ].map((item) => (
          <div
            key={item.title}
            className="rounded-2xl px-3 py-2.5 text-left"
            style={{ background: "var(--yunque-bg-muted)", border: "1px solid var(--yunque-border)" }}
          >
            <div className="mb-1 flex items-center gap-1.5 text-[12px] font-semibold" style={{ color: "var(--yunque-text)" }}>
              <span style={{ color: "var(--yunque-accent)" }}>{item.icon}</span>
              {item.title}
            </div>
            <div className="text-[10px] leading-4" style={{ color: "var(--yunque-text-muted)" }}>{item.desc}</div>
          </div>
        ))}
      </div>

      <div className="mt-1 grid w-full max-w-[620px] grid-cols-2 gap-2.5">
        {(() => {
          const fixedCards = [
            {
              icon: <Cpu size={14} />,
              label: "请把这个需求派给 AI IDE 执行，过程中需要我确认时回到 Chat/IM 询问。",
              desc: "适合代码修改、重构、测试、修 Bug。",
              displayLabel: "写代码 / 修 Bug",
            },
            {
              icon: <Search size={14} />,
              label: "/research 最新的AI Agent技术趋势，并生成可分享的结构化报告",
              desc: "自动研究、提炼信息、沉淀为报告。",
              displayLabel: "研究一个主题",
              autoSend: true,
            },
          ];
          const fallbackCards = [
            { icon: <BookOpen size={14} />, label: "帮我总结这份文档，并输出待办事项和可交付结论", desc: "贴入文档或笔记，提炼要点与行动项。", displayLabel: "总结文档" },
            { icon: <Zap size={14} />, label: "帮我把这个复杂任务拆成可执行计划，并告诉我第一步怎么验收", desc: "从想法变成可执行任务列表。", displayLabel: "拆解任务" },
          ];
          const dynamicCards = heroSkills.slice(0, 2).map((sk) => ({
            icon: <Package size={14} />,
            label: sk.name,
            desc: sk.description || "已安装技能，点击直接使用",
          }));
          const cards = [...fixedCards, ...(dynamicCards.length >= 2 ? dynamicCards : fallbackCards)];
          return cards.map((card) => (
            <button
              key={card.label}
              onClick={() => {
                const text: string = ("prompt" in card && typeof card.prompt === "string" && card.prompt) ? card.prompt : card.label;
                if ("autoSend" in card && card.autoSend && onSend) {
                  onSend(text);
                } else {
                  chatD({ type: "SET_INPUT", value: text });
                  inputRef.current?.focus();
                }
              }}
              className="flex items-start gap-2.5 rounded-[16px] p-3 text-left transition-all duration-200 hover-lift group/card"
              style={{ background: "var(--glass-card, var(--yunque-card))", border: "1px solid var(--glass-edge, var(--yunque-border))" }}
            >
              <span className="mt-0.5 shrink-0 flex items-center justify-center w-7 h-7 rounded-lg" style={{ background: "var(--yunque-accent-soft)", color: "var(--yunque-accent)" }}>{card.icon}</span>
              <div className="min-w-0 flex-1">
                <div className="text-[13px] font-medium flex items-center gap-1" style={{ color: "var(--yunque-text)" }}>
                  {"displayLabel" in card ? (card as any).displayLabel : card.label}
                  <ArrowRight size={10} className="opacity-0 group-hover/card:opacity-60 transition-opacity" style={{ color: "var(--yunque-text-muted)" }} />
                </div>
                <div className="mt-0.5 text-[10px] leading-[1.5]" style={{ color: "var(--yunque-text-muted)" }}>{card.desc}</div>
              </div>
            </button>
          ));
        })()}
      </div>

      <div className="mt-3 flex w-full max-w-[620px] items-center justify-center gap-3">
        <a href="/knowledge" className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-colors hover:bg-[var(--yunque-bg-muted)]"
          style={{ color: "var(--yunque-text-secondary)", border: "1px solid var(--yunque-border)" }}>
          <BookOpen size={12} /> 导入知识库 <ArrowRight size={10} />
        </a>
        <a href="/workspace" className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-colors hover:bg-[var(--yunque-bg-muted)]"
          style={{ color: "var(--yunque-text-secondary)", border: "1px solid var(--yunque-border)" }}>
          <FolderOpen size={12} /> 打开 Workspace <ArrowRight size={10} />
        </a>
        <a href="/workers" className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-colors hover:bg-[var(--yunque-bg-muted)]"
          style={{ color: "var(--yunque-text-secondary)", border: "1px solid var(--yunque-border)" }}>
          <Cpu size={12} /> 连接 AI IDE <ArrowRight size={10} />
        </a>
      </div>
    </div>
  );
}
