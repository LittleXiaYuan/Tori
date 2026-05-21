function primitiveMessage(value: unknown): string {
  if (typeof value === "string") return value.trim();
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

function friendlyKnownError(message: string): string {
  if (message.includes("等待依赖步骤完成")) {
    return "等待前置步骤完成，可进入任务详情确认依赖后继续。";
  }
  if (message.includes("任务已执行但连接中断")) {
    return "连接暂时中断，现场已保留；如果任务已经推进，可以从最近可恢复任务继续。";
  }
  if (/当前模型响应失败|备用模型|调用栈降级|级联唤醒/i.test(message)) {
    return "模型暂时没有回应，已保留现场，正在换用可用模型继续。";
  }
  if (/all fallback llm clients failed|moonshot.*EOF|Post .*\/chat\/completions.*EOF/i.test(message)) {
    return "所有可用模型通道暂时失败，已保留现场；可以稍后重试，或先切换模型/供应商继续。";
  }
  if (/context canceled|context cancelled|连接中断|连接已断开/i.test(message)) {
    return "连接暂时中断，已保留现场，可稍后继续或先查看阶段结果。";
  }
  if (/context deadline exceeded|响应超时|timeout/i.test(message)) {
    return "响应暂时超时，已保留现场，可稍后重试或继续。";
  }
  if (/unknown skill|未知工具|未找到工具/i.test(message)) {
    return "所需工具暂时不可用，已保留现场，可换用可用工具或调整步骤继续。";
  }
  if (/blocked by trust gate|trust gate|信任/i.test(message)) {
    return "这一步需要更高信任或确认，已保留现场，可确认后继续。";
  }
  if (/tool panic|panic/i.test(message)) {
    return "工具运行时遇到异常，已保留现场，可重试或切换策略继续。";
  }
  if (/execution failed|handoff agent|EOF/i.test(message)) {
    return "任务暂时没有完成，已保留现场，可切换策略或稍后继续。";
  }
  return "";
}

export function formatErrorMessage(value: unknown, fallback = "Unknown error"): string {
  const primitive = primitiveMessage(value);
  if (primitive) return friendlyKnownError(primitive) || primitive;
  if (value == null) return fallback;
  if (value instanceof Error) return friendlyKnownError(value.message) || value.message || fallback;
  if (typeof value !== "object") return fallback;

  const record = value as Record<string, unknown>;
  const nestedError = record.error && typeof record.error === "object" ? formatErrorMessage(record.error, "") : "";
  const body =
    primitiveMessage(record.message) ||
    primitiveMessage(record.detail) ||
    primitiveMessage(record.error) ||
    primitiveMessage(record.reason) ||
    nestedError;
  const code = primitiveMessage(record.code);

  const friendly = friendlyKnownError(body);
  if (friendly) return friendly;
  if (body && code && !body.includes(code)) return `${code}: ${body}`;
  if (body) return body;
  if (code) return code;

  try {
    const serialized = JSON.stringify(value);
    return serialized && serialized !== "{}" ? serialized : fallback;
  } catch {
    return fallback;
  }
}
