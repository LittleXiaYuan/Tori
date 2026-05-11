"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Card, Checkbox, FieldError, Input, Label, Spinner, TextField } from "@heroui/react";
import { Eye, EyeOff, Shield, ExternalLink } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { formatErrorMessage } from "@/lib/error-utils";

const AUTH_STATUS_TIMEOUT_MS = 8000;

export default function LoginPage() {
  const router = useRouter();
  const { t } = useI18n();
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [remember, setRemember] = useState(true);
  const [needsSetup, setNeedsSetup] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [toriUrl, setToriUrl] = useState("");
  const [showToriLogin, setShowToriLogin] = useState(false);

  useEffect(() => {
    let mounted = true;
    const controller = new AbortController();
    const timeout = setTimeout(() => {
      controller.abort();
      if (mounted) setCheckingAuth(false);
    }, AUTH_STATUS_TIMEOUT_MS);

    (async () => {
      try {
        const token = localStorage.getItem("yunque_token");
        const res = await fetch("/v1/auth/status", {
          headers: token ? { Authorization: `Bearer ${token}` } : {},
          signal: controller.signal,
        });
        const data = await res.json();
        if (!mounted) return;
        if (data?.authenticated) {
          router.replace("/");
          return;
        }
        setNeedsSetup(!data?.password_set);
        if (data?.oauth_tori) setShowToriLogin(true);
      } catch {
        // ignore
      } finally {
        clearTimeout(timeout);
        if (mounted) setCheckingAuth(false);
      }
    })();
    return () => {
      mounted = false;
      clearTimeout(timeout);
      controller.abort();
    };
  }, [router]);

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError("");
    setSuccess("");

    if (needsSetup) {
      if (password.length < 8) {
        setError(t("auth.passwordTooShort"));
        return;
      }
      if (password !== confirmPassword) {
        setError(t("auth.passwordMismatch"));
        return;
      }
    }

    setLoading(true);
    try {
      if (needsSetup) {
        const res = await fetch("/v1/auth/set-password", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ password }),
        });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) {
          setError(formatErrorMessage(data?.error, t("auth.networkError")));
          return;
        }
        setNeedsSetup(false);
        setSuccess(t("auth.passwordSet") || "密码已设置，请登录");
        setPassword("");
        setConfirmPassword("");
        return;
      }

      const res = await fetch("/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password, remember }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        setError(formatErrorMessage(data?.error, "Login failed"));
        return;
      }
      localStorage.setItem("yunque_token", data.token);
      localStorage.removeItem("yunque_api_key");
      router.replace("/");
    } catch {
      setError(t("auth.networkError"));
    } finally {
      setLoading(false);
    }
  }

  if (checkingAuth) {
    return (
      <div className="fixed inset-0 flex flex-col items-center justify-center gap-3" style={{ background: "var(--yunque-bg)" }}>
        <Spinner size="lg" />
        <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{t("auth.loading")}</div>
      </div>
    );
  }

  return (
    <div className="fixed inset-0 flex items-center justify-center px-4" style={{ background: "var(--yunque-bg)" }}>
      <div
        className="pointer-events-none absolute"
        style={{
          width: 600,
          height: 600,
          borderRadius: "50%",
          background: "radial-gradient(circle, rgba(59,130,246,0.055) 0%, rgba(168,85,247,0.025) 42%, transparent 70%)",
          filter: "blur(58px)",
        }}
      />
      <div className="relative flex flex-col items-center gap-6 w-full max-w-[420px] animate-scale-in">
        <div className="text-center space-y-2 mb-2">
          <div className="flex items-center justify-center gap-3">
            <div
              className="flex h-12 w-12 items-center justify-center rounded-xl"
              style={{
                background: "linear-gradient(135deg, var(--yunque-accent), var(--yunque-success))",
                boxShadow: "0 0 24px rgba(59,130,246,0.22)",
              }}
            >
              <Shield size={24} className="text-white" />
            </div>
            <h1 className="text-2xl font-bold tracking-tight" style={{ color: "var(--yunque-text)" }}>
              云雀 Agent
            </h1>
          </div>
          <p className="text-sm" style={{ color: "var(--yunque-text-muted)", maxWidth: 320, margin: "0 auto" }}>
            全栈 AI Agent 平台 — 对话、思考、行动、进化
          </p>
        </div>

        <div className="grid grid-cols-2 gap-2 w-full">
          {[
            { icon: "💬", label: "智能对话", desc: "多模型路由 · 深度思考" },
            { icon: "🌐", label: "浏览器操控", desc: "自动化网页操作" },
            { icon: "🧠", label: "认知架构", desc: "记忆 · 反思 · 进化" },
            { icon: "⚡", label: "工作流引擎", desc: "技能编排 · 定时任务" },
          ].map((f) => (
            <div
              key={f.label}
              className="flex items-center gap-2.5 rounded-xl px-3 py-2.5"
              style={{ background: "var(--yunque-bg-muted)", border: "1px solid var(--yunque-border)" }}
            >
              <span className="text-base">{f.icon}</span>
              <div className="min-w-0">
                <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>{f.label}</div>
                <div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{f.desc}</div>
              </div>
            </div>
          ))}
        </div>

      <Card className="section-card relative w-full">
        <Card.Header className="flex flex-col items-center gap-3 pt-8 pb-3">
          <div className="space-y-1 text-center">
            <h2 className="text-lg font-semibold" style={{ color: "var(--yunque-text)" }}>
              {needsSetup ? t("auth.setupTitle") : t("auth.loginTitle")}
            </h2>
            <p className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
              {needsSetup ? t("auth.setupSubtitle") : t("auth.loginSubtitle")}
            </p>
          </div>
        </Card.Header>

        <Card.Content className="px-8 pb-8">
          <form onSubmit={handleSubmit} className="flex flex-col gap-5">
            {error && (
              <div className="flex items-center gap-2 rounded-lg bg-red-400/10 px-3 py-2.5 text-xs text-red-300">
                <span className="h-1.5 w-1.5 rounded-full bg-red-400" />
                {error}
              </div>
            )}
            {success && (
              <div className="flex items-center gap-2 rounded-lg px-3 py-2.5 text-xs" style={{ background: "rgba(34,197,94,0.1)", color: "#22c55e" }}>
                <span className="h-1.5 w-1.5 rounded-full" style={{ background: "#22c55e" }} />
                {success}
              </div>
            )}

            <TextField isRequired name="password" type={showPassword ? "text" : "password"} value={password} onChange={setPassword} autoFocus>
              <Label>{t("auth.password")}</Label>
              <div className="relative">
                <Input placeholder={t("auth.passwordPlaceholder")} />
                <Button
                  isIconOnly
                  aria-label={showPassword ? "Hide password" : "Show password"}
                  variant="ghost"
                  size="sm"
                  onPress={() => setShowPassword((v) => !v)}
                  className="absolute right-3 top-1/2 -translate-y-1/2"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </Button>
              </div>
            </TextField>

            {needsSetup && (
              <TextField
                isRequired
                name="confirm-password"
                type={showPassword ? "text" : "password"}
                value={confirmPassword}
                onChange={setConfirmPassword}
                isInvalid={confirmPassword.length > 0 && confirmPassword !== password}
              >
                <Label>{t("auth.confirmPassword")}</Label>
                <Input placeholder={t("auth.confirmPasswordPlaceholder")} />
                {confirmPassword.length > 0 && confirmPassword !== password && (
                  <FieldError>{t("auth.passwordMismatch")}</FieldError>
                )}
              </TextField>
            )}

            {!needsSetup && (
              <Checkbox id="remember-login" isSelected={remember} onChange={setRemember}>
                <Checkbox.Control>
                  <Checkbox.Indicator />
                </Checkbox.Control>
                <Checkbox.Content>
                  <Label htmlFor="remember-login" className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                    {t("auth.remember")}
                  </Label>
                </Checkbox.Content>
              </Checkbox>
            )}

            <Button type="submit" isPending={loading} variant="primary" className="btn-accent mt-1 w-full rounded-lg">
              {loading ? t("auth.submitting") : needsSetup ? t("auth.setupSubmit") : t("auth.submit")}
            </Button>

            {showToriLogin && !needsSetup && (
              <>
                <div className="flex items-center gap-3 mt-2">
                  <div className="h-px flex-1" style={{ background: "var(--yunque-border)" }} />
                  <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>OR</span>
                  <div className="h-px flex-1" style={{ background: "var(--yunque-border)" }} />
                </div>
                <Button
                  variant="outline"
                  className="w-full rounded-lg"
                  onPress={() => {
                    const url = toriUrl || "";
                    if (!url) {
                      window.location.href = "/v1/auth/oauth/tori";
                    } else {
                      window.location.href = `/v1/auth/oauth/tori?tori_url=${encodeURIComponent(url)}`;
                    }
                  }}
                  style={{ borderColor: "var(--yunque-border)" }}
                >
                  <ExternalLink size={16} className="mr-2" />
                  Login with Tori
                </Button>
              </>
            )}
          </form>
        </Card.Content>
      </Card>
      </div>
    </div>
  );
}
