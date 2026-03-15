"use client";

import { useEffect } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("[page error]", error);
  }, [error]);

  return (
    <div className="flex flex-col items-center justify-center h-[50vh] gap-4">
      <AlertTriangle size={48} style={{ color: "var(--danger)" }} />
      <h2 className="text-lg font-medium">页面出错了</h2>
      <p
        className="text-sm max-w-md text-center"
        style={{ color: "var(--text-muted)" }}
      >
        {error.message || "发生了未知错误"}
      </p>
      <button
        onClick={reset}
        className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm"
        style={{ background: "var(--accent)", color: "white" }}
      >
        <RefreshCw size={14} /> 重试
      </button>
    </div>
  );
}
