import {
  BookOpen, Package, Sparkles,
  AlertTriangle, ArrowRight, Cpu, FolderOpen, MessageCircle,
} from "lucide-react";
import type { SkillInfo } from "@/lib/api";
import type { ChatDispatch } from "@/lib/chat-state";
import { CHAT_EMPTY_SCENARIOS } from "@/lib/product-scenarios";
import { buildWorkloadCatalogHref, WORKLOAD_PRESETS } from "@/lib/workload-presets";

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
          你的个人 AI 工作伙伴。先选一个场景，我会把目标推进到行动、产物、反馈和记忆。
        </p>
      </div>

      <div className="grid w-full max-w-[620px] grid-cols-3 gap-2">
        {[
          { icon: <MessageCircle size={13} />, title: "先说目标", desc: "不用先理解工具、模型或 Pack。" },
          { icon: <Cpu size={13} />, title: "按需执行", desc: "复杂任务再派给 AI IDE 或浏览器。" },
          { icon: <FolderOpen size={13} />, title: "验收产物", desc: "文件、结论和下一步都回到工作台。" },
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
          const fixedCards = CHAT_EMPTY_SCENARIOS.slice(0, 2).map((scenario) => ({
            icon: scenario.icon,
            prompt: scenario.prompt,
            desc: scenario.description,
            displayLabel: scenario.label,
            autoSend: true,
          }));
          const fallbackCards = CHAT_EMPTY_SCENARIOS.slice(2, 4).map((scenario) => ({
            icon: scenario.icon,
            prompt: scenario.prompt,
            desc: scenario.description,
            displayLabel: scenario.label,
            autoSend: true,
          }));
          const dynamicCards = heroSkills.slice(0, 2).map((sk) => ({
            icon: <Package size={14} />,
            prompt: sk.name,
            desc: sk.description || "已安装技能，点击直接使用",
            displayLabel: sk.name,
          }));
          const cards = [...fixedCards, ...(dynamicCards.length >= 2 ? dynamicCards : fallbackCards)];
          return cards.map((card) => (
            <button
              key={card.displayLabel}
              onClick={() => {
                const text: string = card.prompt;
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
                  {card.displayLabel}
                  <ArrowRight size={10} className="opacity-0 group-hover/card:opacity-60 transition-opacity" style={{ color: "var(--yunque-text-muted)" }} />
                </div>
                <div className="mt-0.5 text-[10px] leading-[1.5]" style={{ color: "var(--yunque-text-muted)" }}>{card.desc}</div>
              </div>
            </button>
          ));
        })()}
      </div>

      <div className="w-full max-w-[620px] space-y-2">
        <div className="flex items-center justify-between gap-2 px-1">
          <div className="text-[11px] font-semibold uppercase tracking-[0.16em]" style={{ color: "var(--yunque-text-muted)" }}>
            工作负载入口
          </div>
          <a href="/packs" className="text-[11px] font-medium" style={{ color: "var(--yunque-accent)" }}>
            去 Packs 先选能力 →
          </a>
        </div>
        <div className="grid grid-cols-1 gap-2 md:grid-cols-3">
          {WORKLOAD_PRESETS.slice(0, 3).map((preset) => (
            <a
              key={preset.id}
              href={buildWorkloadCatalogHref(preset)}
              className="flex items-start gap-2 rounded-[16px] p-3 text-left transition-all duration-200 hover-lift group/card"
              style={{ background: "var(--glass-card, var(--yunque-card))", border: "1px solid var(--glass-edge, var(--yunque-border))" }}
            >
              <span className="mt-0.5 shrink-0 flex items-center justify-center w-7 h-7 rounded-lg" style={{ background: "var(--yunque-accent-soft)", color: "var(--yunque-accent)" }}>
                <Package size={14} />
              </span>
              <div className="min-w-0 flex-1">
                <div className="text-[13px] font-medium flex items-center gap-1" style={{ color: "var(--yunque-text)" }}>
                  {preset.title}
                  <ArrowRight size={10} className="opacity-0 group-hover/card:opacity-60 transition-opacity" style={{ color: "var(--yunque-text-muted)" }} />
                </div>
                <div className="mt-0.5 text-[10px] leading-[1.5]" style={{ color: "var(--yunque-text-muted)" }}>{preset.subtitle}</div>
              </div>
            </a>
          ))}
        </div>
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
