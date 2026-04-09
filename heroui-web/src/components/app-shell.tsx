"use client";

import { usePathname } from "next/navigation";
import Sidebar from "@/components/sidebar";
import AuthGuard from "@/components/auth-guard";
import { Toaster } from "@/components/toast-provider";
import CommandPalette from "@/components/command-palette";
import { I18nProvider } from "@/lib/i18n";

const NO_SIDEBAR_PATHS = ["/login", "/setup"];

export default function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const hideSidebar = NO_SIDEBAR_PATHS.some((path) => pathname?.startsWith(path));

  return (
    <I18nProvider>
      <AuthGuard>
        {!hideSidebar && <Sidebar />}
        <main id="main-content" className="flex min-h-screen flex-1 flex-col overflow-hidden">
          {children}
        </main>
        <Toaster />
        {!hideSidebar && <CommandPalette />}
      </AuthGuard>
    </I18nProvider>
  );
}
