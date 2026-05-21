"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { Spinner } from "@heroui/react";
import { api } from "@/lib/api";

// Root landing page.
//
// The desktop client (and bare browser visits to `/`) land here. We dispatch
// in the client because SSR doesn't know about the user's Tori binding state,
// and running the probe server-side would add a hop that fails whenever the
// Go sidecar isn't up yet.
//
// Branches:
//   1. LLM fully configured         → /dashboard (scenario-first workspace)
//   2. First run, nothing bound yet → /setup (choose Tori vs API key)
//   3. Probe failed (backend down)  → /dashboard (let the shell show the error)
export default function Home() {
  const router = useRouter();

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [env, tori] = await Promise.all([
          api.setupDetect().catch(() => null),
          api.toriStatus().catch(() => ({ bound: false })),
        ]);

        if (cancelled) return;

        const needsSetup = Boolean(env?.first_run) && !tori?.bound;
        router.replace(needsSetup ? "/setup" : "/dashboard");
      } catch {
        if (!cancelled) router.replace("/dashboard");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [router]);

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Spinner size="lg" />
    </div>
  );
}
