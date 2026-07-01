/**
 * Builds the hero greeting shown on the empty chat screen.
 *
 * Four branches, two axes:
 *   - axis 1: nickname known / nickname unknown
 *   - axis 2: most-recent conversation exists / does not
 *
 * Why this matters: "你" is a fine *object* pronoun ("帮你做点什么？") but a
 * jarring *vocative* in Chinese — "晚上好，你，..." reads like a missing
 * variable, not a greeting. So we never emit "你" as a vocative.
 *
 * Strategy:
 *   - **Known nickname** → second-person warmth: "晚上好，夏鸢，..."
 *   - **Unknown nickname** → switch to first-person voice for Yunque itself:
 *     "我是云雀 —— 今晚要不要收个尾？". This both avoids the "你" placeholder
 *     and gives the agent a stable subject, which fits the Cogni philosophy
 *     (the agent has its own voice, not just a fill-in-the-blank UI string).
 *
 * The plain string output keeps the consumer (`ChatEmptyState`) simple; later
 * we can lift parts to clickable spans (e.g. the recent-title link) by
 * returning a structured array instead.
 */

type TimeBucket = "late" | "morning" | "noon" | "afternoon" | "evening";

// Nickname read/write is owned by user-profile.ts (single source of truth for
// both the greeting and the title-bar chip). Re-exported here so existing
// chat-empty-state imports keep working.
export { getNickname, setNickname } from "@/lib/user-profile";

function bucket(now: Date): TimeBucket {
  const h = now.getHours();
  if (h < 5) return "late";
  if (h < 11) return "morning";
  if (h < 13) return "noon";
  if (h < 18) return "afternoon";
  return "evening";
}

function timeWord(b: TimeBucket): string {
  switch (b) {
    case "late": return "夜深了";
    case "morning": return "早上好";
    case "noon": return "中午好";
    case "afternoon": return "下午好";
    case "evening": return "晚上好";
  }
}

function tailNamed(b: TimeBucket): string {
  switch (b) {
    case "late": return "还在工作？要不先做完这一件再睡。";
    case "morning": return "今天先解决哪件事？";
    case "noon": return "趁中午把这件事推一推？";
    case "afternoon": return "继续来一段？";
    case "evening": return "今晚要不要收个尾？";
  }
}

function firstPersonOpener(b: TimeBucket): string {
  switch (b) {
    case "late": return "我是云雀 —— 夜深了，要做点什么？";
    case "morning": return "我是云雀 —— 今天先解决哪件事？";
    case "noon": return "我是云雀 —— 中午想推什么进展？";
    case "afternoon": return "我是云雀 —— 下午继续来一段？";
    case "evening": return "我是云雀 —— 今晚要不要收个尾？";
  }
}

export interface HeroContext {
  nickname: string | null;
  recentTitle: string | null;
}

/** Trim a conversation title so it fits in the greeting line. */
export function truncateTitle(s: string, max = 18): string {
  const arr = [...s];
  if (arr.length <= max) return s;
  return arr.slice(0, max).join("") + "…";
}

export function buildHero(ctx: HeroContext, now: Date = new Date()): string {
  const b = bucket(now);
  const greet = timeWord(b);
  const title = ctx.recentTitle ? truncateTitle(ctx.recentTitle) : null;

  if (ctx.nickname) {
    if (title) {
      return `${greet}，${ctx.nickname}，上次的「${title}」要继续吗？`;
    }
    return `${greet}，${ctx.nickname}，${tailNamed(b)}`;
  }

  // No nickname — switch to first-person to avoid the "你" vocative.
  if (title) {
    return `云雀在线 · 上次的「${title}」还没收尾，我接着做？`;
  }
  return firstPersonOpener(b);
}
