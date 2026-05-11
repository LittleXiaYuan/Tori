"use client";

import { useState, useEffect, useCallback } from "react";
import { Button } from "@heroui/react";
import {
  Sparkles, ArrowRight, X, CheckCircle2, Circle, Loader2,
  Zap, MessageCircle, BookOpen, Settings, LayoutDashboard,
  Search, Brain, Code2, Layers,
} from "lucide-react";
import { useOnboarding, type OnboardingPhase } from "@/hooks/use-onboarding";
import { formatErrorMessage } from "@/lib/error-utils";

const PROFILE_KEY = "yunque_profile_mode";

function WelcomeStep({ onNext, onSkip }: { onNext: () => void; onSkip: () => void }) {
  return (
    <div className="px-8 pt-12 pb-8 text-center animate-fade-in-up">
      <div
        className="mx-auto w-20 h-20 rounded-3xl flex items-center justify-center mb-6"
        style={{ background: "linear-gradient(135deg, rgba(59,130,246,0.15), rgba(168,85,247,0.10))", boxShadow: "0 0 32px rgba(59,130,246,0.2)" }}
      >
        <Sparkles size={36} style={{ color: "var(--yunque-accent)" }} />
      </div>
      <h1 className="text-2xl font-bold" style={{ color: "var(--yunque-text)" }}>你好，我是云雀</h1>
      <p className="mt-3 text-sm leading-relaxed" style={{ color: "var(--yunque-text-secondary)", maxWidth: 340, margin: "12px auto 0" }}>
        你的全能 AI 助手 — 对话、研究、编码、浏览器操控、任务编排，描述你的需求，我来完成。
      </p>

      <div className="mt-8 space-y-3">
        <Button
          className="w-full rounded-xl"
          style={{ background: "var(--neutral-strong-bg)", color: "var(--neutral-strong-fg)", fontWeight: 600 }}
          onPress={onNext}
          aria-label="开始30秒体验"
        >
          30 秒快速体验 <ArrowRight size={14} className="ml-1" />
        </Button>
        <button
          onClick={onSkip}
          className="text-xs font-medium block mx-auto"
          style={{ color: "var(--yunque-text-muted)" }}
          aria-label="跳过引导"
        >
          跳过，我是老用户
        </button>
      </div>
    </div>
  );
}

interface CheckItem { label: string; done: boolean; loading: boolean }

function SetupCheckStep({ onNext, onProviderSetup }: { onNext: () => void; onProviderSetup: () => void }) {
  const [checks, setChecks] = useState<CheckItem[]>([
    { label: "服务连接", done: false, loading: true },
    { label: "模型配置", done: false, loading: false },
    { label: "对话引擎", done: false, loading: false },
  ]);

  useEffect(() => {
    let cancelled = false;
    const run = async () => {
      await new Promise((r) => setTimeout(r, 800));
      if (cancelled) return;

      let serverOk = false;
      try {
        const token = localStorage.getItem("yunque_token") || localStorage.getItem("yunque_api_key");
        const headers: Record<string, string> = {};
        if (token) headers["Authorization"] = `Bearer ${token}`;
        const res = await fetch("/v1/version", { headers });
        serverOk = res.ok;
      } catch { /* offline */ }

      if (cancelled) return;
      setChecks((prev) => prev.map((c, i) => i === 0 ? { ...c, done: serverOk, loading: false } : i === 1 ? { ...c, loading: true } : c));

      await new Promise((r) => setTimeout(r, 600));
      if (cancelled) return;

      let modelOk = false;
      if (serverOk) {
        try {
          const token = localStorage.getItem("yunque_token") || localStorage.getItem("yunque_api_key");
          const headers: Record<string, string> = {};
          if (token) headers["Authorization"] = `Bearer ${token}`;
        const res = await fetch("/api/providers", { headers });
        if (res.ok) {
          const data = await res.json();
          modelOk = Array.isArray(data?.providers) && data.providers.length > 0;
          }
        } catch { /* no model */ }
      }

      if (cancelled) return;
      setChecks((prev) => prev.map((c, i) => i === 1 ? { ...c, done: modelOk, loading: false } : i === 2 ? { ...c, loading: true } : c));

      await new Promise((r) => setTimeout(r, 500));
      if (cancelled) return;

      setChecks((prev) => prev.map((c, i) => i === 2 ? { ...c, done: serverOk, loading: false } : c));

      await new Promise((r) => setTimeout(r, 400));
      if (cancelled) return;
      if (modelOk) onNext();
      else onProviderSetup();
    };
    run();
    return () => { cancelled = true; };
  }, [onNext, onProviderSetup]);

  return (
    <div className="px-8 pt-10 pb-8 animate-fade-in-up">
      <h2 className="text-lg font-bold text-center" style={{ color: "var(--yunque-text)" }}>正在检查你的环境…</h2>
      <div className="mt-6 space-y-1">
        {checks.map((c, i) => (
          <div key={i} className="onboard-check-item" data-done={c.done || undefined} style={{ animationDelay: `${i * 100}ms` }}>
            {c.loading ? (
              <Loader2 size={16} className="animate-spin" style={{ color: "var(--yunque-accent)" }} />
            ) : c.done ? (
              <CheckCircle2 size={16} style={{ color: "var(--yunque-success)" }} />
            ) : (
              <Circle size={16} style={{ color: "var(--yunque-text-disabled)" }} />
            )}
            <span>{c.label}</span>
            {c.done && <span className="ml-auto text-xs" style={{ color: "var(--yunque-success)" }}>✓</span>}
          </div>
        ))}
      </div>
    </div>
  );
}

interface PresetInfo { id: string; name: string; base_url: string; type: string }

function ProviderSetupStep({ onNext, onSkip }: { onNext: () => void; onSkip: () => void }) {
  const [presets, setPresets] = useState<PresetInfo[]>([]);
  const [selectedPreset, setSelectedPreset] = useState<string | null>(null);
  const [apiKey, setApiKey] = useState("");
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const token = localStorage.getItem("yunque_token") || localStorage.getItem("yunque_api_key");
    const headers: Record<string, string> = {};
    if (token) headers["Authorization"] = `Bearer ${token}`;
    fetch("/api/providers/presets", { headers })
      .then((r) => r.ok ? r.json() : { presets: [] })
      .then((data) => {
        const list = (data.presets || []) as PresetInfo[];
        setPresets(list);
        if (list.length > 0) setSelectedPreset(list[0].id);
      })
      .catch(() => {});
  }, []);

  const fallbackProviders = [
    { id: "openai", name: "OpenAI" },
    { id: "anthropic", name: "Claude" },
    { id: "other", name: "其他" },
  ];
  const displayList = presets.length > 0
    ? presets.slice(0, 4).map((p) => ({ id: p.id, name: p.name }))
    : fallbackProviders;

  const autoDetect = useCallback((key: string) => {
    if (key.startsWith("sk-ant-")) setSelectedPreset("anthropic");
    else if (key.startsWith("sk-")) setSelectedPreset("openai");
  }, []);

  const testConnection = useCallback(async () => {
    if (!apiKey.trim()) return;
    setTesting(true);
    setError("");
    try {
      const token = localStorage.getItem("yunque_token") || localStorage.getItem("yunque_api_key");
      const headers: Record<string, string> = { "Content-Type": "application/json" };
      if (token) headers["Authorization"] = `Bearer ${token}`;

      const body: Record<string, string> = { api_key: apiKey.trim() };
      if (selectedPreset) body.preset_id = selectedPreset;

      const res = await fetch("/api/providers/register", {
        method: "POST",
        headers,
        body: JSON.stringify(body),
      });
      if (res.ok) {
        onNext();
      } else {
        const data = await res.json().catch(() => ({}));
        setError(formatErrorMessage(data?.error, "连接失败，请检查 API Key"));
      }
    } catch {
      setError("网络错误，请检查服务是否在线");
    }
    setTesting(false);
  }, [apiKey, selectedPreset, onNext]);

  return (
    <div className="px-8 pt-8 pb-8 animate-fade-in-up">
      <h2 className="text-lg font-bold text-center" style={{ color: "var(--yunque-text)" }}>还差一步就能开始</h2>
      <p className="text-xs text-center mt-1" style={{ color: "var(--yunque-text-muted)" }}>选择你的 AI 模型提供商</p>

      <div className="mt-5 flex gap-3 justify-center flex-wrap">
        {displayList.map((p, i) => (
          <button
            key={p.id}
            className="onboard-provider-card"
            data-selected={selectedPreset === p.id || undefined}
            onClick={() => setSelectedPreset(p.id)}
          >
            <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{p.name}</span>
            {i === 0 && <span className="text-[10px]" style={{ color: "var(--yunque-accent)" }}>⭐ 推荐</span>}
          </button>
        ))}
      </div>

      <div className="mt-5">
        <label className="text-xs font-medium block mb-1.5" style={{ color: "var(--yunque-text-secondary)" }}>API Key</label>
        <input
          type="password"
          value={apiKey}
          onChange={(e) => { setApiKey(e.target.value); autoDetect(e.target.value); }}
          placeholder="粘贴你的 API Key…"
          className="w-full px-3 py-2.5 rounded-xl text-sm outline-none"
          style={{
            background: "var(--yunque-surface-2)",
            border: `1px solid ${error ? "var(--yunque-danger)" : "var(--yunque-border)"}`,
            color: "var(--yunque-text)",
          }}
          onKeyDown={(e) => { if (e.key === "Enter") testConnection(); }}
        />
        {error && <p className="text-xs mt-1.5" style={{ color: "var(--yunque-danger)" }}>{error}</p>}
        <p className="text-[10px] mt-1.5" style={{ color: "var(--yunque-text-disabled)" }}>
          💡 直接粘贴 Key，我会自动识别提供商类型
        </p>
      </div>

      <div className="mt-5 flex items-center justify-between">
        <button onClick={onSkip} className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>稍后配置</button>
        <Button
          size="sm"
          className="rounded-xl px-6"
          style={{ background: "var(--neutral-strong-bg)", color: "var(--neutral-strong-fg)" }}
          onPress={testConnection}
          isDisabled={!apiKey.trim() || testing}
          aria-label="测试连接"
        >
          {testing ? <Loader2 size={14} className="animate-spin mr-1" /> : null}
          {testing ? "测试中…" : "测试连接"}
        </Button>
      </div>
    </div>
  );
}

function InteractiveDemoStep({ onNext }: { onNext: () => void }) {
  const [sent, setSent] = useState(false);

  const demos = [
    { icon: <Search size={14} />, label: "帮我搜索最新的 AI 趋势", desc: "自动浏览网页并汇总" },
    { icon: <Brain size={14} />, label: "总结一下今天的科技新闻", desc: "信息提取与摘要" },
    { icon: <Code2 size={14} />, label: "写一个 Python hello world", desc: "代码生成与运行" },
  ];

  const handleSend = useCallback((text: string) => {
    setSent(true);
    const encoded = encodeURIComponent(text);
    setTimeout(() => {
      localStorage.setItem("yunque_onboarding_done", "1");
      window.location.href = `/chat?q=${encoded}`;
    }, 300);
  }, []);

  return (
    <div className="px-8 pt-8 pb-8 animate-fade-in-up">
      <h2 className="text-lg font-bold text-center" style={{ color: "var(--yunque-text)" }}>
        试试对我说点什么 <Sparkles size={16} className="inline" style={{ color: "var(--yunque-accent)" }} />
      </h2>
      <p className="text-xs text-center mt-1" style={{ color: "var(--yunque-text-muted)" }}>点击下方卡片，或输入自定义需求</p>

      <div className="mt-5 space-y-2.5">
        {demos.map((d) => (
          <button
            key={d.label}
            className="onboard-demo-card w-full text-left flex items-start gap-3"
            onClick={() => handleSend(d.label)}
            disabled={sent}
          >
            <span className="mt-0.5 flex-shrink-0 w-7 h-7 rounded-lg flex items-center justify-center"
              style={{ background: "var(--yunque-accent-soft)", color: "var(--yunque-accent)" }}>
              {d.icon}
            </span>
            <div>
              <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{d.label}</div>
              <div className="text-[10px] mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>{d.desc}</div>
            </div>
          </button>
        ))}
      </div>

      <div className="mt-4 flex items-center gap-2">
        <button onClick={onNext} className="text-xs mx-auto" style={{ color: "var(--yunque-text-muted)" }}>
          跳过演示，直接选择模式 →
        </button>
      </div>
    </div>
  );
}

function ModeSelectStep({ onFinish }: { onFinish: (mode: "easy" | "full") => void }) {
  const [selected, setSelected] = useState<"easy" | "full">("easy");

  const modes = [
    {
      id: "easy" as const,
      icon: <Sparkles size={20} />,
      title: "轻松模式",
      desc: "5 个核心功能，适合日常使用",
      items: ["概览", "对话", "任务中心", "知识库", "设置"],
      recommended: true,
    },
    {
      id: "full" as const,
      icon: <Layers size={20} />,
      title: "完整模式",
      desc: "35+ 全部功能，适合专业用户",
      items: ["所有轻松模式", "工作流编排", "记忆管理", "角色定制", "审计监控", "更多…"],
      recommended: false,
    },
  ];

  return (
    <div className="px-8 pt-8 pb-8 animate-fade-in-up">
      <h2 className="text-lg font-bold text-center" style={{ color: "var(--yunque-text)" }}>选择适合你的模式</h2>
      <p className="text-xs text-center mt-1" style={{ color: "var(--yunque-text-muted)" }}>随时可在侧栏底部切换</p>

      <div className="mt-5 flex gap-3">
        {modes.map((m) => (
          <button
            key={m.id}
            className="onboard-mode-card"
            data-selected={selected === m.id || undefined}
            onClick={() => setSelected(m.id)}
          >
            <div className="flex justify-center mb-3" style={{ color: selected === m.id ? "var(--yunque-accent)" : "var(--yunque-text-muted)" }}>
              {m.icon}
            </div>
            <div className="text-sm font-bold" style={{ color: "var(--yunque-text)" }}>{m.title}</div>
            <div className="text-[10px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>{m.desc}</div>
            <div className="mt-3 space-y-1 text-left">
              {m.items.map((item) => (
                <div key={item} className="text-[10px] flex items-center gap-1.5" style={{ color: "var(--yunque-text-secondary)" }}>
                  <span style={{ color: "var(--yunque-success)", fontSize: 8 }}>●</span> {item}
                </div>
              ))}
            </div>
            {m.recommended && (
              <div className="mt-3 text-[10px] font-medium px-2 py-0.5 rounded-full inline-block"
                style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent)" }}>
                推荐
              </div>
            )}
          </button>
        ))}
      </div>

      <div className="mt-6">
        <Button
          className="w-full rounded-xl"
          style={{ background: "var(--neutral-strong-bg)", color: "var(--neutral-strong-fg)", fontWeight: 600 }}
          onPress={() => onFinish(selected)}
          aria-label={`选择${selected === "easy" ? "轻松" : "完整"}模式并开始`}
        >
          开始使用 <ArrowRight size={14} className="ml-1" />
        </Button>
      </div>
    </div>
  );
}

export function OnboardingGuide() {
  const { phase, setPhase, visible, finish } = useOnboarding();

  const handleModeSelect = useCallback((mode: "easy" | "full") => {
    localStorage.setItem(PROFILE_KEY, mode);
    finish();
  }, [finish]);

  const handleSkipToDemo = useCallback(() => setPhase("interactive-demo"), [setPhase]);
  const handleSkipToMode = useCallback(() => setPhase("mode-select"), [setPhase]);

  if (!visible) return null;

  const phaseIndex: Record<OnboardingPhase, number> = {
    "welcome": 0, "setup-check": 1, "provider-setup": 2,
    "interactive-demo": 3, "mode-select": 4, "done": 5,
  };
  const currentIdx = phaseIndex[phase] || 0;
  const totalSteps = 5;

  return (
    <div className="onboard-backdrop" role="dialog" aria-modal="true" aria-label="新手引导">
      <div className="onboard-card">
        <button
          onClick={finish}
          className="absolute top-4 right-4 p-1.5 rounded-lg z-10"
          style={{ color: "var(--yunque-text-muted)" }}
          aria-label="关闭引导"
        >
          <X size={16} />
        </button>

        {phase !== "welcome" && phase !== "done" && (
          <div className="onboard-progress" style={{ padding: "16px 32px 0" }}>
            {Array.from({ length: totalSteps }).map((_, i) => (
              <div
                key={i}
                className="onboard-progress-dot"
                data-active={i === currentIdx || undefined}
                data-done={i < currentIdx || undefined}
              />
            ))}
          </div>
        )}

        {phase === "welcome" && (
          <WelcomeStep
            onNext={() => setPhase("setup-check")}
            onSkip={finish}
          />
        )}

        {phase === "setup-check" && (
          <SetupCheckStep
            onNext={handleSkipToDemo}
            onProviderSetup={() => setPhase("provider-setup")}
          />
        )}

        {phase === "provider-setup" && (
          <ProviderSetupStep
            onNext={handleSkipToDemo}
            onSkip={handleSkipToMode}
          />
        )}

        {phase === "interactive-demo" && (
          <InteractiveDemoStep onNext={handleSkipToMode} />
        )}

        {phase === "mode-select" && (
          <ModeSelectStep onFinish={handleModeSelect} />
        )}
      </div>
    </div>
  );
}
