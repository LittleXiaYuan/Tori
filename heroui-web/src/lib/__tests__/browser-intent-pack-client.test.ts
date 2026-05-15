import { afterEach, describe, expect, it, vi } from "vitest";
import { createBrowserIntentPackClient } from "../browser-intent-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("browser-intent-pack-client", () => {
  it("reads browser pack status, config, screenshot and scenarios through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ connected: true, state: "extension" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ mode: "extension", connected: true }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ screenshot: "abc", timestamp: "now" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ connected: true, pending: 0 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ scenarios: [{ id: "open-page", name: "Open", description: "", icon: "link", steps: [] }] }), { status: 200 }));

    const client = createBrowserIntentPackClient();
    await client.status();
    await client.config();
    await client.screenshotLatest();
    await client.extensionStatus();
    await client.scenarios();

    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/browser/status",
      "/v1/browser/config",
      "/v1/browser/screenshot/latest",
      "/api/browser/ext/status",
      "/api/browser/ext/scenarios",
    ]);
  });

  it("mutates browser pack routes with method-aware payloads", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ text: "page text" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true, ws_url: "ws://localhost:9090/ws/browser", ticket: "t1", nonce: "n1", expires_at: "now", ttl_sec: 120 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "resolved" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true, scenario: "open-page", results: [] }), { status: 200 }));

    const client = createBrowserIntentPackClient();
    await client.ocr("dom");
    await client.extensionSession();
    await client.oppDecide("opp-1", "allow");
    await client.runScenario("open-page");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/browser/ocr");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ mode: "dom" });
    expect(spy.mock.calls[1]?.[0]).toBe("/api/browser/ext/session");
    expect((spy.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/browser/opp/decide");
    expect(JSON.parse(String((spy.mock.calls[2]?.[1] as RequestInit).body))).toEqual({ id: "opp-1", decision: "allow" });
    expect(spy.mock.calls[3]?.[0]).toBe("/api/browser/ext/scenarios/run");
    expect(JSON.parse(String((spy.mock.calls[3]?.[1] as RequestInit).body))).toEqual({ scenario_id: "open-page" });
  });

  it("keeps desktop helpers colocated with the browser pack workspace", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true, running: false }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true, sandbox: { id: "d1", stream_url: "https://desk", created_at: "now" } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true }), { status: 200 }));

    const client = createBrowserIntentPackClient();
    await client.desktopStatus();
    await client.desktopCreate();
    await client.desktopDestroy();

    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/sandbox/desktop/status",
      "/v1/sandbox/desktop",
      "/v1/sandbox/desktop/destroy",
    ]);
    expect((spy.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
  });
});
