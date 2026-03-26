"use client";

import { useState, useEffect, useRef } from "react";
import { BlurFade } from "@/components/ui/blur-fade";
import { useI18n } from "@/lib/i18n";
import { ThemeConfig, DEFAULT_THEME, loadTheme, applyTheme, loadThemeImages } from "@/lib/theme";
import {
  Palette, Sun, Moon, Monitor, Upload, Image as ImageIcon,
  Layout, Type, LogIn, Check, X, SlidersHorizontal, LayoutDashboard
} from "lucide-react";

type PresetTheme = "auto" | "light" | "dark";
type ColorTheme = "time_monologue" | "deep_sea" | "purple_jade" | "mint_ice" | "sakura_fall" | "gold_sand" | "custom";
type RadiusOption = "right" | "default" | "small" | "medium" | "large";
type HomeDisplay = "card" | "classic";

const COLOR_THEMES: { id: ColorTheme; name: string; color: string }[] = [
  { id: "time_monologue", name: "时光独白", color: "#a1a1aa" },
  { id: "deep_sea", name: "深海微光", color: "#0ea5e9" },
  { id: "purple_jade", name: "紫玉幻境", color: "#a855f7" },
  { id: "mint_ice", name: "薄荷冰蓝", color: "#2dd4bf" },
  { id: "sakura_fall", name: "落樱飞雪", color: "#f472b6" },
  { id: "gold_sand", name: "流金岁月", color: "#d97706" },
];

const RADIUS_OPTIONS: { id: RadiusOption; name: string; val: string }[] = [
  { id: "right", name: "直角", val: "0px" },
  { id: "default", name: "默认", val: "8px" },
  { id: "small", name: "小", val: "4px" },
  { id: "medium", name: "中", val: "12px" },
  { id: "large", name: "大", val: "16px" },
];

/**
 * Common Slider Component
 */
function RangeSlider({ label, value, min, max, onChange, hint }: { label: string, value: number, min: number, max: number, onChange: (v: number) => void, hint?: string }) {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <label className="text-sm font-medium" style={{ color: "var(--text)" }}>{label}</label>
        <span className="text-xs font-mono px-2 py-0.5 rounded" style={{ background: "var(--bg-hover)", color: "var(--accent)" }}>{value}</span>
      </div>
      <input
        type="range" min={min} max={max} value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="w-full h-1.5 rounded-lg appearance-none cursor-pointer"
        style={{
          background: `linear-gradient(to right, var(--accent) ${(value - min) / (max - min) * 100}%, var(--bg-hover) ${(value - min) / (max - min) * 100}%)`,
          outline: "none"
        }}
      />
      {hint && <span className="text-[10px]" style={{ color: "var(--text-muted)" }}>{hint}</span>}
    </div>
  );
}

/**
 * Common Image Upload Box
 */
function ImageBox({ label, hint, imageUrl, onChange, onClear }: { label: string, hint: string, imageUrl?: string | null, onChange: (url: string) => void, onClear: () => void }) {
  const fileRef = useRef<HTMLInputElement>(null);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (ev) => {
      if (ev.target?.result) {
        onChange(ev.target.result as string);
      }
    };
    reader.readAsDataURL(file);
    e.target.value = ''; // reset
  };

  return (
    <div className="flex flex-col gap-2">
      <label className="text-sm font-medium" style={{ color: "var(--text)" }}>{label}</label>
      <div 
        className="relative flex flex-col items-center justify-center p-6 border border-dashed rounded-xl cursor-pointer card-hover overflow-hidden transition-all"
        style={{ borderColor: "var(--border)", background: "var(--bg-hover)", minHeight: "120px" }}
        onClick={() => !imageUrl && fileRef.current?.click()}
      >
        <input type="file" ref={fileRef} className="hidden" accept="image/*" onChange={handleFileChange} />
        {imageUrl ? (
           <>
             <img src={imageUrl} alt="preview" className="absolute inset-0 w-full h-full object-cover opacity-60 pointer-events-none" />
             <div className="absolute top-2 right-2 p-1.5 rounded-full z-10 hover:bg-black/50 transition-colors shadow-sm"
                  style={{ background: 'rgba(0,0,0,0.3)' }}
                  onClick={(e) => { e.stopPropagation(); onClear(); }}>
               <X size={14} className="text-white" />
             </div>
             <div 
               className="relative z-10 text-xs font-medium text-white shadow-md bg-black/40 px-3 py-1.5 rounded-lg backdrop-blur-sm"
               onClick={(e) => { e.stopPropagation(); fileRef.current?.click(); }}>
               更换图片
             </div>
           </>
        ) : (
           <>
             <div className="w-10 h-10 rounded-full flex items-center justify-center mb-3 transition-colors" style={{ background: "var(--accent-subtle)", color: "var(--accent)" }}>
               <Upload size={18} />
             </div>
             <span className="text-xs font-medium mb-1" style={{ color: "var(--text)" }}>点击上传或拖拽文件</span>
             <span className="text-[10px] text-center px-4" style={{ color: "var(--text-muted)" }}>{hint}</span>
           </>
        )}
      </div>
    </div>
  );
}

export default function ThemeSettingsPage() {
  const { locale } = useI18n();
  const [config, setConfig] = useState<ThemeConfig>(DEFAULT_THEME);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    const base = loadTheme();
    setConfig(base);
    setMounted(true);
    // Load images from IndexedDB
    loadThemeImages().then(full => { if (full) setConfig(full); });
  }, []);

  const updateConfig = (updates: Partial<ThemeConfig>) => {
    const newConfig = { ...config, ...updates };
    setConfig(newConfig);
    applyTheme(newConfig);
  };

  if (!mounted) {
    return (
      <div className="max-w-4xl pb-16 animate-in flex items-center justify-center h-64">
        <span className="text-sm" style={{ color: "var(--text-muted)" }}>正在加载主题引擎...</span>
      </div>
    );
  }

  return (
    <div className="max-w-4xl pb-16 animate-in">
      <BlurFade delay={0}>
        <div className="mb-8">
          <h1 className="text-2xl font-semibold tracking-tight mb-2 flex items-center gap-2">
            <Palette className="text-[var(--accent)]" size={24} /> 
            主题设置
          </h1>
          <p className="text-sm" style={{ color: "var(--text-muted)" }}>
            自定义云雀 Agent 的视觉外观、颜色搭配与全局交互风格。
          </p>
        </div>
      </BlurFade>

      <div className="space-y-6">
        {/* --- 预设主题 & 颜色主题 --- */}
        <BlurFade delay={0.05}>
          <section className="rounded-2xl border p-6" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <h2 className="text-base font-medium mb-5 flex items-center gap-2">
              <Sun size={18} style={{ color: "var(--text-secondary)" }} />
              外观组合
            </h2>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
              {/* Preset Theme */}
              <div>
                <label className="text-sm font-medium mb-3 block" style={{ color: "var(--text)" }}>预设主题</label>
                <div className="flex bg-[var(--bg)] p-1 rounded-xl border" style={{ borderColor: "var(--border)" }}>
                  {[
                    { id: "auto", icon: Monitor, label: "自动" },
                    { id: "light", icon: Sun, label: "浅色" },
                    { id: "dark", icon: Moon, label: "深色" }
                  ].map(t => (
                    <button key={t.id} onClick={() => updateConfig({ presetTheme: t.id })}
                      className="flex-1 flex items-center justify-center gap-2 py-2 rounded-lg text-sm font-medium transition-all"
                      style={{
                        background: config.presetTheme === t.id ? "var(--bg-elevated)" : "transparent",
                        color: config.presetTheme === t.id ? "var(--accent)" : "var(--text-muted)",
                        boxShadow: config.presetTheme === t.id ? "var(--shadow-sm)" : "none"
                      }}>
                      <t.icon size={16} /> {t.label}
                    </button>
                  ))}
                </div>
                <p className="text-[10px] mt-2" style={{ color: "var(--text-muted)" }}>选择界面预设主题，影响整体界面的明暗风格和其他风格。</p>
              </div>

              {/* Color Theme */}
              <div>
                <label className="text-sm font-medium mb-3 block" style={{ color: "var(--text)" }}>颜色主题</label>
                <div className="flex flex-wrap gap-3">
                  {COLOR_THEMES.map(c => (
                    <button key={c.id} onClick={() => updateConfig({ colorTheme: c.id })} title={c.name}
                      className="relative w-8 h-8 rounded-full transition-all focus:outline-none"
                      style={{ 
                        background: c.color,
                        boxShadow: config.colorTheme === c.id ? `0 0 0 2px var(--bg), 0 0 0 4px ${c.color}` : "none",
                        transform: config.colorTheme === c.id ? "scale(1.1)" : "scale(1)"
                      }}
                    >
                      {config.colorTheme === c.id && <Check size={14} className="absolute inset-0 m-auto text-white drop-shadow-md" />}
                    </button>
                  ))}
                  
                  {/* Custom Color */}
                  <div className="relative flex items-center gap-2">
                    <div className="w-px h-6 mx-1" style={{ background: "var(--border)" }} />
                    <label 
                      title="自定义"
                      className="relative w-8 h-8 rounded-full flex items-center justify-center cursor-pointer overflow-hidden transition-all"
                      style={{ 
                        background: config.colorTheme === "custom" ? config.customColor : "conic-gradient(red, yellow, green, cyan, blue, magenta, red)",
                        boxShadow: config.colorTheme === "custom" ? `0 0 0 2px var(--bg), 0 0 0 4px ${config.customColor}` : "var(--shadow-sm)",
                        transform: config.colorTheme === "custom" ? "scale(1.1)" : "scale(1)"
                      }}>
                      {config.colorTheme === "custom" && <Check size={14} className="absolute inset-0 m-auto text-white drop-shadow-md z-10" />}
                      <input 
                        type="color" value={config.customColor} 
                        onChange={(e) => updateConfig({ customColor: e.target.value, colorTheme: "custom" })}
                        className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
                      />
                    </label>
                  </div>
                </div>
                <p className="text-[10px] mt-3" style={{ color: "var(--text-muted)" }}>选择预设的颜色主题方案或自定义颜色，会影响整体界面色调。</p>
              </div>
            </div>
          </section>
        </BlurFade>

        {/* --- Logo & 网站图标设置 --- */}
        <BlurFade delay={0.1}>
          <section className="rounded-2xl border p-6" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <h2 className="text-base font-medium mb-5 flex items-center gap-2">
              <ImageIcon size={18} style={{ color: "var(--text-secondary)" }} />
              品牌图标
            </h2>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <ImageBox 
                label="Logo 设置" 
                hint="将会显示在左侧菜单栏顶部（建议图片大小: 32px*32px，推荐使用SVG格式）" 
                imageUrl={config.logoImage}
                onChange={(url) => updateConfig({ logoImage: url })}
                onClear={() => updateConfig({ logoImage: null })}
              />
              <ImageBox 
                label="网站图标设置 (Favicon)" 
                hint="将会显示在浏览器标签页（建议图片大小: 16px*16px，推荐使用ICO格式或PNG、SVG格式）" 
                imageUrl={config.faviconImage}
                onChange={(url) => updateConfig({ faviconImage: url })}
                onClear={() => updateConfig({ faviconImage: null })}
              />
            </div>
          </section>
        </BlurFade>

        {/* --- 侧边栏 & 界面设置 --- */}
        <BlurFade delay={0.15}>
          <section className="rounded-2xl border p-6" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <h2 className="text-base font-medium mb-5 flex items-center gap-2">
              <Layout size={18} style={{ color: "var(--text-secondary)" }} />
              结构与层级
            </h2>

            <div className="space-y-8">
              {/* Radius & Sidebar Opacity */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                <div>
                  <label className="text-sm font-medium mb-3 block" style={{ color: "var(--text)" }}>界面圆角</label>
                  <div className="flex bg-[var(--bg)] p-1 rounded-xl border" style={{ borderColor: "var(--border)" }}>
                    {RADIUS_OPTIONS.map(r => (
                      <button key={r.id} onClick={() => updateConfig({ radius: r.id })}
                        className="flex-1 py-2 rounded-lg text-xs font-medium transition-all"
                        style={{
                          background: config.radius === r.id ? "var(--bg-elevated)" : "transparent",
                          color: config.radius === r.id ? "var(--accent)" : "var(--text-muted)",
                          boxShadow: config.radius === r.id ? "var(--shadow-sm)" : "none"
                        }}>
                        {r.name}
                      </button>
                    ))}
                  </div>
                  <p className="text-[10px] mt-2" style={{ color: "var(--text-muted)" }}>选择界面元素的圆角大小，影响按钮、卡片等元素的圆角效果。</p>
                </div>

                <RangeSlider 
                  label="侧边栏背景透明度" value={config.sidebarOpacity} min={1} max={100} onChange={v => updateConfig({ sidebarOpacity: v })} 
                  hint="（1-100，数值越小越透明）" 
                />
              </div>

              {/* Background & Content Opacity */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-8 pt-4 border-t" style={{ borderColor: "var(--border)" }}>
                <div>
                  <ImageBox 
                    label="主界面背景图片" 
                    hint="将会显示在主界面背景（本地预览模式仅供本次会话测试）" 
                    imageUrl={config.interfaceBgImage}
                    onChange={(url) => updateConfig({ interfaceBgImage: url })}
                    onClear={() => updateConfig({ interfaceBgImage: null })}
                  />
                  <div className="mt-6 space-y-4">
                    <RangeSlider 
                      label="背景图片透明度" value={config.interfaceBgOpacity} min={1} max={100} onChange={v => updateConfig({ interfaceBgOpacity: v })} 
                      hint="（1-100，数值越大背景图越清晰可见）" 
                    />
                    <RangeSlider 
                      label="背景模糊程度" value={config.interfaceBgBlur} min={0} max={20} onChange={v => updateConfig({ interfaceBgBlur: v })} 
                      hint="（0-20，数值越大背景越模糊，类似毛玻璃效果）" 
                    />
                  </div>
                </div>

                <div className="space-y-6">
                  <RangeSlider 
                    label="全局内容透明度" value={config.contentOpacity} min={1} max={100} onChange={v => updateConfig({ contentOpacity: v })} 
                    hint="（1-100，数值越小组件越透明）" 
                  />
                  
                  <div className="pt-2">
                    <div className="flex items-center gap-4 mb-4">
                      <div className="flex-1">
                        <label className="text-sm font-medium block mb-2" style={{ color: "var(--text)" }}>阴影颜色</label>
                        <div className="flex items-center gap-3">
                           <input type="color" value={config.shadowColor} onChange={e => updateConfig({ shadowColor: e.target.value })} 
                             className="w-10 h-10 p-1 rounded-lg border cursor-pointer bg-[var(--bg-hover)]" 
                             style={{ borderColor: "var(--border)" }} />
                           <span className="text-xs font-mono" style={{ color: "var(--text-muted)" }}>{config.shadowColor.toUpperCase()}</span>
                        </div>
                      </div>
                    </div>
                    
                    <RangeSlider 
                      label="阴影透明度" value={config.shadowOpacity} min={0} max={100} onChange={v => updateConfig({ shadowOpacity: v })} 
                      hint="（0-100，数值越大阴影越浓）" 
                    />
                  </div>
                </div>
              </div>
            </div>
          </section>
        </BlurFade>

        {/* --- 首页设置 --- */}
        <BlurFade delay={0.2}>
          <section className="rounded-2xl border p-6" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <h2 className="text-base font-medium mb-5 flex items-center gap-2">
              <LayoutDashboard size={18} style={{ color: "var(--text-secondary)" }} />
              首页展示
            </h2>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-8 items-start">
              <div className="space-y-6">
                <div>
                  <label className="text-sm font-medium mb-3 block" style={{ color: "var(--text)" }}>状态显示方式</label>
                  <div className="flex bg-[var(--bg)] p-1 rounded-xl border w-fit" style={{ borderColor: "var(--border)" }}>
                    <button onClick={() => updateConfig({ homeMode: "card" })}
                      className="px-6 py-2 rounded-lg text-xs font-medium transition-all"
                      style={{
                        background: config.homeMode === "card" ? "var(--bg-elevated)" : "transparent",
                        color: config.homeMode === "card" ? "var(--accent)" : "var(--text-muted)",
                        boxShadow: config.homeMode === "card" ? "var(--shadow-sm)" : "none"
                      }}>
                      卡片模式
                    </button>
                    <button onClick={() => updateConfig({ homeMode: "classic" })}
                      className="px-6 py-2 rounded-lg text-xs font-medium transition-all"
                      style={{
                        background: config.homeMode === "classic" ? "var(--bg-elevated)" : "transparent",
                        color: config.homeMode === "classic" ? "var(--accent)" : "var(--text-muted)",
                        boxShadow: config.homeMode === "classic" ? "var(--shadow-sm)" : "none"
                      }}>
                      经典模式
                    </button>
                  </div>
                  <p className="text-[10px] mt-2" style={{ color: "var(--text-muted)" }}>选择首页概览信息的显示方式，影响概览卡片的布局样式。</p>
                </div>

                <RangeSlider 
                  label="状态字体大小" value={config.homeFontSize} min={12} max={50} onChange={v => updateConfig({ homeFontSize: v })} 
                  hint="（12-50，数值越大字体越大）" 
                />
              </div>

              {/* Preview specific to Home Settings */}
              <div className="p-5 rounded-xl border flex flex-col items-center justify-center space-y-3 relative overflow-hidden" 
                   style={{ background: "var(--bg-hover)", borderColor: "var(--border)", height: "160px" }}>
                 <div className="text-[10px] absolute top-3 left-4 uppercase tracking-wider font-semibold" style={{ color: "var(--text-muted)" }}>
                   PREVIEW
                 </div>
                 <div className={`transition-all flex items-center justify-center rounded-2xl ${config.homeMode === 'card' ? 'bg-[var(--bg-card)] shadow-[var(--shadow-sm)] border border-[var(--border)] p-6' : 'p-2'}`}>
                   <span className="font-bold bg-clip-text text-transparent bg-gradient-to-r from-[var(--accent)] to-[var(--success)]" 
                         style={{ fontSize: `${config.homeFontSize}px` }}>
                     运行流畅
                   </span>
                 </div>
                 <div className="flex items-center gap-1.5 mt-2">
                   <span className="w-2 h-2 rounded-full bg-[var(--success)] pulse-dot"></span>
                   <span className="text-xs" style={{ color: "var(--text-secondary)" }}>系统负载状态</span>
                 </div>
              </div>
            </div>
          </section>
        </BlurFade>

        {/* --- 登录界面设置 --- */}
        <BlurFade delay={0.25}>
          <section className="rounded-2xl border p-6" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <h2 className="text-base font-medium mb-5 flex items-center gap-2">
              <LogIn size={18} style={{ color: "var(--text-secondary)" }} />
              登录界面
            </h2>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-8 items-start">
              <div className="space-y-6">
                <ImageBox 
                  label="登录 Logo 图片" 
                  hint="将会显示在登录页面背景（建议图片大小: 64px*64px，推荐 SVG 格式）" 
                  imageUrl={config.logoImage}
                  onChange={(url) => updateConfig({ logoImage: url })}
                  onClear={() => updateConfig({ logoImage: null })}
                />
                <RangeSlider 
                  label="登录内容透明度" value={config.loginContentOpacity} min={1} max={100} onChange={v => updateConfig({ loginContentOpacity: v })} 
                  hint="（1-100，数值越小越透明）" 
                />
              </div>
              
              <div>
                <ImageBox 
                  label="登录背景图片" 
                  hint="登录页面全屏背景（建议大小: 1920*1080）" 
                  imageUrl={config.loginBgImage}
                  onChange={(url) => updateConfig({ loginBgImage: url })}
                  onClear={() => updateConfig({ loginBgImage: null })}
                />
              </div>
            </div>
          </section>
        </BlurFade>
      </div>

    </div>
  );
}
