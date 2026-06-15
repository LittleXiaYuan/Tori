import { describe, expect, it, vi } from "vitest";
import {
  BRIDGE_VERSION,
  BackendCallMaxBodyChars,
  BridgeViolationError,
  createBridgeRateLimiter,
  createSSEParser,
  dispatchBridgeRequest,
  eventPathsFromPermissions,
  isBridgeRequest,
  isRouteAllowed,
  makeBackendCallHandler,
  makeNavHandler,
  makeStorageHandlers,
  PackEventSubscriptions,
  type AllowedRoute,
  type BridgeEnvelope,
  type BridgeContext,
  type BridgeViolation,
  type PackSSEEvent,
} from "../pack-bridge";

function makeCtx(handlers: BridgeContext["handlers"], extra?: Partial<BridgeContext>) {
  const posted: BridgeEnvelope[] = [];
  const ctx: BridgeContext = {
    packId: "yunque.pack.demo",
    post: (m) => posted.push(m),
    handlers,
    ...extra,
  };
  return { ctx, posted };
}

describe("pack-bridge/isBridgeRequest", () => {
  it("accepts a well-formed request and rejects junk", () => {
    expect(isBridgeRequest({ kind: "req", id: "1", method: "ui.toast" })).toBe(true);
    expect(isBridgeRequest({ kind: "res", id: "1", method: "x" })).toBe(false);
    expect(isBridgeRequest({ kind: "req", method: "x" })).toBe(false); // no id
    expect(isBridgeRequest(null)).toBe(false);
    expect(isBridgeRequest("nope")).toBe(false);
  });
});

describe("pack-bridge/dispatchBridgeRequest", () => {
  it("routes to a handler and posts the result with the same id", async () => {
    const handler = vi.fn(() => ({ ok: true }));
    const { ctx, posted } = makeCtx({ "ui.resize": handler });

    await dispatchBridgeRequest(ctx, { kind: "req", id: "c1", method: "ui.resize", payload: { height: 200 } });

    expect(handler).toHaveBeenCalledWith({ height: 200 }, { packId: "yunque.pack.demo" });
    expect(posted).toEqual([
      { v: BRIDGE_VERSION, id: "c1", kind: "res", payload: { ok: true } },
    ]);
  });

  it("rejects unknown methods with forbidden (default-deny)", async () => {
    const { ctx, posted } = makeCtx({});
    await dispatchBridgeRequest(ctx, { kind: "req", id: "c2", method: "backend.call", payload: {} });
    expect(posted).toHaveLength(1);
    expect(posted[0].error?.code).toBe("forbidden");
    expect(posted[0].id).toBe("c2");
  });

  it("returns an error response when a handler throws", async () => {
    const { ctx, posted } = makeCtx({
      "host.handshake": () => { throw new Error("boom"); },
    });
    await dispatchBridgeRequest(ctx, { kind: "req", id: "c3", method: "host.handshake" });
    expect(posted[0].error).toEqual({ code: "error", message: "boom" });
  });

  it("ignores non-request envelopes (responses/events/garbage)", async () => {
    const handler = vi.fn();
    const { ctx, posted } = makeCtx({ "ui.toast": handler });
    await dispatchBridgeRequest(ctx, { kind: "res", id: "x", method: "ui.toast" });
    await dispatchBridgeRequest(ctx, { kind: "event", method: "ui.toast" });
    await dispatchBridgeRequest(ctx, 42);
    expect(handler).not.toHaveBeenCalled();
    expect(posted).toHaveLength(0);
  });

  it("normalizes undefined handler results to null payload", async () => {
    const { ctx, posted } = makeCtx({ "ui.toast": () => undefined });
    await dispatchBridgeRequest(ctx, { kind: "req", id: "c4", method: "ui.toast", payload: { message: "hi" } });
    expect(posted[0]).toEqual({ v: BRIDGE_VERSION, id: "c4", kind: "res", payload: null });
  });

  it("keeps typed violation codes and notifies onViolation", async () => {
    const violations: BridgeViolation[] = [];
    const { ctx, posted } = makeCtx(
      { "nav.push": () => { throw new BridgeViolationError("forbidden", "path not declared"); } },
      { onViolation: (v) => violations.push(v) },
    );

    await dispatchBridgeRequest(ctx, { kind: "req", id: "v1", method: "nav.push", payload: { path: "/settings" } });
    await dispatchBridgeRequest(ctx, { kind: "req", id: "v2", method: "no.such.method" });

    expect(posted[0].error?.code).toBe("forbidden");
    expect(violations).toEqual([
      { method: "nav.push", code: "forbidden", message: "path not declared" },
      { method: "no.such.method", code: "forbidden", message: "unknown or unpermitted method: no.such.method" },
    ]);
  });

  it("does not treat operational handler errors as violations", async () => {
    const violations: BridgeViolation[] = [];
    const { ctx, posted } = makeCtx(
      { "backend.call": () => { throw new Error("network down"); } },
      { onViolation: (v) => violations.push(v) },
    );
    await dispatchBridgeRequest(ctx, { kind: "req", id: "e1", method: "backend.call" });
    expect(posted[0].error?.code).toBe("error");
    expect(violations).toHaveLength(0);
  });

  it("rate-limits inbound requests and reports the breach", async () => {
    let t = 0;
    const violations: BridgeViolation[] = [];
    const { ctx, posted } = makeCtx(
      { "ui.resize": () => ({ ok: true }) },
      {
        rateLimit: createBridgeRateLimiter({ capacity: 2, refillPerSecond: 1, now: () => t }),
        onViolation: (v) => violations.push(v),
      },
    );

    await dispatchBridgeRequest(ctx, { kind: "req", id: "r1", method: "ui.resize" });
    await dispatchBridgeRequest(ctx, { kind: "req", id: "r2", method: "ui.resize" });
    await dispatchBridgeRequest(ctx, { kind: "req", id: "r3", method: "ui.resize" });
    expect(posted[2].error?.code).toBe("rate_limited");
    expect(violations).toHaveLength(1);

    // Tokens refill with time, so a well-behaved pack recovers.
    t += 2000;
    await dispatchBridgeRequest(ctx, { kind: "req", id: "r4", method: "ui.resize" });
    expect(posted[3].error).toBeUndefined();
  });
});

describe("pack-bridge/isRouteAllowed", () => {
  const routes: AllowedRoute[] = [
    { method: "POST", path: "/v1/hello/ping" },
    { method: "GET", path: "/v1/hello/state" },
  ];
  it("matches declared method+path exactly, ignoring query", () => {
    expect(isRouteAllowed(routes, "POST", "/v1/hello/ping")).toBe(true);
    expect(isRouteAllowed(routes, "get", "/v1/hello/state?x=1")).toBe(true);
    expect(isRouteAllowed(routes, "HEAD", "/v1/hello/state")).toBe(true); // HEAD→GET
  });
  it("rejects undeclared routes and method mismatches", () => {
    expect(isRouteAllowed(routes, "POST", "/v1/hello/state")).toBe(false);
    expect(isRouteAllowed(routes, "DELETE", "/v1/admin/wipe")).toBe(false);
  });
});

describe("pack-bridge/makeBackendCallHandler", () => {
  it("injects auth, gates on the whitelist, and returns status+body", async () => {
    const fetchImpl = vi.fn(async () =>
      new Response(JSON.stringify({ pong: true }), { status: 200 }),
    );
    const handler = makeBackendCallHandler({
      routes: [{ method: "POST", path: "/v1/hello/ping" }],
      authHeaders: () => ({ Authorization: "Bearer secret" }),
      fetchImpl: fetchImpl as unknown as typeof fetch,
      baseUrl: "http://localhost:9090",
    });

    const out = (await handler({ method: "POST", path: "/v1/hello/ping", body: { x: 1 } }, { packId: "p" })) as {
      status: number; ok: boolean; body: string;
    };

    expect(out.status).toBe(200);
    expect(out.ok).toBe(true);
    expect(JSON.parse(out.body)).toEqual({ pong: true });
    const [url, init] = fetchImpl.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("http://localhost:9090/v1/hello/ping");
    expect(init.body).toBe(JSON.stringify({ x: 1 }));
    expect(init.headers).toMatchObject({ Authorization: "Bearer secret" });
  });

  it("refuses routes not declared by the pack (no fetch)", async () => {
    const fetchImpl = vi.fn();
    const handler = makeBackendCallHandler({
      routes: [{ method: "GET", path: "/v1/hello/state" }],
      authHeaders: () => ({}),
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    await expect(handler({ method: "POST", path: "/v1/admin/wipe" }, { packId: "p" })).rejects.toThrow(BridgeViolationError);
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it("rejects oversized request bodies before fetching", async () => {
    const fetchImpl = vi.fn();
    const handler = makeBackendCallHandler({
      routes: [{ method: "POST", path: "/v1/hello/ping" }],
      authHeaders: () => ({}),
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    const huge = "x".repeat(BackendCallMaxBodyChars + 1);
    await expect(handler({ method: "POST", path: "/v1/hello/ping", body: huge }, { packId: "p" }))
      .rejects.toThrow(/body too large/);
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it("aborts a hung backend call after the host-side timeout", async () => {
    vi.useFakeTimers();
    try {
      const fetchImpl = vi.fn(
        (_url: string, init?: RequestInit) =>
          new Promise<Response>((_resolve, reject) => {
            init?.signal?.addEventListener("abort", () => {
              const err = new Error("aborted");
              err.name = "AbortError";
              reject(err);
            });
          }),
      );
      const handler = makeBackendCallHandler({
        routes: [{ method: "GET", path: "/v1/hello/state" }],
        authHeaders: () => ({}),
        fetchImpl: fetchImpl as unknown as typeof fetch,
        timeoutMs: 5000,
      });
      const pending = handler({ method: "GET", path: "/v1/hello/state" }, { packId: "p" }) as Promise<unknown>;
      const assertion = expect(pending).rejects.toThrow(/timeout after 5000ms/);
      await vi.advanceTimersByTimeAsync(5001);
      await assertion;
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("pack-bridge/makeNavHandler", () => {
  it("navigates to declared paths and refuses others", () => {
    const navigate = vi.fn();
    const handler = makeNavHandler(["/packs/demo", "/packs/demo/detail"], navigate);
    expect(handler({ path: "/packs/demo/" }, { packId: "p" })).toEqual({ ok: true });
    expect(navigate).toHaveBeenCalledWith("/packs/demo");
    expect(() => handler({ path: "/settings" }, { packId: "p" })).toThrow(/not declared/);
  });
});

describe("pack-bridge/createSSEParser", () => {
  it("parses events across chunk boundaries with event names and multi-line data", () => {
    const p = createSSEParser();
    expect(p.push("event: trace\ndata: {\"a\":")).toEqual([]);
    const events = p.push("1}\n\ndata: line1\ndata: line2\n\n: keepalive\n\n");
    expect(events).toEqual([
      { event: "trace", data: '{"a":1}' },
      { event: undefined, data: "line1\nline2" },
    ]);
  });
});

describe("pack-bridge/eventPathsFromPermissions", () => {
  it("extracts declared SSE paths and ignores everything else", () => {
    expect(eventPathsFromPermissions([
      "dlc:demo",
      "events:subscribe:/v1/events/stream",
      "events:subscribe:not-absolute",
    ])).toEqual(["/v1/events/stream"]);
    expect(eventPathsFromPermissions(undefined)).toEqual([]);
  });
});

function sseResponse(frames: string[]): Response {
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      const enc = new TextEncoder();
      for (const f of frames) controller.enqueue(enc.encode(f));
      controller.close();
    },
  });
  return new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

describe("pack-bridge/PackEventSubscriptions", () => {
  it("rejects undeclared streams (default-deny) without fetching", () => {
    const fetchImpl = vi.fn();
    const subs = new PackEventSubscriptions({
      paths: ["/v1/events/stream"],
      authHeaders: () => ({}),
      emit: () => {},
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    expect(() => subs.subscribe("/v1/admin/stream")).toThrow(/not permitted/);
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it("streams declared SSE events into emit with injected auth, then closes", async () => {
    const emitted: Array<{ subID: string; evt: PackSSEEvent }> = [];
    const closed: string[] = [];
    const fetchImpl = vi.fn(async (_url: string, init?: RequestInit) => {
      expect((init?.headers as Record<string, string>).Authorization).toBe("Bearer secret");
      return sseResponse(["event: trace\ndata: one\n\n", "data: two\n\n"]);
    });
    const subs = new PackEventSubscriptions({
      paths: ["/v1/events/stream"],
      authHeaders: () => ({ Authorization: "Bearer secret" }),
      emit: (subID, evt) => emitted.push({ subID, evt }),
      onClose: (subID) => closed.push(subID),
      fetchImpl: fetchImpl as unknown as typeof fetch,
      baseUrl: "http://localhost:9090",
    });

    const { sub_id } = subs.subscribe("/v1/events/stream?filter=x");
    await vi.waitFor(() => expect(closed).toContain(sub_id));
    expect(fetchImpl.mock.calls[0][0]).toBe("http://localhost:9090/v1/events/stream?filter=x");
    expect(emitted.map((e) => e.evt)).toEqual([
      { event: "trace", data: "one" },
      { event: undefined, data: "two" },
    ]);
  });

  it("unsubscribe aborts the stream and enforces the concurrency cap", async () => {
    let abortSignal: AbortSignal | undefined;
    const fetchImpl = vi.fn(async (_url: string, init?: RequestInit) => {
      abortSignal = init?.signal as AbortSignal;
      // Never-ending stream.
      const body = new ReadableStream<Uint8Array>({ start() {} });
      return new Response(body, { status: 200 });
    });
    const subs = new PackEventSubscriptions({
      paths: ["/v1/events/stream"],
      authHeaders: () => ({}),
      emit: () => {},
      fetchImpl: fetchImpl as unknown as typeof fetch,
      maxSubscriptions: 1,
    });

    const { sub_id } = subs.subscribe("/v1/events/stream");
    expect(() => subs.subscribe("/v1/events/stream")).toThrow(/too many/);
    expect(subs.unsubscribe(sub_id)).toBe(true);
    await vi.waitFor(() => expect(abortSignal?.aborted).toBe(true));
    expect(subs.unsubscribe(sub_id)).toBe(false);
  });

  it("closeAll aborts every active stream", async () => {
    const signals: AbortSignal[] = [];
    const fetchImpl = vi.fn(async (_url: string, init?: RequestInit) => {
      signals.push(init?.signal as AbortSignal);
      return new Response(new ReadableStream<Uint8Array>({ start() {} }), { status: 200 });
    });
    const subs = new PackEventSubscriptions({
      paths: ["/a", "/b"],
      authHeaders: () => ({}),
      emit: () => {},
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    subs.subscribe("/a");
    subs.subscribe("/b");
    subs.closeAll();
    await vi.waitFor(() => expect(signals.every((s) => s.aborted)).toBe(true));
  });

  it("exposes events.subscribe/unsubscribe bridge handlers", async () => {
    const fetchImpl = vi.fn(async () => sseResponse(["data: hi\n\n"]));
    const subs = new PackEventSubscriptions({
      paths: ["/v1/events/stream"],
      authHeaders: () => ({}),
      emit: () => {},
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    const handlers = subs.handlers();
    const res = (await handlers["events.subscribe"]({ path: "/v1/events/stream" }, { packId: "p" })) as { sub_id: string };
    expect(res.sub_id).toMatch(/^sub-/);
    expect(await handlers["events.unsubscribe"]({ sub_id: res.sub_id }, { packId: "p" })).toEqual({ ok: true });
  });
});

describe("pack-bridge/makeStorageHandlers", () => {
  function mapStore() {
    const mem = new Map<string, string>();
    return {
      mem,
      store: {
        getItem: (k: string) => (mem.has(k) ? mem.get(k)! : null),
        setItem: (k: string, v: string) => void mem.set(k, v),
        get length() { return mem.size; },
        key: (i: number) => [...mem.keys()][i] ?? null,
      },
    };
  }

  it("namespaces keys per pack", () => {
    const { mem, store } = mapStore();
    const h = makeStorageHandlers("yunque.pack.demo", store);
    h["storage.set"]({ key: "theme", value: "dark" }, { packId: "yunque.pack.demo" });
    expect(mem.get("pack:yunque.pack.demo:theme")).toBe("dark");
    expect(h["storage.get"]({ key: "theme" }, { packId: "yunque.pack.demo" })).toEqual({ value: "dark" });
    expect(h["storage.get"]({ key: "missing" }, { packId: "yunque.pack.demo" })).toEqual({ value: null });
  });

  it("enforces value-size and key-count quotas", () => {
    const { mem, store } = mapStore();
    // Foreign keys (host or other packs) must not count against this pack.
    mem.set("yunque_token", "host-secret");
    mem.set("pack:other.pack:x", "1");

    const h = makeStorageHandlers("yunque.pack.demo", store, { maxValueChars: 8, maxKeys: 2 });
    const ctx = { packId: "yunque.pack.demo" };

    expect(() => h["storage.set"]({ key: "big", value: "123456789" }, ctx)).toThrow(/value too large/);
    h["storage.set"]({ key: "a", value: "1" }, ctx);
    h["storage.set"]({ key: "b", value: "2" }, ctx);
    expect(() => h["storage.set"]({ key: "c", value: "3" }, ctx)).toThrow(/too many keys/);
    // Overwriting an existing key stays allowed at the cap.
    h["storage.set"]({ key: "a", value: "9" }, ctx);
    expect(mem.get("pack:yunque.pack.demo:a")).toBe("9");
  });

  it("rejects oversized keys", () => {
    const { store } = mapStore();
    const h = makeStorageHandlers("yunque.pack.demo", store, { maxKeyChars: 4 });
    expect(() => h["storage.set"]({ key: "abcde", value: "1" }, { packId: "yunque.pack.demo" }))
      .toThrow(/key too long/);
  });
});

describe("pack-bridge/createBridgeRateLimiter", () => {
  it("allows bursts up to capacity and refills over time", () => {
    let t = 0;
    const rl = createBridgeRateLimiter({ capacity: 3, refillPerSecond: 2, now: () => t });
    expect([rl.allow(), rl.allow(), rl.allow(), rl.allow()]).toEqual([true, true, true, false]);
    t += 500; // +1 token
    expect(rl.allow()).toBe(true);
    expect(rl.allow()).toBe(false);
  });
});
