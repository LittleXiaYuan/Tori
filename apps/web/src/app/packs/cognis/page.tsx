"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

// The Cogni assistant management UI was consolidated into the single canonical
// "/cognis" (我的 Cogni) page. This route now redirects there so old links,
// bookmarks, and the pack menu entry keep working.
export default function PacksCognisRedirect() {
  const router = useRouter();
  useEffect(() => {
    router.replace("/cognis");
  }, [router]);
  return (
    <div className="p-10 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>
      正在前往「Cogni」…
    </div>
  );
}
