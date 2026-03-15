"use client";

import { useEffect, useState } from "react";
import { api, type InboxItem } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { AnimatedList } from "@/components/ui/animated-list";
import { NumberTicker } from "@/components/ui/number-ticker";
import { Inbox, CheckCheck, Mail, MailOpen, Send } from "lucide-react";

export default function InboxPage() {
  const [items, setItems] = useState<InboxItem[]>([]);
  const [unread, setUnread] = useState(0);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [showCompose, setShowCompose] = useState(false);
  const [compose, setCompose] = useState({ source: "", content: "", action: "" });

  const load = async () => {
    try {
      const res = await api.getInbox();
      setItems(res.items || []);
      setUnread(res.count?.unread || 0);
      setTotal(res.count?.total || 0);
    } catch {
      /* offline */
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const markAllRead = async () => {
    await api.markAllInboxRead();
    load();
  };

  const pushMessage = async () => {
    if (!compose.content) return;
    await api.pushInbox(compose.source || "manual", compose.content, compose.action || "none");
    setCompose({ source: "", content: "", action: "" });
    setShowCompose(false);
    load();
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin" style={{ borderColor: "var(--text-muted)", borderTopColor: "transparent" }} />
      </div>
    );
  }

  return (
    <div className="max-w-4xl">
      <BlurFade delay={0}>
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-3">
            <Inbox size={20} />
            <h1 className="text-xl font-semibold tracking-tight">Inbox</h1>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setShowCompose(!showCompose)}
              className="flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium transition-colors cursor-pointer"
              style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}
            >
              <Send size={12} />
              Compose
            </button>
            {unread > 0 && (
              <button
                onClick={markAllRead}
                className="flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium transition-colors cursor-pointer"
                style={{ background: "var(--text)", color: "var(--bg)" }}
              >
                <CheckCheck size={12} />
                Mark all read
              </button>
            )}
          </div>
        </div>
      </BlurFade>

      <BlurFade delay={0.05}>
        <div className="grid grid-cols-2 gap-4 mb-6">
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--text-muted)" }}>Unread</div>
            <div className="text-3xl font-bold"><NumberTicker value={unread} /></div>
          </div>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-xs font-medium uppercase tracking-wider mb-2" style={{ color: "var(--text-muted)" }}>Total</div>
            <div className="text-3xl font-bold"><NumberTicker value={total} /></div>
          </div>
        </div>
      </BlurFade>

      {showCompose && (
        <BlurFade delay={0}>
          <div className="rounded-xl border p-5 mb-6 space-y-3" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="grid grid-cols-2 gap-3">
              <input
                value={compose.source}
                onChange={(e) => setCompose({ ...compose, source: e.target.value })}
                placeholder="Source (e.g. telegram, email)"
                className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none"
                style={{ borderColor: "var(--border)" }}
              />
              <input
                value={compose.action}
                onChange={(e) => setCompose({ ...compose, action: e.target.value })}
                placeholder="Action (optional)"
                className="bg-transparent border rounded-lg px-3 py-2 text-sm focus:outline-none"
                style={{ borderColor: "var(--border)" }}
              />
            </div>
            <textarea
              value={compose.content}
              onChange={(e) => setCompose({ ...compose, content: e.target.value })}
              placeholder="Message content..."
              className="w-full bg-transparent border rounded-lg px-3 py-2 text-sm resize-none focus:outline-none"
              style={{ borderColor: "var(--border)", minHeight: 80 }}
            />
            <div className="flex justify-end gap-2">
              <button onClick={() => setShowCompose(false)} className="px-3 py-1.5 text-xs rounded-lg cursor-pointer" style={{ color: "var(--text-muted)" }}>Cancel</button>
              <button onClick={pushMessage} className="px-3 py-1.5 text-xs rounded-lg font-medium cursor-pointer" style={{ background: "var(--text)", color: "var(--bg)" }}>Send</button>
            </div>
          </div>
        </BlurFade>
      )}

      <BlurFade delay={0.1}>
        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          {items.length === 0 ? (
            <div className="text-sm text-center py-12" style={{ color: "var(--text-muted)" }}>
              Inbox is empty
            </div>
          ) : (
            <AnimatedList>
              {items.map((item) => (
                <div
                  key={item.id}
                  className="flex items-start gap-3 p-3 rounded-lg transition-colors"
                  style={{ background: item.is_read ? "transparent" : "var(--bg-hover)" }}
                >
                  <div className="mt-0.5 shrink-0" style={{ color: "var(--text-muted)" }}>
                    {item.is_read ? <MailOpen size={14} /> : <Mail size={14} />}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-xs font-medium px-2 py-0.5 rounded" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
                        {item.source}
                      </span>
                      {!item.is_read && (
                        <span className="w-1.5 h-1.5 rounded-full" style={{ background: "var(--text)" }} />
                      )}
                    </div>
                    <div className="text-sm">{item.content}</div>
                    <div className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>
                      {new Date(item.created_at).toLocaleString()}
                    </div>
                  </div>
                </div>
              ))}
            </AnimatedList>
          )}
        </div>
      </BlurFade>
    </div>
  );
}
