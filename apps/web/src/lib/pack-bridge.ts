// Framework-agnostic postMessage bridge core for the Pack DLC host.
// See docs/spec/pack-frontend-dlc.md §7. Kept free of React/DOM so the request
// dispatch + capability gating can be unit-tested in isolation.

export const BRIDGE_VERSION = 1;

export interface BridgeEnvelope {
  v?: number;
  id?: string;
  kind?: "req" | "res" | "event";
  method?: string;
  payload?: unknown;
  error?: BridgeError;
}

export interface BridgeError {
  code: string;
  message: string;
}

export interface BridgeHandlerCtx {
  packId: string;
}

export type BridgeMethodHandler = (
  payload: unknown,
  ctx: BridgeHandlerCtx,
) => Promise<unknown> | unknown;

export interface BridgeContext {
  packId: string;
  /** Posts a message into the sandboxed iframe. */
  post: (msg: BridgeEnvelope) => void;
  /** Capability-gated method handlers, keyed by method name. */
  handlers: Record<string, BridgeMethodHandler>;
}

// ── M3 capability handlers (pure factories, testable in isolation) ──

export interface AllowedRoute {
  method: string;
  path: string;
}

function normMethod(m: string | undefined): string {
  const v = (m || "GET").toUpperCase().trim();
  return v === "HEAD" ? "GET" : v;
}

/** isRouteAllowed reports whether (method, path) is declared by the pack. Exact
 *  path + method match — packs must enumerate every route they call. */
export function isRouteAllowed(routes: AllowedRoute[], method: string, path: string): boolean {
  const m = normMethod(method);
  const p = (path || "").split("?")[0];
  return (routes || []).some((r) => normMethod(r.method) === m && r.path === p);
}

export interface BackendCallOptions {
  /** The pack's declared backend.routeSpecs — the call whitelist. */
  routes: AllowedRoute[];
  /** Returns the host auth headers; injected here so the token never crosses the bridge. */
  authHeaders: () => Record<string, string>;
  /** Defaults to global fetch; injectable for tests. */
  fetchImpl?: typeof fetch;
  /** Backend base URL (api-core BASE); resolves desktop dynamic port. */
  baseUrl?: string;
}

/**
 * makeBackendCallHandler builds the `backend.call` handler. The pack names a
 * (method, path[, body]); the host validates it against the pack's routeSpecs,
 * injects auth, forwards the request, and returns the status + raw body. The
 * token is never exposed to the sandboxed pack.
 */
export function makeBackendCallHandler(opts: BackendCallOptions): BridgeMethodHandler {
  return async (payload) => {
    const p = (payload || {}) as { method?: string; path?: string; body?: unknown };
    const method = normMethod(p.method);
    const path = String(p.path || "");
    if (!path.startsWith("/")) {
      throw new Error("backend.call: path must be an absolute /v1 or /api route");
    }
    if (!isRouteAllowed(opts.routes, method, path)) {
      throw new Error(`backend.call: route not permitted for this pack: ${method} ${path}`);
    }
    const f = opts.fetchImpl || fetch;
    const init: RequestInit = {
      method,
      headers: { "Content-Type": "application/json", ...opts.authHeaders() },
    };
    if (p.body !== undefined && method !== "GET") {
      init.body = typeof p.body === "string" ? p.body : JSON.stringify(p.body);
    }
    const res = await f(`${opts.baseUrl || ""}${path}`, init);
    const text = await res.text();
    return { status: res.status, ok: res.ok, body: text };
  };
}

/** makeNavHandler builds `nav.push`, restricted to the pack's own frontend routes. */
export function makeNavHandler(allowedPaths: string[], navigate: (path: string) => void): BridgeMethodHandler {
  const allow = new Set((allowedPaths || []).map((p) => p.replace(/\/+$/, "") || "/"));
  return (payload) => {
    const p = (payload || {}) as { path?: string };
    const target = String(p.path || "").replace(/\/+$/, "") || "/";
    if (!allow.has(target)) {
      throw new Error(`nav.push: path not declared by this pack: ${target}`);
    }
    navigate(target);
    return { ok: true };
  };
}

// ── SSE-over-bridge (events.subscribe / events.unsubscribe) ──

export interface PackSSEEvent {
  event?: string;
  data: string;
}

/** Permission prefix a pack uses to declare a host SSE stream it may subscribe
 *  to, e.g. "events:subscribe:/v1/events/stream" in backend.permissions. Kept
 *  separate from routeSpecs because wasm routeSpecs are MOUNTED as pack routes
 *  (declaring a host endpoint there would shadow it). */
export const EventSubscribePermPrefix = "events:subscribe:";

/** eventPathsFromPermissions extracts the SSE paths a pack declared via
 *  "events:subscribe:<path>" permissions. */
export function eventPathsFromPermissions(permissions: string[] | undefined): string[] {
  return (permissions || [])
    .filter((p) => p.startsWith(EventSubscribePermPrefix))
    .map((p) => p.slice(EventSubscribePermPrefix.length).trim())
    .filter((p) => p.startsWith("/"));
}

export interface PackEventSubscriptionsOptions {
  /** Declared subscribable SSE paths (from events:subscribe:* permissions). */
  paths: string[];
  /** Host auth headers; injected here so the token never crosses the bridge. */
  authHeaders: () => Record<string, string>;
  /** Forwards one SSE event into the iframe (kind:"event" envelope, host-side). */
  emit: (subID: string, event: PackSSEEvent) => void;
  /** Notified when a stream ends/errors so the guest can resubscribe. */
  onClose?: (subID: string, reason: string) => void;
  fetchImpl?: typeof fetch;
  baseUrl?: string;
  /** Per-pack concurrent stream cap (default 4). */
  maxSubscriptions?: number;
}

/** parseSSEChunks incrementally parses an SSE byte stream. Feed it decoded text
 *  chunks; it returns completed events and keeps the remainder buffered. */
export function createSSEParser(): { push: (chunk: string) => PackSSEEvent[] } {
  let buf = "";
  return {
    push(chunk: string): PackSSEEvent[] {
      buf += chunk;
      const events: PackSSEEvent[] = [];
      let idx: number;
      // Frames are separated by a blank line (\n\n; tolerate \r\n line ends).
      while ((idx = buf.indexOf("\n\n")) >= 0) {
        const frame = buf.slice(0, idx);
        buf = buf.slice(idx + 2);
        let eventName: string | undefined;
        const dataLines: string[] = [];
        for (const rawLine of frame.split("\n")) {
          const line = rawLine.endsWith("\r") ? rawLine.slice(0, -1) : rawLine;
          if (!line || line.startsWith(":")) continue;
          if (line.startsWith("event:")) eventName = line.slice(6).trim();
          else if (line.startsWith("data:")) dataLines.push(line.slice(5).replace(/^ /, ""));
        }
        if (dataLines.length > 0) events.push({ event: eventName, data: dataLines.join("\n") });
      }
      return events;
    },
  };
}

/**
 * PackEventSubscriptions manages SSE streams a sandboxed pack subscribes to via
 * the bridge. The bundle itself cannot reach the network (CSP connect-src
 * 'none'), so the host holds the EventSource-style connections and forwards
 * each event into the iframe. Subscribe targets are gated to the pack's
 * declared GET routes — same default-deny model as backend.call.
 */
export class PackEventSubscriptions {
  private opts: PackEventSubscriptionsOptions;
  private subs = new Map<string, AbortController>();
  private seq = 0;

  constructor(opts: PackEventSubscriptionsOptions) {
    this.opts = opts;
  }

  /** handlers returns the bridge method handlers to merge into the host set. */
  handlers(): Record<string, BridgeMethodHandler> {
    return {
      "events.subscribe": (payload) => {
        const p = (payload || {}) as { path?: string };
        return this.subscribe(String(p.path || ""));
      },
      "events.unsubscribe": (payload) => {
        const p = (payload || {}) as { sub_id?: string };
        return { ok: this.unsubscribe(String(p.sub_id || "")) };
      },
    };
  }

  subscribe(path: string): { sub_id: string } {
    if (!path.startsWith("/")) {
      throw new Error("events.subscribe: path must be an absolute route");
    }
    const bare = path.split("?")[0];
    if (!this.opts.paths.includes(bare)) {
      throw new Error(`events.subscribe: stream not permitted for this pack: ${bare}`);
    }
    const max = this.opts.maxSubscriptions ?? 4;
    if (this.subs.size >= max) {
      throw new Error(`events.subscribe: too many concurrent subscriptions (max ${max})`);
    }
    const subID = `sub-${++this.seq}`;
    const controller = new AbortController();
    this.subs.set(subID, controller);
    void this.pump(subID, path, controller);
    return { sub_id: subID };
  }

  unsubscribe(subID: string): boolean {
    const controller = this.subs.get(subID);
    if (!controller) return false;
    this.subs.delete(subID);
    controller.abort();
    return true;
  }

  /** closeAll aborts every stream; called by the host on iframe unmount. */
  closeAll(): void {
    for (const [, controller] of this.subs) controller.abort();
    this.subs.clear();
  }

  private async pump(subID: string, path: string, controller: AbortController): Promise<void> {
    const f = this.opts.fetchImpl || fetch;
    let reason = "closed";
    try {
      const res = await f(`${this.opts.baseUrl || ""}${path}`, {
        headers: { Accept: "text/event-stream", ...this.opts.authHeaders() },
        signal: controller.signal,
      });
      if (!res.ok || !res.body) {
        reason = `http ${res.status}`;
        return;
      }
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      const parser = createSSEParser();
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        for (const evt of parser.push(decoder.decode(value, { stream: true }))) {
          // Drop events for streams the guest already unsubscribed.
          if (!this.subs.has(subID)) return;
          this.opts.emit(subID, evt);
        }
      }
    } catch (e) {
      reason = controller.signal.aborted ? "unsubscribed" : ((e as Error)?.message || "stream error");
    } finally {
      if (this.subs.delete(subID)) {
        this.opts.onClose?.(subID, reason);
      }
    }
  }
}

/** makeStorageHandlers builds namespaced `storage.get` / `storage.set` bound to
 *  the pack id, so packs cannot read each other's (or the host's) keys. */
export function makeStorageHandlers(
  packId: string,
  store: Pick<Storage, "getItem" | "setItem"> | undefined,
): Record<string, BridgeMethodHandler> {
  const ns = (key: string) => `pack:${packId}:${String(key)}`;
  return {
    "storage.get": (payload) => {
      const p = (payload || {}) as { key?: string };
      if (!p.key || !store) return { value: null };
      return { value: store.getItem(ns(p.key)) };
    },
    "storage.set": (payload) => {
      const p = (payload || {}) as { key?: string; value?: string };
      if (!p.key || !store) return { ok: false };
      store.setItem(ns(p.key), String(p.value ?? ""));
      return { ok: true };
    },
  };
}

/** Type guard: a structurally-valid inbound request envelope. */
export function isBridgeRequest(value: unknown): value is Required<Pick<BridgeEnvelope, "id" | "method">> & BridgeEnvelope {
  if (!value || typeof value !== "object") return false;
  const env = value as BridgeEnvelope;
  return env.kind === "req" && typeof env.id === "string" && typeof env.method === "string";
}

/**
 * dispatchBridgeRequest validates an inbound envelope, routes it to a registered
 * handler, and posts the matching response. Unknown methods are rejected with
 * `forbidden` (default-deny). Handlers that throw yield an `error` response.
 */
export async function dispatchBridgeRequest(ctx: BridgeContext, msg: unknown): Promise<void> {
  if (!isBridgeRequest(msg)) return;
  const handler = ctx.handlers[msg.method];
  if (!handler) {
    ctx.post({
      v: BRIDGE_VERSION,
      id: msg.id,
      kind: "res",
      error: { code: "forbidden", message: `unknown or unpermitted method: ${msg.method}` },
    });
    return;
  }
  try {
    const result = await handler(msg.payload, { packId: ctx.packId });
    ctx.post({ v: BRIDGE_VERSION, id: msg.id, kind: "res", payload: result ?? null });
  } catch (e) {
    ctx.post({
      v: BRIDGE_VERSION,
      id: msg.id,
      kind: "res",
      error: { code: "error", message: (e as Error)?.message || "handler failed" },
    });
  }
}
