"use client";

/**
 * DesktopUpdater — checks for a new desktop release on launch and offers a
 * non-intrusive "update available" prompt.
 *
 * Only active inside the Tauri shell (guarded by __TAURI_INTERNALS__); in the
 * browser build it renders nothing and does no work. Uses the updater plugin's
 * JS API via dynamic import so the web bundle never hard-depends on it.
 *
 * Flow: check() → if available, show a banner → user clicks "更新" → the
 * plugin downloads + installs the signed artifact, then we relaunch. The whole
 * chain is gated by the pubkey in tauri.conf.json, so an unsigned or tampered
 * artifact is rejected by the plugin before we ever get here.
 */

import { useEffect, useState, useCallback } from "react";

type UpdateHandle = {
  version: string;
  downloadAndInstall: (onEvent?: (e: { event: string; data?: unknown }) => void) => Promise<void>;
};

function isDesktop(): boolean {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

export default function DesktopUpdater() {
  const [update, setUpdate] = useState<UpdateHandle | null>(null);
  const [phase, setPhase] = useState<"idle" | "downloading" | "error">("idle");
  const [dismissed, setDismissed] = useState(false);

  // Check once on mount. A failed check (offline, endpoint down, no release
  // yet) is non-fatal and silent — updates are a nicety, not a blocker.
  useEffect(() => {
    if (!isDesktop()) return;
    let cancelled = false;
    void (async () => {
      try {
        const mod = await import("@tauri-apps/plugin-updater").catch(() => null);
        if (!mod || cancelled) return;
        const result = await mod.check();
        if (!cancelled && result) {
          setUpdate(result as unknown as UpdateHandle);
        }
      } catch {
        // Silent: no endpoint configured yet, offline, etc.
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const install = useCallback(async () => {
    if (!update) return;
    setPhase("downloading");
    try {
      await update.downloadAndInstall();
      // Relaunch into the freshly-installed version.
      const proc = await import("@tauri-apps/plugin-process").catch(() => null);
      if (proc?.relaunch) {
        await proc.relaunch();
      }
    } catch {
      setPhase("error");
    }
  }, [update]);

  if (!update || dismissed) return null;

  return (
    <div
      role="status"
      aria-live="polite"
      style={{
        position: "fixed",
        bottom: 20,
        right: 20,
        zIndex: 9997,
        maxWidth: 340,
        padding: "14px 16px",
        borderRadius: 14,
        background: "var(--yunque-elevated, var(--yunque-card))",
        border: "1px solid var(--yunque-border)",
        boxShadow: "0 12px 40px rgba(0,0,0,0.28)",
        display: "flex",
        flexDirection: "column",
        gap: 10,
      }}
    >
      <div style={{ fontSize: 13.5, fontWeight: 600, color: "var(--yunque-text)" }}>
        发现新版本 v{update.version}
      </div>
      <div style={{ fontSize: 12, color: "var(--yunque-text-secondary)" }}>
        {phase === "downloading"
          ? "正在下载并安装，完成后将自动重启…"
          : phase === "error"
          ? "更新失败，请稍后重试或手动下载。"
          : "更新已就绪，点击安装并重启即可用上最新版本。"}
      </div>
      {phase !== "downloading" && (
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <button
            type="button"
            onClick={() => setDismissed(true)}
            style={{
              padding: "5px 12px",
              borderRadius: 999,
              fontSize: 12,
              color: "var(--yunque-text-muted)",
              background: "transparent",
              border: "1px solid var(--yunque-border)",
              cursor: "pointer",
            }}
          >
            稍后
          </button>
          <button
            type="button"
            onClick={() => void install()}
            style={{
              padding: "5px 14px",
              borderRadius: 999,
              fontSize: 12,
              fontWeight: 600,
              color: "#fff",
              background: "var(--yunque-accent)",
              border: "1px solid transparent",
              cursor: "pointer",
            }}
          >
            {phase === "error" ? "重试" : "立即更新"}
          </button>
        </div>
      )}
    </div>
  );
}
