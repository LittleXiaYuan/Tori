/** Shared task status color map — used across missions, task-run, workflows */
export const STATUS_COLORS: Record<string, string> = {
  running: "var(--yunque-accent)",
  active: "var(--yunque-success)",
  completed: "var(--yunque-success)",
  done: "var(--yunque-success)",
  pending: "#9ca3af",
  failed: "var(--yunque-danger)",
  idle: "#9ca3af",
  cancelled: "#6b7280",
  paused: "var(--yunque-warning)",
  planning: "#a78bfa",
};

export const STATUS_LABELS: Record<string, string> = {
  running: "运行中",
  active: "活跃",
  completed: "已完成",
  done: "已完成",
  pending: "等待中",
  failed: "失败",
  idle: "空闲",
  cancelled: "已取消",
  paused: "已暂停",
  planning: "规划中",
  queued: "排队中",
  scheduled: "已调度",
  success: "成功",
  error: "错误",
  enabled: "已启用",
  disabled: "已禁用",
  healthy: "正常",
  unknown: "未知",
};

/** Relative time formatter */
export function relTime(ts?: string): string {
  if (!ts) return "—";
  const d = Date.now() - new Date(ts).getTime();
  if (d < 60000) return `${Math.floor(d / 1000)}s 前`;
  if (d < 3600000) return `${Math.floor(d / 60000)}m 前`;
  if (d < 86400000) return `${Math.floor(d / 3600000)}h 前`;
  return `${Math.floor(d / 86400000)}d 前`;
}

/** Short date-time formatter (zh-CN) */
export function fmtTime(ts?: string): string {
  if (!ts) return "-";
  return new Date(ts).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

/** Duration between two timestamps */
export function dur(start?: string, end?: string): string {
  if (!start) return "-";
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const d = Math.max(0, e - s);
  if (d < 1000) return `${d}ms`;
  if (d < 60000) return `${(d / 1000).toFixed(1)}s`;
  return `${(d / 60000).toFixed(1)}min`;
}

/** Friendly date for lists (today => time, yesterday => "昨天", else => short date) */
export function formatDate(dateStr: string): string {
  if (!dateStr) return "";
  const d = new Date(dateStr);
  const now = new Date();
  const diff = now.getTime() - d.getTime();
  if (diff < 86400000) return d.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  if (diff < 172800000) return "昨天";
  return d.toLocaleDateString("zh-CN", { month: "short", day: "numeric" });
}
