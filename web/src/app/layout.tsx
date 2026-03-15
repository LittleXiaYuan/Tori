import type { Metadata } from "next";
import "./globals.css";
import "katex/dist/katex.min.css";
import "highlight.js/styles/github-dark.min.css";
import { TopNav } from "@/components/top-nav";
import { ErrorBoundary } from "@/components/error-boundary";
import { I18nProvider } from "@/lib/i18n";

export const metadata: Metadata = {
  title: "Yunque Agent",
  description: "Yunque Agent Dashboard",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh">
      <body>
        <I18nProvider>
          <TopNav />
          <main className="pt-20 min-h-screen px-6 pb-6 max-w-6xl mx-auto">
            <ErrorBoundary>{children}</ErrorBoundary>
          </main>
        </I18nProvider>
      </body>
    </html>
  );
}
