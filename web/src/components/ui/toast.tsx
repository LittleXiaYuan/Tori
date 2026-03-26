"use client";

import { useState, useEffect, useCallback, createContext, useContext, type ReactNode } from "react";
import { CheckCircle2, XCircle, AlertTriangle, Info, X } from "lucide-react";

// ── Types ──
type ToastType = "success" | "error" | "warning" | "info";

interface Toast {
  id: number;
  type: ToastType;
  message: string;
  duration: number;
}

interface ToastContextValue {
  success: (msg: string) => void;
  error: (msg: string) => void;
  warning: (msg: string) => void;
  info: (msg: string) => void;
}

// ── Context ──
const ToastContext = createContext<ToastContextValue | null>(null);

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}

// ── Toast Item ──
const iconMap: Record<ToastType, React.ElementType> = {
  success: CheckCircle2,
  error: XCircle,
  warning: AlertTriangle,
  info: Info,
};

const colorMap: Record<ToastType, { bg: string; border: string; text: string; icon: string }> = {
  success: { bg: "rgba(34,197,94,0.08)", border: "rgba(34,197,94,0.25)", text: "#22c55e", icon: "#22c55e" },
  error: { bg: "rgba(239,68,68,0.08)", border: "rgba(239,68,68,0.25)", text: "#ef4444", icon: "#ef4444" },
  warning: { bg: "rgba(234,179,8,0.08)", border: "rgba(234,179,8,0.25)", text: "#eab308", icon: "#eab308" },
  info: { bg: "rgba(59,130,246,0.08)", border: "rgba(59,130,246,0.25)", text: "#3b82f6", icon: "#3b82f6" },
};

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: () => void }) {
  const [exiting, setExiting] = useState(false);
  const Icon = iconMap[toast.type];
  const colors = colorMap[toast.type];

  useEffect(() => {
    const t1 = setTimeout(() => setExiting(true), toast.duration - 300);
    const t2 = setTimeout(onDismiss, toast.duration);
    return () => { clearTimeout(t1); clearTimeout(t2); };
  }, [toast.duration, onDismiss]);

  return (
    <div
      className="flex items-center gap-2.5 px-4 py-3 rounded-xl border shadow-lg backdrop-blur-md max-w-sm"
      style={{
        background: colors.bg,
        borderColor: colors.border,
        animation: exiting ? "toast-exit 0.3s ease forwards" : "toast-enter 0.3s ease forwards",
      }}
    >
      <Icon size={16} style={{ color: colors.icon, flexShrink: 0 }} />
      <span className="text-xs font-medium flex-1" style={{ color: "var(--text)" }}>
        {toast.message}
      </span>
      <button onClick={() => { setExiting(true); setTimeout(onDismiss, 300); }}
        className="p-0.5 rounded hover:opacity-70 transition-opacity" style={{ color: "var(--text-muted)" }}>
        <X size={12} />
      </button>
    </div>
  );
}

// ── Provider ──
let nextId = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const push = useCallback((type: ToastType, message: string, duration = 4000) => {
    const id = ++nextId;
    setToasts(prev => [...prev.slice(-4), { id, type, message, duration }]); // max 5
  }, []);

  const dismiss = useCallback((id: number) => {
    setToasts(prev => prev.filter(t => t.id !== id));
  }, []);

  const ctx: ToastContextValue = {
    success: (msg) => push("success", msg),
    error: (msg) => push("error", msg, 6000),
    warning: (msg) => push("warning", msg, 5000),
    info: (msg) => push("info", msg),
  };

  return (
    <ToastContext.Provider value={ctx}>
      {children}
      {/* Toast container */}
      <div className="fixed bottom-4 right-4 z-[9999] flex flex-col gap-2 pointer-events-auto">
        {toasts.map(t => (
          <ToastItem key={t.id} toast={t} onDismiss={() => dismiss(t.id)} />
        ))}
      </div>
      <style>{`
        @keyframes toast-enter {
          from { opacity: 0; transform: translateY(12px) scale(0.95); }
          to { opacity: 1; transform: translateY(0) scale(1); }
        }
        @keyframes toast-exit {
          from { opacity: 1; transform: translateY(0) scale(1); }
          to { opacity: 0; transform: translateY(-8px) scale(0.95); }
        }
      `}</style>
    </ToastContext.Provider>
  );
}
