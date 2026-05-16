import { afterEach, describe, expect, it, vi } from "vitest";
import { createRPAReplayPackClient } from "../rpa-replay-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("rpa-replay-pack-client", () => {
  it("reads RPA replay pack status and trace list through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.rpa-replay", stage: "pack-shell-before-executor", executor_plan_ready: true, executor_ready: false, action_tracer_plan_ready: true, action_tracer_ready: false, browser_intent_gate_plan_ready: true, browser_intent_ready: false, consumes_browser_intent: false, executes_browser_actions: false, writes_browser_state: false, writes_files: false, network_access: false, trace_count: 1, active_recordings: 0, capabilities: ["rpa.executor.plan"] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ traces: [{ slug: "export-report", name: "Export", recorded_at: "now", step_count: 1 }], count: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ trace: { slug: "export-report", name: "Export", type: "rpa-replay", recorded_at: "now", steps: [] } }), { status: 200 }));

    const client = createRPAReplayPackClient();
    const status = await client.status();
    await client.traces();
    await client.trace("export-report");

    expect(status.executor_plan_ready).toBe(true);
    expect(status.executes_browser_actions).toBe(false);
    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/rpa-replay/status",
      "/v1/rpa-replay/traces",
      "/v1/rpa-replay/traces/export-report",
    ]);
  });

  it("plans Browser Intent / ActionTracer executor handoffs without execution", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: {
        pack_id: "yunque.pack.rpa-replay",
        status: "rpa_executor_handoff_plan_ready_pending_executor",
        executor_plan_ready: true,
        executor_ready: false,
        action_tracer_plan_ready: true,
        action_tracer_ready: false,
        browser_intent_gate_plan_ready: true,
        browser_intent_ready: false,
        consumes_browser_intent: false,
        executes_browser_actions: false,
        writes_browser_state: false,
        writes_files: false,
        network_access: false,
        action_count: 1,
        planned_steps: [{ index: 1, action: "navigate", executor_action: "navigate", value: "https://erp.example.com/reports?month=2026-05", requires_browser_intent: true, requires_action_tracer: true, executes_browser_action: false, writes_browser_state: false, consumes_external_target: false }],
        executor_handoff_plan: { target: "rpa.replay.executor.browser_intent", executor_plan_ready: true, executor_ready: false, step_count: 1, steps: [], blocked_by: ["browser-intent-runtime-not-wired"] },
        browser_intent_gate_plan: { target: "yunque.pack.browser-intent", browser_intent_gate_plan_ready: true, browser_intent_ready: false, consumes_browser_intent: false, executes_browser_actions: false, writes_browser_state: false, network_access: false, blocked_by: ["browser-intent-runtime-not-wired"] },
        action_tracer_handoff_plan: { target: "rpa.action_tracer.trace_sink", action_tracer_plan_ready: true, action_tracer_ready: false, captures_runtime_trace: false, writes_trace_store: false, expected_artifacts: ["runtime-trace.json"], blocked_by: ["action-tracer-not-wired"] },
        artifacts: ["executor-handoff-plan.json", "browser-intent-gate-plan.json", "action-tracer-plan.json"],
      } }), { status: 200 }));

    const client = createRPAReplayPackClient();
    const plan = await client.executorPlan({ slug: "export-report", params: { month: "2026-05" }, dry_run: true });

    expect(plan.plan.executor_plan_ready).toBe(true);
    expect(plan.plan.executor_ready).toBe(false);
    expect(plan.plan.executes_browser_actions).toBe(false);
    expect(plan.plan.writes_browser_state).toBe(false);
    expect(plan.plan.network_access).toBe(false);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/rpa-replay/executor/plan");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ slug: "export-report", params: { month: "2026-05" }, dry_run: true });
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
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.rpa-replay", exported_at: "now", format: "json-evidence-pack", files: ["trace.json", "executor-handoff-plan.json"], trace: { slug: "export-report" }, executor_plan: { executor_plan_ready: true, executor_ready: false } }), { status: 200 }));

    const client = createRPAReplayPackClient();
    await client.evidence("export-report");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/rpa-replay/evidence/export-report");
  });
});
