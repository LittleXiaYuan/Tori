"use client";

import { useEffect, useState } from "react";
import { api, type BotInfo } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { AnimatedList } from "@/components/ui/animated-list";
import { NumberTicker } from "@/components/ui/number-ticker";
import { Blocks, Plus, Trash2, Circle, ChevronRight } from "lucide-react";
import Link from "next/link";

export default function BotsPage() {
  const [bots, setBots] = useState<BotInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [active, setActive] = useState(0);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState({ name: "", description: "" });

  const load = async () => {
    try {
      const res = await api.getBots();
      setBots(res.bots || []);
      setTotal(res.total || 0);
      setActive(res.active || 0);
    } catch {
      /* offline */
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const create = async () => {
    if (!form.name) return;
    await api.createBot(form.name, form.description);
    setForm({ name: "", description: "" });
    setShowCreate(false);
    load();
  };

  const remove = async (id: string) => {
    await api.deleteBot(id);
    load();
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin" style={{ borderColor: "var(--text-muted)", borderTopColor: "transparent" }} />
      </div>
    );
  }

  return (
    <div className="max-w-4xl">
      <BlurFade delay={0}>
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-3">
            <Blocks size={20} />
            <h1 className="text-xl font-semibold tracking-tight">Bots</h1>
          </div>
          <button
            onClick={() => setShowCreate(!showCreate)}
            className="flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-medium transition-colors cursor-pointer"
            style={{ background: "var(--text)", color: "var(--bg)" }}
          >
            <Plus size={12} />
            New Bot
          </button>
        </div>
      </BlurFade>

      <BlurFade delay={0.05}>
        <div className="grid grid-cols-2 gap-4 mb-6">
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--text-muted)" }}>Total Bots</div>
            <div className="text-3xl font-bold"><NumberTicker value={total} /></div>
          </div>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--text-muted)" }}>Active</div>
            <div className="text-3xl font-bold"><NumberTicker value={active} /></div>
          </div>
        </div>
      </BlurFade>

      {showCreate && (
        <BlurFade delay={0}>
          <div className="rounded-xl border p-5 mb-6 space-y-3" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <input
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder="Bot name"
              className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none"
              style={{ borderColor: "var(--border)" }}
            />
            <input
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
              placeholder="Description (optional)"
              className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none"
              style={{ borderColor: "var(--border)" }}
            />
            <div className="flex justify-end gap-2">
              <button onClick={() => setShowCreate(false)} className="px-3 py-1.5 text-xs rounded-lg cursor-pointer" style={{ color: "var(--text-muted)" }}>Cancel</button>
              <button onClick={create} className="px-3 py-1.5 text-xs rounded-lg font-medium cursor-pointer" style={{ background: "var(--text)", color: "var(--bg)" }}>Create</button>
            </div>
          </div>
        </BlurFade>
      )}

      <BlurFade delay={0.1}>
        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          {bots.length === 0 ? (
            <div className="text-sm text-center py-12" style={{ color: "var(--text-muted)" }}>
              No bots created yet
            </div>
          ) : (
            <AnimatedList>
              {bots.map((bot) => (
                <div key={bot.id} className="flex items-center justify-between p-4 rounded-lg transition-colors" style={{ background: "var(--bg-hover)" }}>
                  <Link href={`/bots/${bot.id}`} className="flex items-center gap-3 min-w-0 flex-1 cursor-pointer">
                    <div className="relative shrink-0">
                      <Circle
                        size={8}
                        fill={bot.is_active ? "var(--text)" : "var(--text-muted)"}
                        style={{ color: bot.is_active ? "var(--text)" : "var(--text-muted)" }}
                      />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="text-sm font-medium">{bot.name}</div>
                      <div className="text-xs" style={{ color: "var(--text-muted)" }}>
                        {bot.description || bot.id}
                      </div>
                    </div>
                    <ChevronRight size={14} style={{ color: "var(--text-muted)" }} />
                  </Link>
                  <div className="flex items-center gap-2 shrink-0 ml-2">
                    <span className="text-[10px] px-2 py-0.5 rounded-full" style={{ background: "var(--bg-card)", color: "var(--text-muted)", border: "1px solid var(--border)" }}>
                      {bot.status || "idle"}
                    </span>
                    <button
                      onClick={(e) => { e.preventDefault(); remove(bot.id); }}
                      className="p-1.5 rounded-lg transition-colors cursor-pointer"
                      style={{ color: "var(--text-muted)" }}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>
              ))}
            </AnimatedList>
          )}
        </div>
      </BlurFade>
    </div>
  );
}
