"use client";

import { useEffect, useState } from "react";
import { api, type PersonaSkill, type PresetInfo } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { AnimatedList } from "@/components/ui/animated-list";
import { Fingerprint, Plus, Trash2, Save, FileText, Sparkles, ToggleLeft, ToggleRight, Sticker } from "lucide-react";
import { useI18n } from "@/lib/i18n";

function buildPresetIdentity(preset: PresetInfo) {
  const lines = [
    `# ${preset.name}`,
    "",
    preset.description,
    "",
    "## 角色定位",
    `你当前采用 \"${preset.name}\" 预设与用户交流。`,
    preset.tone ? `语气特征：${preset.tone}` : "",
    preset.style ? `表达风格：${preset.style}` : "",
    preset.greeting ? `开场参考：${preset.greeting}` : "",
  ].filter(Boolean);

  return lines.join("\n");
}

function buildPresetSoul(preset: PresetInfo) {
  const lines = [
    "# 行为准则",
    "",
    preset.system_note || `遵循 ${preset.name} 预设的交流方式与执行边界。`,
    "",
    "## 执行偏好",
    preset.tone ? `- 保持 ${preset.tone} 的语气。` : "",
    preset.style ? `- 输出内容遵循 ${preset.style} 的表达风格。` : "",
    preset.greeting ? `- 在合适场景下可参考此开场白：${preset.greeting}` : "",
  ].filter(Boolean);

  return lines.join("\n");
}

export default function PersonaPage() {
  const { t } = useI18n();
  const featureLabels: Record<string, { label: string; desc: string }> = {
    emotion_enabled: { label: t("persona.emotionAnalysis"), desc: t("persona.emotionAnalysisDesc") },
    sticker_enabled: { label: t("persona.stickerSuggestions"), desc: t("persona.stickerSuggestionsDesc") },
  };
  const [identity, setIdentity] = useState("");
  const [soul, setSoul] = useState("");
  const [skills, setSkills] = useState<PersonaSkill[]>([]);
  const [presets, setPresets] = useState<PresetInfo[]>([]);
  const [activePreset, setActivePreset] = useState("default");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [newSkill, setNewSkill] = useState({ name: "", description: "", content: "" });
  const [showAdd, setShowAdd] = useState(false);
  const [showNewPreset, setShowNewPreset] = useState(false);
  const [newPreset, setNewPreset] = useState({ id: "", name: "", description: "", tone: "", style: "", greeting: "", system_note: "" });

  // Sticker management state
  type StickerEntry = { package_id: string; sticker_id: string; platform: string; emotion: string };
  type StickerMapData = Record<string, Record<string, StickerEntry[]>>;
  const [stickerData, setStickerData] = useState<StickerMapData>({});
  const [showStickerAdd, setShowStickerAdd] = useState(false);
  const [newSticker, setNewSticker] = useState({ platform: "line", emotion: "happy", package_id: "", sticker_id: "" });

  const load = async () => {
    try {
      const [p, s] = await Promise.all([api.getPersona(), api.getPersonaSkills()]);
      setIdentity(p.identity || "");
      setSoul(p.soul || "");
      setSkills(Array.isArray(s.skills) ? s.skills : []);
    } catch {
      /* offline */
    }
    try {
      const pr = await api.getPresets();
      setPresets(Array.isArray(pr.presets) ? pr.presets : []);
      setActivePreset(pr.active || "default");
    } catch {
      /* presets not available */
    }
    try {
      const sm = await api.getStickerMap();
      setStickerData(sm || {});
    } catch {
      /* stickers not available */
    }
    setLoading(false);
  };

  useEffect(() => { load(); }, []);

  const savePersona = async () => {
    setSaving(true);
    try {
      await api.updatePersona(identity, soul);
    } finally {
      setSaving(false);
    }
  };

  const addSkill = async () => {
    if (!newSkill.name) return;
    await api.addPersonaSkill(newSkill.name, newSkill.description, newSkill.content);
    setNewSkill({ name: "", description: "", content: "" });
    setShowAdd(false);
    load();
  };

  const removeSkill = async (name: string) => {
    await api.deletePersonaSkill(name);
    load();
  };

  const handleSwitchPreset = async (id: string) => {
    const preset = presets.find((item) => item.id === id);
    try {
      await api.switchPreset(id);
      setActivePreset(id);
      if (preset) {
        setIdentity(buildPresetIdentity(preset));
        setSoul(buildPresetSoul(preset));
      }
    } catch { /* ignore */ }
  };

  const handleToggleFeature = async (presetId: string, feature: string, current: boolean) => {
    const preset = presets.find((p) => p.id === presetId);
    if (!preset) return;
    const updated = { ...preset.features, [feature]: !current };
    try {
      await api.updatePresetFeatures(presetId, updated);
      setPresets((prev) =>
        prev.map((p) => (p.id === presetId ? { ...p, features: updated } : p))
      );
    } catch { /* ignore */ }
  };

  const handleCreatePreset = async () => {
    if (!newPreset.id || !newPreset.name) return;
    try {
      await api.createCustomPreset(newPreset);
      setNewPreset({ id: "", name: "", description: "", tone: "", style: "", greeting: "", system_note: "" });
      setShowNewPreset(false);
      load();
    } catch { /* ignore */ }
  };

  const handleDeletePreset = async (id: string) => {
    try {
      await api.deleteCustomPreset(id);
      if (activePreset === id) setActivePreset("default");
      load();
    } catch { /* ignore */ }
  };

  const builtinIds = new Set(["default", "business", "tech_expert", "butler", "girlfriend", "boyfriend", "family", "jarvis"]);

  const emotionOptions = ["happy", "sad", "angry", "surprised", "fearful", "disgusted", "neutral", "loving"];

  const handleAddSticker = async () => {
    if (!newSticker.sticker_id) return;
    const existing = stickerData[newSticker.platform]?.[newSticker.emotion] || [];
    await api.updateStickerMapping(newSticker.platform, newSticker.emotion, [
      ...existing.map((s) => ({ package_id: s.package_id, sticker_id: s.sticker_id })),
      { package_id: newSticker.package_id, sticker_id: newSticker.sticker_id },
    ]);
    setNewSticker({ ...newSticker, package_id: "", sticker_id: "" });
    setShowStickerAdd(false);
    const sm = await api.getStickerMap();
    setStickerData(sm || {});
  };

  const handleDeleteStickerGroup = async (platform: string, emotion: string) => {
    await api.deleteStickerMapping(platform, emotion);
    const sm = await api.getStickerMap();
    setStickerData(sm || {});
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
            <Fingerprint size={20} />
            <h1 className="text-xl font-semibold tracking-tight">{t("persona.title")}</h1>
          </div>
          <button
            onClick={savePersona}
            disabled={saving}
            className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all cursor-pointer"
            style={{ background: "var(--text)", color: "var(--bg)" }}
          >
            <Save size={14} />
            {saving ? t("persona.saving") : t("persona.save")}
          </button>
        </div>
      </BlurFade>

      <div className="grid gap-6">
        {/* Preset selector */}
        {presets.length > 0 && (
          <BlurFade delay={0.03}>
            <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-2">
                  <Sparkles size={14} style={{ color: "var(--text-muted)" }} />
                  <label className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>
                    {t("persona.preset")} ({presets.length})
                  </label>
                </div>
                <button
                  onClick={() => setShowNewPreset(!showNewPreset)}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer"
                  style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}
                >
                  <Plus size={12} />
                  {t("persona.custom")}
                </button>
              </div>

              {showNewPreset && (
                <div className="mb-4 p-4 rounded-lg border space-y-3" style={{ borderColor: "var(--border)", background: "var(--bg-hover)" }}>
                  <div className="grid grid-cols-2 gap-3">
                    <input value={newPreset.id} onChange={(e) => setNewPreset({ ...newPreset, id: e.target.value })}
                      placeholder={t("persona.idUnique")} className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
                    <input value={newPreset.name} onChange={(e) => setNewPreset({ ...newPreset, name: e.target.value })}
                      placeholder={t("persona.name")} className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
                  </div>
                  <input value={newPreset.description} onChange={(e) => setNewPreset({ ...newPreset, description: e.target.value })}
                    placeholder={t("persona.description")} className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
                  <div className="grid grid-cols-2 gap-3">
                    <input value={newPreset.tone} onChange={(e) => setNewPreset({ ...newPreset, tone: e.target.value })}
                      placeholder={t("persona.tone")} className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
                    <input value={newPreset.style} onChange={(e) => setNewPreset({ ...newPreset, style: e.target.value })}
                      placeholder={t("persona.style")} className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
                  </div>
                  <input value={newPreset.greeting} onChange={(e) => setNewPreset({ ...newPreset, greeting: e.target.value })}
                    placeholder={t("persona.greeting")} className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none" style={{ borderColor: "var(--border)" }} />
                  <textarea value={newPreset.system_note} onChange={(e) => setNewPreset({ ...newPreset, system_note: e.target.value })}
                    placeholder={t("persona.systemNote")} className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm resize-none focus:outline-none" style={{ borderColor: "var(--border)", minHeight: 60 }} />
                  <div className="flex gap-2 justify-end">
                    <button onClick={() => setShowNewPreset(false)} className="px-3 py-1.5 text-xs rounded-lg cursor-pointer" style={{ color: "var(--text-muted)" }}>{t("persona.cancel")}</button>
                    <button onClick={handleCreatePreset} className="px-3 py-1.5 text-xs rounded-lg font-medium cursor-pointer" style={{ background: "var(--text)", color: "var(--bg)" }}>{t("persona.create")}</button>
                  </div>
                </div>
              )}

              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2 mb-4">
                {presets.map((p) => (
                  <button
                    key={p.id}
                    onClick={() => handleSwitchPreset(p.id)}
                    className="p-3 rounded-lg border text-left transition-all cursor-pointer relative group"
                    style={{
                      borderColor: p.id === activePreset ? "var(--text)" : "var(--border)",
                      background: p.id === activePreset ? "var(--bg-hover)" : "transparent",
                    }}
                  >
                    <div className="text-sm font-medium truncate">{p.name}</div>
                    <div className="text-xs mt-0.5 truncate" style={{ color: "var(--text-muted)" }}>{p.description}</div>
                    {!builtinIds.has(p.id) && (
                      <span
                        onClick={(e) => { e.stopPropagation(); handleDeletePreset(p.id); }}
                        className="absolute top-1 right-1 opacity-0 group-hover:opacity-100 transition-opacity p-1 rounded cursor-pointer"
                        style={{ color: "var(--text-muted)" }}
                      >
                        <Trash2 size={12} />
                      </span>
                    )}
                  </button>
                ))}
              </div>

              {/* Feature toggles for active preset */}
              {(() => {
                const active = presets.find((p) => p.id === activePreset);
                if (!active) return null;
                return (
                  <div className="border-t pt-4 space-y-3" style={{ borderColor: "var(--border)" }}>
                    {/* Show system note if preset has one */}
                    {active.system_note && (
                      <div className="p-3 rounded-lg" style={{ background: "var(--bg-hover)" }}>
                        <div className="text-xs font-medium mb-1" style={{ color: "var(--text-muted)" }}>
                          {t("persona.presetNote")}
                        </div>
                        <div className="text-sm whitespace-pre-wrap">{active.system_note}</div>
                        <div className="text-xs mt-2" style={{ color: "var(--text-muted)" }}>
                          {t("persona.presetNoteHint")}
                        </div>
                      </div>
                    )}
                    <div className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>
                      {t("persona.features")} — {active.name}
                    </div>
                    {Object.entries(featureLabels).map(([key, meta]) => {
                      const enabled = active.features?.[key] !== false;
                      return (
                        <div key={key} className="flex items-center justify-between py-1">
                          <div>
                            <div className="text-sm">{meta.label}</div>
                            <div className="text-xs" style={{ color: "var(--text-muted)" }}>{meta.desc}</div>
                          </div>
                          <button
                            onClick={() => handleToggleFeature(activePreset, key, enabled)}
                            className="cursor-pointer transition-colors"
                            style={{ color: enabled ? "var(--text)" : "var(--text-muted)" }}
                          >
                            {enabled ? <ToggleRight size={24} /> : <ToggleLeft size={24} />}
                          </button>
                        </div>
                      );
                    })}
                  </div>
                );
              })()}
            </div>
          </BlurFade>
        )}

        <BlurFade delay={0.05}>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <label className="text-xs font-medium uppercase tracking-wider mb-3 block" style={{ color: "var(--text-muted)" }}>
              {t("persona.identity")}
            </label>
            <textarea
              value={identity}
              onChange={(e) => setIdentity(e.target.value)}
              className="w-full bg-transparent border rounded-lg p-3 text-sm resize-none focus:outline-none focus:ring-1 transition-all"
              style={{ borderColor: "var(--border)", minHeight: 120 }}
              placeholder={t("persona.identityPlaceholder")}
            />
          </div>
        </BlurFade>

        <BlurFade delay={0.1}>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <label className="text-xs font-medium uppercase tracking-wider mb-3 block" style={{ color: "var(--text-muted)" }}>
              {t("persona.soul")}
            </label>
            <textarea
              value={soul}
              onChange={(e) => setSoul(e.target.value)}
              className="w-full bg-transparent border rounded-lg p-3 text-sm resize-none focus:outline-none focus:ring-1 transition-all"
              style={{ borderColor: "var(--border)", minHeight: 120 }}
              placeholder={t("persona.soulPlaceholder")}
            />
          </div>
        </BlurFade>

        <BlurFade delay={0.15}>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center justify-between mb-4">
              <label className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>
                {t("persona.skills")} ({skills.length})
              </label>
              <button
                onClick={() => setShowAdd(!showAdd)}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer"
                style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}
              >
                <Plus size={12} />
                {t("persona.addSkill")}
              </button>
            </div>

            {showAdd && (
              <div className="mb-4 p-4 rounded-lg border space-y-3" style={{ borderColor: "var(--border)", background: "var(--bg-hover)" }}>
                <input
                  value={newSkill.name}
                  onChange={(e) => setNewSkill({ ...newSkill, name: e.target.value })}
                  placeholder={t("persona.skillName")}
                  className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none"
                  style={{ borderColor: "var(--border)" }}
                />
                <input
                  value={newSkill.description}
                  onChange={(e) => setNewSkill({ ...newSkill, description: e.target.value })}
                  placeholder={t("persona.skillDesc")}
                  className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none"
                  style={{ borderColor: "var(--border)" }}
                />
                <textarea
                  value={newSkill.content}
                  onChange={(e) => setNewSkill({ ...newSkill, content: e.target.value })}
                  placeholder={t("persona.skillContent")}
                  className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm resize-none focus:outline-none"
                  style={{ borderColor: "var(--border)", minHeight: 80 }}
                />
                <div className="flex gap-2 justify-end">
                  <button onClick={() => setShowAdd(false)} className="px-3 py-1.5 text-xs rounded-lg cursor-pointer" style={{ color: "var(--text-muted)" }}>{t("persona.cancel")}</button>
                  <button onClick={addSkill} className="px-3 py-1.5 text-xs rounded-lg font-medium cursor-pointer" style={{ background: "var(--text)", color: "var(--bg)" }}>{t("persona.create")}</button>
                </div>
              </div>
            )}

            {skills.length === 0 ? (
              <div className="text-sm text-center py-8" style={{ color: "var(--text-muted)" }}>{t("persona.noSkills")}</div>
            ) : (
              <AnimatedList>
                {skills.map((s) => (
                  <div key={s.name} className="flex items-center justify-between p-3 rounded-lg transition-colors" style={{ background: "var(--bg-hover)" }}>
                    <div className="flex items-center gap-3 min-w-0">
                      <FileText size={14} style={{ color: "var(--text-muted)" }} />
                      <div className="min-w-0">
                        <div className="text-sm font-medium truncate">{s.name}</div>
                        <div className="text-xs truncate" style={{ color: "var(--text-muted)" }}>{s.description || t("persona.noDesc")}</div>
                      </div>
                    </div>
                    <button
                      onClick={() => removeSkill(s.name)}
                      className="p-1.5 rounded-lg transition-colors cursor-pointer shrink-0"
                      style={{ color: "var(--text-muted)" }}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
              </AnimatedList>
            )}
          </div>
        </BlurFade>

        {/* Sticker Mapping Management */}
        <BlurFade delay={0.2}>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center justify-between mb-4">
              <label className="text-xs font-medium uppercase tracking-wider flex items-center gap-2" style={{ color: "var(--text-muted)" }}>
                <Sticker size={14} /> {t("persona.stickerMappings")}
              </label>
              <button
                onClick={() => setShowStickerAdd(!showStickerAdd)}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer"
                style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}
              >
                <Plus size={12} />
                {t("persona.add")}
              </button>
            </div>

            {showStickerAdd && (
              <div className="mb-4 p-4 rounded-lg border space-y-3" style={{ borderColor: "var(--border)", background: "var(--bg-hover)" }}>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>{t("persona.platform")}</label>
                    <select
                      value={newSticker.platform}
                      onChange={(e) => setNewSticker({ ...newSticker, platform: e.target.value })}
                      className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none"
                      style={{ borderColor: "var(--border)" }}
                    >
                      <option value="line">LINE</option>
                      <option value="telegram">Telegram</option>
                      <option value="discord">Discord</option>
                      <option value="wechat">WeChat</option>
                      <option value="qq">QQ</option>
                    </select>
                  </div>
                  <div>
                    <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>{t("persona.emotion")}</label>
                    <select
                      value={newSticker.emotion}
                      onChange={(e) => setNewSticker({ ...newSticker, emotion: e.target.value })}
                      className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none"
                      style={{ borderColor: "var(--border)" }}
                    >
                      {emotionOptions.map((e) => <option key={e} value={e}>{e}</option>)}
                    </select>
                  </div>
                </div>
                <input
                  value={newSticker.package_id}
                  onChange={(e) => setNewSticker({ ...newSticker, package_id: e.target.value })}
                  placeholder="Package ID (e.g. 11537)"
                  className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none"
                  style={{ borderColor: "var(--border)" }}
                />
                <input
                  value={newSticker.sticker_id}
                  onChange={(e) => setNewSticker({ ...newSticker, sticker_id: e.target.value })}
                  placeholder="Sticker ID (e.g. 52002734)"
                  className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none"
                  style={{ borderColor: "var(--border)" }}
                />
                <div className="flex gap-2 justify-end">
                  <button onClick={() => setShowStickerAdd(false)} className="px-3 py-1.5 text-xs rounded-lg cursor-pointer" style={{ color: "var(--text-muted)" }}>{t("persona.cancel")}</button>
                  <button onClick={handleAddSticker} className="px-3 py-1.5 text-xs rounded-lg font-medium cursor-pointer" style={{ background: "var(--text)", color: "var(--bg)" }}>{t("persona.add")}</button>
                </div>
              </div>
            )}

            {Object.keys(stickerData).length === 0 ? (
              <div className="text-sm text-center py-8" style={{ color: "var(--text-muted)" }}>{t("persona.noStickers")}</div>
            ) : (
              <div className="space-y-3">
                {Object.entries(stickerData).map(([platform, emotions]) => (
                  <div key={platform}>
                    <div className="text-xs font-semibold uppercase mb-2" style={{ color: "var(--text-muted)" }}>{platform}</div>
                    <div className="space-y-2">
                      {Object.entries(emotions).map(([emo, stickers]) => (
                        <div key={emo} className="flex items-center justify-between p-3 rounded-lg group" style={{ background: "var(--bg-hover)" }}>
                          <div className="min-w-0">
                            <span className="text-sm font-medium">{emo}</span>
                            <span className="text-xs ml-2" style={{ color: "var(--text-muted)" }}>
                              {stickers.length} {stickers.length !== 1 ? t("persona.stickers") : t("persona.sticker")}
                            </span>
                            <div className="flex gap-1 mt-1 flex-wrap">
                              {stickers.slice(0, 5).map((s) => (
                                <span key={s.sticker_id} className="text-xs px-2 py-0.5 rounded" style={{ background: "var(--bg-card)" }}>
                                  {s.package_id ? `${s.package_id}/${s.sticker_id}` : s.sticker_id}
                                </span>
                              ))}
                              {stickers.length > 5 && <span className="text-xs" style={{ color: "var(--text-muted)" }}>+{stickers.length - 5}</span>}
                            </div>
                          </div>
                          <button
                            onClick={() => handleDeleteStickerGroup(platform, emo)}
                            className="p-1.5 rounded-lg transition-colors cursor-pointer opacity-0 group-hover:opacity-100"
                            style={{ color: "var(--text-muted)" }}
                          >
                            <Trash2 size={14} />
                          </button>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </BlurFade>
      </div>
    </div>
  );
}
