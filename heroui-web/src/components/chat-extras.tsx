"use client";

import type { EmotionResult, StickerSuggestion } from "@/lib/api";

const emotionEmoji: Record<string, string> = {
  happy: "\u{1F60A}", joy: "\u{1F604}", excited: "\u{1F929}", love: "\u{1F60D}", grateful: "\u{1F64F}",
  sad: "\u{1F622}", angry: "\u{1F620}", fear: "\u{1F628}", surprise: "\u{1F632}", disgust: "\u{1F922}",
  neutral: "\u{1F610}", curious: "\u{1F9D0}", confused: "\u{1F615}", confident: "\u{1F60E}",
  anxious: "\u{1F630}", calm: "\u{1F60C}", playful: "\u{1F61C}", tired: "\u{1F634}",
  proud: "\u{1F979}", hopeful: "\u{1F308}", nostalgic: "\u{1F972}", determined: "\u{1F4AA}",
};

const emotionColors: Record<string, string> = {
  happy: "#fbbf24", joy: "#fbbf24", excited: "#f59e0b", love: "#ec4899",
  sad: "#60a5fa", angry: "#ef4444", fear: "#8b5cf6", surprise: "#06b6d4",
  neutral: "#6b7280", curious: "#a78bfa", confused: "#f97316", confident: "#22c55e",
  anxious: "#f87171", calm: "#34d399", playful: "#f472b6", tired: "#94a3b8",
  proud: "#c084fc", hopeful: "#facc15", nostalgic: "#a78bfa", determined: "#ef4444",
};

export function EmotionBadge({ emotion }: { emotion: EmotionResult }) {
  const emoji = emotionEmoji[emotion.emotion] || "\u2728";
  const color = emotionColors[emotion.emotion] || "#6b7280";

  return (
    <span
      className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium"
      style={{ background: `${color}18`, color }}
      title={`${emotion.emotion} (${(emotion.confidence * 100).toFixed(0)}%)`}
    >
      {emoji} {emotion.emotion}
    </span>
  );
}

export function StickerView({ sticker }: { sticker: StickerSuggestion }) {
  if (sticker.cdnurl) {
    return (
      <div className="mt-1.5">
        <img
          src={sticker.cdnurl}
          alt={`${sticker.emotion} sticker`}
          className="w-24 h-24 object-contain rounded-lg"
          loading="lazy"
        />
      </div>
    );
  }

  if (sticker.emoji) {
    return <span className="text-3xl block mt-1">{sticker.emoji}</span>;
  }

  return (
    <span
      className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] mt-1"
      style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)" }}
    >
      Sticker {sticker.platform}:{sticker.package_id}/{sticker.sticker_id}
    </span>
  );
}

export function SkillTags({ skills }: { skills: string[] }) {
  if (!skills || skills.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-1 mt-1.5">
      {skills.map((s) => (
        <span
          key={s}
          className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px]"
          style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)" }}
        >
          ? {s}
        </span>
      ))}
    </div>
  );
}

export interface AgentAction {
  label: string;
  action: string;
  payload?: Record<string, unknown>;
}

export function AgentActions({ actions, onAction }: { actions: AgentAction[]; onAction: (action: AgentAction) => void }) {
  if (!actions || actions.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-1.5 mt-2">
      {actions.map((a, i) => (
        <button
          key={i}
          onClick={() => onAction(a)}
          className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-[11px] font-medium transition-all duration-200"
          style={{
            background: "rgba(0,111,238,0.08)",
            color: "var(--yunque-accent)",
            border: "1px solid rgba(0,111,238,0.2)",
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.background = "rgba(0,111,238,0.15)";
            e.currentTarget.style.borderColor = "rgba(0,111,238,0.4)";
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.background = "rgba(0,111,238,0.08)";
            e.currentTarget.style.borderColor = "rgba(0,111,238,0.2)";
          }}
        >
          {a.label}
        </button>
      ))}
    </div>
  );
}
