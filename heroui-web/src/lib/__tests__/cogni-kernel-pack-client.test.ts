import { afterEach, describe, expect, it, vi } from "vitest";
import { createCogniKernelPackClient } from "../cogni-kernel-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("cogni-kernel-pack-client", () => {
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
