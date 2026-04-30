"use client";

import { useEffect, useState, useCallback } from "react";

interface TauriAPI {
  core: { invoke: (cmd: string, args?: Record<string, unknown>) => Promise<unknown> };
  event: {
    listen: (event: string, handler: (e: { payload: unknown }) => void) => Promise<() => void>;
  };
}

function tauri(): TauriAPI | undefined {
  return typeof window !== "undefined"
    ? (window as unknown as { __TAURI__?: TauriAPI }).__TAURI__
    : undefined;
}

export default function FloatingBallPage() {
  const [count, setCount] = useState(0);
  const [hover, setHover] = useState(false);

  const refreshCount = useCallback(async () => {
    const t = tauri();
    if (!t) return;
    try {
      const n = (await t.core.invoke("get_floating_count")) as number;
      setCount(n);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    refreshCount();
    const t = tauri();
    if (!t) return;
    let unlisten: (() => void) | undefined;
    t.event.listen("yunque:floating-update", () => refreshCount()).then(fn => { unlisten = fn; });
    return () => { unlisten?.(); };
  }, [refreshCount]);

  const handleClick = useCallback(async () => {
    const t = tauri();
    if (!t) return;
    await t.core.invoke("toggle_floating_panel");
  }, []);

  return (
    <>
      <style>{`
        @keyframes float-pulse {
          0%, 100% { box-shadow: 0 4px 16px rgba(59,130,246,0.35); }
          50% { box-shadow: 0 4px 24px rgba(59,130,246,0.5); }
        }
        @keyframes badge-pop {
          0% { transform: scale(0); }
          60% { transform: scale(1.2); }
          100% { transform: scale(1); }
        }
        body { background: transparent !important; margin: 0; overflow: hidden; }
      `}</style>
      <div
        data-tauri-drag-region
        style={{
          width: "100vw",
          height: "100vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          background: "transparent",
        }}
      >
        <button
          onClick={handleClick}
          onMouseEnter={() => setHover(true)}
          onMouseLeave={() => setHover(false)}
          style={{
            width: 48,
            height: 48,
            borderRadius: "50%",
            background: hover
              ? "linear-gradient(135deg, #60a5fa, #a78bfa)"
              : "linear-gradient(135deg, #3b82f6, #8b5cf6)",
            border: "none",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            cursor: "pointer",
            animation: hover ? "none" : "float-pulse 3s ease-in-out infinite",
            boxShadow: hover
              ? "0 6px 24px rgba(59,130,246,0.55)"
              : "0 4px 16px rgba(59,130,246,0.35)",
            transition: "all 0.2s cubic-bezier(0.4, 0, 0.2, 1)",
            transform: hover ? "scale(1.1)" : "scale(1)",
            position: "relative",
            outline: "none",
          }}
        >
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ transition: "transform 0.2s", transform: hover ? "rotate(-8deg)" : "none" }}>
            <path d="M12 2L2 7l10 5 10-5-10-5z" />
            <path d="M2 17l10 5 10-5" />
            <path d="M2 12l10 5 10-5" />
          </svg>
          {count > 0 && (
            <span
              key={count}
              style={{
                position: "absolute",
                top: -4,
                right: -4,
                minWidth: 18,
                height: 18,
                borderRadius: 9,
                background: "#ef4444",
                color: "white",
                fontSize: 11,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                fontWeight: 700,
                padding: "0 4px",
                lineHeight: 1,
                animation: "badge-pop 0.3s cubic-bezier(0.4, 0, 0.2, 1)",
                border: "2px solid rgba(15,16,20,0.9)",
              }}
            >
              {count > 99 ? "99+" : count}
            </span>
          )}
        </button>
      </div>
    </>
  );
}
