"use client";

import { useState, useEffect, useCallback } from "react";
import { BASE, getAuthHeaders } from "@/lib/api-core";

export type OnboardingPhase =
  | "welcome"
  | "setup-check"
  | "provider-setup"
  | "interactive-demo"
  | "done";

interface SetupStatus {
  serverOnline: boolean;
  modelConfigured: boolean;
  ready: boolean;
}

// localStorage is now a fast-path cache; the source of truth is the backend
// Ledger (GET/POST /v1/onboarding/state), so the guide shows exactly once per
// install and stays consistent across web/desktop and devices.
const ONBOARDING_KEY = "yunque_onboarding_done";

async function fetchOnboardingCompleted(): Promise<boolean | null> {
  try {
    const res = await fetch(`${BASE}/v1/onboarding/state`, {
      headers: getAuthHeaders(),
      cache: "no-store",
    });
    if (!res.ok) return null;
    const data = (await res.json()) as { completed?: boolean };
    return Boolean(data?.completed);
  } catch {
    return null; // backend unreachable — caller falls back to local behavior
  }
}

/** Persist onboarding completion to the backend Ledger (and cache locally).
 *  Exported so both finish() and the interactive-demo shortcut stay in sync. */
export async function markOnboardingComplete(): Promise<void> {
  if (typeof window !== "undefined") {
    localStorage.setItem(ONBOARDING_KEY, "1");
  }
  try {
    await fetch(`${BASE}/v1/onboarding/state`, {
      method: "POST",
      headers: { "Content-Type": "application/json", ...getAuthHeaders() },
      body: JSON.stringify({ completed: true }),
    });
  } catch {
    /* best effort — local cache still prevents re-show on this client */
  }
}

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

    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const show = () => {
      timer = setTimeout(() => {
        if (!cancelled) setVisible(true);
      }, 600);
    };
    (async () => {
      // Ledger is the source of truth — query it first so a *stale* local flag
      // (e.g. left on the desktop from a prior session) can't wrongly suppress
      // the guide.
      const completed = await fetchOnboardingCompleted();
      if (cancelled) return;
      if (completed === true) {
        localStorage.setItem(ONBOARDING_KEY, "1"); // cache server truth
        return;
      }
      if (completed === false) {
        localStorage.removeItem(ONBOARDING_KEY); // clear any stale cache
        show();
        return;
      }
      // completed === null → backend unreachable; fall back to local cache.
      if (!localStorage.getItem(ONBOARDING_KEY)) show();
    })();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, []);

  // Allow re-opening the guide on demand (command palette / settings), so it is
  // never permanently "missing" once dismissed — and so it can be replayed on
  // the desktop where localStorage persists across runs.
  useEffect(() => {
    if (typeof window === "undefined") return;
    const open = () => {
      setPhase("welcome");
      setVisible(true);
    };
    window.addEventListener("yunque:open-onboarding", open);
    return () => window.removeEventListener("yunque:open-onboarding", open);
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
    setPhase("done");
    void markOnboardingComplete();
  }, []);

  const goToPhase = useCallback((p: OnboardingPhase) => setPhase(p), []);

  return { phase, setPhase: goToPhase, visible, setupStatus, checkSetup, finish };
}
