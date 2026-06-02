"use client";

import { useState, useEffect, useCallback } from "react";
import { api, type PresetInfo, type SkillInfo } from "@/lib/api";
import type { ModelOption } from "@/components/model-selector-popup";
import { showErrorToast } from "@/components/toast-provider";
import { providerModelLabel, resolveDisplayedChatProvider } from "@/lib/provider-ui";

export interface ChatInitState {
  currentModel: string;
  currentModelId: string;
  availableModels: ModelOption[];
  setupNeeded: boolean;
  presets: PresetInfo[];
  activePreset: string;
  airiAvailable: boolean;
  heroSkills: SkillInfo[];
  setCurrentModel: (v: string) => void;
  setCurrentModelId: (v: string) => void;
  handleSwitchPreset: (presetId: string) => Promise<void>;
}

export function useChatInit(): ChatInitState {
  const [currentModel, setCurrentModel] = useState("");
  const [currentModelId, setCurrentModelId] = useState("");
  const [availableModels, setAvailableModels] = useState<ModelOption[]>([]);
  const [setupNeeded, setSetupNeeded] = useState(false);
  const [presets, setPresets] = useState<PresetInfo[]>([]);
  const [activePreset, setActivePreset] = useState("");
  const [airiAvailable, setAiriAvailable] = useState(false);
  const [heroSkills, setHeroSkills] = useState<SkillInfo[]>([]);

  useEffect(() => {
    api.skills().then((res) => setHeroSkills((res.skills || []).slice(0, 4))).catch(() => {});
  }, []);

  useEffect(() => {
    const t = typeof window !== "undefined" ? localStorage.getItem("yunque_token") || "" : "";
    const k = typeof window !== "undefined" ? localStorage.getItem("yunque_api_key") || "" : "";
    const ah: Record<string, string> = t ? { Authorization: `Bearer ${t}` } : k ? { "X-API-Key": k } : {};
    fetch("/v1/plugins/ui", { headers: ah }).then(r => r.json()).then((data: unknown) => {
      const tabs = (data as Record<string, unknown>)?.tabs || data || [];
      if (Array.isArray(tabs) && tabs.some((t: Record<string, unknown>) => t.key === "airi")) {
        fetch("/v1/ext/airi/status", { headers: ah }).then(r => r.json()).then(() => {
          setAiriAvailable(true);
        }).catch(() => {});
      }
    }).catch(() => {});
  }, []);

  useEffect(() => {
    Promise.all([
      api.providerList(),
      api.execProvider().catch(() => ({ exec_provider: "", available_providers: [] as string[] })),
    ]).then(([data, exec]) => {
      const providers = data.providers || [];
      setAvailableModels(providers.filter(p => p.type === "chat").map(p => ({
        id: p.id, model: p.model, display_name: p.display_name, enabled: p.enabled,
        type: p.id.split("-")[0] || p.id,
        tier: p.tier, capabilities: p.capabilities,
      })));
      const primary = resolveDisplayedChatProvider(
        providers.filter(p => p.type === "chat"),
        exec.exec_provider,
      );
      if (primary) {
        setCurrentModel(providerModelLabel(primary));
        setCurrentModelId(primary.id);
      }
    }).catch(() => {});
  }, []);

  useEffect(() => {
    api.checkSetup().then((chk) => {
      // TEMP 诊断：直接打印 setup_needed 字段，避免对象折叠看不到。
      console.log("[chat-init] setup_needed =", chk.setup_needed, "full=", JSON.stringify(chk));
      setSetupNeeded(chk.setup_needed);
    }).catch((e) => {
      console.warn("[chat-init] checkSetup failed", e);
    });
  }, []);

  useEffect(() => {
    api.getPresets().then((data) => {
      setPresets(data.presets || []);
      setActivePreset(data.active || "");
    }).catch(() => {});
  }, []);

  const handleSwitchPreset = useCallback(async (presetId: string) => {
    try {
      await api.switchPreset(presetId);
      setActivePreset(presetId);
    } catch (e) { showErrorToast(e, "切换预设失败，请稍后重试。"); }
  }, []);

  return {
    currentModel, currentModelId, availableModels,
    setupNeeded, presets, activePreset,
    airiAvailable, heroSkills,
    setCurrentModel, setCurrentModelId,
    handleSwitchPreset,
  };
}
