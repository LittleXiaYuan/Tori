"use client";

import { useState, useEffect } from "react";
import { Button } from "@heroui/react";
import { MessageCircle, Zap, Search, Package, ArrowRight, Sparkles, X } from "lucide-react";

const ONBOARDING_KEY = "yunque_onboarding_done";

const steps = [
  {
    icon: <Sparkles size={28} style={{ color: "#60a5fa" }} />,
    title: "欢迎使用云雀 Agent",
    titleEn: "Welcome to Yunque Agent",
    desc: "云雀不只是聊天——它能浏览网页、执行任务、生成文档，帮你完成真正的工作。",
    descEn: "Yunque isn't just a chatbot — it can browse the web, execute tasks, generate documents, and get real work done.",
  },
  {
    icon: <MessageCircle size={28} style={{ color: "#22c55e" }} />,
    title: "直接说你要什么",
    titleEn: "Just say what you need",
    desc: "不需要学命令。直接用自然语言描述你的需求，Agent 会自动拆解并执行。",
    descEn: "No commands to learn. Describe what you need in natural language, and the Agent will break it down and execute automatically.",
  },
  {
    icon: <Search size={28} style={{ color: "#a78bfa" }} />,
    title: "研究 → 存储 → 复用",
    titleEn: "Research → Save → Reuse",
    desc: "让 Agent 调研任何主题，结果一键存入知识库。下次遇到类似问题，Agent 会自动参考。",
    descEn: "Let the Agent research any topic, save results to the knowledge base with one click. The Agent will reference it next time.",
  },
  {
    icon: <Zap size={28} style={{ color: "#fbbf24" }} />,
    title: "快捷键让你更快",
    titleEn: "Shortcuts make you faster",
    desc: "Alt+N 新对话 · Alt+I 聚焦输入 · Alt+P 截图分析 · Ctrl+K 全局搜索",
    descEn: "Alt+N new chat · Alt+I focus input · Alt+P screenshot · Ctrl+K global search",
  },
];

export function OnboardingGuide() {
  const [visible, setVisible] = useState(false);
  const [step, setStep] = useState(0);

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (!localStorage.getItem(ONBOARDING_KEY)) {
      const timer = setTimeout(() => setVisible(true), 800);
      return () => clearTimeout(timer);
    }
  }, []);

  const finish = () => {
    setVisible(false);
    localStorage.setItem(ONBOARDING_KEY, "1");
  };

  if (!visible) return null;

  const current = steps[step];
  const isLast = step === steps.length - 1;

  return (
    <div className="fixed inset-0 z-[200] flex items-center justify-center" style={{ background: "rgba(0,0,0,0.6)", backdropFilter: "blur(8px)" }}>
      <div
        className="relative w-full max-w-md rounded-3xl overflow-hidden animate-fade-in-up"
        style={{ background: "var(--yunque-bg)", border: "1px solid var(--yunque-border)", boxShadow: "0 32px 80px rgba(0,0,0,0.5)" }}
      >
        <button onClick={finish} className="absolute top-4 right-4 p-1.5 rounded-lg" style={{ color: "var(--yunque-text-muted)" }}>
          <X size={16} />
        </button>

        <div className="px-8 pt-10 pb-6 text-center">
          <div className="mx-auto w-16 h-16 rounded-2xl flex items-center justify-center mb-5" style={{ background: "rgba(59,130,246,0.1)" }}>
            {current.icon}
          </div>
          <h2 className="text-xl font-bold" style={{ color: "var(--yunque-text)" }}>{current.title}</h2>
          <p className="mt-3 text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>{current.desc}</p>
        </div>

        {/* Progress dots */}
        <div className="flex justify-center gap-2 pb-4">
          {steps.map((_, i) => (
            <div
              key={i}
              className="rounded-full transition-all"
              style={{
                width: i === step ? 24 : 8,
                height: 8,
                background: i === step ? "var(--yunque-accent)" : "rgba(255,255,255,0.1)",
              }}
            />
          ))}
        </div>

        <div className="flex items-center justify-between px-8 pb-8">
          <button
            onClick={finish}
            className="text-xs font-medium"
            style={{ color: "var(--yunque-text-muted)" }}
          >
            跳过
          </button>
          <Button
            size="sm"
            className="rounded-full px-6"
            style={{ background: "var(--yunque-accent)", color: "#fff" }}
            onPress={() => isLast ? finish() : setStep(step + 1)}
          >
            {isLast ? "开始使用" : "下一步"} <ArrowRight size={14} className="ml-1" />
          </Button>
        </div>
      </div>
    </div>
  );
}
