"use client";

/**
 * /settings is no longer a standalone full-page workbench — settings now live
 * entirely in the Cherry settings modal (opened from the title-bar gear). This
 * route is kept only so old links / bookmarks / in-app router.push("/settings")
 * calls still work: it bounces back to chat and asks the shell to open the
 * modal on the matching section.
 */

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";

export default function SettingsRedirect() {
  const router = useRouter();
  const params = useSearchParams();

  useEffect(() => {
    const section = params.get("section") || undefined;
    router.replace("/chat");
    // Defer so AppShell (which owns the modal) is mounted on the chat route
    // before we ask it to open.
    const t = setTimeout(() => {
      window.dispatchEvent(new CustomEvent("yunque:open-settings", { detail: { section } }));
    }, 60);
    return () => clearTimeout(t);
  }, [router, params]);

  return (
    <div role="status" aria-label="正在打开设置" style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "60vh", color: "var(--yunque-text-muted)", fontSize: 14 }}>
      正在打开设置…
    </div>
  );
}
