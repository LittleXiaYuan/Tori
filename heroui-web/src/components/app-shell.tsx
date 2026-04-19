"use client";

import { Suspense, useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import Sidebar from "@/components/sidebar";
import AuthGuard from "@/components/auth-guard";
import { Toaster } from "@/components/toast-provider";
import CommandPalette from "@/components/command-palette";
import { CherrySettingsModal } from "@/components/cherry/settings-modal";
import { I18nProvider } from "@/lib/i18n";

const NO_SIDEBAR_PATHS = ["/login", "/setup"];

function PageFallback() {
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "60vh" }}>
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
  useSystemThemeBridge();

  const [settingsOpen, setSettingsOpen] = useState(false);

  useEffect(() => {
    const handler = () => setSettingsOpen(true);
    window.addEventListener("yunque:open-settings", handler);
    return () => window.removeEventListener("yunque:open-settings", handler);
  }, []);

  const onAuthPath = NO_SIDEBAR_PATHS.some((path) => pathname?.startsWith(path));

  return (
    <I18nProvider>
      <AuthGuard>
        {!onAuthPath && <Sidebar />}
        <main id="main-content" className="flex min-h-screen flex-1 flex-col overflow-hidden" style={{ opacity: "var(--yunque-content-opacity, 1)" }}>
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
      </AuthGuard>
    </I18nProvider>
  );
}
