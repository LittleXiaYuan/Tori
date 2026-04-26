"use client";

import { useState, useRef, useCallback, useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
import { MessageCircle, Send, X, Minimize2, Sparkles } from "lucide-react";

const POSITION_KEY = "yunque_widget_pos";

export function FloatingWidget() {
  const router = useRouter();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const [input, setInput] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  const dragRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ x: number; y: number }>({ x: -1, y: -1 });
  const dragging = useRef(false);

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

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    dragging.current = true;
    const startX = e.clientX;
    const startY = e.clientY;
    const startPos = { ...pos };
    const onMove = (ev: MouseEvent) => {
      if (!dragging.current) return;
      const nx = Math.max(0, Math.min(window.innerWidth - 56, startPos.x + (ev.clientX - startX)));
      const ny = Math.max(0, Math.min(window.innerHeight - 56, startPos.y + (ev.clientY - startY)));
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
    setOpen(false);
    if (pathname === "/chat") {
      document.dispatchEvent(new CustomEvent("yunque:quick-send", { detail: text }));
    } else {
      router.push(`/chat?q=${encodeURIComponent(text)}`);
    }
  }, [input, pathname, router]);

  if (pathname === "/login" || pathname === "/setup" || pos.x < 0) return null;

  return (
    <div
      ref={dragRef}
      style={{ position: "fixed", left: pos.x, top: pos.y, zIndex: 9999 }}
    >
      {open && (
        <div
          className="absolute bottom-14 right-0 rounded-2xl overflow-hidden animate-slide-in-right"
          style={{
            width: 320,
            background: "rgba(15,16,20,0.96)",
            border: "1px solid rgba(255,255,255,0.08)",
            boxShadow: "0 20px 60px rgba(0,0,0,0.5)",
            backdropFilter: "blur(18px)",
          }}
        >
          <div className="flex items-center justify-between px-4 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
            <div className="flex items-center gap-2">
              <Sparkles size={14} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>快捷对话</span>
            </div>
            <button onClick={() => setOpen(false)} className="p-1 rounded-lg" style={{ color: "var(--yunque-text-muted)" }}>
              <Minimize2 size={14} />
            </button>
          </div>
          <div className="p-3">
            <div className="flex items-center gap-2 rounded-xl px-3 py-2.5" style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.06)" }}>
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
                className="p-1.5 rounded-lg transition-colors"
                style={{ background: input.trim() ? "var(--yunque-accent)" : "rgba(255,255,255,0.06)", color: input.trim() ? "#fff" : "var(--yunque-text-muted)" }}
              >
                <Send size={12} />
              </button>
            </div>
            <div className="mt-2 flex gap-1.5">
              {["/research ", "/screenshot ", "/search "].map((cmd) => (
                <button
                  key={cmd}
                  onClick={() => { setInput(cmd); inputRef.current?.focus(); }}
                  className="rounded-full px-2.5 py-1 text-[10px] font-medium"
                  style={{ background: "rgba(59,130,246,0.08)", color: "#93c5fd", border: "1px solid rgba(59,130,246,0.15)" }}
                >
                  {cmd.trim()}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}

      <button
        onMouseDown={handleMouseDown}
        onClick={(e) => { if (!dragging.current) setOpen(!open); }}
        className="w-12 h-12 rounded-full flex items-center justify-center transition-all hover:scale-110"
        style={{
          background: open ? "var(--yunque-accent)" : "linear-gradient(135deg, var(--yunque-accent), rgba(37,99,235,0.9))",
          color: "#fff",
          boxShadow: "0 8px 24px rgba(59,130,246,0.3), 0 0 0 1px rgba(255,255,255,0.1)",
        }}
      >
        {open ? <X size={18} /> : <MessageCircle size={18} />}
      </button>
    </div>
  );
}
