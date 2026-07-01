"use client";

import { usePathname, useRouter } from "next/navigation";
import { useTransition } from "react";
import { Settings, Cpu, Plug, Bell, Palette } from "lucide-react";


const tabs = [
  { href: "/settings", label: "通用配置", labelEn: "General", icon: <Settings size={14} />, exact: true },
  { href: "/settings/providers", label: "模型提供商", labelEn: "Providers", icon: <Cpu size={14} /> },
  { href: "/settings/connectors", label: "连接器", labelEn: "Connectors", icon: <Plug size={14} /> },
  { href: "/settings/notifications", label: "通知", labelEn: "Notifications", icon: <Bell size={14} /> },
  { href: "/settings/theme", label: "主题", labelEn: "Theme", icon: <Palette size={14} /> },
];

export default function SettingsLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [, startTransition] = useTransition();

  const activeTab = tabs.find((t) =>
    t.exact ? pathname === t.href : pathname?.startsWith(t.href),
  );
  const selectedKey = activeTab?.href || "/settings";

  return (
    <div className="flex flex-col h-full animate-fade-in-up">
      <div className="w-full border-b border-white/5 bg-transparent backdrop-blur-md">
        <div className="w-full max-w-5xl mx-auto px-6 pt-4">
          <nav className="flex gap-6 relative" aria-label="设置分类">
            {tabs.map((tab) => {
              const isSelected = selectedKey === tab.href;
              return (
                <button
                  key={tab.href}
                  aria-current={isSelected ? "page" : undefined}
                  onClick={() => startTransition(() => router.push(tab.href))}
                  className={`relative h-12 flex items-center space-x-2 px-1 text-sm font-medium transition-colors ${isSelected ? 'text-[var(--yunque-accent)]' : 'text-[var(--yunque-text-secondary)] hover:text-[var(--yunque-text)]'}`}
                >
                  {tab.icon}
                  <span>{tab.label}</span>
                  {isSelected && (
                    <span className="absolute bottom-[-1px] left-0 w-full h-[2px] bg-[var(--yunque-accent)]" />
                  )}
                </button>
              );
            })}
          </nav>
        </div>
      </div>

      <div className="flex-1 overflow-auto">
        <div className="w-full max-w-5xl mx-auto px-6 py-8">
          {children}
        </div>
      </div>
    </div>
  );
}
