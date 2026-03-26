"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Lock, Eye, EyeOff, Shield } from "lucide-react";

export default function LoginPage() {
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(true);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [needsSetup, setNeedsSetup] = useState(false);
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
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
      } catch {}
    })();
  }, [router]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (needsSetup) {
      if (password.length < 4) { setError("密码至少 4 位"); return; }
      if (password !== confirmPassword) { setError("两次密码不一致"); return; }
      setLoading(true);
      try {
        const res = await fetch("/v1/auth/set-password", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ password }),
        });
        if (!res.ok) {
          const d = await res.json().catch(() => ({ error: "设置失败" }));
          setError(d.error); setLoading(false); return;
        }
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
      if (!res.ok) {
        const d = await res.json().catch(() => ({ error: "密码错误" }));
        setError(d.error || "密码错误"); setLoading(false); return;
      }
      const data = await res.json();
      localStorage.setItem("yunque_token", data.token);
      localStorage.setItem("yunque_api_key", data.token);
      router.replace("/");
    } catch { setError("连接失败，请检查服务是否运行"); }
    setLoading(false);
  }

  return (
    <div className="fixed inset-0 flex items-center justify-center overflow-hidden"
      style={{ background: "var(--bg)" }}>
      {/* Subtle ambient glow */}
      <div className="fixed w-[600px] h-[600px] rounded-full pointer-events-none"
        style={{
          top: "20%", left: "50%", transform: "translateX(-50%)",
          background: "radial-gradient(circle, rgba(56,189,248,0.06) 0%, transparent 70%)",
        }}
      />

      <div className="relative z-10 w-full max-w-[380px] mx-4">
        {/* Card */}
        <div className="rounded-2xl p-8" style={{
          background: "var(--bg-card)",
          backdropFilter: "var(--backdrop-blur)",
          border: "1px solid var(--border)",
          boxShadow: "var(--shadow-lg)",
        }}>
          {/* Icon */}
          <div className="flex justify-center mb-5">
            <div className="w-12 h-12 rounded-xl flex items-center justify-center" style={{
              background: "var(--accent-subtle)",
              border: "1px solid var(--border)",
            }}>
              {needsSetup
                ? <Shield size={22} style={{ color: "var(--accent)" }} />
                : <Lock size={22} style={{ color: "var(--accent)" }} />
              }
            </div>
          </div>

          {/* Title */}
          <h1 className="text-center text-xl font-semibold mb-1" style={{ color: "var(--text)" }}>
            {needsSetup ? "初始设置" : "登录"}
          </h1>
          <p className="text-center text-sm mb-6" style={{ color: "var(--text-muted)" }}>
            {needsSetup ? "设置管理密码以保护你的 Agent" : "输入密码访问控制面板"}
          </p>

          <form onSubmit={handleSubmit} className="space-y-3">
            {/* Password field */}
            <div>
              <label className="block text-xs mb-1.5 font-medium" style={{ color: "var(--text-secondary)" }}>
                密码
              </label>
              <div className="relative">
                <input
                  type={showPassword ? "text" : "password"}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoFocus
                  autoComplete="current-password"
                  className="w-full rounded-lg outline-none transition-colors duration-200"
                  style={{
                    padding: "10px 40px 10px 12px",
                    background: "var(--bg-elevated)",
                    border: "1px solid var(--border)",
                    color: "var(--text)",
                    fontSize: 14,
                  }}
                  onFocus={(e) => e.target.style.borderColor = "var(--accent)"}
                  onBlur={(e) => e.target.style.borderColor = "var(--border)"}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 cursor-pointer opacity-40 hover:opacity-70 transition-opacity duration-200"
                >
                  {showPassword
                    ? <EyeOff size={16} style={{ color: "var(--text-secondary)" }} />
                    : <Eye size={16} style={{ color: "var(--text-secondary)" }} />
                  }
                </button>
              </div>
            </div>

            {/* Confirm password (setup mode) */}
            {needsSetup && (
              <div>
                <label className="block text-xs mb-1.5 font-medium" style={{ color: "var(--text-secondary)" }}>
                  确认密码
                </label>
                <input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  className="w-full rounded-lg outline-none transition-colors duration-200"
                  style={{
                    padding: "10px 12px",
                    background: "var(--bg-elevated)",
                    border: "1px solid var(--border)",
                    color: "var(--text)",
                    fontSize: 14,
                  }}
                />
              </div>
            )}

            {/* Remember me */}
            {!needsSetup && (
              <label className="flex items-center gap-2 cursor-pointer pt-1" style={{ color: "var(--text-muted)", fontSize: 13 }}>
                <input
                  type="checkbox"
                  checked={remember}
                  onChange={(e) => setRemember(e.target.checked)}
                  className="rounded"
                  style={{ accentColor: "var(--accent)" }}
                />
                记住我（7 天免登录）
              </label>
            )}

            {/* Error */}
            {error && (
              <p className="text-center text-xs py-2 rounded-lg" style={{
                color: "var(--danger)",
                background: "var(--danger-bg)",
              }}>
                {error}
              </p>
            )}

            {/* Submit */}
            <button
              type="submit"
              disabled={loading || !password}
              className="w-full rounded-lg font-medium cursor-pointer transition-all duration-200"
              style={{
                padding: "10px 0",
                background: loading || !password ? "var(--bg-elevated)" : "var(--accent)",
                color: loading || !password ? "var(--text-muted)" : "#fff",
                fontSize: 14,
                border: "none",
                marginTop: 8,
              }}
            >
              {loading ? "处理中..." : needsSetup ? "设置密码" : "登录"}
            </button>
          </form>
        </div>

        {/* Footer */}
        <p className="text-center mt-4" style={{ color: "var(--text-muted)", fontSize: 11, opacity: 0.5 }}>
          Yunque Agent
        </p>
      </div>
    </div>
  );
}
