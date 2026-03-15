"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  MessageSquare,
  Wrench,
  Puzzle,
  Users,
  Activity,
  Settings,
  Inbox,
  HeartPulse,
  Bot,
  Fingerprint,
  SmilePlus,
} from "lucide-react";

const navItems = [
  { href: "/", label: "Dashboard", icon: LayoutDashboard },
  { href: "/chat", label: "Chat", icon: MessageSquare },
  { href: "/persona", label: "Persona", icon: Fingerprint },
  { href: "/emotions", label: "Emotions", icon: SmilePlus },
  { href: "/heartbeat", label: "Heartbeat", icon: HeartPulse },
  { href: "/skills", label: "Skills", icon: Wrench },
  { href: "/plugins", label: "Plugins", icon: Puzzle },
  { href: "/tenants", label: "Tenants", icon: Users },
  { href: "/metrics", label: "Metrics", icon: Activity },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="fixed left-0 top-0 h-screen w-56 flex flex-col border-r"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
      <div className="flex items-center gap-2 px-5 py-5">
        <div className="w-8 h-8 rounded-lg flex items-center justify-center text-white font-bold text-sm"
          style={{ background: "var(--accent)" }}>
          Y
        </div>
        <span className="font-semibold text-base tracking-tight">Yunque Agent</span>
      </div>

      <nav className="flex-1 px-3 py-2 space-y-0.5">
        {navItems.map(({ href, label, icon: Icon }) => {
          const active = pathname === href;
          return (
            <Link
              key={href}
              href={href}
              className="flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors"
              style={{
                background: active ? "var(--bg-hover)" : "transparent",
                color: active ? "var(--text)" : "var(--text-muted)",
              }}
            >
              <Icon size={18} />
              {label}
            </Link>
          );
        })}
      </nav>

      <div className="px-3 pb-4">
        <Link
          href="/settings"
          className="flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors"
          style={{ color: "var(--text-muted)" }}
        >
          <Settings size={18} />
          Settings
        </Link>
      </div>
    </aside>
  );
}
