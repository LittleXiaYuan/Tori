"use client";

import { Suspense } from "react";
import { usePathname } from "next/navigation";
import Sidebar from "@/components/sidebar";
import AuthGuard from "@/components/auth-guard";
import { Toaster } from "@/components/toast-provider";
import CommandPalette from "@/components/command-palette";
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

export default function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const hideSidebar = NO_SIDEBAR_PATHS.some((path) => pathname?.startsWith(path));

  return (
    <I18nProvider>
      <AuthGuard>
        {!hideSidebar && <Sidebar />}
        <main id="main-content" className="flex min-h-screen flex-1 flex-col overflow-hidden">
          <Suspense fallback={<PageFallback />}>
            {children}
          </Suspense>
        </main>
        <Toaster />
        {!hideSidebar && <CommandPalette />}
      </AuthGuard>
    </I18nProvider>
  );
}
