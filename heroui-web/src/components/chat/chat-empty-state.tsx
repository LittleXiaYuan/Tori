import {
  BookOpen, Search, Brain, Zap, Package, Sparkles,
  AlertTriangle, ArrowRight, Blocks,
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
      <div className="w-12 h-12 rounded-2xl flex items-center justify-center chat-hero-icon" style={{ background: "rgba(0,111,238,0.1)" }}>
        <Sparkles size={24} style={{ color: "var(--yunque-accent)" }} />
      </div>
      <div className="max-w-lg text-center space-y-1.5">
        <h1 className="text-[28px] font-bold tracking-tight" style={{ color: "var(--yunque-text)" }}>有什么可以帮你的？</h1>
        <p className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>研究、写作、编码、分析 —— 描述你的需求，我来帮你完成。</p>
      </div>

      <div className="mt-1 grid w-full max-w-[520px] grid-cols-2 gap-2">
        {(() => {
          const fixedCards = [
            { icon: <BookOpen size={14} />, label: "帮我总结这份文档", desc: "贴入文档或笔记，提炼要点与行动项。", prompt: "" },
            { icon: <Search size={14} />, label: "/research 最新的AI Agent技术趋势", desc: "自动浏览、提取、对比，生成结构化报告。", displayLabel: "深度研究一个主题", autoSend: true },
          ];
          const fallbackCards = [
            { icon: <Brain size={14} />, label: "帮我拆解这个任务", desc: "把复杂需求拆解成可执行的步骤。", prompt: "" },
            { icon: <Zap size={14} />, label: "写一个Python脚本，监控指定文件夹的变化并发送通知", desc: "点击直接运行，查看效果。", displayLabel: "帮我写段代码", autoSend: true },
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
                const text = ("prompt" in card && card.prompt) || card.label;
                if ("autoSend" in card && card.autoSend && onSend) {
                  onSend(text);
                } else {
                  chatD({ type: "SET_INPUT", value: text });
                  inputRef.current?.focus();
                }
              }}
              className="flex items-start gap-2.5 rounded-[16px] p-2.5 text-left transition-all duration-200 hover-lift"
              style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}
            >
              <span className="mt-0.5 shrink-0" style={{ color: "var(--yunque-accent)" }}>{card.icon}</span>
              <div className="min-w-0">
                <div className="text-[13px] font-medium" style={{ color: "var(--yunque-text)" }}>{"displayLabel" in card ? (card as any).displayLabel : card.label}</div>
                <div className="mt-0.5 text-[10px] leading-5" style={{ color: "var(--yunque-text-muted)" }}>{card.desc}</div>
              </div>
            </button>
          ));
        })()}
      </div>

      <div className="mt-2 flex w-full max-w-[520px] items-center justify-center gap-3">
        <a href="/skills" className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-colors hover:bg-white/5"
          style={{ color: "var(--yunque-text-secondary)", border: "1px solid var(--yunque-border)" }}>
          <Package size={12} /> 浏览技能库 <ArrowRight size={10} />
        </a>
        <a href="/workflows" className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-colors hover:bg-white/5"
          style={{ color: "var(--yunque-text-secondary)", border: "1px solid var(--yunque-border)" }}>
          <Blocks size={12} /> 浏览工作流 <ArrowRight size={10} />
        </a>
      </div>
    </div>
  );
}
