"use client";

import { useEffect, useState, useCallback } from "react";
import { api, type TriggerItem } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  Zap,
  Plus,
  Trash2,
  Clock,
  Radio,
  FlaskConical,
  ToggleLeft,
  ToggleRight,
  ChevronDown,
  ChevronRight,
} from "lucide-react";

const kindIcon: Record<string, React.ReactNode> = {
  time: <Clock size={14} className="text-blue-400" />,
  event: <Radio size={14} className="text-green-400" />,
  condition: <FlaskConical size={14} className="text-amber-400" />,
};

const kindLabel: Record<string, string> = {
  time: "Time",
  event: "Event",
  condition: "Condition",
};

export default function TriggersPage() {
  const { t } = useI18n();
  const [triggers, setTriggers] = useState<TriggerItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [showCreate, setShowCreate] = useState(false);

  // Create form state
  const [form, setForm] = useState({
    name: "",
    kind: "event" as TriggerItem["kind"],
    event: "task_completed",
    event_filter: "",
    condition_expr: "",
    action_type: "log" as TriggerItem["action"]["type"],
    action_message: "",
  });

  const load = useCallback(async () => {
    try {
      const res = await api.getTriggers();
      setTriggers(res.triggers || []);
    } catch {
      /* ignore */
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const toggle = (id: string) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });

  const handleCreate = async () => {
    if (!form.name) return;
    try {
      await api.createTrigger({
        name: form.name,
        kind: form.kind,
        event: form.kind === "event" ? form.event : undefined,
        event_filter: form.kind === "event" ? form.event_filter : undefined,
        condition_expr: form.kind === "condition" ? form.condition_expr : undefined,
        action: {
          type: form.action_type,
          message: form.action_message || undefined,
        },
      } as Partial<TriggerItem>);
      setShowCreate(false);
      setForm({ name: "", kind: "event", event: "task_completed", event_filter: "", condition_expr: "", action_type: "log", action_message: "" });
      load();
    } catch {
      /* ignore */
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.deleteTrigger(id);
      load();
    } catch {
      /* ignore */
    }
  };

  return (
    <main className="max-w-4xl mx-auto px-6 pt-24 pb-16">
      <BlurFade delay={0.1}>
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <Zap size={22} style={{ color: "var(--accent)" }} />
            <h1 className="text-xl font-bold">{t("triggers.title")}</h1>
          </div>
          <button
            onClick={() => setShowCreate(!showCreate)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
            style={{ background: "var(--accent)", color: "#000" }}
          >
            <Plus size={14} />
            {t("triggers.create")}
          </button>
        </div>
      </BlurFade>

      {/* Create Form */}
      {showCreate && (
        <BlurFade delay={0.15}>
          <div
            className="rounded-xl border p-5 mb-6"
            style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
          >
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
              <div>
                <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
                  {t("triggers.name")}
                </label>
                <input
                  className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent"
                  style={{ borderColor: "var(--border)" }}
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                />
              </div>
              <div>
                <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
                  {t("triggers.kind")}
                </label>
                <select
                  className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent"
                  style={{ borderColor: "var(--border)" }}
                  value={form.kind}
                  onChange={(e) => setForm({ ...form, kind: e.target.value as TriggerItem["kind"] })}
                >
                  <option value="event">Event</option>
                  <option value="condition">Condition</option>
                </select>
              </div>
            </div>

            {form.kind === "event" && (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                <div>
                  <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
                    {t("triggers.event")}
                  </label>
                  <select
                    className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent"
                    style={{ borderColor: "var(--border)" }}
                    value={form.event}
                    onChange={(e) => setForm({ ...form, event: e.target.value })}
                  >
                    <option value="task_completed">task_completed</option>
                    <option value="task_failed">task_failed</option>
                    <option value="memory_updated">memory_updated</option>
                    <option value="cost_alert">cost_alert</option>
                    <option value="skill_installed">skill_installed</option>
                    <option value="channel_message">channel_message</option>
                    <option value="custom">custom</option>
                  </select>
                </div>
                <div>
                  <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
                    {t("triggers.filter")}
                  </label>
                  <input
                    className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent"
                    style={{ borderColor: "var(--border)" }}
                    value={form.event_filter}
                    onChange={(e) => setForm({ ...form, event_filter: e.target.value })}
                    placeholder={t("triggers.filterPlaceholder")}
                  />
                </div>
              </div>
            )}

            {form.kind === "condition" && (
              <div className="mb-4">
                <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
                  {t("triggers.condition")}
                </label>
                <input
                  className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent"
                  style={{ borderColor: "var(--border)" }}
                  value={form.condition_expr}
                  onChange={(e) => setForm({ ...form, condition_expr: e.target.value })}
                  placeholder="cost > 10"
                />
              </div>
            )}

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
              <div>
                <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
                  {t("triggers.actionType")}
                </label>
                <select
                  className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent"
                  style={{ borderColor: "var(--border)" }}
                  value={form.action_type}
                  onChange={(e) => setForm({ ...form, action_type: e.target.value as TriggerItem["action"]["type"] })}
                >
                  <option value="log">Log</option>
                  <option value="agent_turn">Agent Turn</option>
                  <option value="thread_post">Thread Post</option>
                  <option value="webhook">Webhook</option>
                </select>
              </div>
              <div>
                <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>
                  {t("triggers.actionMessage")}
                </label>
                <input
                  className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent"
                  style={{ borderColor: "var(--border)" }}
                  value={form.action_message}
                  onChange={(e) => setForm({ ...form, action_message: e.target.value })}
                />
              </div>
            </div>

            <button
              onClick={handleCreate}
              disabled={!form.name}
              className="px-4 py-2 rounded-lg text-xs font-medium transition-opacity disabled:opacity-40"
              style={{ background: "var(--accent)", color: "#000" }}
            >
              {t("triggers.save")}
            </button>
          </div>
        </BlurFade>
      )}

      {/* Trigger List */}
      {loading ? (
        <div className="text-center py-12" style={{ color: "var(--text-muted)" }}>
          Loading...
        </div>
      ) : triggers.length === 0 ? (
        <div className="text-center py-12" style={{ color: "var(--text-muted)" }}>
          {t("triggers.empty")}
        </div>
      ) : (
        <div className="space-y-3">
          {triggers.map((trig, i) => (
            <BlurFade key={trig.id} delay={0.1 + i * 0.03}>
              <div
                className="rounded-xl border overflow-hidden"
                style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
              >
                {/* Header */}
                <div
                  className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:opacity-80 transition-opacity"
                  onClick={() => toggle(trig.id)}
                >
                  {expanded.has(trig.id) ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                  {kindIcon[trig.kind]}
                  <div className="flex-1 min-w-0">
                    <span className="font-medium text-sm">{trig.name}</span>
                    <span className="text-xs ml-2" style={{ color: "var(--text-muted)" }}>
                      {kindLabel[trig.kind] || trig.kind}
                    </span>
                  </div>
                  <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                    <span className="text-xs" style={{ color: "var(--text-muted)" }}>
                      {trig.fire_count}x
                    </span>
                    {trig.enabled ? (
                      <ToggleRight size={18} className="text-green-400" />
                    ) : (
                      <ToggleLeft size={18} className="text-gray-500" />
                    )}
                    <button
                      onClick={() => handleDelete(trig.id)}
                      className="p-1 rounded hover:bg-white/10 transition-colors"
                    >
                      <Trash2 size={13} className="text-gray-400" />
                    </button>
                  </div>
                </div>

                {/* Detail */}
                {expanded.has(trig.id) && (
                  <div
                    className="px-4 py-3 border-t text-xs space-y-1"
                    style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}
                  >
                    {trig.event && <div><strong>{t("triggers.event")}:</strong> {trig.event}</div>}
                    {trig.event_filter && <div><strong>{t("triggers.filter")}:</strong> {trig.event_filter}</div>}
                    {trig.condition_expr && <div><strong>{t("triggers.condition")}:</strong> {trig.condition_expr}</div>}
                    <div><strong>{t("triggers.actionType")}:</strong> {trig.action.type}</div>
                    {trig.action.message && <div><strong>{t("triggers.actionMessage")}:</strong> {trig.action.message}</div>}
                    {trig.last_fired_at && <div><strong>{t("triggers.lastFired")}:</strong> {new Date(trig.last_fired_at).toLocaleString()}</div>}
                    <div><strong>ID:</strong> {trig.id}</div>
                  </div>
                )}
              </div>
            </BlurFade>
          ))}
        </div>
      )}
    </main>
  );
}
