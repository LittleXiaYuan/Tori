import { formatErrorMessage } from "./error-utils";

export type AgenticChatStreamItem =
  | { kind: "delta"; content: string }
  | { kind: "agent_event"; event: Record<string, unknown>; raw: string }
  | { kind: "done"; data: Record<string, unknown>; raw: string }
  | { kind: "actions"; actions: unknown[]; raw: string }
  | { kind: "thinking"; content: string; data: Record<string, unknown> | null; raw: string }
  | { kind: "error"; message: string; data: unknown; raw: string }
  | { kind: "raw"; data: string; event: string };

export class ChatStreamTimeoutError extends Error {
  constructor(timeoutMs: number) {
    super(`响应暂时超时，已保留现场；${Math.round(timeoutMs / 1000)} 秒内没有收到新内容，可以稍后重试或继续。`);
    this.name = "ChatStreamTimeoutError";
  }
}

type SSEFrame = { event: string; data: string };

type ReadResult = ReadableStreamReadResult<Uint8Array>;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function parseJSON(raw: string): unknown {
  return JSON.parse(raw);
}

function parseJSONRecord(raw: string): Record<string, unknown> | null {
  try {
    const parsed = parseJSON(raw);
    return isRecord(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function frameDataLine(line: string): string {
  let data = line.slice(5);
  if (data.startsWith(" ")) data = data.slice(1);
  return data;
}

async function readWithIdleTimeout(
  reader: ReadableStreamDefaultReader<Uint8Array>,
  timeoutMs?: number,
): Promise<ReadResult> {
  if (!timeoutMs || timeoutMs <= 0) return reader.read();
  let timerId: ReturnType<typeof setTimeout> | null = null;
  const timeout = new Promise<never>((_, reject) => {
    timerId = setTimeout(() => reject(new ChatStreamTimeoutError(timeoutMs)), timeoutMs);
  });
  try {
    return await Promise.race([reader.read(), timeout]);
  } finally {
    if (timerId) clearTimeout(timerId);
  }
}

async function* readSSEFrames(
  body: ReadableStream<Uint8Array>,
  options?: { idleTimeoutMs?: number },
): AsyncGenerator<SSEFrame> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let event = "";
  let dataLines: string[] = [];
  let finished = false;

  const reset = () => {
    event = "";
    dataLines = [];
  };

  try {
    while (true) {
      const { done, value } = await readWithIdleTimeout(reader, options?.idleTimeoutMs);
      if (done) {
        finished = true;
        buffer += decoder.decode();
        break;
      }
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";
      for (const rawLine of lines) {
        const line = rawLine.endsWith("\r") ? rawLine.slice(0, -1) : rawLine;
        if (line === "") {
          if (event || dataLines.length > 0) yield { event, data: dataLines.join("\n") };
          reset();
        } else if (line.startsWith("event:")) {
          event = line.slice(6).trim();
        } else if (line.startsWith("data:")) {
          dataLines.push(frameDataLine(line));
        }
      }
    }

    if (buffer) {
      const line = buffer.endsWith("\r") ? buffer.slice(0, -1) : buffer;
      if (line.startsWith("event:")) event = line.slice(6).trim();
      if (line.startsWith("data:")) dataLines.push(frameDataLine(line));
    }
    if (event || dataLines.length > 0) yield { event, data: dataLines.join("\n") };
  } finally {
    if (!finished) {
      try { await reader.cancel(); } catch { }
    }
    reader.releaseLock();
  }
}

function parseAgenticFrame(frame: SSEFrame): AgenticChatStreamItem[] {
  const raw = frame.data;
  const event = frame.event;

  if (event === "error") {
    let parsed: unknown = null;
    try { parsed = parseJSON(raw); } catch { }
    const message = isRecord(parsed) && typeof parsed.message === "string" ? parsed.message : raw;
    return [{ kind: "error", message: formatErrorMessage(message, "任务暂时没有顺利完成，已保留现场。"), data: parsed, raw }];
  }

  if (event === "done") {
    const data = parseJSONRecord(raw);
    return data ? [{ kind: "done", data, raw }] : [{ kind: "raw", data: raw, event }];
  }

  if (event === "actions") {
    let actions: unknown[] = [];
    try {
      const parsed = parseJSON(raw);
      if (Array.isArray(parsed)) actions = parsed;
      else if (isRecord(parsed) && Array.isArray(parsed.actions)) actions = parsed.actions;
    } catch { }
    return [{ kind: "actions", actions, raw }];
  }

  if (event === "thinking") {
    const data = parseJSONRecord(raw);
    const content = data && typeof data.content === "string" ? data.content : raw;
    return [{ kind: "thinking", content, data, raw }];
  }

  try {
    const parsed = parseJSON(raw);
    if (isRecord(parsed)) {
      if (typeof parsed.content === "string" || parsed.type === "delta") {
        return [{ kind: "delta", content: typeof parsed.content === "string" ? parsed.content : "" }];
      }
      if (typeof parsed.id === "string" && typeof parsed.domain === "string") {
        return [{ kind: "agent_event", event: parsed, raw }];
      }
    }
  } catch { }

  return raw.trim() ? [{ kind: "raw", data: raw, event }] : [];
}

export async function* parseAgenticChatStream(
  body: ReadableStream<Uint8Array>,
  options?: { idleTimeoutMs?: number },
): AsyncGenerator<AgenticChatStreamItem> {
  for await (const frame of readSSEFrames(body, options)) {
    if (frame.data === "[DONE]") return;
    for (const item of parseAgenticFrame(frame)) yield item;
  }
}

export function legacyChatStreamChunks(item: AgenticChatStreamItem): string[] {
  if (item.kind === "delta") return [item.content];
  if (item.kind === "thinking") return item.content ? [item.content] : [];
  if (item.kind === "agent_event") return [item.raw];
  if (item.kind === "raw") return item.data.trim() ? [item.data] : [];
  if (item.kind === "actions") return [`\n<!--actions:${item.raw}-->`];
  if (item.kind !== "done") return [];

  const chunks: string[] = [];
  if (item.data.emotion) chunks.push(`\n<!--emotion:${JSON.stringify(item.data.emotion)}-->`);
  if (item.data.sticker_suggestion) chunks.push(`\n<!--sticker:${JSON.stringify(item.data.sticker_suggestion)}-->`);
  if (item.data.sticker_suggestions) chunks.push(`\n<!--stickers:${JSON.stringify(item.data.sticker_suggestions)}-->`);
  if (Array.isArray(item.data.actions) && item.data.actions.length > 0) {
    chunks.push(`\n<!--actions:${JSON.stringify(item.data.actions)}-->`);
  }
  return chunks;
}
