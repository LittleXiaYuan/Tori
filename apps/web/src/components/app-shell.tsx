"use client";

import { Suspense, useEffect, useState, useCallback } from "react";
import { usePathname, useRouter } from "next/navigation";
import AccountRail from "@/components/layout/account-rail";
import AuthGuard from "@/components/auth-guard";
import { showToast, Toaster } from "@/components/toast-provider";
import { ConfirmDialogProvider } from "@/components/confirm-dialog";
import CommandPalette from "@/components/command-palette";
import { CherrySettingsModal } from "@/components/cherry/settings-modal";
import { SelectionToolbar } from "@/components/selection-toolbar";
import ServiceConnectionGuard from "@/components/service-connection-guard";
import DesktopUpdater from "@/components/desktop-updater";
import { I18nProvider } from "@/lib/i18n";
import { AppTitleBar } from "@/components/layout/app-title-bar";

const NO_SIDEBAR_PATHS = ["/login", "/setup"];
const BARE_PATHS = ["/selection-popup"];

function PageFallback() {
  return (
    <div role="status" aria-label="加载中" style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "60vh" }}>
      <div style={{
        width: 24, height: 24, borderRadius: "50%",
        border: "2.5px solid var(--yunque-border)",
        borderTopColor: "var(--yunque-accent)",
        animation: "spin 0.6s linear infinite",
      }} />
    </div>
  );
}

function useSystemThemeBridge(): void {
  useEffect(() => {
    if (typeof window === "undefined") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const readPreset = (): string => {
      try {
        const raw = localStorage.getItem("yunque_theme");
        if (!raw) return "dark";
        const parsed = JSON.parse(raw) as { presetTheme?: string };
        return parsed?.presetTheme || "dark";
      } catch { return "dark"; }
    };
    const apply = () => {
      if (readPreset() !== "auto") return;
      const mode = mq.matches ? "dark" : "light";
      const html = document.documentElement;
      html.classList.remove("dark", "light");
      html.classList.add(mode);
      html.setAttribute("data-theme", mode);
    };
    apply();
    mq.addEventListener("change", apply);
    const onStorage = (e: StorageEvent) => {
      if (e.key === "yunque_theme") apply();
    };
    window.addEventListener("storage", onStorage);
    return () => {
      mq.removeEventListener("change", apply);
      window.removeEventListener("storage", onStorage);
    };
  }, []);
}

function useFontScaleBridge(): void {
  useEffect(() => {
    if (typeof window === "undefined") return;
    const apply = () => {
      try {
        const raw = localStorage.getItem("yunque_user_preferences");
        if (!raw) {
          document.documentElement.style.setProperty("--font-scale", "1");
          return;
        }
        const parsed = JSON.parse(raw);
        const fontSize = parsed?.interface?.fontSize || "default";
        const scaleMap: Record<string, string> = {
          "small": "0.9",
          "default": "1",
          "large": "1.15"
        };
        document.documentElement.style.setProperty("--font-scale", scaleMap[fontSize] || "1");
      } catch {
        document.documentElement.style.setProperty("--font-scale", "1");
      }
    };
    apply();
    const onStorage = (e: StorageEvent) => {
      if (e.key === "yunque_user_preferences") apply();
    };
    window.addEventListener("storage", onStorage);
    // Custom event for immediate cross-component sync
    window.addEventListener("yunque:preferences-updated", apply);
    return () => {
      window.removeEventListener("storage", onStorage);
      window.removeEventListener("yunque:preferences-updated", apply);
    };
  }, []);
}

export default function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  useSystemThemeBridge();
  useFontScaleBridge();

  const [settingsOpen, setSettingsOpen] = useState(false);
  const [settingsSection, setSettingsSection] = useState<string | undefined>(undefined);
  const [zenMode, setZenMode] = useState(false);

  useEffect(() => {
    const handler = (e: Event) => {
      const detail = (e as CustomEvent).detail as { section?: string } | undefined;
      setSettingsSection(detail?.section);
      setSettingsOpen(true);
    };
    window.addEventListener("yunque:open-settings", handler);
    const zenOn = () => setZenMode(true);
    const zenOff = () => setZenMode(false);
    const zenToggle = () => setZenMode(v => !v);
    window.addEventListener("yunque:zen-on", zenOn);
    window.addEventListener("yunque:zen-off", zenOff);
    window.addEventListener("yunque:zen-toggle", zenToggle);
    return () => {
      window.removeEventListener("yunque:open-settings", handler);
      window.removeEventListener("yunque:zen-on", zenOn);
      window.removeEventListener("yunque:zen-off", zenOff);
      window.removeEventListener("yunque:zen-toggle", zenToggle);
    };
  }, []);

  useEffect(() => {
    let unlistenTray: (() => void) | undefined;
    let unlistenReady: (() => void) | undefined;
    let unlistenError: (() => void) | undefined;
    void import("@tauri-apps/api/event")
      .then(async ({ listen }) => {
        unlistenTray = await listen("yunque:window-hidden-to-tray", () => {
          showToast("云雀仍在托盘运行，可点击托盘图标重新打开。", "info");
        });
        unlistenReady = await listen<{ port?: number }>("backend:ready", (event) => {
          const port = event.payload?.port;
          showToast(port ? `本地后端已就绪：127.0.0.1:${port}` : "本地后端已就绪", "success");
        });
        unlistenError = await listen<{ message?: string }>("backend:error", (event) => {
          showToast(event.payload?.message || "本地后端启动异常，可在连接页重试。", "error");
        });
      })
      .catch(() => {});
    return () => {
      unlistenTray?.();
      unlistenReady?.();
      unlistenError?.();
    };
  }, []);

  const handleSelectionAction = useCallback((action: string, text: string) => {
    const prompts: Record<string, string> = {
      ai_search: `搜索：${text}`,
      translate: `翻译以下内容（如果是中文则翻译为英文，如果是英文则翻译为中文）：\n\n${text}`,
      explain: `解释：${text}`,
      save: `将以下内容保存到知识库：\n\n${text}`,
    };
    const prompt = prompts[action] || text;
    if (pathname === "/chat") {
      document.dispatchEvent(new CustomEvent("yunque:quick-send", { detail: prompt }));
    } else {
      router.push(`/chat?q=${encodeURIComponent(prompt)}`);
    }
  }, [pathname, router]);

  const onAuthPath = NO_SIDEBAR_PATHS.some((path) => pathname?.startsWith(path));
  const onBarePath = BARE_PATHS.some((path) => pathname?.startsWith(path));

  if (onBarePath) return <>{children}</>;

  return (
    <I18nProvider>
      <ServiceConnectionGuard>
        <AuthGuard>
        {/* Unified full-width title bar (frameless window): brand left,
            theme + settings + window controls right. All columns sit below
            it so the chrome reads as one app surface (QQ/Cherry-style). */}
        <AppTitleBar />
        {!onAuthPath && (
          <div style={{
            width: zenMode ? 0 : "var(--rail-w, 64px)",
            minWidth: zenMode ? 0 : "var(--rail-w, 64px)",
            height: "calc(100vh - var(--titlebar-h, 32px))",
            position: "fixed",
            left: 0,
            top: "var(--titlebar-h, 32px)",
            overflow: "hidden",
            transition: "width 0.3s cubic-bezier(.22,1,.36,1), min-width 0.3s cubic-bezier(.22,1,.36,1)",
            zIndex: 100,
          }}>
            <AccountRail />
          </div>
        )}
        <main id="main-content" className="flex flex-1 flex-col overflow-hidden" style={{
          opacity: "var(--yunque-content-opacity, 1)",
          marginLeft: !onAuthPath && !zenMode ? "var(--rail-w, 64px)" : 0,
          height: "calc(100vh - var(--titlebar-h, 32px))",
          marginTop: "var(--titlebar-h, 32px)",
          transition: "margin-left 0.3s cubic-bezier(.22,1,.36,1)",
        }}>
          <Suspense fallback={<PageFallback />}>
            {children}
          </Suspense>
        </main>
        <Toaster />
        <DesktopUpdater />
        <ConfirmDialogProvider />
        {!onAuthPath && <CommandPalette />}
        {!onAuthPath && (
          <CherrySettingsModal
            open={settingsOpen}
            onClose={() => setSettingsOpen(false)}
            initialSection={settingsSection as never}
          />
        )}
        {!onAuthPath && <SelectionToolbar onAction={handleSelectionAction} />}
        </AuthGuard>
      </ServiceConnectionGuard>
    </I18nProvider>
  );
}
