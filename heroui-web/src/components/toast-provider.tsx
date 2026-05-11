"use client";

import { useEffect, useState, useCallback, createContext, useContext } from "react";
import { formatErrorMessage } from "@/lib/error-utils";

interface ToastItem {
  id: string;
  message: string;
  type: "success" | "error" | "info" | "warning";
}

interface ToastCtx {
  toast: (message: string, type?: ToastItem["type"]) => void;
}

const ToastContext = createContext<ToastCtx>({ toast: () => {} });
export const useToast = () => useContext(ToastContext);

let globalToast: ToastCtx["toast"] = () => {};
export function showToast(message: string, type: ToastItem["type"] = "info") {
  globalToast(type === "error" ? formatErrorMessage(message, "操作失败") : message, type);
}

/** Shorthand for error toasts from catch blocks */
export function showErrorToast(e: unknown, fallback = "操作失败") {
  globalToast(formatErrorMessage(e, fallback), "error");
}

const typeStyles: Record<ToastItem["type"], { bg: string; border: string; text: string }> = {
  success: { bg: "rgba(34,197,94,0.12)", border: "rgba(34,197,94,0.3)", text: "#22c55e" },
  error: { bg: "rgba(239,68,68,0.12)", border: "rgba(239,68,68,0.3)", text: "#ef4444" },
  warning: { bg: "rgba(245,158,11,0.12)", border: "rgba(245,158,11,0.3)", text: "#f59e0b" },
  info: { bg: "rgba(0,111,238,0.12)", border: "rgba(0,111,238,0.3)", text: "#006fee" },
};

export function Toaster() {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const toast = useCallback((message: string, type: ToastItem["type"] = "info") => {
    const id = `t-${Date.now()}-${Math.random().toString(36).slice(2)}`;
    const displayMessage = type === "error" ? formatErrorMessage(message, "操作失败") : message;
    setToasts((prev) => [...prev, { id, message: displayMessage, type }]);
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 3500);
  }, []);

  useEffect(() => { globalToast = toast; }, [toast]);

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-4 right-4 z-[9999] flex flex-col gap-2 max-w-sm" role="status" aria-live="polite">
      {toasts.slice(-5).map((t) => {
        const s = typeStyles[t.type];
        return (
          <div
            key={t.id}
            className="px-4 py-3 rounded-lg text-sm font-medium shadow-lg animate-fade-in-up flex items-start gap-2"
            style={{ background: s.bg, border: `1px solid ${s.border}`, color: s.text }}
          >
            <span className="flex-1">{t.message}</span>
            <button onClick={() => dismiss(t.id)} className="shrink-0 opacity-60 hover:opacity-100 transition-opacity" aria-label="关闭">×</button>
          </div>
        );
      })}
    </div>
  );
}
