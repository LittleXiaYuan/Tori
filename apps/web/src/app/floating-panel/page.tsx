"use client";

import { useEffect, useState, useCallback } from "react";

interface TauriAPI {
  core: { invoke: (cmd: string, args?: Record<string, unknown>) => Promise<unknown> };
  event: {
    listen: (event: string, handler: (e: { payload: unknown }) => void) => Promise<() => void>;
    emit: (event: string, payload?: unknown) => Promise<void>;
  };
}

function tauri(): TauriAPI | undefined {
  return typeof window !== "undefined"
    ? (window as unknown as { __TAURI__?: TauriAPI }).__TAURI__
    : undefined;
}

interface FloatingItem {
  id: string;
  text: string;
  timestamp: number;
}

export default function FloatingPanelPage() {
  const [items, setItems] = useState<FloatingItem[]>([]);

  const loadItems = useCallback(async () => {
    const t = tauri();
    if (!t) return;
    try {
      const list = (await t.core.invoke("get_floating_items")) as FloatingItem[];
      setItems(list);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    loadItems();
    const t = tauri();
    if (!t) return;
    let unlisten: (() => void) | undefined;
    t.event.listen("yunque:floating-update", () => loadItems()).then(fn => { unlisten = fn; });
    return () => { unlisten?.(); };
  }, [loadItems]);

  const removeItem = useCallback(async (id: string) => {
    const t = tauri();
    if (!t) return;
    await t.core.invoke("remove_floating_item", { id });
  }, []);

  const clearAll = useCallback(async () => {
    const t = tauri();
    if (!t) return;
    await t.core.invoke("clear_floating_items");
  }, []);

  const openItem = useCallback(async (item: FloatingItem) => {
    const t = tauri();
    if (!t) return;
    await t.core.invoke("floating_send_to_chat", { text: item.text });
  }, []);

  const closePanel = useCallback(async () => {
    const t = tauri();
    if (!t) return;
    await t.core.invoke("toggle_floating_panel");
  }, []);

  const formatTime = (ts: number) => {
    const d = new Date(ts * 1000);
    const now = new Date();
    if (d.toDateString() === now.toDateString()) {
      return d.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
    }
    return (
      d.toLocaleDateString("zh-CN", { month: "short", day: "numeric" }) +
      " " +
      d.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })
    );
  };

  return (
    <>
      <style>{`
        @keyframes panel-in {
          from { opacity: 0; transform: translateY(12px) scale(0.96); }
          to { opacity: 1; transform: translateY(0) scale(1); }
        }
        @keyframes item-in {
          from { opacity: 0; transform: translateX(-8px); }
          to { opacity: 1; transform: translateX(0); }
        }
        body { background: transparent !important; margin: 0; overflow: hidden; }
        ::-webkit-scrollbar { width: 4px; }
        ::-webkit-scrollbar-track { background: transparent; }
        ::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.1); border-radius: 2px; }
        ::-webkit-scrollbar-thumb:hover { background: rgba(255,255,255,0.2); }
      `}</style>
      <div style={{ width: "100vw", height: "100vh", background: "transparent", padding: 8 }}>
      <div
        style={{
          animation: "panel-in 0.25s cubic-bezier(0.4, 0, 0.2, 1)",
          width: "100%",
          height: "100%",
          background: "rgba(22,22,26,0.97)",
          borderRadius: 14,
          border: "1px solid rgba(255,255,255,0.07)",
          boxShadow: "0 24px 80px rgba(0,0,0,0.45), 0 0 0 0.5px rgba(255,255,255,0.05)",
          backdropFilter: "blur(24px)",
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        {/* Header — draggable, macOS-style */}
        <div
          data-tauri-drag-region
          style={{
            padding: "14px 16px 12px",
            display: "flex",
            flexDirection: "column",
            gap: 10,
            borderBottom: "1px solid rgba(255,255,255,0.06)",
            cursor: "grab",
            flexShrink: 0,
          }}
        >
          {/* Window controls row */}
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
              <button
                onClick={closePanel}
                style={{
                  width: 12, height: 12, borderRadius: "50%",
                  background: "#ff5f57", border: "none", cursor: "pointer",
                  transition: "opacity 0.15s", padding: 0,
                }}
                onMouseEnter={(e) => (e.currentTarget.style.opacity = "0.8")}
                onMouseLeave={(e) => (e.currentTarget.style.opacity = "1")}
              />
              <div style={{ width: 12, height: 12, borderRadius: "50%", background: "#febc2e" }} />
              <div style={{ width: 12, height: 12, borderRadius: "50%", background: "#28c840" }} />
            </div>
            {items.length > 0 && (
              <button
                onClick={clearAll}
                style={{
                  background: "rgba(255,255,255,0.06)",
                  border: "1px solid rgba(255,255,255,0.08)",
                  color: "rgba(255,255,255,0.5)",
                  fontSize: 11,
                  cursor: "pointer",
                  padding: "3px 10px",
                  borderRadius: 6,
                  transition: "all 0.15s",
                  letterSpacing: 0.3,
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = "rgba(239,68,68,0.12)";
                  e.currentTarget.style.color = "#f87171";
                  e.currentTarget.style.borderColor = "rgba(239,68,68,0.2)";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = "rgba(255,255,255,0.06)";
                  e.currentTarget.style.color = "rgba(255,255,255,0.5)";
                  e.currentTarget.style.borderColor = "rgba(255,255,255,0.08)";
                }}
              >
                清空
              </button>
            )}
          </div>
          {/* Title row */}
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <div style={{
              width: 28, height: 28, borderRadius: 8,
              background: "linear-gradient(135deg, #3b82f6, #8b5cf6)",
              display: "flex", alignItems: "center", justifyContent: "center",
              flexShrink: 0,
            }}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="M12 2L2 7l10 5 10-5-10-5z" />
                <path d="M2 17l10 5 10-5" />
                <path d="M2 12l10 5 10-5" />
              </svg>
            </div>
            <div>
              <div style={{ color: "rgba(255,255,255,0.92)", fontSize: 14, fontWeight: 600, lineHeight: 1.2 }}>
                浮窗收集
              </div>
              <div style={{ color: "rgba(255,255,255,0.35)", fontSize: 11, marginTop: 1 }}>
                {items.length > 0 ? `${items.length} 条内容` : "空"}
              </div>
            </div>
          </div>
        </div>

        {/* Items */}
        <div style={{ flex: 1, overflowY: "auto" }}>
          {items.length === 0 ? (
            <div
              style={{
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                justifyContent: "center",
                height: "100%",
                color: "rgba(255,255,255,0.25)",
                fontSize: 13,
                gap: 12,
                padding: 32,
              }}
            >
              <div style={{
                width: 48, height: 48, borderRadius: 12,
                background: "rgba(255,255,255,0.04)",
                display: "flex", alignItems: "center", justifyContent: "center",
              }}>
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="rgba(255,255,255,0.2)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="8" y="2" width="8" height="4" rx="1" ry="1" />
                  <path d="M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2" />
                  <line x1="12" y1="11" x2="12" y2="17" />
                  <line x1="9" y1="14" x2="15" y2="14" />
                </svg>
              </div>
              <span style={{ fontWeight: 500 }}>暂无收集</span>
              <span style={{ fontSize: 11, color: "rgba(255,255,255,0.15)", textAlign: "center", lineHeight: 1.5 }}>
                选中文字后按 Alt+Y<br />即可收入浮窗
              </span>
            </div>
          ) : (
            <div style={{ padding: "4px 8px" }}>
              {items.map((item, idx) => (
                <div
                  key={item.id}
                  onClick={() => openItem(item)}
                  style={{
                    padding: "10px 12px",
                    cursor: "pointer",
                    borderRadius: 10,
                    margin: "2px 0",
                    transition: "background 0.15s",
                    display: "flex",
                    alignItems: "flex-start",
                    gap: 10,
                    animation: `item-in 0.2s cubic-bezier(0.4,0,0.2,1) ${idx * 0.03}s both`,
                  }}
                  onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(255,255,255,0.05)")}
                  onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                >
                  <div style={{
                    width: 6, height: 6, borderRadius: "50%",
                    background: "rgba(59,130,246,0.6)",
                    marginTop: 6, flexShrink: 0,
                  }} />
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div
                      style={{
                        color: "rgba(255,255,255,0.88)",
                        fontSize: 13,
                        lineHeight: 1.6,
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        display: "-webkit-box",
                        WebkitLineClamp: 3,
                        WebkitBoxOrient: "vertical",
                        letterSpacing: 0.2,
                      }}
                    >
                      {item.text}
                    </div>
                    <div style={{ color: "rgba(255,255,255,0.25)", fontSize: 11, marginTop: 4, letterSpacing: 0.3 }}>
                      {formatTime(item.timestamp)}
                    </div>
                  </div>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      removeItem(item.id);
                    }}
                    style={{
                      background: "none",
                      border: "none",
                      color: "rgba(255,255,255,0.15)",
                      fontSize: 12,
                      cursor: "pointer",
                      padding: "4px",
                      lineHeight: 1,
                      flexShrink: 0,
                      marginTop: 0,
                      borderRadius: 4,
                      transition: "all 0.15s",
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.color = "#f87171";
                      e.currentTarget.style.background = "rgba(239,68,68,0.1)";
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.color = "rgba(255,255,255,0.15)";
                      e.currentTarget.style.background = "none";
                    }}
                  >
                    ✕
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
    </>
  );
}
