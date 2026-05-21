"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect, useLayoutEffect, useRef, useState, useTransition } from "react";
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
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const [indicator, setIndicator] = useState<{ left: number; width: number; ready: boolean }>({
    left: 0,
    width: 0,
    ready: false,
  });

  const activeIdx = tabs.findIndex((t) =>
    t.exact ? pathname === t.href : pathname?.startsWith(t.href),
  );

  // 同步药丸指示器位置，匹配当前 active tab 的 offsetLeft/offsetWidth。
  useLayoutEffect(() => {
    const el = activeIdx >= 0 ? tabRefs.current[activeIdx] : null;
    if (!el) return;
    setIndicator({ left: el.offsetLeft, width: el.offsetWidth, ready: true });
  }, [activeIdx]);

  // 字体加载或容器宽度变化时（如窗口缩放）重新计算位置。
  useEffect(() => {
    const recalc = () => {
      const el = activeIdx >= 0 ? tabRefs.current[activeIdx] : null;
      if (!el) return;
      setIndicator({ left: el.offsetLeft, width: el.offsetWidth, ready: true });
    };
    window.addEventListener("resize", recalc);
    return () => window.removeEventListener("resize", recalc);
  }, [activeIdx]);

  return (
    <div className="page-root animate-fade-in-up">
      <div className="page-header">
        <div>
          <h1 className="page-title" style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <Settings size={20} style={{ color: "var(--yunque-accent)" }} /> 设置
          </h1>
          <p className="page-subtitle">系统配置、模型参数与安全选项</p>
        </div>
      </div>

      <nav className="settings-tabs" role="tablist">
        <span
          className="settings-tab-indicator"
          aria-hidden="true"
          data-ready={indicator.ready || undefined}
          style={{
            transform: `translateX(${indicator.left}px)`,
            width: `${indicator.width}px`,
          }}
        />
        {tabs.map((tab, i) => {
          const active = i === activeIdx;
          return (
            <button
              key={tab.href}
              ref={(el) => {
                tabRefs.current[i] = el;
              }}
              role="tab"
              aria-selected={active}
              className="settings-tab"
              data-active={active || undefined}
              onClick={() => {
                if (!active) startTransition(() => router.push(tab.href));
              }}
            >
              {tab.icon}
              <span>{tab.label}</span>
            </button>
          );
        })}
      </nav>

      <div className="settings-tab-content">
        {children}
      </div>
    </div>
  );
}
