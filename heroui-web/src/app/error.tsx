"use client";

import { Button } from "@heroui/react";
import { AlertTriangle, Home, RefreshCw } from "lucide-react";
import { formatErrorMessage } from "@/lib/error-utils";

export default function Error({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <div className="flex h-[70vh] flex-col items-center justify-center gap-4 px-6 text-center animate-fade-in-up">
      <div className="flex h-16 w-16 items-center justify-center rounded-3xl text-2xl font-black text-white shadow-lg" style={{ background: "linear-gradient(135deg, var(--yunque-accent), #7c3aed)" }}>
        云
      </div>
      <div className="text-center space-y-1.5">
        <div className="inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs" style={{ background: "rgba(239,68,68,0.10)", color: "rgb(220,38,38)" }}>
          <AlertTriangle size={13} /> 页面没有正常打开
        </div>
        <h2 className="text-xl font-black" style={{ color: "var(--yunque-text)" }}>云雀 Agent 遇到了前端错误</h2>
        <p className="text-sm max-w-md" style={{ color: "var(--yunque-text-muted)" }}>
          {formatErrorMessage(error, "页面加载失败。请先点击重试；如果仍失败，可以重启桌面端后再打开。")}
        </p>
      </div>
      <div className="flex gap-2">
        <Button size="sm" className="gap-1.5 rounded-lg btn-accent" onPress={reset}>
          <RefreshCw size={14} /> 重试
        </Button>
        <Button size="sm" variant="ghost" className="gap-1.5 rounded-lg" onPress={() => { window.location.href = "/"; }}>
          <Home size={14} /> 回首页
        </Button>
      </div>
    </div>
  );
}
