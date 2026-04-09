"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Card, Button, TextField, Input, Label, FieldError, Checkbox, Spinner } from "@heroui/react";
import { Shield, Eye, EyeOff } from "lucide-react";

export default function LoginPage() {
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(true);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [needsSetup, setNeedsSetup] = useState(false);
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const router = useRouter();

  useEffect(() => {
    (async () => {
      try {
        const token = localStorage.getItem("yunque_token");
        const res = await fetch("/v1/auth/status", {
          headers: token ? { Authorization: `Bearer ${token}` } : {},
        });
        const data = await res.json();
        if (data.authenticated) { router.replace("/"); return; }
        if (!data.password_set) setNeedsSetup(true);
      } catch { /* ignore */ }
      setCheckingAuth(false);
    })();
  }, [router]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (needsSetup) {
      if (password.length < 8) { setError("Password must be at least 8 characters"); return; }
      if (password !== confirmPassword) { setError("两次密码不一致"); return; }
      setLoading(true);
      try {
        const res = await fetch("/v1/auth/set-password", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ password }),
        });
        if (!res.ok) { const d = await res.json().catch(() => ({ error: "设置失败" })); setError(d.error); setLoading(false); return; }
        setNeedsSetup(false);
        setPassword(""); setConfirmPassword("");
      } catch { setError("连接失败"); }
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const res = await fetch("/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password, remember }),
      });
      if (!res.ok) { const d = await res.json().catch(() => ({ error: "密码错误" })); setError(d.error || "密码错误"); setLoading(false); return; }
      const data = await res.json();
      localStorage.setItem("yunque_token", data.token);
      localStorage.removeItem("yunque_api_key");
      router.replace("/");
    } catch { setError("连接失败，请检查服务是否运行"); }
    setLoading(false);
  }

  if (checkingAuth) {
    return <div className="fixed inset-0 flex items-center justify-center" style={{ background: "var(--yunque-bg)" }}><Spinner size="lg" /></div>;
  }

  return (
    <div className="fixed inset-0 flex items-center justify-center" style={{ background: "var(--yunque-bg)" }}>
      {/* Subtle radial glow behind the card */}
      <div
        className="absolute pointer-events-none"
        style={{
          width: 480, height: 480,
          borderRadius: "50%",
          background: "radial-gradient(circle, rgba(59,130,246,0.08) 0%, transparent 70%)",
          filter: "blur(40px)",
        }}
      />
      <Card className="section-card w-full max-w-[400px] animate-scale-in relative">
        <Card.Header className="flex flex-col items-center gap-4 pt-10 pb-4">
          <div
            className="w-16 h-16 rounded-2xl flex items-center justify-center transition-transform duration-300 hover:scale-110"
            style={{
              background: "var(--yunque-accent)",
              boxShadow: "0 0 32px rgba(59,130,246,0.3), 0 0 8px rgba(59,130,246,0.2)",
            }}
          >
            <Shield size={32} className="text-white" />
          </div>
          <div className="text-center space-y-1">
            <h2 className="text-xl font-bold" style={{ color: "var(--yunque-text)" }}>
              {needsSetup ? "设置访问密码" : "登录云雀 Agent"}
            </h2>
            <p className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>
              {needsSetup ? "首次使用，请设置保护密码" : "输入密码以继续"}
            </p>
          </div>
        </Card.Header>
        <Card.Content className="px-8 pb-8">
          <form onSubmit={handleSubmit} className="flex flex-col gap-5">
            {error && (
              <div className="text-xs text-red-400 bg-red-400/10 px-3 py-2.5 rounded-lg animate-fade-in flex items-center gap-2">
                <span className="w-1.5 h-1.5 rounded-full bg-red-400 animate-pulse-dot" />
                {error}
              </div>
            )}

            <TextField isRequired name="password" type={showPassword ? "text" : "password"} value={password} onChange={setPassword} autoFocus>
              <Label>密码</Label>
              <div className="relative">
                <Input placeholder="请输入密码" />
                <Button isIconOnly aria-label="隐藏" variant="ghost" size="sm" onPress={() => setShowPassword(!showPassword)} className="absolute right-3 top-1/2 -translate-y-1/2" style={{ color: "var(--yunque-text-muted)" }}>
                  {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </Button>
              </div>
            </TextField>

            {needsSetup && (
              <TextField isRequired name="confirm" type={showPassword ? "text" : "password"} value={confirmPassword} onChange={setConfirmPassword} isInvalid={confirmPassword.length > 0 && password !== confirmPassword}>
                <Label>确认密码</Label>
                <Input placeholder="再次输入密码" />
                {confirmPassword.length > 0 && password !== confirmPassword && (
                  <FieldError>两次密码不一致</FieldError>
                )}
              </TextField>
            )}

            {!needsSetup && (
              <Checkbox id="remember-login" isSelected={remember} onChange={setRemember}>
                <Checkbox.Control>
                  <Checkbox.Indicator />
                </Checkbox.Control>
                <Checkbox.Content>
                  <Label htmlFor="remember-login" className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>记住登录</Label>
                </Checkbox.Content>
              </Checkbox>
            )}

            <Button
              type="submit"
              isPending={loading}
              variant="primary"
              className="w-full rounded-lg transition-all duration-200 mt-1 btn-accent"
            >
              {loading ? <><Spinner size="sm" color="current" /> 请稍候...</> : (needsSetup ? "设置密码" : "登录")}
            </Button>
          </form>
        </Card.Content>
      </Card>
    </div>
  );
}
