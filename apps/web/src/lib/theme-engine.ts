/**
 * Shared theme engine.
 *
 * Both the classic dashboard (`/settings/theme`) and the Cherry settings
 * overlay call into `applyTheme()` so that accent/radius/shadow/bg vars
 * stay in lockstep. Flipping just `html.classList` is not enough — older
 * user configs may have set translucent `--yunque-bg`, `--yunque-bg-overlay`,
 * and a body background-image that need to be re-evaluated whenever the
 * preset or any token changes.
 */
import { isSafeAssetURL } from "@/lib/safe-url";

export type ColorThemeId =
  | "time_monologue"
  | "deep_sea"
  | "purple_jade"
  | "mint_ice"
  | "sakura_fall"
  | "gold_sand"
  | "custom";

export interface ThemeConfig {
  presetTheme: string; // "auto" | "dark" | "light"
  colorTheme: string; // ColorThemeId
  customColor: string;
  radius: string; // "right" | "default" | "small" | "medium" | "large"
  sidebarOpacity: number;
  contentOpacity: number;
  interfaceBgImage: string | null;
  interfaceBgOpacity: number;
  interfaceBgBlur: number;
  shadowColor: string;
  shadowOpacity: number;
  logoImage: string | null;
  faviconImage: string | null;
  homeMode: string;
  homeFontSize: number;
  loginBgImage: string | null;
  loginContentOpacity: number;
}

export const DEFAULT_THEME: ThemeConfig = {
  presetTheme: "dark",
  colorTheme: "deep_sea",
  customColor: "#0284c7",
  radius: "default",
  sidebarOpacity: 100,
  contentOpacity: 100,
  interfaceBgImage: null,
  interfaceBgOpacity: 30,
  interfaceBgBlur: 8,
  shadowColor: "#0f172a",
  shadowOpacity: 8,
  logoImage: null,
  faviconImage: null,
  homeMode: "card",
  homeFontSize: 28,
  loginBgImage: null,
  loginContentOpacity: 100,
};

export const COLOR_THEMES: { id: string; name: string; color: string }[] = [
  { id: "time_monologue", name: "时光独白", color: "#a1a1aa" },
  { id: "deep_sea", name: "深海微光", color: "#0284c7" },
  { id: "purple_jade", name: "紫玉幻境", color: "#a855f7" },
  { id: "mint_ice", name: "薄荷冰蓝", color: "#2dd4bf" },
  { id: "sakura_fall", name: "落樱飞雪", color: "#f472b6" },
  { id: "gold_sand", name: "流金岁月", color: "#d97706" },
];

export const RADIUS_OPTIONS: { id: string; name: string }[] = [
  { id: "right", name: "直角" },
  { id: "default", name: "默认" },
  { id: "small", name: "小" },
  { id: "medium", name: "中" },
  { id: "large", name: "大" },
];

export const THEME_STORAGE_KEY = "yunque_theme";

export function loadTheme(): ThemeConfig {
  if (typeof window === "undefined") return DEFAULT_THEME;
  try {
    const s = localStorage.getItem(THEME_STORAGE_KEY);
    return s ? { ...DEFAULT_THEME, ...(JSON.parse(s) as Partial<ThemeConfig>) } : DEFAULT_THEME;
  } catch {
    return DEFAULT_THEME;
  }
}

export function saveTheme(config: ThemeConfig): void {
  if (typeof window === "undefined") return;
  localStorage.setItem(THEME_STORAGE_KEY, JSON.stringify(config));
}

/* ---------------- colour helpers ---------------- */

function hexToRgb(hex: string): { r: number; g: number; b: number } {
  const h = hex.replace("#", "");
  return {
    r: parseInt(h.slice(0, 2), 16),
    g: parseInt(h.slice(2, 4), 16),
    b: parseInt(h.slice(4, 6), 16),
  };
}

function darkenHex(hex: string, amount: number): string {
  const { r, g, b } = hexToRgb(hex);
  const f = 1 - amount;
  const clamp = (v: number) => Math.max(0, Math.min(255, Math.round(v * f)));
  return `#${clamp(r).toString(16).padStart(2, "0")}${clamp(g).toString(16).padStart(2, "0")}${clamp(b)
    .toString(16)
    .padStart(2, "0")}`;
}

/* ---------------- WCAG contrast helpers ---------------- */

function sRGBtoLinear(c: number): number {
  const s = c / 255;
  return s <= 0.04045 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
}

function relativeLuminance(hex: string): number {
  const { r, g, b } = hexToRgb(hex);
  return 0.2126 * sRGBtoLinear(r) + 0.7152 * sRGBtoLinear(g) + 0.0722 * sRGBtoLinear(b);
}

function contrastRatio(hex1: string, hex2: string): number {
  const l1 = relativeLuminance(hex1);
  const l2 = relativeLuminance(hex2);
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

/**
 * Check WCAG AA contrast for the accent colour against the current theme
 * background. Returns a warning string if the ratio is below 3:1 (the
 * minimum for UI components and large text), or null if acceptable.
 */
export function checkAccentContrast(accentHex: string, mode: "dark" | "light"): string | null {
  const bg = mode === "light" ? "#ffffff" : "#0a0a0c";
  const ratio = contrastRatio(accentHex, bg);
  if (ratio < 3) {
    return `WCAG warning: accent ${accentHex} vs ${mode} background has contrast ratio ${ratio.toFixed(2)}:1 (minimum 3:1 for UI components)`;
  }
  return null;
}

/* ---------------- apply ---------------- */

/**
 * Ask the Tauri host to re-apply the platform-native window appearance
 * (NSAppearance + vibrancy on macOS, set_theme + acrylic tint on Windows)
 * for every window we own. Silently no-ops in a plain browser tab.
 *
 * We intentionally read `__TAURI_INTERNALS__.invoke` rather than importing
 * `@tauri-apps/api` so this module stays usable from a non-Tauri build of
 * the same UI (e.g. when running the dashboard in a regular browser for
 * design QA).
 */
function syncTauriWindowTheme(mode: "light" | "dark"): void {
  if (typeof window === "undefined") return;
  const tauri = (window as unknown as {
    __TAURI_INTERNALS__?: { invoke?: (cmd: string, args?: Record<string, unknown>) => Promise<unknown> };
  }).__TAURI_INTERNALS__;
  if (!tauri?.invoke) return;
  try {
    void tauri.invoke("apply_window_theme", { theme: mode });
  } catch {
    // Tauri may fail to deliver the IPC during teardown; nothing useful
    // to surface to the user.
  }
}

export function applyTheme(cfg: ThemeConfig): void {
  if (typeof document === "undefined") return;
  const html = document.documentElement;
  const s = html.style;

  let mode = cfg.presetTheme;
  if (mode === "auto") {
    mode = window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
  html.classList.remove("dark", "light");
  html.classList.add(mode);
  html.setAttribute("data-theme", mode);
  const isLight = mode === "light";

  syncTauriWindowTheme(isLight ? "light" : "dark");

  const palette =
    cfg.colorTheme === "custom"
      ? cfg.customColor
      : COLOR_THEMES.find((c) => c.id === cfg.colorTheme)?.color ?? "#0284c7";
  const hoverColor = darkenHex(palette, 0.15);
  const { r: pr, g: pg, b: pb } = hexToRgb(palette);

  const contrastWarning = checkAccentContrast(palette, isLight ? "light" : "dark");
  if (contrastWarning) {
    console.warn(`[Theme] ${contrastWarning}`);
  }

  s.setProperty("--yunque-accent", palette);
  s.setProperty("--yunque-accent-hover", hoverColor);
  s.setProperty("--yunque-accent-muted", `rgba(${pr},${pg},${pb},${isLight ? "0.10" : "0.12"})`);
  s.setProperty("--yunque-accent-soft", `rgba(${pr},${pg},${pb},${isLight ? "0.05" : "0.06"})`);
  s.setProperty("--yunque-accent-glow", `rgba(${pr},${pg},${pb},${isLight ? "0.12" : "0.15"})`);
  s.setProperty("--yunque-border-focus", `rgba(${pr},${pg},${pb},0.5)`);
  s.setProperty("--shadow-glow", `0 0 20px rgba(${pr},${pg},${pb},${isLight ? "0.12" : "0.15"})`);

  const radiusMap: Record<string, string> = {
    right: "0px",
    default: "10px",
    small: "6px",
    medium: "14px",
    large: "18px",
  };
  const rv = radiusMap[cfg.radius] ?? "10px";
  const rvNum = parseInt(rv);
  s.setProperty("--radius-sm", rvNum === 0 ? "0px" : `${Math.max(rvNum - 2, 2)}px`);
  s.setProperty("--radius-md", rv);
  s.setProperty("--radius-lg", rvNum === 0 ? "0px" : `${rvNum + 4}px`);
  s.setProperty("--radius-xl", rvNum === 0 ? "0px" : `${rvNum + 8}px`);

  const sidebarEl = document.querySelector<HTMLElement>("[data-sidebar]");
  if (sidebarEl) sidebarEl.style.opacity = String(cfg.sidebarOpacity / 100);
  s.setProperty("--yunque-content-opacity", String(cfg.contentOpacity / 100));

  const shadowAlpha = (cfg.shadowOpacity / 100).toFixed(2);
  const { r: sr, g: sg, b: sb } = hexToRgb(cfg.shadowColor);
  s.setProperty("--shadow-sm", `0 1px 2px rgba(${sr},${sg},${sb},${shadowAlpha})`);
  s.setProperty(
    "--shadow-md",
    `0 2px 8px rgba(${sr},${sg},${sb},${shadowAlpha}), 0 0 0 1px rgba(${isLight ? "0,0,0" : "255,255,255"},0.03)`,
  );
  s.setProperty("--shadow-lg", `0 8px 24px rgba(${sr},${sg},${sb},${shadowAlpha})`);
  s.setProperty("--shadow-card", `0 1px 3px rgba(${sr},${sg},${sb},${shadowAlpha})`);

  // Background image + overlay are user-controllable so we guard against
  // unsafe URLs via isSafeAssetURL (https: or data:image/ only).
  const safeBg = cfg.interfaceBgImage && isSafeAssetURL(cfg.interfaceBgImage) ? cfg.interfaceBgImage : null;
  if (safeBg) {
    document.body.style.backgroundImage = `url(${CSS.escape(safeBg)})`;
    document.body.style.backgroundSize = "cover";
    document.body.style.backgroundPosition = "center";
    document.body.style.backgroundAttachment = "fixed";
    const bgAlpha = cfg.interfaceBgOpacity / 100;
    const overlayAlpha = (1 - bgAlpha) * 0.85;
    const overlayBase = isLight ? "255,255,255" : "10,10,12";
    s.setProperty("--yunque-bg-overlay", `rgba(${overlayBase},${overlayAlpha.toFixed(2)})`);
    const baseColor = isLight ? "255,255,255" : "10,10,12";
    s.setProperty("--yunque-bg", `rgba(${baseColor},${(1 - bgAlpha * 0.6).toFixed(2)})`);
    const overlayEl = document.getElementById("bg-overlay");
    if (overlayEl) {
      overlayEl.style.backdropFilter = cfg.interfaceBgBlur > 0 ? `blur(${cfg.interfaceBgBlur}px)` : "";
      // Replace the ambient gradient with a translucent scrim so the body
      // wallpaper shows through, dimmed for text readability.
      overlayEl.style.background = "var(--yunque-bg-overlay)";
    }
  } else {
    // Clear previously-applied bg so switching away removes it.
    document.body.style.backgroundImage = "";
    const overlayEl = document.getElementById("bg-overlay");
    if (overlayEl) {
      overlayEl.style.backdropFilter = "";
      // Restore the CSS-defined ambient gradient base (no inline override).
      overlayEl.style.background = "";
    }
    s.setProperty("--yunque-bg-overlay", "transparent");
    s.removeProperty("--yunque-bg");
  }

  if (cfg.faviconImage && isSafeAssetURL(cfg.faviconImage)) {
    let link = document.querySelector<HTMLLinkElement>("link[rel='icon']");
    if (!link) {
      link = document.createElement("link");
      link.rel = "icon";
      document.head.appendChild(link);
    }
    link.href = cfg.faviconImage;
  }
}

/**
 * Convenience: read the stored config, merge in a partial update, write it
 * back, and re-apply. Used by the Cherry settings overlay whenever it wants
 * to flip a single token (e.g. presetTheme) without caring about the rest.
 */
export function patchAndApply(updates: Partial<ThemeConfig>): ThemeConfig {
  const cur = loadTheme();
  const next = { ...cur, ...updates };
  saveTheme(next);
  applyTheme(next);
  if (typeof window !== "undefined") {
    window.dispatchEvent(new StorageEvent("storage", { key: THEME_STORAGE_KEY }));
  }
  return next;
}
