import type { Metadata } from "next";
import "./globals.css";
import "katex/dist/katex.min.css";
import "highlight.js/styles/github-dark.min.css";
import { I18nProvider } from "@/lib/i18n";
import { ThemeInit } from "@/components/theme-init";
import { ToastProvider } from "@/components/ui/toast";
import { CursorGlow } from "@/components/cursor-glow";
import { AuthGuard } from "@/components/auth-guard";
import { AppShellClient } from "@/components/app-shell";

export const metadata: Metadata = {
  title: "Yunque Agent",
  description: "Yunque Agent Dashboard",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh">
      <body>
        <ThemeInit />
        <CursorGlow />
        <I18nProvider>
          <ToastProvider>
            <AuthGuard>
              <AppShellClient>{children}</AppShellClient>
            </AuthGuard>
          </ToastProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
