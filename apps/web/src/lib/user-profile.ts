"use client";

/**
 * Local user profile — nickname + avatar, stored in localStorage.
 *
 * This is deliberately NOT authentication. Yunque is a local-first desktop
 * app; a real account/login system would add friction without value for a
 * single-seat owner. What actually warms the product is letting the app know
 * *who you are* so it can address you by name (see chat-hero) and show your
 * face in the chrome — Codex-style personalization, no server, no password.
 *
 * Both fields live under stable keys so the greeting (chat-hero) and the
 * title-bar chip read the same source. Writes broadcast `PROFILE_EVENT` so
 * every mounted consumer re-reads without a full reload.
 */

import { useEffect, useState } from "react";

const NICK_KEY = "yunque_user_nickname";
const AVATAR_KEY = "yunque_user_avatar";
export const PROFILE_EVENT = "yunque:profile-updated";

export interface UserProfile {
  nickname: string | null;
  avatar: string | null; // data: URL (downscaled) or null
}

function read(key: string): string | null {
  if (typeof window === "undefined") return null;
  const v = localStorage.getItem(key);
  return v && v.trim() ? v : null;
}

export function getProfile(): UserProfile {
  return { nickname: read(NICK_KEY), avatar: read(AVATAR_KEY) };
}

export function getNickname(): string | null {
  const v = read(NICK_KEY);
  return v ? v.trim() : null;
}

function broadcast(): void {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent(PROFILE_EVENT));
  }
}

export function setNickname(name: string): void {
  if (typeof window === "undefined") return;
  const v = name.trim();
  if (v) localStorage.setItem(NICK_KEY, v);
  else localStorage.removeItem(NICK_KEY);
  broadcast();
}

export function setAvatar(dataUrl: string | null): void {
  if (typeof window === "undefined") return;
  if (dataUrl) localStorage.setItem(AVATAR_KEY, dataUrl);
  else localStorage.removeItem(AVATAR_KEY);
  broadcast();
}

/**
 * Read an image File, downscale it to a small square data URL so we never
 * bloat localStorage with a multi-MB photo. Returns a JPEG data: URL.
 */
export function fileToAvatarDataUrl(file: File, size = 128): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error("read failed"));
    reader.onload = () => {
      const img = new Image();
      img.onerror = () => reject(new Error("decode failed"));
      img.onload = () => {
        const canvas = document.createElement("canvas");
        canvas.width = size;
        canvas.height = size;
        const ctx = canvas.getContext("2d");
        if (!ctx) return reject(new Error("no 2d context"));
        // Cover-crop to a centered square.
        const min = Math.min(img.width, img.height);
        const sx = (img.width - min) / 2;
        const sy = (img.height - min) / 2;
        ctx.drawImage(img, sx, sy, min, min, 0, 0, size, size);
        resolve(canvas.toDataURL("image/jpeg", 0.85));
      };
      img.src = String(reader.result);
    };
    reader.readAsDataURL(file);
  });
}

/** Reactive profile that re-reads on PROFILE_EVENT and cross-tab storage. */
export function useUserProfile(): UserProfile {
  const [profile, setProfile] = useState<UserProfile>({ nickname: null, avatar: null });
  useEffect(() => {
    const sync = () => setProfile(getProfile());
    sync();
    window.addEventListener(PROFILE_EVENT, sync);
    window.addEventListener("storage", sync);
    return () => {
      window.removeEventListener(PROFILE_EVENT, sync);
      window.removeEventListener("storage", sync);
    };
  }, []);
  return profile;
}

/** Initial display letter when there's no avatar. */
export function profileInitial(nickname: string | null): string {
  if (!nickname) return "我";
  return [...nickname][0] || "我";
}
