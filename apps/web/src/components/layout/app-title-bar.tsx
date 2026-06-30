"use client";

/**
 * AppTitleBar — the unified, full-width custom title bar (Cherry/QQ-style).
 *
 * Replaces the empty transparent drag strip that made the app read as a
 * "webpage inside an OS window". This bar is a solid, app-colored surface
 * spanning the whole window width: brand on the left, then the user profile
 * chip + theme + settings + window controls on the right. All three columns
 * of the app sit below it, stitched together under one continuous header.
 *
 * The whole bar is the drag region (-webkit-app-region: drag); interactive
 * children opt out via the `.no-drag` class.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Tooltip, Popover, Button, TextField, Input, Label } from "@heroui/react";
import { Bird, Moon, Sun, Settings, Upload, Trash2 } from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { loadTheme, patchAndApply } from "@/lib/theme-engine";
import { WindowControls } from "@/components/title-bar";
import { showToast } from "@/components/toast-provider";
import {
  useUserProfile,
  setNickname as persistNickname,
  setAvatar as persistAvatar,
  fileToAvatarDataUrl,
  profileInitial,
} from "@/lib/user-profile";

export function AppTitleBar() {
  const router = useRouter();
  const { locale } = useI18n();
  const profile = useUserProfile();
  const [online, setOnline] = useState<boolean | null>(null);
  const [version, setVersion] = useState("");
  const [themeMode, setThemeMode] = useState<"dark" | "light">("dark");
  const [profileOpen, setProfileOpen] = useState(false);
  const [draftNick, setDraftNick] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);

  // Lightweight online heartbeat (10s), paused when hidden.
  useEffect(() => {
    let timer: ReturnType<typeof setInterval> | undefined;
    const probe = () => {
      api.healthz()
        .then((h) => { setOnline(true); setVersion(h?.version || ""); })
        .catch(() => setOnline(false));
    };
    probe();
    timer = setInterval(probe, 10000);
    const onVis = () => { if (!document.hidden) probe(); };
    document.addEventListener("visibilitychange", onVis);
    return () => { if (timer) clearInterval(timer); document.removeEventListener("visibilitychange", onVis); };
  }, []);

  useEffect(() => {
    if (typeof document === "undefined") return;
    const detect = () => {
      const t = document.documentElement.getAttribute("data-theme");
      setThemeMode(t === "light" ? "light" : "dark");
    };
    detect();
    const obs = new MutationObserver(detect);
    obs.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme", "class"] });
    return () => obs.disconnect();
  }, []);

  const toggleTheme = useCallback(() => {
    const cur = loadTheme();
    patchAndApply({ presetTheme: cur.presetTheme === "light" ? "dark" : "light" });
  }, []);

  const openSettings = useCallback(() => {
    window.dispatchEvent(new CustomEvent("yunque:open-settings"));
  }, []);

  const onPickAvatar = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    try {
      const dataUrl = await fileToAvatarDataUrl(file);
      persistAvatar(dataUrl);
    } catch {
      showToast(locale === "zh" ? "头像处理失败" : "Failed to process image", "error");
    }
  }, [locale]);

  const zh = locale === "zh";
  const appName = zh ? "云雀" : "Yunque";
  const onlineColor = online === true ? "var(--yunque-success)" : online === false ? "var(--yunque-danger)" : "var(--yunque-text-muted)";
  const onlineLabel = online === true ? `${zh ? "在线" : "Online"}${version ? ` · v${version}` : ""}` : online === false ? (zh ? "离线" : "Offline") : (zh ? "连接中…" : "Connecting…");
  const displayName = profile.nickname || (zh ? "设置你的称呼" : "Set your name");

  return (
    <div className="app-titlebar">
      {/* Brand (left) — clickable, aligns over the nav rail */}
      <button
        type="button"
        className="app-titlebar__brand no-drag"
        onClick={() => router.push("/chat")}
        aria-label={zh ? "回到对话" : "Back to chat"}
      >
        <span className="app-titlebar__logo" aria-hidden>
          {/* 云雀 brand mark — clean lark glyph (lucide Bird), white on accent */}
          <Bird size={15} strokeWidth={2.2} style={{ color: "#fff" }} aria-hidden />
          <span
            className={online === true ? "online-dot" : ""}
            style={{
              position: "absolute", bottom: -1, right: -3,
              width: 7, height: 7, borderRadius: "50%",
              background: onlineColor, border: "2px solid var(--yunque-titlebar-bg, var(--yunque-sidebar))",
            }}
          />
        </span>
        <span className="app-titlebar__name">{appName}</span>
      </button>

      {/* Right cluster: profile + theme + settings + window controls */}
      <div className="app-titlebar__right no-drag">
        <Popover
          isOpen={profileOpen}
          onOpenChange={(next) => {
            setProfileOpen(next);
            if (next) setDraftNick(profile.nickname || "");
          }}
        >
          <Popover.Trigger>
            <button
              type="button"
              className="app-titlebar__profile"
              aria-label={zh ? "个人资料" : "Profile"}
            >
              {profile.avatar ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={profile.avatar} alt="" className="app-titlebar__avatar-img" />
              ) : (
                <span className="app-titlebar__avatar-fallback">{profileInitial(profile.nickname)}</span>
              )}
              <span className="app-titlebar__profile-name">{displayName}</span>
            </button>
          </Popover.Trigger>
          <Popover.Content placement="bottom end" offset={8}>
            <Popover.Dialog
              className="flex flex-col"
              style={{ padding: 16, minWidth: 260, gap: 14, borderRadius: 14, background: "var(--yunque-elevated, var(--yunque-card))", border: "1px solid var(--yunque-border)", boxShadow: "0 12px 40px rgba(0,0,0,0.35)" }}
            >
              <div className="flex items-center gap-3">
                <div className="app-titlebar__avatar-lg">
                  {profile.avatar ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={profile.avatar} alt="" className="app-titlebar__avatar-img" />
                  ) : (
                    <span className="app-titlebar__avatar-fallback" style={{ fontSize: 20 }}>{profileInitial(profile.nickname)}</span>
                  )}
                </div>
                <div className="flex flex-col gap-1.5">
                  <Button size="sm" variant="ghost" onPress={() => fileRef.current?.click()} style={{ gap: 6 }}>
                    <Upload size={13} /> {zh ? "上传头像" : "Upload"}
                  </Button>
                  {profile.avatar && (
                    <Button size="sm" variant="ghost" onPress={() => persistAvatar(null)} style={{ gap: 6, color: "var(--yunque-text-muted)" }}>
                      <Trash2 size={13} /> {zh ? "移除" : "Remove"}
                    </Button>
                  )}
                </div>
                <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={onPickAvatar} />
              </div>

              <TextField
                value={draftNick}
                onChange={setDraftNick}
                aria-label={zh ? "称呼" : "Name"}
              >
                <Label>{zh ? "你希望被怎么称呼" : "What should we call you"}</Label>
                <Input
                  placeholder={zh ? "比如：夏鸢" : "e.g. Alex"}
                  onBlur={() => persistNickname(draftNick)}
                  onKeyDown={(e) => { if (e.key === "Enter") { persistNickname(draftNick); setProfileOpen(false); } }}
                />
              </TextField>

              <Button
                size="sm"
                onPress={() => { persistNickname(draftNick); setProfileOpen(false); }}
                style={{ background: "var(--yunque-accent)", color: "#fff", fontWeight: 600 }}
              >
                {zh ? "完成" : "Done"}
              </Button>
            </Popover.Dialog>
          </Popover.Content>
        </Popover>

        <Tooltip delay={0}>
          <Tooltip.Trigger>
            <button type="button" className="app-titlebar__icon-btn" onClick={toggleTheme} aria-label={themeMode === "dark" ? (zh ? "切换到亮色" : "Switch to Light") : (zh ? "切换到暗色" : "Switch to Dark")}>
              {themeMode === "dark" ? <Moon size={15} /> : <Sun size={15} />}
            </button>
          </Tooltip.Trigger>
          <Tooltip.Content placement="bottom">{onlineLabel}</Tooltip.Content>
        </Tooltip>
        <button type="button" className="app-titlebar__icon-btn" onClick={openSettings} aria-label={zh ? "设置" : "Settings"}>
          <Settings size={15} />
        </button>
        <WindowControls />
      </div>
    </div>
  );
}
