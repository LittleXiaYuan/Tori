"use client";

import Link from "next/link";
import { ArrowRight, Search } from "lucide-react";
import { Button, Card, Chip } from "@heroui/react";
import type { PackSurfaceKey } from "@/lib/pack-surface-guidance";
import { packSurfaceGuide } from "@/lib/pack-surface-guidance";

const kindTone: Record<string, { bg: string; fg: string }> = {
  "可操作": { bg: "rgba(34,197,94,0.10)", fg: "var(--yunque-success)" },
  "基础能力": { bg: "rgba(59,130,246,0.08)", fg: "var(--yunque-primary)" },
  "治理": { bg: "rgba(245,158,11,0.10)", fg: "var(--yunque-warning)" },
};

export default function PackSurfaceGuide({ surface, compact = false }: { surface: PackSurfaceKey; compact?: boolean }) {
  const guide = packSurfaceGuide(surface);
  const Icon = guide.icon;

  if (compact) {
    return (
      <Card className="section-card p-3">
        <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
          <Icon size={15} aria-hidden={true} />
          {guide.title}
        </div>
        <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
          {guide.description}
        </div>
        <div className="mt-3 grid gap-2 md:grid-cols-2 xl:grid-cols-3">
          {guide.items.map((item) => {
            const tone = kindTone[item.kind] || kindTone["基础能力"];
            return (
              <div key={item.id} className="rounded-lg p-2" style={{ border: "1px solid var(--yunque-border)", background: "rgba(255,255,255,0.025)" }}>
                <div className="flex items-center justify-between gap-2">
                  <div className="truncate text-xs font-medium" style={{ color: "var(--yunque-text)" }}>{item.title}</div>
                  <Chip size="sm" style={{ background: tone.bg, color: tone.fg, fontSize: "var(--text-2xs)" }}>{item.kind}</Chip>
                </div>
                <div className="mt-1 text-[11px] leading-4" style={{ color: "var(--yunque-text-muted)" }}>{item.summary}</div>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  <Link href={`/packs/detail?id=${encodeURIComponent(item.id)}`}>
                    <Button size="sm" variant="ghost">
                      详情 <ArrowRight size={12} />
                    </Button>
                  </Link>
                  <Link href={`/packs?q=${encodeURIComponent(item.id)}`}>
                    <Button size="sm" variant="ghost">
                      中心 <Search size={12} />
                    </Button>
                  </Link>
                </div>
              </div>
            );
          })}
        </div>
      </Card>
    );
  }

  return (
    <Card className="section-card overflow-hidden p-0">
      <div className="grid gap-0 lg:grid-cols-[280px_minmax(0,1fr)]">
        <div className="p-4" style={{ background: "rgba(255,255,255,0.025)", borderRight: "1px solid var(--yunque-border)" }}>
          <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
            <Icon size={16} aria-hidden={true} />
            {guide.title}
          </div>
          <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
            {guide.description}
          </div>
        </div>
        <div className="grid gap-3 p-4 md:grid-cols-2 xl:grid-cols-3">
          {guide.items.map((item) => {
            const tone = kindTone[item.kind] || kindTone["基础能力"];
            return (
              <div key={item.id} className="rounded-lg p-3" style={{ border: "1px solid var(--yunque-border)", background: "var(--yunque-bg-hover)" }}>
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{item.title}</div>
                    <div className="mt-1 truncate text-[11px]" style={{ color: "var(--yunque-text-muted)" }} translate="no">{item.id}</div>
                  </div>
                  <Chip size="sm" style={{ background: tone.bg, color: tone.fg, fontSize: "var(--text-2xs)" }}>{item.kind}</Chip>
                </div>
                <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>{item.summary}</div>
                <div className="mt-3 flex flex-wrap gap-1.5">
                  {item.actions.map((action) => (
                    <span key={action} className="rounded-full px-2 py-1 text-[11px]" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
                      {action}
                    </span>
                  ))}
                </div>
                <div className="mt-3 flex flex-wrap gap-2">
                  <Link href={`/packs/detail?id=${encodeURIComponent(item.id)}`}>
                    <Button size="sm" variant="outline">
                      查看详情 <ArrowRight size={13} />
                    </Button>
                  </Link>
                  <Link href={`/packs?q=${encodeURIComponent(item.id)}`}>
                    <Button size="sm" variant="ghost">
                      回中心 <Search size={13} />
                    </Button>
                  </Link>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </Card>
  );
}
