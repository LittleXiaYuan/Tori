"use client";

import { useState, useEffect, useCallback } from "react";
import { Button } from "@heroui/react";
import {
  Sparkles, ArrowRight, X, CheckCircle2, Circle, Loader2,
  Zap, Layers,
} from "lucide-react";
import { useOnboarding, markOnboardingComplete, type OnboardingPhase } from "@/hooks/use-onboarding";
import { useI18n } from "@/lib/i18n";
import { formatErrorMessage } from "@/lib/error-utils";
import { ONBOARDING_SCENARIOS, scenarioChatHref } from "@/lib/product-scenarios";
import { writeProfileMode } from "@/lib/profile-mode";

function WelcomeAuroraMark() {
  return (
    <svg viewBox="0 0 48 48" width="34" height="34" fill="none" aria-hidden>
      <g stroke="currentColor" strokeLinecap="round" fill="none">
        <path d="M14 38 C 11 28, 19 23, 15 10" strokeWidth="2.4" opacity="0.95" />
        <path d="M24 40 C 21 27, 30 21, 25 9" strokeWidth="2.4" opacity="0.7" />
        <path d="M34 38 C 33 28, 39 23, 35 12" strokeWidth="2.4" opacity="0.48" />
      </g>
    </svg>
  );
}

function WelcomeStep({ onNext, onSkip }: { onNext: () => void; onSkip: () => void }) {
  const { t } = useI18n();
  return (
    <div className="px-8 pt-12 pb-8 text-center animate-fade-in-up">
      <div
        className="mx-auto w-16 h-16 rounded-2xl flex items-center justify-center mb-6"
        style={{
          color: "var(--yunque-accent)",
          background: "var(--yunque-accent-soft, rgba(2,132,199,0.08))",
          border: "1px solid var(--yunque-accent-muted, rgba(2,132,199,0.14))",
        }}
      >
        <WelcomeAuroraMark />
      </div>
      <h1 className="text-2xl font-bold" style={{ color: "var(--yunque-text)" }}>{t("onboarding.welcome.title")}</h1>
      <p className="mt-3 text-sm leading-relaxed" style={{ color: "var(--yunque-text-secondary)", maxWidth: 340, margin: "12px auto 0" }}>
        {t("onboarding.welcome.subtitle")}
      </p>

      <div className="mt-8 space-y-3">
        <Button
          className="w-full rounded-xl"
          style={{ background: "var(--neutral-strong-bg)", color: "var(--neutral-strong-fg)", fontWeight: 600 }}
          onPress={onNext}
          aria-label={t("onboarding.welcome.start")}
        >
          {t("onboarding.welcome.start")} <ArrowRight size={14} className="ml-1" />
        </Button>
        <a
          href="/packs"
          className="text-xs font-medium block mx-auto text-center"
          style={{ color: "var(--yunque-accent)" }}
        >
          {t("onboarding.welcome.browse")}
        </a>
        <button
          onClick={onSkip}
          className="text-xs font-medium block mx-auto"
          style={{ color: "var(--yunque-text-muted)" }}
          aria-label={t("onboarding.close")}
        >
          {t("onboarding.welcome.skip")}
        </button>
      </div>
    </div>
  );
}

interface CheckItem { label: string; done: boolean; loading: boolean }

function SetupCheckStep({ onNext, onProviderSetup }: { onNext: () => void; onProviderSetup: () => void }) {
  const { t } = useI18n();
  const [checks, setChecks] = useState<CheckItem[]>([
    { label: t("onboarding.check.server"), done: false, loading: true },
    { label: t("onboarding.check.model"), done: false, loading: false },
    { label: t("onboarding.check.engine"), done: false, loading: false },
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
      <h2 className="text-lg font-bold text-center" style={{ color: "var(--yunque-text)" }}>{t("onboarding.check.title")}</h2>
      <p className="text-xs text-center mt-1" style={{ color: "var(--yunque-text-muted)" }}>{t("onboarding.check.subtitle")}</p>
      <div className="mt-6 space-y-2">
        {checks.map((c, i) => (
          <div key={i} className="onboard-check-item" data-done={c.done || undefined} style={{ animationDelay: `${i * 100}ms` }}>
            {c.loading ? (
              <Loader2 size={16} className="animate-spin" style={{ color: "var(--yunque-accent)" }} />
            ) : c.done ? (
              <CheckCircle2 size={16} style={{ color: "var(--yunque-accent)" }} />
            ) : (
              <Circle size={16} style={{ color: "var(--yunque-text-disabled)" }} />
            )}
            <span>{c.label}</span>
            {c.done && <CheckCircle2 size={13} className="ml-auto" style={{ color: "var(--yunque-accent)" }} />}
          </div>
        ))}
      </div>
    </div>
  );
}

interface PresetInfo { id: string; name: string; base_url: string; type: string }

function ProviderSetupStep({ onNext, onSkip }: { onNext: () => void; onSkip: () => void }) {
  const { t } = useI18n();
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
    { id: "other", name: t("onboarding.provider.other") },
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
        setError(formatErrorMessage(data?.error, t("onboarding.provider.errFail")));
      }
    } catch {
      setError(t("onboarding.provider.errNetwork"));
    }
    setTesting(false);
  }, [apiKey, selectedPreset, onNext]);

  return (
    <div className="px-8 pt-8 pb-8 animate-fade-in-up">
      <h2 className="text-lg font-bold text-center" style={{ color: "var(--yunque-text)" }}>{t("onboarding.provider.title")}</h2>
      <p className="text-xs text-center mt-1" style={{ color: "var(--yunque-text-muted)" }}>{t("onboarding.provider.subtitle")}</p>

      <div className="mt-5 flex gap-3 justify-center flex-wrap">
        {displayList.map((p, i) => (
          <button
            key={p.id}
            className="onboard-provider-card"
            data-selected={selectedPreset === p.id || undefined}
            onClick={() => setSelectedPreset(p.id)}
          >
            <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{p.name}</span>
            {i === 0 && <span className="text-[10px]" style={{ color: "var(--yunque-accent)" }}>{t("onboarding.provider.recommended")}</span>}
          </button>
        ))}
      </div>

      <div className="mt-5">
        <label className="text-xs font-medium block mb-1.5" style={{ color: "var(--yunque-text-secondary)" }}>{t("onboarding.provider.apiKey")}</label>
        <input
          type="password"
          value={apiKey}
          onChange={(e) => { setApiKey(e.target.value); autoDetect(e.target.value); }}
          placeholder={t("onboarding.provider.apiKeyPlaceholder")}
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
          {t("onboarding.provider.hint")}
        </p>
      </div>

      <div className="mt-5 flex items-center justify-between">
        <button onClick={onSkip} className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{t("onboarding.provider.later")}</button>
        <Button
          size="sm"
          className="rounded-xl px-6"
          style={{ background: "var(--neutral-strong-bg)", color: "var(--neutral-strong-fg)" }}
          onPress={testConnection}
          isDisabled={!apiKey.trim() || testing}
          aria-label={t("onboarding.provider.test")}
        >
          {testing ? <Loader2 size={14} className="animate-spin mr-1" /> : null}
          {testing ? t("onboarding.provider.testing") : t("onboarding.provider.test")}
        </Button>
      </div>
    </div>
  );
}

function InteractiveDemoStep({ onNext }: { onNext: () => void }) {
  const { t } = useI18n();
  const [sent, setSent] = useState(false);

  const handleSend = useCallback((text: string) => {
    setSent(true);
    void markOnboardingComplete();
    setTimeout(() => {
      window.location.href = scenarioChatHref(text);
    }, 300);
  }, []);

  return (
    <div className="px-8 pt-8 pb-8 animate-fade-in-up">
      <h2 className="text-lg font-bold text-center" style={{ color: "var(--yunque-text)" }}>
        {t("onboarding.demo.title")} <Sparkles size={16} className="inline" style={{ color: "var(--yunque-accent)" }} />
      </h2>
      <p className="text-xs text-center mt-1" style={{ color: "var(--yunque-text-muted)" }}>{t("onboarding.demo.subtitle")}</p>

      <div className="mt-5 space-y-2.5">
        {ONBOARDING_SCENARIOS.map((d) => (
          <button
            key={d.label}
            className="onboard-demo-card w-full text-left flex items-start gap-3"
            onClick={() => handleSend(d.prompt)}
            disabled={sent}
          >
            <span className="mt-0.5 flex-shrink-0 w-7 h-7 rounded-lg flex items-center justify-center"
              style={{ background: "var(--yunque-accent-soft)", color: "var(--yunque-accent)" }}>
              {d.icon}
            </span>
            <div>
              <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{d.label}</div>
              <div className="text-[10px] mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>{d.description}</div>
            </div>
          </button>
        ))}
      </div>

      <div className="mt-4 flex items-center gap-2">
        <button onClick={onNext} className="text-xs mx-auto" style={{ color: "var(--yunque-text-muted)" }}>
          {t("onboarding.demo.skip")}
        </button>
      </div>
    </div>
  );
}

function ModeSelectStep({ onFinish }: { onFinish: (mode: "easy" | "full") => void }) {
  const { t } = useI18n();
  const [selected, setSelected] = useState<"easy" | "full">("easy");

  const modes = [
    {
      id: "easy" as const,
      icon: <Sparkles size={20} />,
      title: t("onboarding.mode.easy"),
      desc: t("onboarding.mode.easyDesc"),
      items: t("onboarding.mode.easyItems").split(","),
      recommended: true,
    },
    {
      id: "full" as const,
      icon: <Layers size={20} />,
      title: t("onboarding.mode.full"),
      desc: t("onboarding.mode.fullDesc"),
      items: t("onboarding.mode.fullItems").split(","),
      recommended: false,
    },
  ];

  return (
    <div className="px-8 pt-8 pb-8 animate-fade-in-up">
      <h2 className="text-lg font-bold text-center" style={{ color: "var(--yunque-text)" }}>{t("onboarding.mode.title")}</h2>
      <p className="text-xs text-center mt-1" style={{ color: "var(--yunque-text-muted)" }}>{t("onboarding.mode.subtitle")}</p>

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
                {t("onboarding.mode.recommended")}
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
          aria-label={t("onboarding.mode.start")}
        >
          {t("onboarding.mode.start")} <ArrowRight size={14} className="ml-1" />
        </Button>
      </div>
    </div>
  );
}

export function OnboardingGuide() {
  const { t } = useI18n();
  const { phase, setPhase, visible, finish } = useOnboarding();

  const handleModeSelect = useCallback((mode: "easy" | "full") => {
    writeProfileMode(mode);
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
    <div className="onboard-backdrop" role="dialog" aria-modal="true" aria-label={t("onboarding.aria")}>
      <div className="onboard-card">
        <button
          onClick={finish}
          className="absolute top-4 right-4 p-1.5 rounded-lg z-10"
          style={{ color: "var(--yunque-text-muted)" }}
          aria-label={t("onboarding.close")}
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
