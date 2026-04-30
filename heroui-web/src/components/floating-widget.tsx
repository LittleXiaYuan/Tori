"use client";

import { useState, useRef, useCallback, useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
import {
  MessageCircle, Send, X, Minimize2, Sparkles,
  Settings, Plus, Loader2, Wifi, WifiOff,
} from "lucide-react";
import { api, getAuthHeaders } from "@/lib/api";

const POSITION_KEY = "yunque_widget_pos";

interface RecentMsg { role: string; content: string }

export function FloatingWidget() {
  const router = useRouter();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const [input, setInput] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  const dragRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ x: number; y: number }>({ x: -1, y: -1 });
  const dragging = useRef(false);
  const dragMoved = useRef(false);
  const [status, setStatus] = useState<"idle" | "loading" | "connected" | "error">("idle");
  const [recentMsgs, setRecentMsgs] = useState<RecentMsg[]>([]);
  const statusTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;
    try {
      const stored = localStorage.getItem(POSITION_KEY);
      if (stored) setPos(JSON.parse(stored));
      else setPos({ x: window.innerWidth - 72, y: window.innerHeight - 72 });
    } catch {
      setPos({ x: window.innerWidth - 72, y: window.innerHeight - 72 });
    }
  }, []);

  useEffect(() => {
    const checkStatus = async () => {
      try {
        const res = await fetch("/v1/auth/status", { headers: getAuthHeaders() });
        if (res.ok) setStatus("connected");
        else setStatus("error");
      } catch { setStatus("error"); }
    };
    checkStatus();
    statusTimer.current = setInterval(checkStatus, 30_000);
    return () => { if (statusTimer.current) clearInterval(statusTimer.current); };
  }, []);

  useEffect(() => {
    if (!open) return;
    api.conversationMessages("default").then((data) => {
      const msgs = (data.messages || []).slice(-3).map((m: { role: string; content: string }) => ({
        role: m.role,
        content: m.content.length > 60 ? m.content.slice(0, 60) + "…" : m.content,
      }));
      setRecentMsgs(msgs);
    }).catch(() => setRecentMsgs([]));
  }, [open]);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    dragging.current = true;
    dragMoved.current = false;
    const startX = e.clientX;
    const startY = e.clientY;
    const startPos = { ...pos };
    const onMove = (ev: MouseEvent) => {
      if (!dragging.current) return;
      const dx = ev.clientX - startX;
      const dy = ev.clientY - startY;
      if (Math.abs(dx) > 3 || Math.abs(dy) > 3) dragMoved.current = true;
      const nx = Math.max(0, Math.min(window.innerWidth - 56, startPos.x + dx));
      const ny = Math.max(0, Math.min(window.innerHeight - 56, startPos.y + dy));
      setPos({ x: nx, y: ny });
    };
    const onUp = () => {
      dragging.current = false;
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
      setPos((p) => { localStorage.setItem(POSITION_KEY, JSON.stringify(p)); return p; });
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }, [pos]);

  const handleSend = useCallback(() => {
    const text = input.trim();
    if (!text) return;
    setInput("");
    setStatus("loading");
    setOpen(false);
    if (pathname === "/chat") {
      document.dispatchEvent(new CustomEvent("yunque:quick-send", { detail: text }));
    } else {
      router.push(`/chat?q=${encodeURIComponent(text)}`);
    }
    setTimeout(() => setStatus("connected"), 1500);
  }, [input, pathname, router]);

  if (pathname === "/login" || pathname === "/setup" || pos.x < 0) return null;

  const statusColor = status === "connected" ? "var(--yunque-success)"
    : status === "loading" ? "var(--yunque-warning)"
    : status === "error" ? "var(--yunque-danger)"
    : "var(--yunque-text-muted)";

  return (
    <div
      ref={dragRef}
      style={{ position: "fixed", left: pos.x, top: pos.y, zIndex: 9999 }}
    >
      {open && (
        <div
          className="absolute bottom-14 right-0 rounded-2xl overflow-hidden"
          style={{
            width: 340,
            background: "var(--yunque-elevated)",
            border: "1px solid var(--yunque-border)",
            boxShadow: "0 8px 32px rgba(0,0,0,0.24), 0 2px 8px rgba(0,0,0,0.12)",
            animation: "widget-open 0.2s ease-out",
          }}
        >
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-3" style={{ borderBottom: "1px solid var(--yunque-border)" }}>
            <div className="flex items-center gap-2">
              <Sparkles size={14} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>快捷对话</span>
              <span className="flex items-center gap-1 text-[10px]" style={{ color: statusColor }}>
                {status === "connected" ? <Wifi size={10} /> : status === "loading" ? <Loader2 size={10} className="animate-spin" /> : <WifiOff size={10} />}
                {status === "connected" ? "在线" : status === "loading" ? "发送中" : status === "error" ? "离线" : ""}
              </span>
            </div>
            <button onClick={() => setOpen(false)} className="p-1 rounded-lg transition-colors hover:bg-white/10" style={{ color: "var(--yunque-text-muted)" }}>
              <Minimize2 size={14} />
            </button>
          </div>

          {/* Recent messages preview */}
          {recentMsgs.length > 0 && (
            <div className="px-3 pt-2 pb-1" style={{ maxHeight: 120, overflowY: "auto" }}>
              {recentMsgs.map((m, i) => (
                <div key={i} className="flex gap-2 py-1" style={{ fontSize: "var(--text-xs)" }}>
                  <span style={{ color: m.role === "user" ? "var(--yunque-accent)" : "var(--yunque-text-muted)", fontWeight: 600, flexShrink: 0 }}>
                    {m.role === "user" ? "你" : "AI"}
                  </span>
                  <span style={{ color: "var(--yunque-text-secondary)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {m.content}
                  </span>
                </div>
              ))}
            </div>
          )}

          {/* Input */}
          <div className="p-3">
            <div className="flex items-center gap-2 rounded-xl px-3 py-2.5" style={{ background: "var(--yunque-bg-muted)", border: "1px solid var(--yunque-border)", transition: "border-color 0.15s" }}>
              <input
                ref={inputRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") handleSend(); if (e.key === "Escape") setOpen(false); }}
                placeholder="输入消息或命令…"
                className="flex-1 bg-transparent outline-none text-sm"
                style={{ color: "var(--yunque-text)" }}
                autoFocus
              />
              <button
                onClick={handleSend}
                disabled={!input.trim()}
                className="p-1.5 rounded-lg transition-all"
                style={{
                  background: input.trim() ? "var(--neutral-strong-bg)" : "transparent",
                  color: input.trim() ? "var(--neutral-strong-fg)" : "var(--yunque-text-muted)",
                  opacity: input.trim() ? 1 : 0.5,
                  transform: input.trim() ? "scale(1)" : "scale(0.9)",
                }}
              >
                <Send size={12} />
              </button>
            </div>

            {/* Quick commands */}
            <div className="mt-2 flex gap-1.5 flex-wrap">
              {["/research ", "/screenshot ", "/search "].map((cmd) => (
                <button
                  key={cmd}
                  onClick={() => { setInput(cmd); inputRef.current?.focus(); }}
                  className="rounded-full px-2.5 py-1 text-[10px] font-medium transition-colors hover:opacity-80"
                  style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent)", border: "1px solid var(--yunque-border)" }}
                >
                  {cmd.trim()}
                </button>
              ))}
            </div>

            {/* Quick action buttons */}
            <div className="mt-2 flex gap-2" style={{ borderTop: "1px solid var(--yunque-border)", paddingTop: 8 }}>
              <button
                onClick={() => { setOpen(false); router.push("/chat"); }}
                className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[11px] font-medium transition-colors hover:bg-white/8"
                style={{ color: "var(--yunque-text-secondary)" }}
              >
                <Plus size={11} /> 新对话
              </button>
              <button
                onClick={() => { setOpen(false); router.push("/settings"); }}
                className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[11px] font-medium transition-colors hover:bg-white/8"
                style={{ color: "var(--yunque-text-secondary)" }}
              >
                <Settings size={11} /> 设置
              </button>
            </div>
          </div>
        </div>
      )}

      {/* FAB button */}
      <button
        onMouseDown={handleMouseDown}
        onClick={() => { if (!dragMoved.current) setOpen(!open); }}
        className="w-12 h-12 rounded-full flex items-center justify-center transition-all hover:scale-110 active:scale-95"
        style={{
          background: "var(--neutral-strong-bg)",
          color: "var(--neutral-strong-fg)",
          boxShadow: "0 4px 16px rgba(0,0,0,0.2), 0 1px 4px rgba(0,0,0,0.1)",
          position: "relative",
        }}
      >
        {open ? <X size={18} /> : status === "loading" ? <Loader2 size={18} className="animate-spin" /> : <MessageCircle size={18} />}
        {/* Status dot */}
        <span
          style={{
            position: "absolute", top: 2, right: 2,
            width: 8, height: 8, borderRadius: "50%",
            background: statusColor,
            border: "2px solid var(--neutral-strong-bg)",
          }}
        />
      </button>
    </div>
  );
}
