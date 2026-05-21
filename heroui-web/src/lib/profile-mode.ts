export type ProfileMode = "easy" | "full";

export const PROFILE_MODE_KEY = "yunque_profile_mode";
export const DEFAULT_PROFILE_MODE: ProfileMode = "easy";

export function readProfileMode(): ProfileMode {
  if (typeof window === "undefined") return DEFAULT_PROFILE_MODE;
  try {
    return localStorage.getItem(PROFILE_MODE_KEY) === "full" ? "full" : DEFAULT_PROFILE_MODE;
  } catch {
    return DEFAULT_PROFILE_MODE;
  }
}

export function writeProfileMode(mode: ProfileMode): void {
  if (typeof window === "undefined") return;
  localStorage.setItem(PROFILE_MODE_KEY, mode);
  window.dispatchEvent(new StorageEvent("storage", { key: PROFILE_MODE_KEY, newValue: mode }));
}
