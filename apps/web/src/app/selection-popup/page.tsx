"use client";

import { useCallback, useEffect, useState, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { SelectionToolbarBar } from "@/components/selection-toolbar";

function tauriInvoke(cmd: string, args?: Record<string, unknown>): Promise<unknown> | undefined {
  const invoke = (
    window as unknown as {
      __TAURI_INTERNALS__?: { invoke?: (c: string, a?: Record<string, unknown>) => Promise<unknown> };
    }
  ).__TAURI_INTERNALS__?.invoke;
  return invoke?.(cmd, args);
}

function SelectionPopupInner() {
  const params = useSearchParams();
  const [text, setText] = useState(() => params.get("text")?.trim() ?? "");

  useEffect(() => {
    const initial = params.get("text")?.trim();
    if (initial) setText(initial);
  }, [params]);

  useEffect(() => {
    let unlisten: (() => void) | undefined;
    void import("@tauri-apps/api/event")
      .then(({ listen }) => listen<string>("yunque:selection-text", (e) => {
        const payload = typeof e.payload === "string" ? e.payload : String(e.payload ?? "");
        if (payload.trim()) setText(payload.trim());
      }))
      .then((fn) => {
        unlisten = fn;
      })
      .catch((err) => {
        console.warn("[selection-popup] tauri listen yunque:selection-text failed", err);
      });

    return () => unlisten?.();
  }, []);

  const dismiss = useCallback(() => {
    void tauriInvoke("selection_popup_dismiss");
  }, []);

  const handleAction = useCallback(
    (action: string, selected: string) => {
      const prompts: Record<string, string> = {
        ai_search: `搜索：${selected}`,
        translate: `翻译以下内容（如果是中文则翻译为英文，如果是英文则翻译为中文）：\n\n${selected}`,
        explain: `解释：${selected}`,
        save: `将以下内容保存到知识库：\n\n${selected}`,
      };
      const prompt = prompts[action] ?? selected;
      void tauriInvoke("floating_send_to_chat", { text: prompt });
      dismiss();
    },
    [dismiss],
  );

  if (!text || text.length < 2) {
    return <div className="selection-popup-root" />;
  }

  return (
    <div className="selection-popup-root">
      <SelectionToolbarBar
        text={text}
        inline
        onAction={handleAction}
        onDismiss={dismiss}
      />
    </div>
  );
}

export default function SelectionPopupPage() {
  return (
    <Suspense fallback={<div className="selection-popup-root" />}>
      <SelectionPopupInner />
    </Suspense>
  );
}
