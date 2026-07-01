"use client";

import { useCallback, useEffect, useRef, useState } from "react";

function tauriInvoke(cmd: string, args?: Record<string, unknown>): Promise<unknown> | null {
  if (typeof window === "undefined") return null;
  const ti = (window as any).__TAURI_INTERNALS__;
  if (!ti?.invoke) {
    console.warn("[WindowControls] __TAURI_INTERNALS__ not available, IPC call skipped:", cmd);
    return null;
  }
  return ti.invoke(cmd, args).catch((err: unknown) => {
    console.error("[WindowControls] IPC failed:", cmd, err);
  });
}

export function WindowControls() {
  const [maximized, setMaximized] = useState(false);

  useEffect(() => {
    tauriInvoke("plugin:window|is_maximized")
      ?.then((v: unknown) => setMaximized(!!v))
      ?.catch(() => {});

    const handler = () => {
      tauriInvoke("plugin:window|is_maximized")
        ?.then((v: unknown) => setMaximized(!!v))
        ?.catch(() => {});
    };
    window.addEventListener("resize", handler);
    return () => window.removeEventListener("resize", handler);
  }, []);

  const firedRef = useRef<string | null>(null);

  const fireAction = useCallback((action: string, invoke: () => void) => {
    if (firedRef.current === action) return;
    firedRef.current = action;
    invoke();
    setTimeout(() => { firedRef.current = null; }, 300);
  }, []);

  const close = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    fireAction("close", () => tauriInvoke("plugin:window|close"));
  }, [fireAction]);

  const minimize = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    fireAction("minimize", () => tauriInvoke("plugin:window|minimize"));
  }, [fireAction]);

  const toggleMax = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    fireAction("toggleMax", () => tauriInvoke("plugin:window|toggle_maximize"));
  }, [fireAction]);

  return (
    <div className="window-controls">
      <button className="ctl ctl-minimize" aria-label="最小化" onMouseDown={minimize} onClick={minimize}>
        <MinimizeSvg />
      </button>
      <button className="ctl ctl-maximize" aria-label={maximized ? "还原" : "最大化"} onMouseDown={toggleMax} onClick={toggleMax}>
        {maximized ? <RestoreSvg /> : <MaximizeSvg />}
      </button>
      <button className="ctl ctl-close" aria-label="关闭" onMouseDown={close} onClick={close}>
        <CloseSvg />
      </button>
    </div>
  );
}

export function DragRegion() {
  return <div className="titlebar-drag" />;
}

const CloseSvg = () => (
  <svg width="8" height="8" viewBox="0 0 8 8" fill="none">
    <path d="M1 1L7 7M7 1L1 7" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
  </svg>
);
const MinimizeSvg = () => (
  <svg width="8" height="8" viewBox="0 0 8 8" fill="none">
    <path d="M1 4H7" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
  </svg>
);
const MaximizeSvg = () => (
  <svg width="8" height="8" viewBox="0 0 8 8" fill="none">
    <rect x="1" y="1" width="6" height="6" rx="0.5" stroke="currentColor" strokeWidth="1.1" />
  </svg>
);
const RestoreSvg = () => (
  <svg width="8" height="8" viewBox="0 0 8 8" fill="none">
    <rect x="0.5" y="2" width="5" height="5" rx="0.5" stroke="currentColor" strokeWidth="1.1" />
    <path d="M2.5 2V1.5C2.5 1.22 2.72 1 3 1H6.5C6.78 1 7 1.22 7 1.5V5C7 5.28 6.78 5.5 6.5 5.5H6" stroke="currentColor" strokeWidth="1.1" />
  </svg>
);
