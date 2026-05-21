"use client";

import { useState, useEffect, useCallback } from "react";

export type OnboardingPhase =
  | "welcome"
  | "setup-check"
  | "provider-setup"
  | "interactive-demo"
  | "mode-select"
  | "done";

interface SetupStatus {
  serverOnline: boolean;
  modelConfigured: boolean;
  ready: boolean;
}

const ONBOARDING_KEY = "yunque_onboarding_done";

export function useOnboarding() {
  const [phase, setPhase] = useState<OnboardingPhase>("welcome");
  const [visible, setVisible] = useState(false);
  const [setupStatus, setSetupStatus] = useState<SetupStatus>({
    serverOnline: false,
    modelConfigured: false,
    ready: false,
  });

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (localStorage.getItem(ONBOARDING_KEY)) return;
    const timer = setTimeout(() => setVisible(true), 600);
    return () => clearTimeout(timer);
  }, []);

  const checkSetup = useCallback(async () => {
    const status: SetupStatus = { serverOnline: false, modelConfigured: false, ready: false };
    try {
      const token = localStorage.getItem("yunque_token") || localStorage.getItem("yunque_api_key");
      const headers: Record<string, string> = { "Content-Type": "application/json" };
      if (token) headers["Authorization"] = `Bearer ${token}`;

      const vRes = await fetch("/v1/version", { headers });
      status.serverOnline = vRes.ok;

      if (status.serverOnline) {
        const pRes = await fetch("/api/providers", { headers });
        if (pRes.ok) {
          const data = await pRes.json();
          status.modelConfigured = Array.isArray(data?.providers) && data.providers.length > 0;
        }
      }
    } catch { /* offline */ }
    status.ready = status.serverOnline && status.modelConfigured;
    setSetupStatus(status);
    return status;
  }, []);

  const finish = useCallback(() => {
    setVisible(false);
    localStorage.setItem(ONBOARDING_KEY, "1");
    setPhase("done");
  }, []);

  const goToPhase = useCallback((p: OnboardingPhase) => setPhase(p), []);

  return { phase, setPhase: goToPhase, visible, setupStatus, checkSetup, finish };
}
