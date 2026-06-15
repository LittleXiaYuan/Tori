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

/** A bridge request the host refused: unknown/unpermitted method, undeclared
 *  route, storage quota breach, or message-rate breach. Reported to the audit
 *  trail via BridgeContext.onViolation (spec §7.3). */
export interface BridgeViolation {
  method: string;
  code: string;
  message: string;
}

/** Thrown by capability handlers when the guest asked for something it never
 *  declared (vs. an operational failure, which stays a plain Error). The
 *  dispatcher maps it to res{error:{code}} and fires onViolation. */
export class BridgeViolationError extends Error {
  readonly code: "forbidden" | "quota_exceeded";
  constructor(code: "forbidden" | "quota_exceeded", message: string) {
    super(message);
    this.name = "BridgeViolationError";
    this.code = code;
  }
}

/** Token-bucket limiter for inbound bridge requests. */
export interface BridgeRateLimiter {
  allow(): boolean;
}

/** createBridgeRateLimiter caps how fast a sandboxed pack may issue bridge
 *  requests (default: burst 80, refill 20/s — far above any legitimate UI, but
 *  it stops a looping/malicious bundle from hammering the host and backend). */
export function createBridgeRateLimiter(opts?: {
  capacity?: number;
  refillPerSecond?: number;
  now?: () => number;
}): BridgeRateLimiter {
  const capacity = opts?.capacity ?? 80;
  const refillPerSecond = opts?.refillPerSecond ?? 20;
  const now = opts?.now ?? (() => Date.now());
  let tokens = capacity;
  let last = now();
  return {
    allow() {
      const t = now();
      tokens = Math.min(capacity, tokens + ((t - last) / 1000) * refillPerSecond);
      last = t;
      if (tokens < 1) return false;
      tokens -= 1;
      return true;
    },
  };
}

export interface BridgeContext {
  packId: string;
  /** Posts a message into the sandboxed iframe. */
  post: (msg: BridgeEnvelope) => void;
  /** Capability-gated method handlers, keyed by method name. */
  handlers: Record<string, BridgeMethodHandler>;
  /** Optional inbound request limiter; over-limit requests get res{error:rate_limited}. */
  rateLimit?: BridgeRateLimiter;
  /** Notified once per refused request (forbidden / quota / rate), for auditing. */
  onViolation?: (violation: BridgeViolation) => void;
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
  /** Host-side request deadline in ms (spec §7.3; default 30s, 0 disables). */
  timeoutMs?: number;
}

/** Max request body a pack may send through backend.call (UTF-16 code units). */
export const BackendCallMaxBodyChars = 256 * 1024;

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
      throw new BridgeViolationError("forbidden", "backend.call: path must be an absolute /v1 or /api route");
    }
    if (!isRouteAllowed(opts.routes, method, path)) {
      throw new BridgeViolationError("forbidden", `backend.call: route not permitted for this pack: ${method} ${path}`);
    }
    const f = opts.fetchImpl || fetch;
    const init: RequestInit = {
      method,
      headers: { "Content-Type": "application/json", ...opts.authHeaders() },
    };
    if (p.body !== undefined && method !== "GET") {
      const body = typeof p.body === "string" ? p.body : JSON.stringify(p.body);
      if (body.length > BackendCallMaxBodyChars) {
        throw new BridgeViolationError("quota_exceeded", `backend.call: body too large (max ${BackendCallMaxBodyChars} chars)`);
      }
      init.body = body;
    }
    // Host-side deadline so a hung backend cannot pin a pending bridge request
    // forever (the guest's own 30s timer would give up, but the host fetch and
    // its auth context would still be live).
    const timeoutMs = opts.timeoutMs ?? 30_000;
    let timer: ReturnType<typeof setTimeout> | undefined;
    if (timeoutMs > 0) {
      const controller = new AbortController();
      init.signal = controller.signal;
      timer = setTimeout(() => controller.abort(), timeoutMs);
    }
    try {
      const res = await f(`${opts.baseUrl || ""}${path}`, init);
      const text = await res.text();
      return { status: res.status, ok: res.ok, body: text };
    } catch (e) {
      if ((e as Error)?.name === "AbortError") {
        throw new Error(`backend.call: timeout after ${timeoutMs}ms`);
      }
      throw e;
    } finally {
      if (timer !== undefined) clearTimeout(timer);
    }
  };
}

/** makeNavHandler builds `nav.push`, restricted to the pack's own frontend routes. */
export function makeNavHandler(allowedPaths: string[], navigate: (path: string) => void): BridgeMethodHandler {
  const allow = new Set((allowedPaths || []).map((p) => p.replace(/\/+$/, "") || "/"));
  return (payload) => {
    const p = (payload || {}) as { path?: string };
    const target = String(p.path || "").replace(/\/+$/, "") || "/";
    if (!allow.has(target)) {
      throw new BridgeViolationError("forbidden", `nav.push: path not declared by this pack: ${target}`);
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
      throw new BridgeViolationError("forbidden", "events.subscribe: path must be an absolute route");
    }
    const bare = path.split("?")[0];
    if (!this.opts.paths.includes(bare)) {
      throw new BridgeViolationError("forbidden", `events.subscribe: stream not permitted for this pack: ${bare}`);
    }
    const max = this.opts.maxSubscriptions ?? 4;
    if (this.subs.size >= max) {
      throw new BridgeViolationError("quota_exceeded", `events.subscribe: too many concurrent subscriptions (max ${max})`);
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

/** Per-pack storage quotas. localStorage is shared with the host shell (~5MB
 *  per origin), so an unbounded pack could fill it and break host writes. */
export interface StorageQuota {
  maxKeyChars?: number; // default 128
  maxValueChars?: number; // default 32768 (32K UTF-16 units)
  maxKeys?: number; // default 64 keys per pack namespace
}

type BridgeStore = Pick<Storage, "getItem" | "setItem"> & Partial<Pick<Storage, "length" | "key">>;

/** makeStorageHandlers builds namespaced `storage.get` / `storage.set` bound to
 *  the pack id, so packs cannot read each other's (or the host's) keys. Writes
 *  are quota-gated so a pack cannot exhaust the host origin's localStorage. */
export function makeStorageHandlers(
  packId: string,
  store: BridgeStore | undefined,
  quota?: StorageQuota,
): Record<string, BridgeMethodHandler> {
  const prefix = `pack:${packId}:`;
  const ns = (key: string) => `${prefix}${String(key)}`;
  const maxKeyChars = quota?.maxKeyChars ?? 128;
  const maxValueChars = quota?.maxValueChars ?? 32_768;
  const maxKeys = quota?.maxKeys ?? 64;

  // Counts this pack's keys when the store is enumerable (real localStorage).
  const countNamespaceKeys = (): number | null => {
    if (!store || typeof store.length !== "number" || typeof store.key !== "function") return null;
    let n = 0;
    for (let i = 0; i < store.length; i++) {
      const k = store.key(i);
      if (k && k.startsWith(prefix)) n++;
    }
    return n;
  };

  return {
    "storage.get": (payload) => {
      const p = (payload || {}) as { key?: string };
      if (!p.key || !store) return { value: null };
      return { value: store.getItem(ns(p.key)) };
    },
    "storage.set": (payload) => {
      const p = (payload || {}) as { key?: string; value?: string };
      if (!p.key || !store) return { ok: false };
      const key = String(p.key);
      const value = String(p.value ?? "");
      if (key.length > maxKeyChars) {
        throw new BridgeViolationError("quota_exceeded", `storage.set: key too long (max ${maxKeyChars} chars)`);
      }
      if (value.length > maxValueChars) {
        throw new BridgeViolationError("quota_exceeded", `storage.set: value too large (max ${maxValueChars} chars)`);
      }
      const existing = store.getItem(ns(key));
      if (existing === null) {
        const count = countNamespaceKeys();
        if (count !== null && count >= maxKeys) {
          throw new BridgeViolationError("quota_exceeded", `storage.set: too many keys for this pack (max ${maxKeys})`);
        }
      }
      store.setItem(ns(key), value);
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
 * `forbidden` (default-deny); permission/quota refusals keep their typed code
 * and fire ctx.onViolation so the host can audit them (spec §7.3). Operational
 * handler failures yield a plain `error` response.
 */
export async function dispatchBridgeRequest(ctx: BridgeContext, msg: unknown): Promise<void> {
  if (!isBridgeRequest(msg)) return;
  const refuse = (code: string, message: string) => {
    ctx.post({ v: BRIDGE_VERSION, id: msg.id, kind: "res", error: { code, message } });
    ctx.onViolation?.({ method: msg.method, code, message });
  };
  if (ctx.rateLimit && !ctx.rateLimit.allow()) {
    refuse("rate_limited", "too many bridge requests; slow down");
    return;
  }
  const handler = ctx.handlers[msg.method];
  if (!handler) {
    refuse("forbidden", `unknown or unpermitted method: ${msg.method}`);
    return;
  }
  try {
    const result = await handler(msg.payload, { packId: ctx.packId });
    ctx.post({ v: BRIDGE_VERSION, id: msg.id, kind: "res", payload: result ?? null });
  } catch (e) {
    if (e instanceof BridgeViolationError) {
      refuse(e.code, e.message);
      return;
    }
    ctx.post({
      v: BRIDGE_VERSION,
      id: msg.id,
      kind: "res",
      error: { code: "error", message: (e as Error)?.message || "handler failed" },
    });
  }
}
