"use client";

import { useCallback, useEffect, useState } from "react";

function tauriInvoke(cmd: string, args?: Record<string, unknown>): Promise<unknown> | null {
  if (typeof window === "undefined") return null;
  const ti = (window as any).__TAURI_INTERNALS__;
  if (!ti?.invoke) return null;
  return ti.invoke(cmd, args);
}

export function WindowControls() {
  const [maximized, setMaximized] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [isMac, setIsMac] = useState(false);

  useEffect(() => {
    if (typeof navigator !== "undefined") {
      setIsMac(navigator.platform?.startsWith("Mac") || navigator.userAgent?.includes("Mac"));
    }
  }, []);

  useEffect(() => {
    tauriInvoke("plugin:window|is_maximized")
      ?.then((v: any) => setMaximized(!!v))
      ?.catch(() => {});

    const handler = () => {
      tauriInvoke("plugin:window|is_maximized")
        ?.then((v: any) => setMaximized(!!v))
        ?.catch(() => {});
    };
    window.addEventListener("resize", handler);
    return () => window.removeEventListener("resize", handler);
  }, []);

  const close = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    tauriInvoke("plugin:window|close");
  }, []);

  const minimize = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    tauriInvoke("plugin:window|minimize");
  }, []);

  const toggleMax = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    tauriInvoke("plugin:window|toggle_maximize");
  }, []);

  if (isMac) return null;

  return (
    <div
      className="window-controls"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <button className="ctl ctl-close" aria-label="关闭" onMouseDown={close}>
        {hovered && <CloseSvg />}
      </button>
      <button className="ctl ctl-minimize" aria-label="最小化" onMouseDown={minimize}>
        {hovered && <MinimizeSvg />}
      </button>
      <button className="ctl ctl-maximize" aria-label={maximized ? "还原" : "最大化"} onMouseDown={toggleMax}>
        {hovered && (maximized ? <RestoreSvg /> : <MaximizeSvg />)}
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
