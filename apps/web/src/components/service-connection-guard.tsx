"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { RefreshCw } from "lucide-react";
import { BASE, ensureApiBase } from "@/lib/api-core";

type ServiceState = "checking" | "ready" | "offline";

const HEALTH_TIMEOUT_MS = 4500;
const RETRY_INTERVAL_MS = 3000;
const POLL_INTERVAL_MS = 10000;
// Once we've connected at least once, tolerate this many consecutive failed
// probes before dropping to the full-screen offline splash. Transient blips
// (dev rebuilds, a single slow health check) no longer flash the splash.
const FAIL_THRESHOLD = 3;
const BLIP_RECHECK_MS = 1200;
// Before the very first successful connect, a few failed probes just mean the
// backend is still cold-starting (migrations, plugin warmup, etc. per
// CLAUDE.md) — not an actual outage. Keep the copy calm ("starting up") for
// this many attempts before switching to the more urgent "unavailable" wording.
const PRE_CONNECT_GRACE_ATTEMPTS = 5;

function Splash({ state, detail, onRetry }: { state: ServiceState; detail?: string; onRetry: () => void }) {
  const [elapsedMs, setElapsedMs] = useState(0);

  useEffect(() => {
    const start = Date.now();
    const interval = setInterval(() => {
      setElapsedMs(Date.now() - start);
    }, 100);
    return () => clearInterval(interval);
  }, []);

  const elapsedSecs = (elapsedMs / 1000).toFixed(1);

  return (
    <div style={{ position: "fixed", top: 0, left: 0, right: 0, bottom: 0, width: "100vw", height: "100vh", zIndex: 9999, background: "var(--yunque-bg, #000)", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", color: "#fff", overflow: "hidden" }}>
      <div style={{ position: "absolute", top: 0, left: 0, right: 0, bottom: 0, backgroundImage: "radial-gradient(circle at 50% 50%, rgba(255,255,255,0.03) 1px, transparent 1px)", backgroundSize: "32px 32px", opacity: 0.5 }} />
      <div style={{ position: "absolute", width: "60vw", height: "60vw", background: "radial-gradient(circle, var(--yunque-accent, rgba(255,255,255,0.1)) 0%, transparent 60%)", opacity: 0.15, filter: "blur(80px)", animation: "pulse 4s ease-in-out infinite alternate" }} />
      
      <div style={{ zIndex: 1, display: "flex", flexDirection: "column", alignItems: "center", gap: 24 }}>
        <div style={{ position: "relative", width: 48, height: 48, display: "flex", alignItems: "center", justifyContent: "center" }}>
          <div style={{ position: "absolute", top: 0, left: 0, right: 0, bottom: 0, borderRadius: "50%", border: "1px solid rgba(255,255,255,0.1)" }} />
          <div style={{ position: "absolute", top: 0, left: 0, right: 0, bottom: 0, borderRadius: "50%", borderTop: "1px solid var(--yunque-accent, #fff)", animation: "spin 1s linear infinite" }} />
          <div style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--yunque-accent, #fff)", boxShadow: "0 0 12px var(--yunque-accent, #fff)" }} />
        </div>
        <div style={{ textAlign: "center" }}>
          <div style={{ fontSize: 16, fontWeight: 500, letterSpacing: "0.1em", marginBottom: 8 }}>等待健康信号...</div>
          <div style={{ fontSize: 12, color: "rgba(255,255,255,0.4)", letterSpacing: "0.05em" }}>{detail || "CONNECTING"}</div>
        </div>
      </div>
      
      <div style={{ position: "absolute", bottom: 40, fontSize: 11, fontFamily: "ui-monospace, monospace", color: "rgba(255,255,255,0.3)" }}>
        {elapsedSecs}s
      </div>

      <style>{`
        @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
        @keyframes pulse { from { opacity: 0.1; transform: scale(0.9); } to { opacity: 0.2; transform: scale(1.1); } }
      `}</style>
    </div>
  );
}
function getHealthUrl(): string {
  if (BASE) return `${BASE.replace(/\/$/, "")}/healthz`;
  // Prefer the same-origin health endpoint. In Next dev this goes through
  // next.config.js rewrites, and in the packaged desktop app it is served by
  // the Go backend directly. Hard-coding http://127.0.0.1:9090 here turns the
  // probe into a cross-origin request and makes the browser report a vague
  // "Failed to fetch" unless CORS is configured.
  return "/healthz";
}

async function checkHealth(signal?: AbortSignal): Promise<void> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), HEALTH_TIMEOUT_MS);
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  try {
    const res = await fetch(getHealthUrl(), { cache: "no-store", signal: controller.signal });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
  } finally {
    clearTimeout(timer);
    signal?.removeEventListener("abort", abort);
  }
}

/** Non-blocking reconnect banner shown (instead of the full splash) after the
 *  service has been connected at least once, so a transient drop doesn't hide
 *  the whole app. */
function ReconnectBanner({ detail, onRetry }: { detail: string; onRetry: () => void }) {
  return (
    <div
      role="status"
      aria-live="polite"
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        zIndex: 9998,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        gap: 10,
        padding: "6px 12px",
        fontSize: 12.5,
        color: "var(--yunque-text)",
        background: "var(--glass-flyout, var(--yunque-card))",
        borderBottom: "1px solid var(--glass-edge, var(--yunque-border))",
        backdropFilter: "blur(12px) saturate(1.2)",
        WebkitBackdropFilter: "blur(12px) saturate(1.2)",
        boxShadow: "0 4px 16px rgba(0,0,0,0.12)",
      }}
    >
      <span
        aria-hidden
        style={{
          width: 12,
          height: 12,
          borderRadius: "50%",
          border: "2px solid var(--yunque-border)",
          borderTopColor: "var(--yunque-accent)",
          animation: "taskPlanSpin 0.7s linear infinite",
        }}
      />
      <span style={{ color: "var(--yunque-text-secondary)" }}>{detail || "本地服务重连中…"}</span>
      <button
        type="button"
        onClick={onRetry}
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 4,
          padding: "2px 10px",
          borderRadius: 999,
          fontSize: 11.5,
          fontWeight: 600,
          color: "var(--yunque-accent)",
          background: "var(--yunque-accent-soft, rgba(2,132,199,0.08))",
          border: "1px solid var(--yunque-accent-muted, rgba(2,132,199,0.16))",
        }}
      >
        <RefreshCw size={12} /> 立即重连
      </button>
    </div>
  );
}

const BARE_SERVICE_PATHS = ["/selection-popup"];

export default function ServiceConnectionGuard({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  if (BARE_SERVICE_PATHS.some((p) => pathname?.startsWith(p))) {
    return <>{children}</>;
  }

  const [state, setState] = useState<ServiceState>("checking");
  const [detail, setDetail] = useState("正在连接 127.0.0.1:9090...");
  const everReadyRef = useRef(false);
  const failRef = useRef(0);

  const probe = useCallback(async (signal?: AbortSignal) => {
    try {
      await checkHealth(signal);
      everReadyRef.current = true;
      failRef.current = 0;
      setState("ready");
      setDetail("本地服务已连接");
    } catch (error) {
      if (signal?.aborted) return;
      failRef.current += 1;
      // After a successful connect, ride out brief blips without yanking the
      // whole UI behind the splash — just re-check soon.
      if (everReadyRef.current && failRef.current < FAIL_THRESHOLD) {
        setTimeout(() => { void probe(); }, BLIP_RECHECK_MS);
        return;
      }
      setState("offline");
      if (!everReadyRef.current && failRef.current <= PRE_CONNECT_GRACE_ATTEMPTS) {
        // First connect still pending and within the cold-start grace period —
        // read as "loading", not "something broke".
        setDetail("正在启动本地服务，请稍候…");
      } else {
        setDetail(error instanceof Error && error.name === "AbortError"
          ? "本地服务响应超时，正在重试..."
          : "本地服务暂时不可用，正在重试...");
      }
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    // Resolve the real backend base (desktop port may differ from the
    // build-time default) before the first probe so we never poll a dead port.
    void ensureApiBase().then(() => probe(controller.signal));
    const retry = setInterval(() => {
      if (document.visibilityState !== "hidden") void probe(controller.signal);
    }, state === "ready" ? POLL_INTERVAL_MS : RETRY_INTERVAL_MS);
    return () => {
      controller.abort();
      clearInterval(retry);
    };
  }, [probe, state]);

  // Full-screen splash ONLY before the very first successful connect. Once
  // we've been connected, a later disconnect shows a non-blocking top banner
  // instead of yanking the whole UI away (the app stays usable while it
  // reconnects in the background).
  if (state !== "ready" && !everReadyRef.current) {
    return <Splash state={state} detail={detail} onRetry={() => void probe()} />;
  }

  if (state === "offline") {
    return (
      <>
        <ReconnectBanner detail={detail} onRetry={() => void probe()} />
        {children}
      </>
    );
  }

  return <>{children}</>;
}
