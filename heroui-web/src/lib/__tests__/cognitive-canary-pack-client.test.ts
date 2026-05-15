import { afterEach, describe, expect, it, vi } from "vitest";
import { createCognitiveCanaryPackClient } from "../cognitive-canary-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("cognitive-canary-pack-client", () => {
  it("reads Cognitive Canary status, scenarios, and reports through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.cognitive-canary", stage: "pack-shell-before-shadow-traffic", shadow_plan_ready: true, shadow_traffic_ready: false, judge_plan_ready: true, judge_pipeline_ready: false, metrics_plan_ready: true, prometheus_ready: false, quality_sli_ready: true, auto_rollback_plan_ready: true, auto_rollback_ready: false, scenario_count: 1, report_count: 1, policy: {}, capabilities: [] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ scenarios: [{ id: "troubleshooting-summary", name: "Troubleshooting summary", category: "planner", question: "q", stable_response: "stable", canary_response: "canary", enabled: true, weight: 1 }], count: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ reports: [{ id: "canary-1", created_at: "now", scenario_count: 1, safety_failure_count: 0, error_count: 0, quality_score: 4.2, safety_pass_rate: 100, delta_score: 0.1, latency_p99_ratio: 1.1, canary_error_rate: 0, gate_status: "pass", promotion_decision: "promote" }], count: 1 }), { status: 200 }));

    const client = createCognitiveCanaryPackClient();
    await client.status();
    await client.scenarios();
    await client.reports();

    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/cognitive-canary/status",
      "/v1/cognitive-canary/scenarios",
      "/v1/cognitive-canary/reports",
    ]);
  });

  it("saves scenarios, evaluates canaries, and plans shadow rollout with method-aware payloads", async () => {
    const scenario = { id: "runtime-quality-check", name: "Runtime quality check", category: "planner", question: "q", stable_response: "stable", canary_response: "canary", enabled: true, weight: 1 };
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ scenarios: [], count: 0, status: "saved" }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ report: { id: "canary-1", gate_status: "pass", quality_score: 4.2, promotion_decision: "promote", results: [] }, status: "dry_run" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.cognitive-canary", status: "shadow_plan", generated_at: "now", traffic_percent: 5, sample_percent: 5, shadow_plan_ready: true, shadow_traffic_ready: false, judge_plan_ready: true, judge_pipeline_ready: false, metrics_plan_ready: true, prometheus_ready: false, auto_rollback_plan_ready: true, auto_rollback_ready: false, quality_score: 4.2, safety_pass_rate: 100, delta_score: 0.1, latency_p99_ratio: 1.1, canary_error_rate: 0, gate_status: "pass", promotion_decision: "promote", shadow_pairs: [], judge_batches: [], metrics: [], rollback_actions: [], actions: [] } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ report: { id: "canary-1", results: [] } }), { status: 200 }));

    const client = createCognitiveCanaryPackClient();
    await client.saveScenarios({ scenarios: [scenario], replace: true });
    await client.evaluate({ scenario_ids: ["runtime-quality-check"], persist: false, candidate_version: "1.1.0-rc1" });
    await client.shadowPlan({ report_id: "canary-1", traffic_percent: 5, requested_by: "unit" });
    await client.report("canary-1");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/cognitive-canary/scenarios");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ scenarios: [scenario], replace: true });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/cognitive-canary/evaluate");
    expect((spy.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/cognitive-canary/shadow/plan");
    expect((spy.mock.calls[2]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[2]?.[1] as RequestInit).body))).toEqual({ report_id: "canary-1", traffic_percent: 5, requested_by: "unit" });
    expect(spy.mock.calls[3]?.[0]).toBe("/v1/cognitive-canary/reports/canary-1");
  });

  it("exports JSON evidence packs by report id", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.cognitive-canary", exported_at: "now", format: "json-cognitive-canary-evidence", files: ["canary-report.json", "shadow-plan.json", "judge-plan.json", "metrics-plan.json", "rollback-plan.json"], report: { id: "canary-1", results: [] }, shadow_plan: { shadow_plan_ready: true, shadow_traffic_ready: false } }), { status: 200 }));

    const client = createCognitiveCanaryPackClient();
    const evidence = await client.evidence("canary-1");

    expect(evidence.files).toContain("shadow-plan.json");
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/cognitive-canary/evidence/canary-1");
  });
});
