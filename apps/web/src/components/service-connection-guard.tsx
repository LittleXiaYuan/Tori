"use client";

import { useCallback, useEffect, useState } from "react";
import { RefreshCw } from "lucide-react";
import { BASE, ensureApiBase } from "@/lib/api-core";

type ServiceState = "checking" | "ready" | "offline";

const HEALTH_TIMEOUT_MS = 2500;
const RETRY_INTERVAL_MS = 3000;
const POLL_INTERVAL_MS = 10000;

function Splash({ state, detail, onRetry }: { state: ServiceState; detail?: string; onRetry: () => void }) {
  const offline = state === "offline";

  // Paint a dark backdrop while the splash is up. The desktop runs in a
  // transparent window (for Mica): that transparent WebView2 only composites
  // the ROOT background layer opaquely, so a fill on the high z-index splash
  // stays see-through and the light, near-opaque #bg-overlay (z-index:-1)
  // bleeds in — the "washed-out white" loader. #bg-overlay itself DOES paint,
  // so we darken it (plus html/body) while the splash is mounted and restore
  // everything on unmount so Mica returns for the app.
  useEffect(() => {
    const root = document.documentElement;
    const body = document.body;
    const overlay = document.getElementById("bg-overlay");
    const prevRoot = root.style.backgroundColor;
    const prevBody = body.style.backgroundColor;
    const prevOverlay = overlay?.style.background ?? "";
    const dark =
      "radial-gradient(130% 100% at 50% -12%, rgba(56,189,248,0.20), transparent 46%)," +
      "radial-gradient(120% 110% at 50% 116%, rgba(139,92,246,0.18), transparent 50%)," +
      "linear-gradient(165deg, #0a1020 0%, #0e1628 48%, #0a0f1c 100%)";
    root.style.backgroundColor = "#0a0f1c";
    body.style.backgroundColor = "#0a0f1c";
    if (overlay) overlay.style.background = dark;
    return () => {
      root.style.backgroundColor = prevRoot;
      body.style.backgroundColor = prevBody;
      if (overlay) overlay.style.background = prevOverlay;
    };
  }, []);

  return (
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center overflow-hidden"
      style={{ backgroundColor: "#0a0f1c", userSelect: "none", WebkitUserSelect: "none" }}
    >
      {/* Opaque dark backdrop promoted to its own compositing layer. In Tauri's
          transparent WebView2 window (body is transparent for Mica), a fill on a
          non-layered element stays see-through and the light #bg-overlay bleeds
          in — that is why the splash looked washed-out/white. translateZ forces a
          real layer, the same reason the blurred auroras/badge paint opaque. */}
      <div
        aria-hidden
        className="absolute inset-0"
        style={{
          transform: "translateZ(0)",
          willChange: "transform",
          background:
            "radial-gradient(130% 100% at 50% -12%, rgba(56,189,248,0.20), transparent 46%)," +
            "radial-gradient(120% 110% at 50% 116%, rgba(139,92,246,0.18), transparent 50%)," +
            "linear-gradient(165deg, #0a1020 0%, #0e1628 48%, #0a0f1c 100%)",
        }}
      />
      <div aria-hidden className="yq-aurora yq-aurora-a" />
      <div aria-hidden className="yq-aurora yq-aurora-b" />
      <div aria-hidden className="absolute inset-0 yq-grain" />

      <div className="relative flex flex-col items-center gap-7 px-8 text-center yq-rise">
        <div className="relative flex h-24 w-24 items-center justify-center">
          {/* plain rotating ring */}
          <div
            aria-hidden
            className="absolute inset-0 rounded-full yq-spin"
            style={{ border: "3px solid rgba(148,163,184,0.16)", borderTopColor: "#60a5fa" }}
          />
          {/* brand badge — Mica dark translucent surface, soft glyph */}
          <div
            className="flex h-12 w-12 items-center justify-center rounded-xl text-lg font-bold"
            style={{
              background: "rgba(148,163,184,0.10)",
              border: "1px solid rgba(148,163,184,0.16)",
              color: "#94a3b8",
              backdropFilter: "blur(6px)",
            }}
          >
            云
          </div>
        </div>

        <div className="space-y-2">
          <div
            className="text-[22px] font-extrabold"
            style={{ color: "#f1f5f9", letterSpacing: "0.18em", textShadow: "0 1px 24px rgba(56,189,248,.25)" }}
          >
            云雀 Agent
          </div>
          <div
            aria-live="polite"
            className="text-[13px] leading-6"
            style={{ color: offline ? "rgba(252,165,165,.92)" : "rgba(148,163,184,.92)" }}
          >
            {offline ? detail || "本地服务连接断开，正在自动重试…" : "正在连接本地服务，请稍候…"}
          </div>
        </div>

        {offline && (
          <button
            type="button"
            onClick={onRetry}
            className="yq-btn group inline-flex items-center gap-2 rounded-full px-5 py-2 text-[13px] font-semibold text-white"
          >
            <RefreshCw size={14} className="transition-transform duration-500 group-hover:rotate-180" />
            立即重新连接
          </button>
        )}
      </div>

      <style>{`
        @keyframes yqSpin { to { transform: rotate(360deg); } }
        .yq-spin { animation: yqSpin 0.9s linear infinite; }
        @keyframes yqRise { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: none; } }
        .yq-rise { animation: yqRise .5s ease-out both; }
        /* static blurred glows (no per-frame animation) to stay light in the webview */
        .yq-aurora { position:absolute; width:42vw; height:42vw; border-radius:9999px; filter:blur(80px); opacity:.45; pointer-events:none; }
        .yq-aurora-a { top:-12%; left:10%; background:radial-gradient(circle, rgba(56,189,248,.5), transparent 60%); }
        .yq-aurora-b { bottom:-14%; right:8%; background:radial-gradient(circle, rgba(139,92,246,.45), transparent 60%); }
        .yq-grain { opacity:.05; background-image:radial-gradient(rgba(255,255,255,.7) .5px, transparent .6px); background-size:3px 3px; }
        .yq-btn { background:rgba(255,255,255,.08); border:1px solid rgba(148,163,184,.28); backdrop-filter:blur(8px); transition:background .2s,border-color .2s,transform .2s; }
        .yq-btn:hover { background:rgba(56,189,248,.16); border-color:rgba(56,189,248,.5); transform:translateY(-1px); }
        .yq-btn:active { transform:translateY(0); }
        @media (prefers-reduced-motion: reduce) {
          .yq-spin,.yq-rise { animation:none !important; }
        }
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

export default function ServiceConnectionGuard({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<ServiceState>("checking");
  const [detail, setDetail] = useState("正在连接 127.0.0.1:9090...");

  const probe = useCallback(async (signal?: AbortSignal) => {
    try {
      await checkHealth(signal);
      setState("ready");
      setDetail("本地服务已连接");
    } catch (error) {
      if (signal?.aborted) return;
      setState("offline");
      setDetail(error instanceof Error && error.name === "AbortError"
        ? "本地服务响应超时，正在重试..."
        : "本地服务暂时不可用，正在重试...");
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

  if (state !== "ready") {
    return <Splash state={state} detail={detail} onRetry={() => void probe()} />;
  }

  return <>{children}</>;
}
