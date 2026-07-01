"use client";

// Provider / channel management now lives in the Cherry settings modal
// (Models section). This route renders the same shared <ProvidersPanel/> so
// old links (e.g. /settings/providers?tab=tori&focus=...) keep working with
// no duplicated UI.

import { useRouter, useSearchParams } from "next/navigation";
import { ProvidersPanel } from "@/components/settings/providers-panel";

export default function ProvidersPage() {
  const router = useRouter();
  const params = useSearchParams();
  const focus = params.get("focus") || params.get("provider") || params.get("provider_id");
  const tab = params.get("tab");
  return (
    <div className="page-root">
      <ProvidersPanel
        initialTab={tab ?? undefined}
        focusProviderId={focus}
        onNavigateChat={() => router.push("/chat")}
      />
    </div>
  );
}
