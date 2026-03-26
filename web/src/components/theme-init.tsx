"use client";
import { useEffect } from "react";
import { loadTheme, applyTheme, loadThemeImages } from "@/lib/theme";

export function ThemeInit() {
  useEffect(() => {
    // Apply settings immediately (no images yet)
    applyTheme(loadTheme());
    // Then load images from IndexedDB and re-apply
    loadThemeImages();
  }, []);
  return null;
}
