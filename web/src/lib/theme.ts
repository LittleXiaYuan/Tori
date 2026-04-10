export type ThemeConfig = {
  presetTheme: string;
  colorTheme: string;
  customColor: string;
  sidebarOpacity: number;
  radius: string;
  interfaceBgOpacity: number;
  interfaceBgBlur: number;
  contentOpacity: number;
  shadowColor: string;
  shadowOpacity: number;
  homeMode: string;
  homeFontSize: number;
  loginContentOpacity: number;
  interfaceBgImage: string | null;
  loginBgImage: string | null;
  logoImage: string | null;
  faviconImage: string | null;
};

export const DEFAULT_THEME: ThemeConfig = {
  presetTheme: "auto",
  colorTheme: "deep_sea",
  customColor: "#ff3366",
  sidebarOpacity: 100,
  radius: "default",
  interfaceBgOpacity: 20,
  interfaceBgBlur: 0,
  contentOpacity: 100,
  shadowColor: "#000000",
  shadowOpacity: 30,
  homeMode: "card",
  homeFontSize: 24,
  loginContentOpacity: 100,
  interfaceBgImage: null,
  loginBgImage: null,
  logoImage: null,
  faviconImage: null,
};

function hexToRgba(hex: string, alpha: number) {
  const r = parseInt(hex.slice(1, 3), 16) || 0;
  const g = parseInt(hex.slice(3, 5), 16) || 0;
  const b = parseInt(hex.slice(5, 7), 16) || 0;
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

// ── Light mode color overrides ──
const LIGHT_VARS: Record<string, string> = {
  '--bg': '#f8f9fc',
  '--bg-card': 'rgba(255, 255, 255, 0.8)',
  '--bg-hover': 'rgba(0, 0, 0, 0.03)',
  '--bg-elevated': 'rgba(255, 255, 255, 0.9)',
  '--border': 'rgba(0, 0, 0, 0.08)',
  '--border-hover': 'rgba(0, 0, 0, 0.15)',
  '--text': '#1a1a2e',
  '--text-secondary': '#5a5a72',
  '--text-muted': '#8e8ea0',
  '--sidebar-bg': 'rgba(255, 255, 255, 0.95)',
  '--shadow-sm': '0 1px 3px rgba(0, 0, 0, 0.08)',
  '--shadow-md': '0 4px 12px rgba(0, 0, 0, 0.06)',
  '--shadow-lg': '0 8px 24px rgba(0, 0, 0, 0.1)',
};

const DARK_VARS: Record<string, string> = {
  '--bg': '#0d1117',
  '--bg-card': 'rgba(22, 27, 34, 0.5)',
  '--bg-hover': 'rgba(255, 255, 255, 0.04)',
  '--bg-elevated': 'rgba(33, 38, 45, 0.6)',
  '--border': 'rgba(255, 255, 255, 0.1)',
  '--border-hover': 'rgba(255, 255, 255, 0.18)',
  '--text': '#e6edf3',
  '--text-secondary': '#9198a1',
  '--text-muted': '#656d76',
  '--sidebar-bg': 'rgba(13, 17, 23, 0.95)',
  '--shadow-sm': '0 4px 12px rgba(0, 0, 0, 0.3)',
  '--shadow-md': '0 8px 24px rgba(0, 0, 0, 0.4)',
  '--shadow-lg': '0 16px 48px rgba(0, 0, 0, 0.5)',
};

function resolvePreset(preset: string): 'light' | 'dark' {
  if (preset === 'light') return 'light';
  if (preset === 'dark') return 'dark';
  // auto: follow system preference
  if (typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: light)').matches) {
    return 'light';
  }
  return 'dark';
}

export function applyTheme(config: ThemeConfig) {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  
  // ── Light / Dark mode ──
  const mode = resolvePreset(config.presetTheme);
  const vars = mode === 'light' ? LIGHT_VARS : DARK_VARS;
  for (const [key, value] of Object.entries(vars)) {
    root.style.setProperty(key, value);
  }
  root.setAttribute('data-theme', mode);

  // Apply accent color
  let accent = "#0ea5e9";
  const colors: Record<string, string> = {
    "time_monologue": "#a1a1aa",
    "deep_sea": "#0ea5e9",
    "purple_jade": "#a855f7",
    "mint_ice": "#2dd4bf",
    "sakura_fall": "#f472b6",
    "gold_sand": "#d97706"
  };
  if (config.colorTheme === "custom") {
    accent = config.customColor;
  } else if (colors[config.colorTheme]) {
    accent = colors[config.colorTheme];
  }
  
  root.style.setProperty('--accent', accent);
  root.style.setProperty('--accent-glow', hexToRgba(accent, 0.2));
  root.style.setProperty('--accent-subtle', hexToRgba(accent, 0.1));
  root.style.setProperty('--accent-hover', accent);
  
  // Apply radius
  const radiuses: Record<string, string> = {
    "right": "0px", "default": "8px", "small": "4px", "medium": "12px", "large": "16px"
  };
  root.style.setProperty('--radius', radiuses[config.radius] || "8px");
  
  // Sidebar opacity
  const sidebarAlpha = config.sidebarOpacity / 100;
  root.style.setProperty('--sidebar-bg', `rgba(10, 10, 15, ${sidebarAlpha})`);
  
  // Interface background image
  if (config.interfaceBgImage) {
    document.body.style.backgroundImage = `url(${config.interfaceBgImage})`;
    document.body.style.backgroundSize = "cover";
    document.body.style.backgroundPosition = "center";
    document.body.style.backgroundAttachment = "fixed";
    // Dark overlay: higher opacity value = more background visible
    const overlayAlpha = 1 - (config.interfaceBgOpacity / 100);
    const overlayColor = mode === 'light' ? '248, 249, 250' : '3, 3, 5';
    root.style.setProperty('--bg', `rgba(${overlayColor}, ${overlayAlpha})`);
    // More translucent cards
    const cardColor = mode === 'light' ? '255, 255, 255' : '15, 15, 20';
    root.style.setProperty('--bg-card', `rgba(${cardColor}, ${0.3 + overlayAlpha * 0.4})`);
    // Background blur
    root.style.setProperty('--bg-blur', `blur(${config.interfaceBgBlur}px)`);
    document.body.classList.add('has-bg-image');
  } else {
    document.body.style.backgroundImage = "none";
    // Reset to mode defaults
    root.style.setProperty('--bg', vars['--bg']);
    root.style.setProperty('--bg-card', vars['--bg-card']);
    document.body.classList.remove('has-bg-image');
  }
  
  // Content opacity
  root.style.setProperty('--content-opacity', (config.contentOpacity / 100).toString());
  
  // Shadow
  const shadowRgbaStr = hexToRgba(config.shadowColor, config.shadowOpacity / 100);
  root.style.setProperty('--shadow-sm', `0 4px 12px ${shadowRgbaStr}`);
  root.style.setProperty('--shadow-md', `0 8px 24px ${shadowRgbaStr}`);
  root.style.setProperty('--shadow-lg', `0 16px 48px ${shadowRgbaStr}`);
  
  // Home Settings
  root.style.setProperty('--home-font-size', `${config.homeFontSize}px`);
  
  // Set favicon if present
  if (config.faviconImage) {
    let link = document.querySelector("link[rel~='icon']") as HTMLLinkElement;
    if (!link) {
      link = document.createElement('link');
      link.rel = 'icon';
      document.head.appendChild(link);
    }
    link.href = config.faviconImage;
  }
  
  // Save locally — separate images into IndexedDB to avoid localStorage quota
  saveTheme(config);
}

// ── Persistence: localStorage for settings, IndexedDB for images ──

const IMAGE_KEYS: (keyof ThemeConfig)[] = ['interfaceBgImage', 'loginBgImage', 'logoImage', 'faviconImage'];
const DB_NAME = 'yunque-theme-db';
const STORE_NAME = 'images';

function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, 1);
    req.onupgradeneeded = () => req.result.createObjectStore(STORE_NAME);
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

async function saveImages(config: ThemeConfig) {
  try {
    const db = await openDB();
    const tx = db.transaction(STORE_NAME, 'readwrite');
    const store = tx.objectStore(STORE_NAME);
    for (const key of IMAGE_KEYS) {
      const val = config[key];
      if (val) {
        store.put(val, key);
      } else {
        store.delete(key);
      }
    }
  } catch { /* IndexedDB unavailable */ }
}

async function loadImages(): Promise<Partial<ThemeConfig>> {
  try {
    const db = await openDB();
    const tx = db.transaction(STORE_NAME, 'readonly');
    const store = tx.objectStore(STORE_NAME);
    const result: Partial<ThemeConfig> = {};
    await Promise.all(IMAGE_KEYS.map(key =>
      new Promise<void>((resolve) => {
        const req = store.get(key);
        req.onsuccess = () => { if (req.result) (result as any)[key] = req.result; resolve(); };
        req.onerror = () => resolve();
      })
    ));
    return result;
  } catch { return {}; }
}

function saveTheme(config: ThemeConfig) {
  // Strip image data from localStorage payload
  const slim: Record<string, unknown> = { ...config };
  for (const key of IMAGE_KEYS) {
    slim[key] = config[key] ? '__idb__' : null; // marker
  }
  try {
    localStorage.setItem('yunque-theme', JSON.stringify(slim));
  } catch { /* quota full — ignore */ }
  // Save images to IndexedDB (async, non-blocking)
  saveImages(config);
}

export function loadTheme(): ThemeConfig {
  if (typeof localStorage === 'undefined') return DEFAULT_THEME;
  try {
    const saved = localStorage.getItem('yunque-theme');
    if (saved) {
      const parsed = { ...DEFAULT_THEME, ...JSON.parse(saved) };
      // Clear markers — images will be loaded async
      for (const key of IMAGE_KEYS) {
        if (parsed[key] === '__idb__') parsed[key] = null;
      }
      return parsed;
    }
  } catch {}
  return DEFAULT_THEME;
}

/** Load images from IndexedDB and apply them. Call after initial applyTheme. */
export async function loadThemeImages(): Promise<ThemeConfig | null> {
  const config = loadTheme();
  const images = await loadImages();
  if (Object.keys(images).length === 0) return null;
  const full = { ...config, ...images };
  applyTheme(full);
  return full;
}
