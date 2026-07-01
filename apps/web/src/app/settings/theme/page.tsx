"use client";

// The theme editor now lives in the Cherry settings modal (Display section).
// This route is kept only so existing links / bookmarks keep working: it
// renders the same shared <ThemePanel/> the modal uses, so there's no
// duplicated UI to drift.

import { ThemePanel } from "@/components/settings/theme-panel";

export default function ThemeSettingsPage() {
  return (
    <div className="page-root">
      <ThemePanel />
    </div>
  );
}
