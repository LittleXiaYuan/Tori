import type { Metadata } from "next";
import { Plus_Jakarta_Sans } from "next/font/google";
import "./globals.css";
import AppShell from "@/components/app-shell";

const jakarta = Plus_Jakarta_Sans({
  subsets: ["latin"],
  variable: "--font-sans",
  display: "swap",
  weight: ["300", "400", "500", "600", "700", "800"],
});

export const metadata: Metadata = {
  title: "Yunque Agent",
  description: "Yunque Agent - Powered by HeroUI",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh" className={`dark ${jakarta.variable}`} data-theme="dark" suppressHydrationWarning>
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=JSON.parse(localStorage.getItem('yunque_theme')||'{}');var m=t.presetTheme||'dark';if(m==='auto')m=matchMedia('(prefers-color-scheme:dark)').matches?'dark':'light';document.documentElement.className=m+' ${jakarta.variable}';document.documentElement.setAttribute('data-theme',m)}catch(e){}})()`,
          }}
        />
      </head>
      <body className="flex min-h-screen bg-background text-foreground">
        <a href="#main-content" className="skip-link">Skip to main content</a>
        <AppShell>{children}</AppShell>
      </body>
    </html>
  );
}
