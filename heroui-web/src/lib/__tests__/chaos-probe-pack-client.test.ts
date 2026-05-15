import { afterEach, describe, expect, it, vi } from "vitest";
import { createChaosProbePackClient } from "../chaos-probe-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("chaos-probe-pack-client", () => {
  it("reads Chaos Probe status, definitions, and reports through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.chaos-probe", stage: "pack-shell-before-scheduler", safe_probe_ready: true, scheduler_ready: false, degrade_engine_ready: false, alert_writeback_ready: false, probe_count: 1, report_count: 1, policy: {}, capabilities: [] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ probes: [{ id: "runtime-healthz-probe", name: "Runtime healthz probe", category: "network", description: "local", safe: true, enabled: true, interval_seconds: 30, weight: 1 }], count: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ reports: [{ id: "chaos-1", created_at: "now", probe_count: 1, pass_count: 1, degraded_count: 0, fail_count: 0, health_score: 100, degrade_level: 0, gate_status: "pass" }], count: 1 }), { status: 200 }));

    const client = createChaosProbePackClient();
    await client.status();
    await client.probes();
    await client.reports();

    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/chaos-probe/status",
      "/v1/chaos-probe/probes",
      "/v1/chaos-probe/reports",
    ]);
  });

  it("saves definitions and runs probes with method-aware payloads", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ probes: [], count: 0, status: "saved" }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ report: { id: "chaos-1", gate_status: "pass", health_score: 100, results: [] }, status: "dry_run" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ report: { id: "chaos-1", results: [] } }), { status: 200 }));

    const client = createChaosProbePackClient();
    await client.saveProbes({ probes: [{ id: "runtime-healthz-probe", name: "Runtime healthz probe", category: "network", description: "local", safe: true, enabled: true, interval_seconds: 30, weight: 1 }], replace: true });
    await client.run({ probe_ids: ["runtime-healthz-probe"], persist: false });
    await client.report("chaos-1");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/chaos-probe/probes");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ probes: [{ id: "runtime-healthz-probe", name: "Runtime healthz probe", category: "network", description: "local", safe: true, enabled: true, interval_seconds: 30, weight: 1 }], replace: true });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/chaos-probe/run");
    expect((spy.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/chaos-probe/reports/chaos-1");
  });

  it("exports JSON evidence packs by report id", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.chaos-probe", exported_at: "now", format: "json-chaos-probe-evidence", files: ["chaos-report.json"], report: { id: "chaos-1", results: [] } }), { status: 200 }));

    const client = createChaosProbePackClient();
    await client.evidence("chaos-1");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/chaos-probe/evidence/chaos-1");
  });
});
