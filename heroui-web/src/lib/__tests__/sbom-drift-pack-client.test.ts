import { afterEach, describe, expect, it, vi } from "vitest";
import { createSBOMDriftPackClient } from "../sbom-drift-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("sbom-drift-pack-client", () => {
  it("reads SBOM Drift pack status and snapshots through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.sbom-drift", stage: "pack-shell-before-ci", scanner_ready: true, vulnerability_ready: false, snapshot_count: 1, capabilities: [] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ snapshots: [{ id: "baseline", source: "unit", created_at: "now", component_count: 1, ecosystems: { gomod: 1 } }], count: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ snapshot: { id: "baseline", source: "unit", created_at: "now", component_count: 1, ecosystems: { gomod: 1 }, components: [] } }), { status: 200 }));

    const client = createSBOMDriftPackClient();
    await client.status();
    await client.snapshots();
    await client.snapshot("baseline");

    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/sbom-drift/status",
      "/v1/sbom-drift/snapshots",
      "/v1/sbom-drift/snapshots/baseline",
    ]);
  });

  it("creates snapshots and diffs with method-aware payloads", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ snapshot: { id: "baseline", components: [] }, status: "created" }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ diff: { base: { id: "baseline" }, target: { id: "current" }, added: [], removed: [], changed: [], risk_level: "none" } }), { status: 200 }));

    const client = createSBOMDriftPackClient();
    await client.createSnapshot({ id: "baseline", source: "manual" });
    await client.diff({ base_id: "baseline", target_current: true });

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/sbom-drift/snapshots");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ id: "baseline", source: "manual" });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/sbom-drift/diff");
    expect(JSON.parse(String((spy.mock.calls[1]?.[1] as RequestInit).body))).toEqual({ base_id: "baseline", target_current: true });
  });

  it("exports JSON evidence packs by snapshot id", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.sbom-drift", exported_at: "now", format: "json-sbom-drift-evidence", files: ["snapshot.json"], snapshot: { id: "baseline" } }), { status: 200 }));

    const client = createSBOMDriftPackClient();
    await client.evidence("baseline");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/sbom-drift/evidence/baseline");
  });
});
