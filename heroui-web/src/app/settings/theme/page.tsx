"use client";

import { useState, useEffect, useRef } from "react";
import { Button, Chip, Tabs, Slider, Label, ToggleButton, ToggleButtonGroup, ColorPicker, ColorArea, ColorSlider, ColorSwatch, ColorField } from "@heroui/react";
import {
  Palette, Sun, Moon, Monitor, Upload, Image as ImageIcon,
  Layout, LogIn, Check, X, LayoutDashboard,
} from "lucide-react";

type ColorTheme = "time_monologue" | "deep_sea" | "purple_jade" | "mint_ice" | "sakura_fall" | "gold_sand" | "custom";

interface ThemeConfig {
  presetTheme: string; colorTheme: string; customColor: string;
  radius: string; sidebarOpacity: number; contentOpacity: number;
  interfaceBgImage: string | null; interfaceBgOpacity: number; interfaceBgBlur: number;
  shadowColor: string; shadowOpacity: number;
  logoImage: string | null; faviconImage: string | null;
  homeMode: string; homeFontSize: number;
  loginBgImage: string | null; loginContentOpacity: number;
}

const DEFAULT_THEME: ThemeConfig = {
  presetTheme: "auto", colorTheme: "time_monologue", customColor: "#8b5cf6",
  radius: "default", sidebarOpacity: 100, contentOpacity: 100,
  interfaceBgImage: null, interfaceBgOpacity: 30, interfaceBgBlur: 8,
  shadowColor: "#000000", shadowOpacity: 20,
  logoImage: null, faviconImage: null,
  homeMode: "card", homeFontSize: 28,
  loginBgImage: null, loginContentOpacity: 100,
};

const COLOR_THEMES = [
  { id: "time_monologue", name: "时光独白", color: "#a1a1aa" },
  { id: "deep_sea", name: "深海微光", color: "#0ea5e9" },
  { id: "purple_jade", name: "紫玉幻境", color: "#a855f7" },
  { id: "mint_ice", name: "薄荷冰蓝", color: "#2dd4bf" },
  { id: "sakura_fall", name: "落樱飞雪", color: "#f472b6" },
  { id: "gold_sand", name: "流金岁月", color: "#d97706" },
];

const RADIUS_OPTIONS = [
  { id: "right", name: "直角" },
  { id: "default", name: "默认" },
  { id: "small", name: "小" },
  { id: "medium", name: "中" },
  { id: "large", name: "大" },
];

/* ---------- Image Upload Box ---------- */
function ImageBox({ label, hint, imageUrl, onChange, onClear }: {
  label: string; hint: string; imageUrl?: string | null;
  onChange: (url: string) => void; onClear: () => void;
}) {
  const fileRef = useRef<HTMLInputElement>(null);
  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]; if (!file) return;
    const reader = new FileReader();
    reader.onload = (ev) => { if (ev.target?.result) onChange(ev.target.result as string); };
    reader.readAsDataURL(file); e.target.value = "";
  };

  return (
    <div className="flex flex-col gap-2">
      <label className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{label}</label>
      <div className="relative flex flex-col items-center justify-center p-6 border border-dashed rounded-xl cursor-pointer overflow-hidden transition-all"
        style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg)", minHeight: "120px" }}
        onClick={() => !imageUrl && fileRef.current?.click()}>
        <input type="file" ref={fileRef} className="hidden" accept="image/*" onChange={handleFileChange} />
        {imageUrl ? (
          <>
            <img src={imageUrl} alt="preview" className="absolute inset-0 w-full h-full object-cover opacity-60 pointer-events-none" />
            <div className="absolute top-2 right-2 p-1.5 rounded-full z-10 cursor-pointer" style={{ background: "rgba(0,0,0,0.3)" }} onClick={(e) => { e.stopPropagation(); onClear(); }}>
              <X size={14} className="text-white" />
            </div>
            <div className="relative z-10 text-xs font-medium text-white bg-black/40 px-3 py-1.5 rounded-lg backdrop-blur-sm cursor-pointer" onClick={(e) => { e.stopPropagation(); fileRef.current?.click(); }}>更换图片</div>
          </>
        ) : (
          <>
            <div className="w-10 h-10 rounded-full flex items-center justify-center mb-3" style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)" }}><Upload size={18} /></div>
            <span className="text-xs font-medium mb-1" style={{ color: "var(--yunque-text)" }}>点击上传或拖拽文件</span>
            <span className="text-[10px] text-center px-4" style={{ color: "var(--yunque-text-muted)" }}>{hint}</span>
          </>
        )}
      </div>
    </div>
  );
}

/* ---------- Theme load/save ---------- */
function loadTheme(): ThemeConfig {
  if (typeof window === "undefined") return DEFAULT_THEME;
  try { const s = localStorage.getItem("yunque_theme"); return s ? { ...DEFAULT_THEME, ...JSON.parse(s) } : DEFAULT_THEME; }
  catch { return DEFAULT_THEME; }
}
function saveTheme(config: ThemeConfig) {
  if (typeof window === "undefined") return;
  localStorage.setItem("yunque_theme", JSON.stringify(config));
}

/* ---------- Main ---------- */
export default function ThemeSettingsPage() {
  const [config, setConfig] = useState<ThemeConfig>(DEFAULT_THEME);
  const [mounted, setMounted] = useState(false);

  useEffect(() => { setConfig(loadTheme()); setMounted(true); }, []);

  const hexToRgb = (hex: string) => {
    const h = hex.replace("#", "");
    return { r: parseInt(h.slice(0, 2), 16), g: parseInt(h.slice(2, 4), 16), b: parseInt(h.slice(4, 6), 16) };
  };

  const darkenHex = (hex: string, amount: number) => {
    const { r, g, b } = hexToRgb(hex);
    const f = 1 - amount;
    const clamp = (v: number) => Math.max(0, Math.min(255, Math.round(v * f)));
    return `#${clamp(r).toString(16).padStart(2, "0")}${clamp(g).toString(16).padStart(2, "0")}${clamp(b).toString(16).padStart(2, "0")}`;
  };

  const applyTheme = (cfg: ThemeConfig) => {
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

    const palette = cfg.colorTheme === "custom"
      ? cfg.customColor
      : COLOR_THEMES.find(c => c.id === cfg.colorTheme)?.color ?? "#3b82f6";
    const hoverColor = darkenHex(palette, 0.15);
    const { r: pr, g: pg, b: pb } = hexToRgb(palette);

    s.setProperty("--yunque-accent", palette);
    s.setProperty("--yunque-accent-hover", hoverColor);
    s.setProperty("--yunque-accent-muted", `rgba(${pr},${pg},${pb},${isLight ? "0.10" : "0.12"})`);
    s.setProperty("--yunque-accent-soft", `rgba(${pr},${pg},${pb},${isLight ? "0.05" : "0.06"})`);
    s.setProperty("--yunque-accent-glow", `rgba(${pr},${pg},${pb},${isLight ? "0.12" : "0.15"})`);
    s.setProperty("--yunque-border-focus", `rgba(${pr},${pg},${pb},0.5)`);
    s.setProperty("--shadow-glow", `0 0 20px rgba(${pr},${pg},${pb},${isLight ? "0.12" : "0.15"})`);

    const radiusMap: Record<string, string> = { right: "0px", default: "8px", small: "4px", medium: "12px", large: "16px" };
    const rv = radiusMap[cfg.radius] ?? "8px";
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
    s.setProperty("--shadow-md", `0 2px 8px rgba(${sr},${sg},${sb},${shadowAlpha}), 0 0 0 1px rgba(${isLight ? "0,0,0" : "255,255,255"},0.03)`);
    s.setProperty("--shadow-lg", `0 8px 24px rgba(${sr},${sg},${sb},${shadowAlpha})`);
    s.setProperty("--shadow-card", `0 1px 3px rgba(${sr},${sg},${sb},${shadowAlpha})`);

    if (cfg.interfaceBgImage) {
      document.body.style.backgroundImage = `url(${cfg.interfaceBgImage})`;
      document.body.style.backgroundSize = "cover";
      document.body.style.backgroundPosition = "center";
      document.body.style.backgroundAttachment = "fixed";
      const bgAlpha = cfg.interfaceBgOpacity / 100;
      const overlayAlpha = (1 - bgAlpha) * 0.85;
      const overlayBase = isLight ? "248,249,251" : "9,9,11";
      s.setProperty("--yunque-bg-overlay", `rgba(${overlayBase},${overlayAlpha.toFixed(2)})`);
      const baseColor = isLight ? "248,249,251" : "9,9,11";
      s.setProperty("--yunque-bg", `rgba(${baseColor},${(1 - bgAlpha * 0.6).toFixed(2)})`);
      const overlayEl = document.getElementById("bg-overlay");
      if (overlayEl) overlayEl.style.backdropFilter = cfg.interfaceBgBlur > 0 ? `blur(${cfg.interfaceBgBlur}px)` : "";
    } else {
      document.body.style.backgroundImage = "";
      const overlayEl = document.getElementById("bg-overlay");
      if (overlayEl) overlayEl.style.backdropFilter = "";
      s.setProperty("--yunque-bg-overlay", "transparent");
      s.removeProperty("--yunque-bg");
    }

    if (cfg.faviconImage) {
      let link = document.querySelector<HTMLLinkElement>("link[rel='icon']");
      if (!link) { link = document.createElement("link"); link.rel = "icon"; document.head.appendChild(link); }
      link.href = cfg.faviconImage;
    }
  };

  useEffect(() => { if (mounted) applyTheme(config); }, [config, mounted]);

  const upd = (updates: Partial<ThemeConfig>) => {
    const next = { ...config, ...updates };
    setConfig(next);
    saveTheme(next);
  };

  if (!mounted) return <div className="flex items-center justify-center h-64"><span className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>正在加载主题引擎...</span></div>;

  return (
    <div className="page-root space-y-6 animate-fade-in-up" style={{ color: "var(--yunque-text)" }}>
      {/* Header */}
      <div className="flex items-center gap-3">
        <Palette size={20} style={{ color: "var(--yunque-accent)" }} />
        <div>
          <h1 className="text-xl font-semibold">主题设置</h1>
          <p className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>自定义云雀 Agent 的视觉外观、颜色搭配与全局交互风格</p>
        </div>
      </div>

      {/* Tabs — HeroUI v3 compound pattern */}
      <Tabs defaultSelectedKey="appearance">
        <Tabs.ListContainer>
          <Tabs.List aria-label="主题设置分类">
            <Tabs.Tab id="appearance"><Sun size={14} /> 外观<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="brand"><Tabs.Separator /><ImageIcon size={14} /> 品牌<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="layout"><Tabs.Separator /><Layout size={14} /> 布局<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="home"><Tabs.Separator /><LayoutDashboard size={14} /> 首页<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="login"><Tabs.Separator /><LogIn size={14} /> 登录<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>

        {/* ===== Appearance Panel ===== */}
        <Tabs.Panel id="appearance" className="space-y-3 pt-4">
          <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
            <div className="flex items-center gap-2 mb-3">
              <Monitor size={14} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>预设主题</span>
            </div>
            <ToggleButtonGroup
              selectionMode="single" disallowEmptySelection
              selectedKeys={new Set([config.presetTheme])}
              onSelectionChange={(keys) => { const k = [...keys][0]; if (k) upd({ presetTheme: String(k) }); }}
            >
              <ToggleButton id="auto"><Monitor size={14} /> 自动</ToggleButton>
              <ToggleButton id="light"><ToggleButtonGroup.Separator /><Sun size={14} /> 浅色</ToggleButton>
              <ToggleButton id="dark"><ToggleButtonGroup.Separator /><Moon size={14} /> 深色</ToggleButton>
            </ToggleButtonGroup>
          </div>

          <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
            <div className="flex items-center gap-2 mb-3">
              <Palette size={14} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>颜色主题</span>
            </div>
            <div className="flex flex-wrap gap-2.5 items-center">
              {COLOR_THEMES.map(c => (
                <Button key={c.id} isIconOnly aria-label={`颜色主题: ${c.name}`} onPress={() => upd({ colorTheme: c.id })}
                  className="relative w-8 h-8 rounded-full min-w-0 p-0"
                  style={{ background: c.color, boxShadow: config.colorTheme === c.id ? `0 0 0 2px var(--yunque-bg), 0 0 0 3px ${c.color}` : "none", transform: config.colorTheme === c.id ? "scale(1.12)" : "scale(1)" }}>
                  {config.colorTheme === c.id && <Check size={12} className="absolute inset-0 m-auto text-white drop-shadow-md" />}
                </Button>
              ))}
              <div className="w-px h-5 mx-0.5" style={{ background: "var(--yunque-border)" }} />
              <ColorPicker value={config.customColor} onChange={(c) => upd({ customColor: c.toString("hex"), colorTheme: "custom" })}>
                <ColorPicker.Trigger className="inline-flex">
                  <ColorSwatch className="w-8 h-8 rounded-full" style={{ boxShadow: config.colorTheme === "custom" ? `0 0 0 2px var(--yunque-bg), 0 0 0 3px ${config.customColor}` : "none" }} />
                </ColorPicker.Trigger>
                <ColorPicker.Popover className="gap-2 p-3">
                  <ColorArea aria-label="颜色区域" className="max-w-full" colorSpace="hsb" xChannel="saturation" yChannel="brightness">
                    <ColorArea.Thumb />
                  </ColorArea>
                  <ColorSlider aria-label="色相" channel="hue" className="gap-1 px-1" colorSpace="hsb">
                    <ColorSlider.Track><ColorSlider.Thumb /></ColorSlider.Track>
                  </ColorSlider>
                  <ColorField aria-label="颜色值">
                    <ColorField.Group variant="secondary">
                      <ColorField.Prefix><ColorSwatch size="xs" /></ColorField.Prefix>
                      <ColorField.Input />
                    </ColorField.Group>
                  </ColorField>
                </ColorPicker.Popover>
              </ColorPicker>
            </div>
          </div>

          <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
            <div className="flex items-center gap-2 mb-3">
              <Layout size={14} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>界面圆角</span>
            </div>
            <ToggleButtonGroup
              selectionMode="single" disallowEmptySelection
              selectedKeys={new Set([config.radius])}
              onSelectionChange={(keys) => { const k = [...keys][0]; if (k) upd({ radius: String(k) }); }}
            >
              {RADIUS_OPTIONS.map((r, i) => (
                <ToggleButton key={r.id} id={r.id}>{i > 0 && <ToggleButtonGroup.Separator />}{r.name}</ToggleButton>
              ))}
            </ToggleButtonGroup>
          </div>
        </Tabs.Panel>

        {/* ===== Brand Panel ===== */}
        <Tabs.Panel id="brand" className="space-y-3 pt-4">
          <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
            <div className="flex items-center gap-2 mb-3">
              <ImageIcon size={14} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>品牌图标</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <ImageBox label="Logo 设置" hint="建议: 32px×32px，推荐 SVG" imageUrl={config.logoImage} onChange={(url) => upd({ logoImage: url })} onClear={() => upd({ logoImage: null })} />
              <ImageBox label="网站图标 (Favicon)" hint="建议: 16px×16px，推荐 ICO/PNG/SVG" imageUrl={config.faviconImage} onChange={(url) => upd({ faviconImage: url })} onClear={() => upd({ faviconImage: null })} />
            </div>
          </div>
        </Tabs.Panel>

        {/* ===== Layout Panel ===== */}
        <Tabs.Panel id="layout" className="space-y-3 pt-4">
          <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
            <div className="flex items-center gap-2 mb-3">
              <Layout size={14} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>透明度与层级</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <Slider value={config.sidebarOpacity} onChange={(v) => upd({ sidebarOpacity: v as number })} minValue={1} maxValue={100}>
                <Label>侧边栏背景透明度</Label>
                <Slider.Output />
                <Slider.Track><Slider.Fill /><Slider.Thumb /></Slider.Track>
              </Slider>
              <Slider value={config.contentOpacity} onChange={(v) => upd({ contentOpacity: v as number })} minValue={1} maxValue={100}>
                <Label>全局内容透明度</Label>
                <Slider.Output />
                <Slider.Track><Slider.Fill /><Slider.Thumb /></Slider.Track>
              </Slider>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-3">
              <div className="flex items-center gap-2">
                <ColorPicker value={config.shadowColor} onChange={(c) => upd({ shadowColor: c.toString("hex") })}>
                  <ColorPicker.Trigger className="inline-flex items-center gap-2">
                    <ColorSwatch className="w-7 h-7 rounded" />
                    <span className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>阴影颜色</span>
                    <span className="text-[10px] font-mono" style={{ color: "var(--yunque-text-muted)" }}>{config.shadowColor.toUpperCase()}</span>
                  </ColorPicker.Trigger>
                  <ColorPicker.Popover className="gap-2 p-3">
                    <ColorArea aria-label="阴影颜色区域" className="max-w-full" colorSpace="hsb" xChannel="saturation" yChannel="brightness">
                      <ColorArea.Thumb />
                    </ColorArea>
                    <ColorSlider aria-label="色相" channel="hue" className="gap-1 px-1" colorSpace="hsb">
                      <ColorSlider.Track><ColorSlider.Thumb /></ColorSlider.Track>
                    </ColorSlider>
                  </ColorPicker.Popover>
                </ColorPicker>
              </div>
              <Slider value={config.shadowOpacity} onChange={(v) => upd({ shadowOpacity: v as number })} minValue={0} maxValue={100}>
                <Label>阴影透明度</Label>
                <Slider.Output />
                <Slider.Track><Slider.Fill /><Slider.Thumb /></Slider.Track>
              </Slider>
            </div>
          </div>

          <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
            <div className="flex items-center gap-2 mb-3">
              <ImageIcon size={14} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>背景图片</span>
            </div>
            <ImageBox label="主界面背景图片" hint="建议: 1920×1080" imageUrl={config.interfaceBgImage} onChange={(url) => upd({ interfaceBgImage: url })} onClear={() => upd({ interfaceBgImage: null })} />
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-3">
              <Slider value={config.interfaceBgOpacity} onChange={(v) => upd({ interfaceBgOpacity: v as number })} minValue={1} maxValue={100}>
                <Label>背景图片透明度</Label>
                <Slider.Output />
                <Slider.Track><Slider.Fill /><Slider.Thumb /></Slider.Track>
              </Slider>
              <Slider value={config.interfaceBgBlur} onChange={(v) => upd({ interfaceBgBlur: v as number })} minValue={0} maxValue={20}>
                <Label>背景模糊程度</Label>
                <Slider.Output />
                <Slider.Track><Slider.Fill /><Slider.Thumb /></Slider.Track>
              </Slider>
            </div>
          </div>
        </Tabs.Panel>

        {/* ===== Home Panel ===== */}
        <Tabs.Panel id="home" className="space-y-3 pt-4">
          <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
            <div className="flex items-center gap-2 mb-3">
              <LayoutDashboard size={14} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>首页展示</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 items-start">
              <div className="space-y-4">
                <div>
                  <label className="text-xs font-medium mb-2 block" style={{ color: "var(--yunque-text)" }}>状态显示方式</label>
                  <ToggleButtonGroup selectionMode="single" disallowEmptySelection
                    selectedKeys={new Set([config.homeMode])}
                    onSelectionChange={(keys) => { const k = [...keys][0]; if (k) upd({ homeMode: String(k) }); }}>
                    <ToggleButton id="card">卡片模式</ToggleButton>
                    <ToggleButton id="classic"><ToggleButtonGroup.Separator />经典模式</ToggleButton>
                  </ToggleButtonGroup>
                </div>
                <Slider value={config.homeFontSize} onChange={(v) => upd({ homeFontSize: v as number })} minValue={12} maxValue={50}>
                  <Label>状态字体大小</Label>
                  <Slider.Output />
                  <Slider.Track><Slider.Fill /><Slider.Thumb /></Slider.Track>
                </Slider>
              </div>
              <div className="p-3 rounded-lg border flex flex-col items-center justify-center"
                style={{ background: "var(--yunque-bg)", borderColor: "var(--yunque-border)", height: "120px" }}>
                <Chip size="sm" style={{ background: "rgba(0,111,238,0.08)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>PREVIEW</Chip>
                <div className={`transition-all flex items-center justify-center rounded-xl mt-2 ${config.homeMode === "card" ? "border px-4 py-2" : "p-1"}`}
                  style={config.homeMode === "card" ? { background: "var(--yunque-card)", borderColor: "var(--yunque-border)" } : undefined}>
                  <span className="font-bold" style={{ fontSize: `${Math.min(config.homeFontSize, 24)}px`, color: "var(--yunque-accent)" }}>运行流畅</span>
                </div>
              </div>
            </div>
          </div>
        </Tabs.Panel>

        {/* ===== Login Panel ===== */}
        <Tabs.Panel id="login" className="space-y-3 pt-4">
          <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
            <div className="flex items-center gap-2 mb-3">
              <LogIn size={14} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>登录界面</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-4">
                <ImageBox label="登录 Logo 图片" hint="建议: 64px×64px，推荐 SVG" imageUrl={config.logoImage} onChange={(url) => upd({ logoImage: url })} onClear={() => upd({ logoImage: null })} />
                <Slider value={config.loginContentOpacity} onChange={(v) => upd({ loginContentOpacity: v as number })} minValue={1} maxValue={100}>
                  <Label>登录内容透明度</Label>
                  <Slider.Output />
                  <Slider.Track><Slider.Fill /><Slider.Thumb /></Slider.Track>
                </Slider>
              </div>
              <ImageBox label="登录背景图片" hint="建议: 1920×1080" imageUrl={config.loginBgImage} onChange={(url) => upd({ loginBgImage: url })} onClear={() => upd({ loginBgImage: null })} />
            </div>
          </div>
        </Tabs.Panel>
      </Tabs>
    </div>
  );
}
