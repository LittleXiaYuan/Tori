"use client";

import { Copy } from "lucide-react";
import { useId } from "react";
import { showToast } from "@/components/toast-provider";

interface JsonViewerProps {
  title: string;
  value: unknown;
  rows?: number;
}

function stringify(value: unknown): string {
  if (typeof value === "string") return value;
  return JSON.stringify(value, null, 2);
}

export function JsonViewer({ title, value, rows = 8 }: JsonViewerProps) {
  const titleId = useId();
  const text = stringify(value);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      showToast("已复制 JSON", "success");
    } catch {
      showToast("复制失败，请手动选择内容", "error");
    }
  };

  return (
    <section className="json-viewer" aria-labelledby={titleId}>
      <div className="json-viewer__header">
        <h3 id={titleId} className="json-viewer__title">
          {title}
        </h3>
        <button type="button" className="json-viewer__copy" onClick={copy}>
          <Copy size={13} aria-hidden="true" />
          复制
        </button>
      </div>
      <pre
        className="json-viewer__pre"
        tabIndex={0}
        aria-label={`${title} JSON`}
        style={{ maxHeight: `${Math.max(rows, 4) * 1.55 + 1.5}rem` }}
      >
        <code>{text}</code>
      </pre>
    </section>
  );
}
