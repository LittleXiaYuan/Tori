"use client";

import { useEffect, useState, useCallback } from "react";
import { api, type PersonaSkill, type PresetInfo, type PersonaMemoryBlock } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { showToast } from "@/components/toast-provider";
import { Card, Button, Spinner, Switch, Select, ListBox, TextField, Input, Label, TextArea, Tabs } from "@heroui/react";
import { Plus, Save, Trash2, FileText, Smile, Brain, X } from "lucide-react";

function buildPresetIdentity(preset: PresetInfo) {
  return [
    `# ${preset.name}`,
    "",
    preset.description,
    "",
    "## 角色定位",
    `你当前采用“${preset.name}”预设与用户交流。`,
    preset.tone ? `语气特征：${preset.tone}` : "",
    preset.style ? `表达风格：${preset.style}` : "",
    preset.greeting ? `开场参考：${preset.greeting}` : "",
  ].filter(Boolean).join("\n");
}

function buildPresetSoul(preset: PresetInfo) {
  return [
    "# 行为准则",
    "",
    preset.system_note || `遵循 ${preset.name} 预设的交流方式与执行边界。`,
    "",
    "## 执行偏好",
    preset.tone ? `- 保持 ${preset.tone} 的语气。` : "",
    preset.style ? `- 输出内容遵循 ${preset.style} 的表达风格。` : "",
    preset.greeting ? `- 在合适场景下可参考此开场白：${preset.greeting}` : "",
  ].filter(Boolean).join("\n");
}

type StickerEntry = { package_id: string; sticker_id: string; platform: string; emotion: string };
type StickerMapData = Record<string, Record<string, StickerEntry[]>>;

const builtinIds = new Set(["default", "business", "tech_expert", "butler", "girlfriend", "boyfriend", "family", "jarvis"]);
const emotionOptions = ["happy", "sad", "angry", "surprised", "fearful", "disgusted", "neutral", "loving"];

export default function PersonaPage() {
  const { t } = useI18n();
  const [tab, setTab] = useState<"persona" | "memory">("persona");
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

  // Memory blocks
  const [memoryBlocks, setMemoryBlocks] = useState<PersonaMemoryBlock[]>([]);
  const [editingBlock, setEditingBlock] = useState<PersonaMemoryBlock | null>(null);
  const [newBlock, setNewBlock] = useState({ label: "user_preference", content: "" });

  // Sticker management
  const [stickerData, setStickerData] = useState<StickerMapData>({});
  const [showStickerAdd, setShowStickerAdd] = useState(false);
  const [newSticker, setNewSticker] = useState({ platform: "line", emotion: "happy", package_id: "", sticker_id: "" });

  const featureLabels: Record<string, { label: string; desc: string }> = {
    emotion_enabled: { label: t("persona.emotionAnalysis"), desc: t("persona.emotionAnalysisDesc") },
    sticker_enabled: { label: t("persona.stickerSuggestions"), desc: t("persona.stickerSuggestionsDesc") },
  };

  const load = useCallback(async () => {
    try {
      const [p, s] = await Promise.all([api.getPersona(), api.getPersonaSkills()]);
      setIdentity(p.identity || "");
      setSoul(p.soul || "");
      setSkills(Array.isArray(s.skills) ? s.skills : []);
    } catch { /* offline */ }
    try {
      const pr = await api.getPresets();
      setPresets(Array.isArray(pr.presets) ? pr.presets : []);
      setActivePreset(pr.active || "default");
    } catch { /* presets not available */ }
    try {
      const sm = await api.getStickerMap();
      setStickerData(sm || {});
    } catch { /* stickers not available */ }
    try {
      const mb = await api.getMemoryPersona();
      setMemoryBlocks(mb || []);
    } catch { /* memory not available */ }
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  const savePersona = async () => {
    setSaving(true);
    try { await api.updatePersona(identity, soul); showToast("已保存", "success"); }
    catch (e) { showToast(e instanceof Error ? e.message : "保存失败", "error"); }
    finally { setSaving(false); }
  };

  const addSkill = async () => {
    if (!newSkill.name) return;
    try {
      await api.addPersonaSkill(newSkill.name, newSkill.description, newSkill.content);
      setNewSkill({ name: "", description: "", content: "" });
      setShowAdd(false);
      load();
    } catch (e) { showToast(e instanceof Error ? e.message : "添加失败", "error"); }
  };

  const removeSkill = async (name: string) => {
    try { await api.deletePersonaSkill(name); load(); }
    catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); }
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
    } catch (e) { showToast(e instanceof Error ? e.message : "切换失败", "error"); }
  };

  const handleToggleFeature = async (presetId: string, feature: string, current: boolean) => {
    const preset = presets.find((p) => p.id === presetId);
    if (!preset) return;
    const updated = { ...preset.features, [feature]: !current };
    try {
      await api.updatePresetFeatures(presetId, updated);
      setPresets((prev) => prev.map((p) => (p.id === presetId ? { ...p, features: updated } : p)));
    } catch (e) { showToast(e instanceof Error ? e.message : "更新失败", "error"); }
  };

  const handleCreatePreset = async () => {
    if (!newPreset.id || !newPreset.name) return;
    try {
      await api.createCustomPreset(newPreset);
      setNewPreset({ id: "", name: "", description: "", tone: "", style: "", greeting: "", system_note: "" });
      setShowNewPreset(false);
      load();
    } catch (e) { showToast(e instanceof Error ? e.message : "创建失败", "error"); }
  };

  const handleDeletePreset = async (id: string) => {
    try {
      await api.deleteCustomPreset(id);
      if (activePreset === id) setActivePreset("default");
      load();
    } catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); }
  };

  const handleAddSticker = async () => {
    if (!newSticker.sticker_id) return;
    try {
      const existing = stickerData[newSticker.platform]?.[newSticker.emotion] || [];
      await api.updateStickerMapping(newSticker.platform, newSticker.emotion, [
        ...existing.map((s) => ({ package_id: s.package_id, sticker_id: s.sticker_id })),
        { package_id: newSticker.package_id, sticker_id: newSticker.sticker_id },
      ]);
      setNewSticker({ ...newSticker, package_id: "", sticker_id: "" });
      setShowStickerAdd(false);
      const sm = await api.getStickerMap();
      setStickerData(sm || {});
    } catch (e) { showToast(e instanceof Error ? e.message : "添加失败", "error"); }
  };

  const handleDeleteStickerGroup = async (platform: string, emotion: string) => {
    try {
      await api.deleteStickerMapping(platform, emotion);
      const sm = await api.getStickerMap();
      setStickerData(sm || {});
    } catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); }
  };

  const handleSaveBlock = async (id: string, label: string, content: string) => {
    try {
      await api.updateMemoryPersona({ id, label, content });
      setEditingBlock(null);
      setNewBlock({ label: "user_preference", content: "" });
      const mb = await api.getMemoryPersona();
      setMemoryBlocks(mb || []);
    } catch (e) { showToast(e instanceof Error ? e.message : "保存失败", "error"); }
  };

  const handleDeleteBlock = async (id: string, label: string) => {
    try {
      await api.updateMemoryPersona({ id, label, content: "" });
      const mb = await api.getMemoryPersona();
      setMemoryBlocks(mb || []);
    } catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Spinner size="lg" />
      </div>
    );
  }

  return (
    <div className="page-root animate-fade-in-up">
      {/* Header */}
      <div className="page-header mb-6">
        <div>
          <h1 className="page-title" style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{ color: "var(--yunque-accent)", display: "flex" }}><Brain size={20} /></span>
            {tab === "persona" ? t("persona.title") : "可编辑记忆"}
          </h1>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <Tabs selectedKey={tab} onSelectionChange={(k) => setTab(k as "persona" | "memory")}>
            <Tabs.ListContainer>
              <Tabs.List aria-label="人格 / 记忆">
                <Tabs.Tab id="persona">{t("persona.title")}<Tabs.Indicator /></Tabs.Tab>
                <Tabs.Tab id="memory"><Tabs.Separator />记忆<Tabs.Indicator /></Tabs.Tab>
              </Tabs.List>
            </Tabs.ListContainer>
          </Tabs>
          {tab === "persona" && (
            <Button size="sm" isPending={saving} onPress={savePersona} className="btn-accent">
              <Save size={14} /> {saving ? t("persona.saving") : t("persona.save")}
            </Button>
          )}
        </div>
      </div>

      {tab === "persona" && (
        <div className="space-y-6">
          {/* Preset Selector */}
          {presets.length > 0 && (
            <Card className="section-card p-5">
              <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-2" style={{ color: "var(--yunque-text-muted)" }}>
                  <Brain size={16} />
                  <span className="text-xs font-medium uppercase tracking-wider">{t("persona.preset")} ({presets.length})</span>
                </div>
                <Button size="sm" variant="ghost" onPress={() => setShowNewPreset(!showNewPreset)}>
                  <Plus size={12} /> {t("persona.custom")}
                </Button>
              </div>

              {/* New Preset Form */}
              {showNewPreset && (
                <div className="mb-4 p-4 rounded-lg border border-divider bg-default-50 space-y-3">
                  <div className="grid grid-cols-2 gap-3">
                    <TextField><Input value={newPreset.id} onChange={(e) => setNewPreset({ ...newPreset, id: e.target.value })} placeholder={t("persona.idUnique")} /></TextField>
                    <TextField><Input value={newPreset.name} onChange={(e) => setNewPreset({ ...newPreset, name: e.target.value })} placeholder={t("persona.name")} /></TextField>
                  </div>
                  <TextField><Input value={newPreset.description} onChange={(e) => setNewPreset({ ...newPreset, description: e.target.value })} placeholder={t("persona.description")} /></TextField>
                  <div className="grid grid-cols-2 gap-3">
                    <TextField><Input value={newPreset.tone} onChange={(e) => setNewPreset({ ...newPreset, tone: e.target.value })} placeholder={t("persona.tone")} /></TextField>
                    <TextField><Input value={newPreset.style} onChange={(e) => setNewPreset({ ...newPreset, style: e.target.value })} placeholder={t("persona.style")} /></TextField>
                  </div>
                  <TextField><Input value={newPreset.greeting} onChange={(e) => setNewPreset({ ...newPreset, greeting: e.target.value })} placeholder={t("persona.greeting")} /></TextField>
                  <TextField><TextArea value={newPreset.system_note} onChange={(e) => setNewPreset({ ...newPreset, system_note: e.target.value })} placeholder={t("persona.systemNote")} rows={3} /></TextField>
                  <div className="flex gap-2 justify-end">
                    <Button size="sm" variant="ghost" onPress={() => setShowNewPreset(false)}>{t("persona.cancel")}</Button>
                    <Button size="sm" onPress={handleCreatePreset} className="btn-accent">{t("persona.create")}</Button>
                  </div>
                </div>
              )}

              {/* Preset Grid */}
              <div className="kpi-grid mb-4">
                {presets.map((p) => (
                  <button
                    key={p.id}
                    onClick={() => handleSwitchPreset(p.id)}
                    className={`p-3 rounded-lg border text-left transition-all relative group ${p.id === activePreset ? "border-primary bg-primary/5" : "border-divider hover:border-default-300"}`}
                  >
                    <div className="text-sm font-medium truncate">{p.name}</div>
                    <div className="text-xs mt-0.5 truncate text-default-400">{p.description}</div>
                    {p.id === activePreset && (
                      <div className="absolute top-1.5 right-1.5 w-2 h-2 rounded-full bg-primary" />
                    )}
                    {!builtinIds.has(p.id) && (
                      <span
                        onClick={(e) => { e.stopPropagation(); handleDeletePreset(p.id); }}
                        className="absolute top-1 right-1 opacity-0 group-hover:opacity-100 transition-opacity p-1 rounded text-default-400 hover:text-danger"
                      >
                        <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                      </span>
                    )}
                  </button>
                ))}
              </div>

              {/* Feature Toggles for Active Preset */}
              {(() => {
                const active = presets.find((p) => p.id === activePreset);
                if (!active) return null;
                return (
                  <div className="border-t border-divider pt-4 space-y-3">
                    {active.system_note && (
                      <div className="p-3 rounded-lg bg-default-50">
                        <div className="text-xs font-medium mb-1 text-default-500">{t("persona.presetNote")}</div>
                        <div className="text-sm whitespace-pre-wrap">{active.system_note}</div>
                        <div className="text-xs mt-2 text-default-400">{t("persona.presetNoteHint")}</div>
                      </div>
                    )}
                    <div className="text-xs font-medium uppercase tracking-wider text-default-500">
                      {t("persona.features")} · {active.name}
                    </div>
                    {Object.entries(featureLabels).map(([key, meta]) => {
                      const enabled = active.features?.[key] !== false;
                      return (
                        <div key={key} className="flex items-center justify-between py-1">
                          <div>
                            <div className="text-sm">{meta.label}</div>
                            <div className="text-xs text-default-400">{meta.desc}</div>
                          </div>
                          <Switch isSelected={enabled} onChange={() => handleToggleFeature(activePreset, key, enabled)} size="sm">
                            <Switch.Control><Switch.Thumb /></Switch.Control>
                          </Switch>
                        </div>
                      );
                    })}
                  </div>
                );
              })()}
            </Card>
          )}

          {/* Identity */}
          <Card className="section-card p-5">
            <TextField name="identity" onChange={(v) => setIdentity(v)}>
              <Label className="text-xs font-medium uppercase tracking-wider mb-3">{t("persona.identity")}</Label>
              <TextArea
                value={identity}
                placeholder={t("persona.identityPlaceholder")}
                rows={5}
              />
            </TextField>
          </Card>

          {/* Soul */}
          <Card className="section-card p-5">
            <TextField name="soul" onChange={(v) => setSoul(v)}>
              <Label className="text-xs font-medium uppercase tracking-wider mb-3">{t("persona.soul")}</Label>
              <TextArea
                value={soul}
                placeholder={t("persona.soulPlaceholder")}
                rows={5}
              />
            </TextField>
          </Card>

          {/* Persona Skills */}
          <Card className="section-card p-5">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2" style={{ color: "var(--yunque-text-muted)" }}>
                <FileText size={16} />
                <span className="text-xs font-medium uppercase tracking-wider">{t("persona.skills")} ({skills.length})</span>
              </div>
              <Button size="sm" variant="ghost" onPress={() => setShowAdd(!showAdd)}>
                <Plus size={12} /> {t("persona.addSkill")}
              </Button>
            </div>

            {showAdd && (
              <div className="mb-4 p-4 rounded-lg border border-divider bg-default-50 space-y-3">
                <TextField><Input value={newSkill.name} onChange={(e) => setNewSkill({ ...newSkill, name: e.target.value })} placeholder={t("persona.skillName")} /></TextField>
                <TextField><Input value={newSkill.description} onChange={(e) => setNewSkill({ ...newSkill, description: e.target.value })} placeholder={t("persona.skillDesc")} /></TextField>
                <TextField><TextArea value={newSkill.content} onChange={(e) => setNewSkill({ ...newSkill, content: e.target.value })} placeholder={t("persona.skillContent")} rows={4} /></TextField>
                <div className="flex gap-2 justify-end">
                  <Button size="sm" variant="ghost" onPress={() => setShowAdd(false)}>{t("persona.cancel")}</Button>
                  <Button size="sm" onPress={addSkill} className="btn-accent">{t("persona.create")}</Button>
                </div>
              </div>
            )}

            {skills.length === 0 ? (
              <div className="text-sm text-center py-8 text-default-400">{t("persona.noSkills")}</div>
            ) : (
              <div className="space-y-2">
                {skills.map((s) => (
                  <div key={s.name} className="flex items-center justify-between p-3 rounded-lg bg-default-50 hover:bg-default-100 transition-colors">
                    <div className="flex items-center gap-3 min-w-0">
                      <svg className="w-3.5 h-3.5 text-default-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
                      <div className="min-w-0">
                        <div className="text-sm font-medium truncate">{s.name}</div>
                        <div className="text-xs truncate text-default-400">{s.description || t("persona.noDesc")}</div>
                        {s.content && <div className="text-xs truncate text-default-300 mt-0.5">{s.content.slice(0, 80)}...</div>}
                      </div>
                    </div>
                    <Button isIconOnly aria-label="删除" variant="ghost" size="sm" onPress={() => removeSkill(s.name)} style={{ color: "var(--yunque-text-muted)" }}>
                      <Trash2 size={14} />
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </Card>

          {/* Sticker Mappings */}
          <Card className="section-card p-5">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2" style={{ color: "var(--yunque-text-muted)" }}>
                <Smile size={16} />
                <span className="text-xs font-medium uppercase tracking-wider">{t("persona.stickerMappings")}</span>
              </div>
              <Button size="sm" variant="ghost" onPress={() => setShowStickerAdd(!showStickerAdd)}>
                <Plus size={12} /> {t("persona.add")}
              </Button>
            </div>

            {showStickerAdd && (
              <div className="mb-4 p-4 rounded-lg border border-divider bg-default-50 space-y-3">
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <Select selectedKey={newSticker.platform} onSelectionChange={(k) => setNewSticker({ ...newSticker, platform: String(k) })} className="w-full" aria-label={t("persona.platform")}>
                      <Label className="text-xs mb-1">{t("persona.platform")}</Label>
                      <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                      <Select.Popover>
                        <ListBox>
                          {[["line","LINE"],["telegram","Telegram"],["discord","Discord"],["wechat","WeChat"],["qq","QQ"]].map(([v,l]) => (
                            <ListBox.Item key={v} id={v} textValue={l}>{l}</ListBox.Item>
                          ))}
                        </ListBox>
                      </Select.Popover>
                    </Select>
                  </div>
                  <div>
                    <Select selectedKey={newSticker.emotion} onSelectionChange={(k) => setNewSticker({ ...newSticker, emotion: String(k) })} className="w-full" aria-label={t("persona.emotion")}>
                      <Label className="text-xs mb-1">{t("persona.emotion")}</Label>
                      <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                      <Select.Popover>
                        <ListBox>
                          {emotionOptions.map((e) => <ListBox.Item key={e} id={e} textValue={e}>{e}</ListBox.Item>)}
                        </ListBox>
                      </Select.Popover>
                    </Select>
                  </div>
                </div>
                <TextField><Input value={newSticker.package_id} onChange={(e) => setNewSticker({ ...newSticker, package_id: e.target.value })} placeholder="Package ID, e.g. 11537" /></TextField>
                <TextField><Input value={newSticker.sticker_id} onChange={(e) => setNewSticker({ ...newSticker, sticker_id: e.target.value })} placeholder="Sticker ID, e.g. 52002734" /></TextField>
                <div className="flex gap-2 justify-end">
                  <Button size="sm" variant="ghost" onPress={() => setShowStickerAdd(false)}>{t("persona.cancel")}</Button>
                  <Button size="sm" onPress={handleAddSticker} className="btn-accent">{t("persona.add")}</Button>
                </div>
              </div>
            )}

            {Object.keys(stickerData).length === 0 ? (
              <div className="text-sm text-center py-8 text-default-400">{t("persona.noStickers")}</div>
            ) : (
              <div className="space-y-4">
                {Object.entries(stickerData).map(([platform, emotions]) => (
                  <div key={platform}>
                    <div className="text-xs font-semibold uppercase mb-2 text-default-500">{platform}</div>
                    <div className="space-y-2">
                      {Object.entries(emotions).map(([emo, stickers]) => (
                        <div key={emo} className="flex items-center justify-between p-3 rounded-lg bg-default-50 group">
                          <div className="min-w-0">
                            <span className="text-sm font-medium">{emo}</span>
                            <span className="text-xs ml-2 text-default-400">
                              {stickers.length} {stickers.length !== 1 ? t("persona.stickers") : t("persona.sticker")}
                            </span>
                            <div className="flex gap-1 mt-1 flex-wrap">
                              {stickers.slice(0, 5).map((s) => (
                                <span key={s.sticker_id} className="text-xs px-2 py-0.5 rounded bg-default-100">
                                  {s.package_id ? `${s.package_id}/${s.sticker_id}` : s.sticker_id}
                                </span>
                              ))}
                              {stickers.length > 5 && <span className="text-xs text-default-400">+{stickers.length - 5}</span>}
                            </div>
                          </div>
                          <Button isIconOnly aria-label="删除" variant="ghost" size="sm" onPress={() => handleDeleteStickerGroup(platform, emo)}
                            className="opacity-0 group-hover:opacity-100 transition-all" style={{ color: "var(--yunque-text-muted)" }}>
                            <Trash2 size={14} />
                          </Button>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </Card>
        </div>
      )}

      {/* Memory Tab */}
      {tab === "memory" && (
        <div className="space-y-6">
          <Card className="section-card p-5">
              <div className="flex items-center gap-2 mb-4" style={{ color: "var(--yunque-text-muted)" }}>
                <Brain size={16} />
                <span className="text-xs font-medium uppercase tracking-wider">记忆块 ({memoryBlocks.length})</span>
              </div>

            {/* Add Block */}
              <div className="mb-6 p-4 rounded-lg border border-divider bg-default-50 space-y-3">
                <div className="text-xs font-medium">新增记忆</div>
                <div className="flex gap-2">
                  <Select selectedKey={newBlock.label} onSelectionChange={(k) => setNewBlock({ ...newBlock, label: String(k) })} className="w-[140px]" aria-label="记忆类型">
                    <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                    <Select.Popover>
                      <ListBox>
                        {[["user_preference","偏好"],["fact","事实"],["system","系统"],["persona","人格"],["human","人工"],["notes","备注"]].map(([v,l]) => (
                          <ListBox.Item key={v} id={v} textValue={l}>{l}</ListBox.Item>
                        ))}
                      </ListBox>
                    </Select.Popover>
                  </Select>
                  <Input value={newBlock.content} onChange={(e) => setNewBlock({ ...newBlock, content: e.target.value })}
                    placeholder="输入记忆内容…" className="flex-1"
                    onKeyDown={(e) => { if (e.key === "Enter" && newBlock.content) handleSaveBlock("", newBlock.label, newBlock.content); }} />
                  <Button size="sm" isDisabled={!newBlock.content} onPress={() => handleSaveBlock("", newBlock.label, newBlock.content)} className="btn-accent">添加</Button>
                </div>
              </div>

              {memoryBlocks.length === 0 ? (
                <div className="text-sm text-center py-8 text-default-400">暂无记忆块。</div>
              ) : (
              <div className="space-y-3">
                {memoryBlocks.map((mb) => (
                  <div key={mb.id} className="p-4 rounded-lg border border-divider bg-default-50 flex gap-4">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-2">
                        <span className="text-[10px] uppercase px-2 py-0.5 rounded-full font-medium bg-default-100 text-default-500">{mb.label}</span>
                        <span className="text-[10px] text-default-300">v{mb.version ?? 0}</span>
                        {mb.read_only && <span className="text-[10px] px-1.5 py-0.5 rounded bg-warning/10 text-warning">只读</span>}
                        {mb.max_chars > 0 && <span className="text-[10px] text-default-300">上限：{mb.max_chars}</span>}
                      </div>
                      {editingBlock?.id === mb.id ? (
                        <div className="flex gap-2">
                          <Input value={editingBlock.content} onChange={(e) => setEditingBlock({ ...editingBlock, content: e.target.value })} className="flex-1" />
                          <Button size="sm" variant="ghost" onPress={() => setEditingBlock(null)}>取消</Button>
                          <Button size="sm" onPress={() => handleSaveBlock(mb.id, mb.label, editingBlock.content)} className="btn-accent">保存</Button>
                        </div>
                      ) : (
                        <div className={`text-sm ${!mb.read_only ? "cursor-pointer hover:opacity-80" : ""}`}
                          onClick={() => !mb.read_only && setEditingBlock(mb)}>{mb.content}</div>
                      )}
                      {mb.updated_at && (
                        <div className="text-[10px] text-default-300 mt-1">更新于：{new Date(mb.updated_at).toLocaleString()}</div>
                      )}
                    </div>
                    {!mb.read_only && (
                      <Button isIconOnly aria-label="删除" size="sm" variant="ghost" onPress={() => handleDeleteBlock(mb.id, mb.label)}
                        className="h-fit text-default-400 hover:text-danger hover:bg-danger/10">
                        <Trash2 size={14} />
                      </Button>
                    )}
                  </div>
                ))}
              </div>
            )}
          </Card>
        </div>
      )}
    </div>
  );
}
