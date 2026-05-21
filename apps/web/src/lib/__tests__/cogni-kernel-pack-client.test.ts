import { afterEach, describe, expect, it, vi } from "vitest";
import { createCogniKernelPackClient } from "../cogni-kernel-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("cogni-kernel-pack-client", () => {
  it("reads runtime loop pack-state through the pack-owned gate", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          pack_id: "yunque.pack.cogni-kernel",
          stage: "runtime-loop-pack-state-gate",
          pack_installed: true,
          pack_enabled: true,
          pack_status: "enabled",
          runtime_loop_pack_state_ready: true,
          runtime_loop_running: true,
          stops_runtime_loops: false,
          starts_runtime_loops: true,
          clears_runtime_state: false,
          sentinel_ready: true,
          scheduler_ready: true,
          bus_ready: true,
          experience_store_ready: true,
          active_bus_cognis: 2,
          experience_store_count: 1,
          generated_at: "2026-05-16T12:00:00Z",
          capabilities: ["cognis.runtime.pack_state", "cognis.runtime.loop_gate"],
          artifacts: ["cogni-runtime-pack-state.json"],
        }),
        { status: 200 },
      ),
    );

    const client = createCogniKernelPackClient();
    const report = await client.runtimePackState();

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/cognis/runtime/pack-state");
    expect(report.runtime_loop_pack_state_ready).toBe(true);
    expect(report.artifacts).toContain("cogni-runtime-pack-state.json");
  });

  it("reads registry, alerts and traces through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ cognis: [], count: 0, version: 1, dir: "data/cognis" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ alerts: [], count: 0 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ id: "code-reviewer", traces: [], count: 0 }), { status: 200 }));

    const client = createCogniKernelPackClient();
    await client.list();
    await client.alerts();
    await client.tracesByID("code-reviewer", 5);

    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/cognis",
      "/v1/cognis/alerts",
      "/v1/cognis/code-reviewer/trace?limit=5",
    ]);
  });

  it("mutates cogni registry through method-aware pack routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "ok", id: "reviewer" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "ok", id: "reviewer" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "ok", id: "reviewer" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "ok", id: "reviewer" }), { status: 200 }));

    const client = createCogniKernelPackClient();
    await client.add({ id: "reviewer" });
    await client.setEnabled("reviewer", false);
    await client.remove("reviewer");
    await client.reload();

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/cognis");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ id: "reviewer" });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/cognis/reviewer/disable");
    expect((spy.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/cognis/reviewer");
    expect((spy.mock.calls[2]?.[1] as RequestInit).method).toBe("DELETE");
    expect(spy.mock.calls[3]?.[0]).toBe("/v1/cognis/reload");
    expect((spy.mock.calls[3]?.[1] as RequestInit).method).toBe("POST");
  });

  it("runs workflow, confirms experience pattern and triggers evolution", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ workflow_name: "review", success: true, outputs: {}, step_results: [], duration: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "ok", id: "reviewer", confirmed: true }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "ok", id: "reviewer" }), { status: 200 }));

    const client = createCogniKernelPackClient();
    await client.runWorkflow("reviewer", "review", { pr: 1 });
    await client.confirmExperiencePattern("reviewer", "pat-1");
    await client.triggerEvolution("reviewer");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/cognis/reviewer/workflow/review");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ pr: 1 });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/cognis/reviewer/experience/patterns/pat-1/confirm");
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/cognis/reviewer/evolve");
  });
});
