"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { api, type BotInfo } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { motion } from "motion/react";
import {
  Blocks, Circle, ArrowLeft, Settings2, ScanFace,
  Database, Radar, MailWarning,
} from "lucide-react";
import Link from "next/link";

const tabs = [
  { key: "overview", label: "Overview", icon: Settings2 },
  { key: "persona", label: "Persona", icon: ScanFace },
  { key: "memory", label: "Memory", icon: Database },
  { key: "heartbeat", label: "Heartbeat", icon: Radar },
  { key: "inbox", label: "Inbox", icon: MailWarning },
];

function OverviewTab({ bot }: { bot: BotInfo }) {
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <Field label="ID" value={bot.id} mono />
        <Field label="Status" value={bot.status || "idle"} />
        <Field label="Active" value={bot.is_active ? "Yes" : "No"} />
        <Field label="Created" value={new Date(bot.created_at).toLocaleString()} />
      </div>
      {bot.description && (
        <div>
          <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--text-muted)" }}>Description</div>
          <div className="text-sm p-3 rounded-lg" style={{ background: "var(--bg-hover)" }}>{bot.description}</div>
        </div>
      )}
      {bot.config && Object.keys(bot.config).length > 0 && (
        <div>
          <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--text-muted)" }}>Configuration</div>
          <pre className="text-xs p-3 rounded-lg overflow-auto font-mono" style={{ background: "var(--bg-hover)" }}>
            {JSON.stringify(bot.config, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="p-3 rounded-lg" style={{ background: "var(--bg-hover)" }}>
      <div className="text-[10px] font-medium uppercase tracking-wider mb-1" style={{ color: "var(--text-muted)" }}>{label}</div>
      <div className={`text-sm ${mono ? "font-mono" : ""} truncate`}>{value}</div>
    </div>
  );
}

function PlaceholderTab({ name }: { name: string }) {
  return (
    <div className="flex items-center justify-center py-16">
      <div className="text-sm" style={{ color: "var(--text-muted)" }}>
        {name} management — connect to backend to see data
      </div>
    </div>
  );
}

export default function BotDetailClient() {
  const params = useParams();
  const id = params.id as string;
  const [bot, setBot] = useState<BotInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState("overview");
  const [pillStyle, setPillStyle] = useState({ left: 0, width: 0 });
  const tabRefs = new Map<string, HTMLButtonElement>();

  useEffect(() => {
    api.getBot(id).then(setBot).catch(() => {}).finally(() => setLoading(false));
  }, [id]);

  useEffect(() => {
    const el = tabRefs.get(activeTab);
    if (el) {
      const parent = el.parentElement;
      if (parent) {
        const pRect = parent.getBoundingClientRect();
        const eRect = el.getBoundingClientRect();
        setPillStyle({ left: eRect.left - pRect.left, width: eRect.width });
      }
    }
  }, [activeTab]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin" style={{ borderColor: "var(--text-muted)", borderTopColor: "transparent" }} />
      </div>
    );
  }

  if (!bot) {
    return (
      <div className="flex flex-col items-center justify-center h-[60vh] gap-4">
        <Blocks size={48} style={{ color: "var(--text-muted)" }} />
        <p className="text-sm" style={{ color: "var(--text-muted)" }}>Bot not found</p>
        <Link href="/bots" className="text-xs underline cursor-pointer" style={{ color: "var(--text-muted)" }}>Back to bots</Link>
      </div>
    );
  }

  return (
    <div className="max-w-4xl">
      <BlurFade delay={0}>
        <div className="flex items-center gap-4 mb-6">
          <Link href="/bots" className="p-2 rounded-lg transition-colors cursor-pointer" style={{ color: "var(--text-muted)" }}>
            <ArrowLeft size={16} />
          </Link>
          <div className="flex items-center gap-3 min-w-0">
            <div className="w-10 h-10 rounded-full flex items-center justify-center text-sm font-bold shrink-0" style={{ background: "var(--bg-hover)", border: "1px solid var(--border)" }}>
              {bot.name.charAt(0).toUpperCase()}
            </div>
            <div className="min-w-0">
              <h1 className="text-xl font-semibold tracking-tight truncate">{bot.name}</h1>
              <div className="flex items-center gap-2 text-xs" style={{ color: "var(--text-muted)" }}>
                <Circle size={6} fill={bot.is_active ? "var(--text)" : "var(--text-muted)"} style={{ color: bot.is_active ? "var(--text)" : "var(--text-muted)" }} />
                {bot.is_active ? "Active" : "Inactive"}
                <span>·</span>
                <span className="font-mono">{bot.id.slice(0, 8)}</span>
              </div>
            </div>
          </div>
        </div>
      </BlurFade>

      <BlurFade delay={0.05}>
        <div className="relative flex items-center gap-0.5 mb-6 p-1 rounded-full border" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <motion.div
            className="absolute rounded-full"
            style={{ height: "calc(100% - 8px)", top: 4, background: "var(--bg-hover)" }}
            animate={{ left: pillStyle.left, width: pillStyle.width }}
            transition={{ type: "spring", stiffness: 350, damping: 30 }}
          />
          {tabs.map(({ key, label, icon: Icon }) => (
            <button
              key={key}
              ref={(el) => { if (el) tabRefs.set(key, el); }}
              onClick={() => setActiveTab(key)}
              className="relative flex items-center gap-1.5 px-4 py-2 rounded-full text-xs font-medium transition-colors cursor-pointer z-10 whitespace-nowrap"
              style={{ color: activeTab === key ? "var(--text)" : "var(--text-muted)" }}
            >
              <Icon size={12} />
              {label}
            </button>
          ))}
        </div>
      </BlurFade>

      <BlurFade delay={0.1}>
        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          {activeTab === "overview" && <OverviewTab bot={bot} />}
          {activeTab === "persona" && <PlaceholderTab name="Persona" />}
          {activeTab === "memory" && <PlaceholderTab name="Memory" />}
          {activeTab === "heartbeat" && <PlaceholderTab name="Heartbeat" />}
          {activeTab === "inbox" && <PlaceholderTab name="Inbox" />}
        </div>
      </BlurFade>
    </div>
  );
}
