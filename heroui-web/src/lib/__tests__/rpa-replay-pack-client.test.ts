import { afterEach, describe, expect, it, vi } from "vitest";
import { createRPAReplayPackClient } from "../rpa-replay-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("rpa-replay-pack-client", () => {
  it("reads RPA replay pack status and trace list through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.rpa-replay", stage: "pack-shell", executor_ready: false, trace_count: 1, active_recordings: 0, capabilities: [] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ traces: [{ slug: "export-report", name: "Export", recorded_at: "now", step_count: 1 }], count: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ trace: { slug: "export-report", name: "Export", type: "rpa-replay", recorded_at: "now", steps: [] } }), { status: 200 }));

    const client = createRPAReplayPackClient();
    await client.status();
    await client.traces();
    await client.trace("export-report");

    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/rpa-replay/status",
      "/v1/rpa-replay/traces",
      "/v1/rpa-replay/traces/export-report",
    ]);
  });

  it("mutates trace, recording and replay routes with method-aware payloads", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ trace: { slug: "export-report", name: "Export", type: "rpa-replay", steps: [] }, status: "created" }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ session: { id: "rec-1", status: "recording", started_at: "now" }, status: "recording" }), { status: 202 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ trace: { slug: "export-report", name: "Export", type: "rpa-replay", steps: [] }, status: "recorded" }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ result: { success: true, dry_run: true, steps_run: 1, failed_step: -1, duration_ms: 0 }, trace: "export-report" }), { status: 200 }));

    const client = createRPAReplayPackClient();
    await client.createTrace({ slug: "export-report", name: "Export", steps: [{ action: "navigate", value: "{{month}}" }] });
    await client.startRecording({ slug: "fill-form", name: "Fill form" });
    await client.stopRecording({ session_id: "rec-1", steps: [{ action: "click", selector: "#submit" }] });
    await client.replay({ slug: "export-report", params: { month: "2026-05" }, dry_run: true });

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/rpa-replay/traces");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toMatchObject({ slug: "export-report" });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/rpa-replay/recordings/start");
    expect((spy.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/rpa-replay/recordings/stop");
    expect(JSON.parse(String((spy.mock.calls[2]?.[1] as RequestInit).body))).toMatchObject({ session_id: "rec-1" });
    expect(spy.mock.calls[3]?.[0]).toBe("/v1/rpa-replay/replay");
    expect(JSON.parse(String((spy.mock.calls[3]?.[1] as RequestInit).body))).toEqual({ slug: "export-report", params: { month: "2026-05" }, dry_run: true });
  });

  it("exports JSON evidence packs by trace slug", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.rpa-replay", exported_at: "now", format: "json-evidence-pack", files: ["trace.json"], trace: { slug: "export-report" } }), { status: 200 }));

    const client = createRPAReplayPackClient();
    await client.evidence("export-report");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/rpa-replay/evidence/export-report");
  });
});
