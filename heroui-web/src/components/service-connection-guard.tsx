"use client";

import { useCallback, useEffect, useState } from "react";
import { Button } from "@heroui/react";
import { RefreshCw, WifiOff } from "lucide-react";

type ServiceState = "checking" | "ready" | "offline";

const HEALTH_TIMEOUT_MS = 2500;
const RETRY_INTERVAL_MS = 3000;
const POLL_INTERVAL_MS = 10000;

function BrandMark() {
  return (
    <div
      aria-hidden="true"
      className="flex h-16 w-16 items-center justify-center rounded-3xl text-2xl font-black shadow-lg"
      style={{
        color: "white",
        background:
          "radial-gradient(circle at 30% 25%, rgba(255,255,255,.38), transparent 28%), linear-gradient(135deg, var(--yunque-accent), #7c3aed)",
        boxShadow: "0 24px 70px rgba(2,132,199,.22)",
      }}
    >
      云
    </div>
  );
}

function Splash({ state, detail, onRetry }: { state: ServiceState; detail?: string; onRetry: () => void }) {
  const offline = state === "offline";
  return (
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center px-6"
      style={{
        background:
          "linear-gradient(180deg, rgba(255,255,255,.96), rgba(248,250,252,.94)), radial-gradient(circle at 50% 20%, var(--yunque-accent-soft), transparent 36%)",
      }}
    >
      <div className="flex max-w-md flex-col items-center gap-5 text-center">
        <BrandMark />
        <div className="space-y-2">
          <div className="text-2xl font-black tracking-tight" style={{ color: "var(--yunque-text)" }}>
            云雀 Agent
          </div>
          <div className="text-sm leading-6" style={{ color: "var(--yunque-text-muted)" }}>
            {offline ? "本地服务暂时不可用，正在重试连接。" : "正在启动本地服务，请稍等。"}
          </div>
        </div>
        <div
          className="flex w-full items-center justify-center gap-2 rounded-2xl border px-4 py-3 text-sm"
          style={{
            borderColor: "var(--yunque-border)",
            background: "rgba(255,255,255,.72)",
            color: offline ? "rgb(185, 28, 28)" : "var(--yunque-text-muted)",
          }}
        >
          {offline ? <WifiOff size={16} /> : <RefreshCw size={16} className="animate-spin" />}
          <span>{detail || (offline ? "正在连接 127.0.0.1:9090..." : "正在连接 127.0.0.1:9090...")}</span>
        </div>
        {offline && (
          <Button size="sm" className="gap-2 rounded-xl btn-accent" onPress={onRetry}>
            <RefreshCw size={14} />
            立即重新连接
          </Button>
        )}
      </div>
    </div>
  );
}

async function checkHealth(signal?: AbortSignal): Promise<void> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), HEALTH_TIMEOUT_MS);
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  try {
    const res = await fetch("/healthz", { cache: "no-store", signal: controller.signal });
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
    void probe(controller.signal);
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
