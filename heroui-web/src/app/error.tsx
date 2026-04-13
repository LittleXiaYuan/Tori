"use client";

import { Button } from "@heroui/react";
import { AlertTriangle, RefreshCw } from "lucide-react";

export default function Error({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center h-[60vh] gap-4 animate-fade-in-up">
      <div className="w-14 h-14 rounded-xl flex items-center justify-center" style={{ background: "rgba(239,68,68,0.12)" }}>
        <AlertTriangle size={28} className="text-red-500" />
      </div>
      <div className="text-center space-y-1.5">
        <h2 className="text-lg font-bold" style={{ color: "var(--yunque-text)" }}>出了点问题</h2>
        <p className="text-sm max-w-md" style={{ color: "var(--yunque-text-muted)" }}>
          {error.message || "页面加载失败，请重试"}
        </p>
      </div>
      <Button
        size="sm"
        className="gap-1.5 rounded-lg btn-accent"
        onPress={reset}
      >
        <RefreshCw size={14} /> 重试
      </Button>
    </div>
  );
}
