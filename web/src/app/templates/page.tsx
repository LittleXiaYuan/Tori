"use client";

import { useEffect, useState, useCallback } from "react";
import {
  api,
  type TaskTemplate,
  type TemplateVar,
  type TemplateStep,
} from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { useI18n } from "@/lib/i18n";
import {
  Layers,
  Plus,
  Trash2,
  Play,
  ChevronDown,
  ChevronRight,
  Tag,
  Variable,
  CheckCircle2,
  X,
} from "lucide-react";

/* ── Create Dialog ── */

function CreateForm({
  onCreated,
  onCancel,
  t,
}: {
  onCreated: () => void;
  onCancel: () => void;
  t: (key: string) => string;
}) {
  const [name, setName] = useState("");
  const [desc, setDesc] = useState("");
  const [tags, setTags] = useState("");
  const [vars, setVars] = useState<TemplateVar[]>([]);
  const [steps, setSteps] = useState<TemplateStep[]>([{ action: "" }]);
  const [saving, setSaving] = useState(false);

  const addVar = () =>
    setVars([...vars, { name: "", required: false }]);
  const removeVar = (i: number) => setVars(vars.filter((_, j) => j !== i));
  const addStep = () => setSteps([...steps, { action: "" }]);
  const removeStep = (i: number) => setSteps(steps.filter((_, j) => j !== i));

  const submit = async () => {
    if (!name.trim()) return;
    setSaving(true);
    try {
      await api.createTemplate({
        name: name.trim(),
        description: desc.trim(),
        variables: vars.filter((v) => v.name.trim()),
        steps: steps.filter((s) => s.action.trim()),
        tags: tags
          .split(",")
          .map((t) => t.trim())
          .filter(Boolean),
      });
      onCreated();
    } catch {
      /* silent */
    } finally {
      setSaving(false);
    }
  };

  const inputStyle = {
    background: "var(--bg-main)",
    borderColor: "var(--border)",
    color: "var(--text-main)",
  };

  return (
    <div
      className="rounded-xl p-5 border mb-6 space-y-4"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
    >
      <div className="flex items-center justify-between">
        <span className="font-semibold">{t("templates.create")}</span>
        <button onClick={onCancel} className="p-1 rounded hover:bg-white/5">
          <X size={16} style={{ color: "var(--text-muted)" }} />
        </button>
      </div>

      {/* Name + Description */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("templates.name")}
          className="rounded-lg px-3 py-2 text-sm border"
          style={inputStyle}
        />
        <input
          value={desc}
          onChange={(e) => setDesc(e.target.value)}
          placeholder={t("templates.desc")}
          className="rounded-lg px-3 py-2 text-sm border"
          style={inputStyle}
        />
      </div>

      {/* Tags */}
      <input
        value={tags}
        onChange={(e) => setTags(e.target.value)}
        placeholder={t("templates.tags") + " (comma separated)"}
        className="w-full rounded-lg px-3 py-2 text-sm border"
        style={inputStyle}
      />

      {/* Variables */}
      <div>
        <div className="flex items-center gap-2 mb-2">
          <Variable size={14} style={{ color: "var(--accent)" }} />
          <span className="text-sm font-medium">{t("templates.variables")}</span>
          <button
            onClick={addVar}
            className="ml-auto text-xs px-2 py-0.5 rounded hover:bg-white/5 border"
            style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}
          >
            +
          </button>
        </div>
        {vars.map((v, i) => (
          <div key={i} className="flex gap-2 mb-2">
            <input
              value={v.name}
              onChange={(e) => {
                const next = [...vars];
                next[i] = { ...next[i], name: e.target.value };
                setVars(next);
              }}
              placeholder={t("templates.varName")}
              className="flex-1 rounded-lg px-2 py-1 text-sm border"
              style={inputStyle}
            />
            <input
              value={v.default || ""}
              onChange={(e) => {
                const next = [...vars];
                next[i] = { ...next[i], default: e.target.value };
                setVars(next);
              }}
              placeholder={t("templates.varDefault")}
              className="flex-1 rounded-lg px-2 py-1 text-sm border"
              style={inputStyle}
            />
            <label className="flex items-center gap-1 text-xs" style={{ color: "var(--text-muted)" }}>
              <input
                type="checkbox"
                checked={v.required}
                onChange={(e) => {
                  const next = [...vars];
                  next[i] = { ...next[i], required: e.target.checked };
                  setVars(next);
                }}
              />
              {t("templates.required")}
            </label>
            <button onClick={() => removeVar(i)} className="p-1 rounded hover:bg-white/5">
              <X size={14} style={{ color: "var(--text-muted)" }} />
            </button>
          </div>
        ))}
      </div>

      {/* Steps */}
      <div>
        <div className="flex items-center gap-2 mb-2">
          <Layers size={14} style={{ color: "var(--accent)" }} />
          <span className="text-sm font-medium">{t("templates.steps")}</span>
          <button
            onClick={addStep}
            className="ml-auto text-xs px-2 py-0.5 rounded hover:bg-white/5 border"
            style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}
          >
            +
          </button>
        </div>
        {steps.map((s, i) => (
          <div key={i} className="flex gap-2 mb-2">
            <span className="text-xs w-6 text-center pt-2" style={{ color: "var(--text-muted)" }}>
              {i + 1}
            </span>
            <input
              value={s.action}
              onChange={(e) => {
                const next = [...steps];
                next[i] = { ...next[i], action: e.target.value };
                setSteps(next);
              }}
              placeholder={`Step ${i + 1} action`}
              className="flex-1 rounded-lg px-2 py-1 text-sm border"
              style={inputStyle}
            />
            <input
              value={s.skill_name || ""}
              onChange={(e) => {
                const next = [...steps];
                next[i] = { ...next[i], skill_name: e.target.value };
                setSteps(next);
              }}
              placeholder="skill (optional)"
              className="w-32 rounded-lg px-2 py-1 text-sm border"
              style={inputStyle}
            />
            <button onClick={() => removeStep(i)} className="p-1 rounded hover:bg-white/5">
              <X size={14} style={{ color: "var(--text-muted)" }} />
            </button>
          </div>
        ))}
      </div>

      <button
        onClick={submit}
        disabled={saving || !name.trim()}
        className="px-4 py-2 rounded-lg text-sm font-medium transition"
        style={{
          background: "var(--accent)",
          color: "#fff",
          opacity: saving || !name.trim() ? 0.5 : 1,
        }}
      >
        {saving ? "..." : t("templates.create")}
      </button>
    </div>
  );
}

/* ── Instantiate Dialog ── */

function InstantiateDialog({
  tpl,
  onDone,
  onCancel,
  t,
}: {
  tpl: TaskTemplate;
  onDone: () => void;
  onCancel: () => void;
  t: (key: string) => string;
}) {
  const [values, setValues] = useState<Record<string, string>>(() => {
    const init: Record<string, string> = {};
    tpl.variables.forEach((v) => {
      init[v.name] = v.default || "";
    });
    return init;
  });
  const [running, setRunning] = useState(false);
  const [done, setDone] = useState(false);

  const submit = async () => {
    setRunning(true);
    try {
      await api.instantiateTemplate(tpl.id, values);
      setDone(true);
      setTimeout(() => onDone(), 1000);
    } catch {
      /* silent */
    } finally {
      setRunning(false);
    }
  };

  return (
    <div
      className="rounded-xl p-5 border mb-4 space-y-3"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
    >
      <div className="flex items-center justify-between">
        <span className="font-semibold text-sm">{t("templates.fillVars")}: {tpl.name}</span>
        <button onClick={onCancel} className="p-1 rounded hover:bg-white/5">
          <X size={16} style={{ color: "var(--text-muted)" }} />
        </button>
      </div>
      {tpl.variables.map((v) => (
        <div key={v.name} className="flex items-center gap-2">
          <span className="text-sm w-32 font-mono" style={{ color: "var(--accent)" }}>
            {"{{" + v.name + "}}"}
            {v.required && <span className="text-red-400">*</span>}
          </span>
          <input
            value={values[v.name] || ""}
            onChange={(e) => setValues({ ...values, [v.name]: e.target.value })}
            placeholder={v.description || v.name}
            className="flex-1 rounded-lg px-3 py-1.5 text-sm border"
            style={{
              background: "var(--bg-main)",
              borderColor: "var(--border)",
              color: "var(--text-main)",
            }}
          />
        </div>
      ))}
      <button
        onClick={submit}
        disabled={running || done}
        className="px-4 py-2 rounded-lg text-sm font-medium transition flex items-center gap-2"
        style={{
          background: done ? "#22c55e" : "var(--accent)",
          color: "#fff",
          opacity: running ? 0.5 : 1,
        }}
      >
        {done ? (
          <>
            <CheckCircle2 size={14} /> {t("templates.created")}
          </>
        ) : (
          <>
            <Play size={14} /> {t("templates.run")}
          </>
        )}
      </button>
    </div>
  );
}

/* ── Main Page ── */

export default function TemplatesPage() {
  const { t } = useI18n();
  const [templates, setTemplates] = useState<TaskTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [instantiate, setInstantiate] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.getTemplates();
      setTemplates(res.templates || []);
    } catch {
      /* silent */
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const handleDelete = async (id: string) => {
    try {
      await api.deleteTemplate(id);
      setTemplates(templates.filter((t) => t.id !== id));
    } catch {
      /* silent */
    }
  };

  const toggleExpand = (id: string) =>
    setExpanded((prev) => ({ ...prev, [id]: !prev[id] }));

  return (
    <div className="space-y-6 max-w-5xl mx-auto">
      {/* Header */}
      <BlurFade delay={0.05}>
        <div className="flex items-center gap-3 mb-2">
          <Layers size={24} style={{ color: "var(--accent)" }} />
          <h1 className="text-2xl font-bold">{t("templates.title")}</h1>
          <button
            onClick={() => setShowCreate(!showCreate)}
            className="ml-auto flex items-center gap-1 px-3 py-1.5 rounded-lg text-sm font-medium transition"
            style={{ background: "var(--accent)", color: "#fff" }}
          >
            <Plus size={14} /> {t("templates.create")}
          </button>
        </div>
      </BlurFade>

      {/* Create Form */}
      {showCreate && (
        <CreateForm
          t={t}
          onCreated={() => {
            setShowCreate(false);
            load();
          }}
          onCancel={() => setShowCreate(false)}
        />
      )}

      {/* Template List */}
      {templates.length === 0 && !loading && (
        <BlurFade delay={0.1}>
          <div
            className="text-center py-12 rounded-xl border"
            style={{
              background: "var(--bg-card)",
              borderColor: "var(--border)",
              color: "var(--text-muted)",
            }}
          >
            {t("templates.empty")}
          </div>
        </BlurFade>
      )}

      {templates.map((tpl, i) => (
        <BlurFade key={tpl.id} delay={0.1 + i * 0.03}>
          {/* Instantiate dialog */}
          {instantiate === tpl.id && (
            <InstantiateDialog
              tpl={tpl}
              t={t}
              onDone={() => {
                setInstantiate(null);
              }}
              onCancel={() => setInstantiate(null)}
            />
          )}
          <div
            className="rounded-xl border overflow-hidden"
            style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
          >
            {/* Header */}
            <div
              className="flex items-center gap-3 p-4 cursor-pointer"
              onClick={() => toggleExpand(tpl.id)}
            >
              {expanded[tpl.id] ? (
                <ChevronDown size={16} style={{ color: "var(--text-muted)" }} />
              ) : (
                <ChevronRight size={16} style={{ color: "var(--text-muted)" }} />
              )}
              <span className="font-medium">{tpl.name}</span>
              {tpl.description && (
                <span className="text-sm" style={{ color: "var(--text-muted)" }}>
                  — {tpl.description}
                </span>
              )}
              <div className="flex gap-1 ml-auto">
                {tpl.tags?.map((tag) => (
                  <span
                    key={tag}
                    className="inline-flex items-center gap-1 text-xs px-1.5 py-0.5 rounded"
                    style={{
                      background: "var(--accent-dim)",
                      color: "var(--accent)",
                    }}
                  >
                    <Tag size={10} /> {tag}
                  </span>
                ))}
              </div>
              <div className="flex gap-1" onClick={(e) => e.stopPropagation()}>
                <button
                  onClick={() => setInstantiate(tpl.id)}
                  className="p-1.5 rounded-lg hover:bg-white/5 transition"
                  title={t("templates.instantiate")}
                >
                  <Play size={14} style={{ color: "#22c55e" }} />
                </button>
                <button
                  onClick={() => handleDelete(tpl.id)}
                  className="p-1.5 rounded-lg hover:bg-white/5 transition"
                  title={t("templates.delete")}
                >
                  <Trash2 size={14} style={{ color: "#ef4444" }} />
                </button>
              </div>
            </div>

            {/* Expanded detail */}
            {expanded[tpl.id] && (
              <div
                className="px-4 pb-4 space-y-3 border-t"
                style={{ borderColor: "var(--border)" }}
              >
                {/* Variables */}
                {tpl.variables.length > 0 && (
                  <div className="mt-3">
                    <div className="text-xs font-medium mb-1" style={{ color: "var(--text-muted)" }}>
                      {t("templates.variables")}
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {tpl.variables.map((v) => (
                        <span
                          key={v.name}
                          className="inline-flex items-center gap-1 text-xs px-2 py-1 rounded font-mono"
                          style={{
                            background: "var(--bg-main)",
                            color: "var(--text-main)",
                          }}
                        >
                          <Variable size={10} style={{ color: "var(--accent)" }} />
                          {"{{" + v.name + "}}"}
                          {v.required && <span className="text-red-400">*</span>}
                          {v.default && (
                            <span style={{ color: "var(--text-muted)" }}>={v.default}</span>
                          )}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {/* Steps */}
                {tpl.steps.length > 0 && (
                  <div>
                    <div className="text-xs font-medium mb-1" style={{ color: "var(--text-muted)" }}>
                      {t("templates.steps")} ({tpl.steps.length})
                    </div>
                    <div className="space-y-1">
                      {tpl.steps.map((s, si) => (
                        <div
                          key={si}
                          className="flex items-center gap-2 text-sm px-3 py-1.5 rounded"
                          style={{ background: "var(--bg-main)" }}
                        >
                          <span className="text-xs w-5 text-center" style={{ color: "var(--text-muted)" }}>
                            {si + 1}
                          </span>
                          <span>{s.action}</span>
                          {s.skill_name && (
                            <span
                              className="text-xs px-1.5 py-0.5 rounded"
                              style={{
                                background: "var(--accent-dim)",
                                color: "var(--accent)",
                              }}
                            >
                              {s.skill_name}
                            </span>
                          )}
                          {s.group && s.group > 0 && (
                            <span
                              className="text-xs px-1.5 py-0.5 rounded"
                              style={{ background: "#a78bfa20", color: "#a78bfa" }}
                            >
                              group {s.group}
                            </span>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
        </BlurFade>
      ))}
    </div>
  );
}
