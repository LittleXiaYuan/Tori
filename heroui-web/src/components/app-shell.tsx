"use client";

import { Suspense, useEffect, useState, useCallback } from "react";
import { usePathname, useRouter } from "next/navigation";
import AccountRail from "@/components/layout/account-rail";
import AuthGuard from "@/components/auth-guard";
import { Toaster } from "@/components/toast-provider";
import CommandPalette from "@/components/command-palette";
import { CherrySettingsModal } from "@/components/cherry/settings-modal";
import { FloatingWidget } from "@/components/floating-widget";
import { OnboardingGuide } from "@/components/onboarding-guide";
import { SelectionToolbar } from "@/components/selection-toolbar";
import { I18nProvider } from "@/lib/i18n";
import { DragRegion } from "@/components/title-bar";

const NO_SIDEBAR_PATHS = ["/login", "/setup"];
const BARE_PATHS = ["/floating-ball", "/floating-panel"];

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

export default function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  useSystemThemeBridge();

  const [settingsOpen, setSettingsOpen] = useState(false);
  const [zenMode, setZenMode] = useState(false);

  useEffect(() => {
    const handler = () => setSettingsOpen(true);
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
      <DragRegion />
      <AuthGuard>
        {!onAuthPath && (
          <div style={{
            width: zenMode ? 0 : "var(--rail-w, 64px)",
            minWidth: zenMode ? 0 : "var(--rail-w, 64px)",
            overflow: "hidden",
            transition: "width 0.3s cubic-bezier(.22,1,.36,1), min-width 0.3s cubic-bezier(.22,1,.36,1)",
          }}>
            <AccountRail />
          </div>
        )}
        <main id="main-content" className="flex min-h-screen flex-1 flex-col overflow-hidden" style={{ opacity: "var(--yunque-content-opacity, 1)", paddingTop: 6 }}>
          <Suspense fallback={<PageFallback />}>
            {children}
          </Suspense>
        </main>
        <Toaster />
        {!onAuthPath && <CommandPalette />}
        {!onAuthPath && (
          <CherrySettingsModal
            open={settingsOpen}
            onClose={() => setSettingsOpen(false)}
          />
        )}
        {!onAuthPath && <FloatingWidget />}
        {!onAuthPath && <OnboardingGuide />}
        {!onAuthPath && <SelectionToolbar onAction={handleSelectionAction} />}
      </AuthGuard>
    </I18nProvider>
  );
}
