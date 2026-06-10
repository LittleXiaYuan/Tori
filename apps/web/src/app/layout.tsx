import type { Metadata } from "next";
import { Plus_Jakarta_Sans } from "next/font/google";
import "./globals.css";
import AppShell from "@/components/app-shell";

const jakarta = Plus_Jakarta_Sans({
  subsets: ["latin"],
  variable: "--font-sans",
  display: "swap",
  weight: ["300", "400", "500", "600", "700", "800"],
});

export const metadata: Metadata = {
  title: "Yunque Agent",
  description: "本地优先的个人 AI 工作伙伴：从场景到行动、产物、反馈和记忆。",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh" className={`light ${jakarta.variable}`} data-theme="light" suppressHydrationWarning>
      <head>
        <script
          dangerouslySetInnerHTML={{
            // First-paint theme bootstrap. The only user-controlled values we
            // act on (other than palette/radius tokens) are interfaceBgImage
            // and the (stringified) opacity/blur numbers; the `safe(u)` check
            // mirrors isSafeAssetURL() from lib/safe-url.ts and refuses any
            // URL that is not https: or data:image/.
            __html: `(function(){try{var t=JSON.parse(localStorage.getItem('yunque_theme')||'{}');var m=t.presetTheme||'light';if(m==='auto')m=matchMedia('(prefers-color-scheme:dark)').matches?'dark':'light';var h=document.documentElement,s=h.style,L=m==='light';h.className=m+' ${jakarta.variable}';h.setAttribute('data-theme',m);try{var ti=window.__TAURI_INTERNALS__;if(ti&&typeof ti.invoke==='function'){setTimeout(function(){try{ti.invoke('apply_window_theme',{theme:m})}catch(e){}},0)};if(navigator.platform&&navigator.platform.startsWith('Mac')){h.setAttribute('data-platform','macos')}else if(navigator.platform&&navigator.platform.startsWith('Linux')){h.setAttribute('data-platform','linux')}}catch(e){}var colors={time_monologue:'#a1a1aa',deep_sea:'#0284c7',purple_jade:'#a855f7',mint_ice:'#2dd4bf',sakura_fall:'#f472b6',gold_sand:'#d97706'};var c=t.colorTheme==='custom'?t.customColor:(colors[t.colorTheme]||'#0284c7');if(typeof c!=='string'||!/^#[0-9a-fA-F]{6}$/.test(c))c='#0284c7';var p=function(x,i){return parseInt(x.slice(i,i+2),16)};var r=p(c,1),g=p(c,3),b=p(c,5);s.setProperty('--yunque-accent',c);s.setProperty('--yunque-accent-hover',c);s.setProperty('--yunque-accent-muted','rgba('+r+','+g+','+b+','+(L?'.10':'.12')+')');s.setProperty('--yunque-accent-soft','rgba('+r+','+g+','+b+','+(L?'.05':'.06')+')');s.setProperty('--yunque-accent-glow','rgba('+r+','+g+','+b+','+(L?'.12':'.15')+')');var rMap={right:'0',default:'10',small:'6',medium:'14',large:'18'};var rv=rMap[t.radius]||'10';s.setProperty('--radius-sm',(rv==='0'?'0':Math.max(rv-2,2))+'px');s.setProperty('--radius-md',rv+'px');s.setProperty('--radius-lg',(rv==='0'?'0':+rv+4)+'px');s.setProperty('--radius-xl',(rv==='0'?'0':+rv+8)+'px');function safe(u){if(typeof u!=='string'||!u)return false;if(u.indexOf('data:image/')===0)return true;try{var pu=new URL(u);return pu.protocol==='https:'}catch(e){return false}}if(t.interfaceBgImage&&safe(t.interfaceBgImage)){document.addEventListener('DOMContentLoaded',function(){var bdy=document.body;bdy.style.backgroundImage='url('+CSS.escape(t.interfaceBgImage)+')';bdy.style.backgroundSize='cover';bdy.style.backgroundPosition='center';bdy.style.backgroundAttachment='fixed';var a=(t.interfaceBgOpacity||30)/100;var oa=(1-a)*0.85;var ob=L?'255,255,255':'10,10,12';s.setProperty('--yunque-bg-overlay','rgba('+ob+','+oa.toFixed(2)+')');var bc=L?'255,255,255':'10,10,12';s.setProperty('--yunque-bg','rgba('+bc+','+(1-a*0.6).toFixed(2)+')');var bl=t.interfaceBgBlur||0;if(bl>0){var ov=document.getElementById('bg-overlay');if(ov)ov.style.backdropFilter='blur('+bl+'px)'}})}}catch(e){}})()`,
          }}
        />
      </head>
      <body className="flex min-h-screen text-foreground" suppressHydrationWarning>
        <div id="bg-overlay" />
        <a href="#main-content" className="skip-link" suppressHydrationWarning>跳到主要内容</a>
        <AppShell>{children}</AppShell>
      </body>
    </html>
  );
}
