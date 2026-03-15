"use client";

import { useEffect, useState } from "react";
import { useI18n } from "@/lib/i18n";
import { api, type EmotionHistoryEntry, type HeartbeatLog } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { Brain, Heart, TrendingUp, Clock, Smile, Frown, Meh } from "lucide-react";

export default function MemoryPage() {
  const { t } = useI18n();
  const [emotions, setEmotions] = useState<EmotionHistoryEntry[]>([]);
  const [heartbeats, setHeartbeats] = useState<HeartbeatLog[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      api.getEmotionHistory({ limit: 50 }).then((res) => setEmotions(res.entries || [])),
      api.getHeartbeatLogs(50).then((logs) => setHeartbeats(logs || [])),
    ]).finally(() => setLoading(false));
  }, []);

  const emotionIcon = (emotion: string) => {
    switch (emotion) {
      case "happy":
      case "surprised":
        return <Smile size={16} style={{ color: "#22c55e" }} />;
      case "sad":
      case "angry":
      case "fearful":
      case "disgusted":
        return <Frown size={16} style={{ color: "#ef4444" }} />;
      default:
        return <Meh size={16} style={{ color: "var(--text-muted)" }} />;
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin"
          style={{ borderColor: "var(--text-muted)", borderTopColor: "transparent" }} />
      </div>
    );
  }

  return (
    <div>
      <BlurFade delay={0}>
        <div className="flex items-center gap-3 mb-6">
          <Brain size={20} />
          <div>
            <h1 className="text-xl font-semibold tracking-tight">{t("memory.title")}</h1>
            <p className="text-xs" style={{ color: "var(--text-muted)" }}>{t("memory.subtitle")}</p>
          </div>
        </div>
      </BlurFade>

      {/* Stats Cards */}
      <BlurFade delay={0.05}>
        <div className="grid grid-cols-3 gap-4 mb-6">
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center gap-2 mb-3">
              <Smile size={16} style={{ color: "var(--accent)" }} />
              <span className="text-sm" style={{ color: "var(--text-muted)" }}>{t("memory.emotionTracking")}</span>
            </div>
            <div className="text-2xl font-bold">{emotions.length}</div>
            <div className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>{t("memory.emotionRecords")}</div>
          </div>

          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center gap-2 mb-3">
              <Heart size={16} style={{ color: "#ef4444" }} />
              <span className="text-sm" style={{ color: "var(--text-muted)" }}>{t("memory.heartbeat")}</span>
            </div>
            <div className="text-2xl font-bold">{heartbeats.length}</div>
            <div className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>{t("memory.heartbeatLogs")}</div>
          </div>

          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center gap-2 mb-3">
              <TrendingUp size={16} style={{ color: "#22c55e" }} />
              <span className="text-sm" style={{ color: "var(--text-muted)" }}>{t("memory.memoryLayers")}</span>
            </div>
            <div className="text-2xl font-bold">5</div>
            <div className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>{t("memory.layersActive")}</div>
          </div>
        </div>
      </BlurFade>

      <div className="grid grid-cols-2 gap-4">
        {/* Emotion History */}
        <BlurFade delay={0.1}>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <h2 className="text-sm font-medium mb-4 flex items-center gap-2" style={{ color: "var(--text-muted)" }}>
              <Smile size={14} /> {t("memory.emotionHistory")}
            </h2>
            {emotions.length === 0 ? (
              <div className="text-sm py-8 text-center" style={{ color: "var(--text-muted)" }}>
                {t("memory.noEmotions")}
              </div>
            ) : (
              <div className="space-y-1">
                {emotions.slice(0, 10).map((entry, idx) => (
                  <div key={idx} className="flex items-center justify-between p-2.5 rounded-lg transition-colors"
                    style={{ cursor: "default" }}
                    onMouseEnter={(e) => e.currentTarget.style.background = "var(--bg-hover)"}
                    onMouseLeave={(e) => e.currentTarget.style.background = "transparent"}>
                    <div className="flex items-center gap-3">
                      {emotionIcon(entry.emotion)}
                      <div>
                        <span className="text-sm font-medium capitalize">{entry.emotion}</span>
                        <span className="text-xs ml-2" style={{ color: "var(--text-muted)" }}>
                          {(entry.confidence * 100).toFixed(0)}%
                        </span>
                      </div>
                    </div>
                    <div className="flex items-center gap-1.5 text-xs" style={{ color: "var(--text-muted)" }}>
                      <Clock size={12} />
                      {new Date(entry.timestamp).toLocaleTimeString()}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </BlurFade>

        {/* Heartbeat Logs */}
        <BlurFade delay={0.15}>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <h2 className="text-sm font-medium mb-4 flex items-center gap-2" style={{ color: "var(--text-muted)" }}>
              <Heart size={14} /> {t("memory.heartbeatLogs")}
            </h2>
            {heartbeats.length === 0 ? (
              <div className="text-sm py-8 text-center" style={{ color: "var(--text-muted)" }}>
                {t("memory.noHeartbeats")}
              </div>
            ) : (
              <div className="space-y-1">
                {heartbeats.slice(0, 10).map((log) => (
                  <div key={log.id} className="p-2.5 rounded-lg transition-colors"
                    onMouseEnter={(e) => e.currentTarget.style.background = "var(--bg-hover)"}
                    onMouseLeave={(e) => e.currentTarget.style.background = "transparent"}>
                    <div className="flex items-center justify-between mb-1">
                      <span className="px-2 py-0.5 rounded text-[10px] font-medium"
                        style={{
                          background: log.status === "ok" ? "rgba(34,197,94,0.15)" : "rgba(239,68,68,0.15)",
                          color: log.status === "ok" ? "#22c55e" : "#ef4444",
                        }}>
                        {log.status}
                      </span>
                      <div className="flex items-center gap-1.5 text-xs" style={{ color: "var(--text-muted)" }}>
                        <Clock size={12} />
                        {new Date(log.started_at).toLocaleTimeString()}
                      </div>
                    </div>
                    {log.result && (
                      <p className="text-xs mt-1 truncate" style={{ color: "var(--text-muted)" }}>{log.result}</p>
                    )}
                    {log.error && (
                      <p className="text-xs mt-1 truncate" style={{ color: "#ef4444" }}>{log.error}</p>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        </BlurFade>
      </div>
    </div>
  );
}

