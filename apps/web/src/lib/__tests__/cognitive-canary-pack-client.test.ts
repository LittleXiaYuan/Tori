import { afterEach, describe, expect, it, vi } from "vitest";
import { createCognitiveCanaryPackClient } from "../cognitive-canary-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("cognitive-canary-pack-client", () => {
  it("reads Cognitive Canary status, scenarios, and reports through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.cognitive-canary", stage: "pack-shell-before-shadow-traffic", shadow_plan_ready: true, shadow_traffic_ready: false, judge_plan_ready: true, judge_pipeline_ready: false, response_collector_plan_ready: true, response_collector_store_ready: true, response_collector_writeback_ready: true, writes_response_collector_store: true, response_collector_pipeline_plan_ready: true, response_collector_pipeline_ready: false, consumes_response_collector_store: true, response_collector_ready: false, response_collector_store: { record_count: 0, artifact: "response-collector-store.json", response_collector_store_ready: true, response_collector_pipeline_plan_ready: true, consumes_response_collector_store: true, writes_response_collector_store: true }, metrics_plan_ready: true, prometheus_ready: false, quality_sli_ready: true, auto_rollback_plan_ready: true, auto_rollback_ready: false, scenario_count: 1, report_count: 1, policy: {}, capabilities: ["canary.response_collector.plan", "canary.response_collector.writeback", "canary.response_collector.pipeline.plan"] }), { status: 200 }))
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

  it("saves scenarios, evaluates canaries, plans shadow rollout, writes response collector store, and plans pipeline handoff", async () => {
    const scenario = { id: "runtime-quality-check", name: "Runtime quality check", category: "planner", question: "q", stable_response: "stable", canary_response: "canary", enabled: true, weight: 1 };
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ scenarios: [], count: 0, status: "saved" }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ report: { id: "canary-1", gate_status: "pass", quality_score: 4.2, promotion_decision: "promote", results: [] }, status: "dry_run" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.cognitive-canary", status: "shadow_plan", generated_at: "now", traffic_percent: 5, sample_percent: 5, shadow_plan_ready: true, shadow_traffic_ready: false, judge_plan_ready: true, judge_pipeline_ready: false, response_collector_plan_ready: true, response_collector_ready: false, metrics_plan_ready: true, prometheus_ready: false, auto_rollback_plan_ready: true, auto_rollback_ready: false, quality_score: 4.2, safety_pass_rate: 100, delta_score: 0.1, latency_p99_ratio: 1.1, canary_error_rate: 0, gate_status: "pass", promotion_decision: "promote", shadow_pairs: [], response_collectors: [{ pair_id: "runtime-quality-check-abc", scenario_id: "runtime-quality-check", category: "planner", stable_version: "1.0.0", candidate_version: "1.1.0-rc1", sample_percent: 5, collector_route: "/v1/cognitive-canary/shadow/collect", artifact: "response-collector/runtime-quality-check-abc.json", artifact_sha256: "a".repeat(64), artifact_bytes: 128, writes_files: false, ready: false, labels: { pack_id: "yunque.pack.cognitive-canary" } }], response_collector_summary: { collector_count: 1, artifact_count: 1, writes_files: false, deterministic: true, hash_algorithm: "sha256", ready: false }, judge_batches: [], metrics: [], rollback_actions: [], actions: [] } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ writeback: { pack_id: "yunque.pack.cognitive-canary", generated_at: "now", status: "response_collector_store_written_pending_shadow_pipeline", report_id: "canary-1", sample_percent: 5, response_collector_store_ready: true, response_collector_writeback_ready: true, writes_response_collector_store: true, response_collector_ready: false, shadow_traffic_ready: false, judge_pipeline_ready: false, prometheus_ready: false, auto_rollback_ready: false, writes_files: false, record_count: 1, records: [{ record_id: "canary-collector-1", record_key: "key", report_id: "canary-1", pair_id: "runtime-quality-check-abc", scenario_id: "runtime-quality-check", category: "planner", stable_version: "1.0.0", candidate_version: "1.1.0-rc1", sample_percent: 5, collector_route: "/v1/cognitive-canary/shadow/collect", artifact: "response-collector/runtime-quality-check-abc.json", artifact_sha256: "a".repeat(64), artifact_bytes: 128, source: "shadow_plan", status: "response_collector_store_written_pending_shadow_pipeline", created_at: "now", updated_at: "now", report_summary: { id: "canary-1" }, collector_plan: { pair_id: "runtime-quality-check-abc", scenario_id: "runtime-quality-check", artifact_sha256: "a".repeat(64), writes_files: false }, response_collector_store_ready: true, response_collector_writeback_ready: true, writes_response_collector_store: true, response_collector_ready: false, shadow_traffic_ready: false, judge_pipeline_ready: false, prometheus_ready: false, auto_rollback_ready: false, writes_files: false, artifacts: ["response-collector-record.json"], labels: ["pack-local-store"] }], response_collector_store: { pack_id: "yunque.pack.cognitive-canary", store: "pack-local-json", store_ready: true, record_count: 1, artifact: "response-collector-store.json", response_collector_store_ready: true, response_collector_writeback_ready: true, writes_response_collector_store: true, response_collector_ready: false, shadow_traffic_ready: false, judge_pipeline_ready: false, prometheus_ready: false, auto_rollback_ready: false }, shadow_plan: { response_collector_plan_ready: true, response_collector_ready: false, response_collectors: [], response_collector_summary: { writes_files: false } }, artifacts: ["response-collector-store.json", "response-collector-record.json"], actions: [], labels: [] } }), { status: 202 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.cognitive-canary", generated_at: "now", status: "response_collector_pipeline_handoff_plan", report_id: "canary-1", record_count: 1, response_collector_pipeline_plan_ready: true, response_collector_pipeline_ready: false, consumes_response_collector_store: true, response_collector_store_ready: true, response_collector_writeback_ready: true, writes_response_collector_store: true, response_collector_ready: false, shadow_traffic_ready: false, judge_pipeline_ready: false, prometheus_ready: false, auto_rollback_ready: false, writes_files: false, records: [], response_collector_store: { artifact: "response-collector-store.json", record_count: 1 }, response_collector_pipeline_plan: { target: "runtime.cognitive_canary.response_collector_pipeline", source_store: "response-collector-store.json", report_id: "canary-1", record_ids: ["canary-collector-1"], pair_ids: ["runtime-quality-check-abc"], artifacts: ["response-collector/runtime-quality-check-abc.json"], artifact: "response-collector-handoff-plan.json", artifact_sha256: "b".repeat(64), artifact_bytes: 192, dedup_key: "c".repeat(64), consumes_response_collector_store: true, writes_live_response_artifacts: false, writes_judge_batches: false, writes_prometheus_metrics: false, writes_rollback_state: false, response_collector_pipeline_ready: false, response_collector_ready: false, shadow_traffic_ready: false, judge_pipeline_ready: false, prometheus_ready: false, auto_rollback_ready: false, approval_required: false, actions: [], blocked_by: ["live-response-collector-not-wired"] }, artifacts: ["response-collector-pipeline-plan.json", "response-collector-handoff-plan.json"], actions: [], labels: [] } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ report: { id: "canary-1", results: [] } }), { status: 200 }));

    const client = createCognitiveCanaryPackClient();
    await client.saveScenarios({ scenarios: [scenario], replace: true });
    await client.evaluate({ scenario_ids: ["runtime-quality-check"], persist: false, candidate_version: "1.1.0-rc1" });
    const shadow = await client.shadowPlan({ report_id: "canary-1", traffic_percent: 5, requested_by: "unit" });
    const writeback = await client.responseCollectorWriteback({ report_id: "canary-1", sample_percent: 5, requested_by: "unit" });
    const pipeline = await client.responseCollectorPipelinePlan({ report_id: "canary-1", requested_by: "unit" });
    await client.report("canary-1");

    expect(shadow.plan.response_collector_plan_ready).toBe(true);
    expect(shadow.plan.response_collector_summary.writes_files).toBe(false);
    expect(shadow.plan.response_collectors[0]?.artifact_sha256).toHaveLength(64);
    expect(writeback.writeback.response_collector_writeback_ready).toBe(true);
    expect(writeback.writeback.writes_response_collector_store).toBe(true);
    expect(writeback.writeback.response_collector_ready).toBe(false);
    expect(writeback.writeback.response_collector_store.artifact).toBe("response-collector-store.json");
    expect(pipeline.plan.response_collector_pipeline_plan_ready).toBe(true);
    expect(pipeline.plan.consumes_response_collector_store).toBe(true);
    expect(pipeline.plan.response_collector_pipeline_ready).toBe(false);
    expect(pipeline.plan.response_collector_pipeline_plan.artifact_sha256).toHaveLength(64);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/cognitive-canary/scenarios");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ scenarios: [scenario], replace: true });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/cognitive-canary/evaluate");
    expect((spy.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/cognitive-canary/shadow/plan");
    expect((spy.mock.calls[2]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[2]?.[1] as RequestInit).body))).toEqual({ report_id: "canary-1", traffic_percent: 5, requested_by: "unit" });
    expect(spy.mock.calls[3]?.[0]).toBe("/v1/cognitive-canary/response-collector/writeback");
    expect((spy.mock.calls[3]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[3]?.[1] as RequestInit).body))).toEqual({ report_id: "canary-1", sample_percent: 5, requested_by: "unit" });
    expect(spy.mock.calls[4]?.[0]).toBe("/v1/cognitive-canary/response-collector/pipeline/plan");
    expect((spy.mock.calls[4]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[4]?.[1] as RequestInit).body))).toEqual({ report_id: "canary-1", requested_by: "unit" });
    expect(spy.mock.calls[5]?.[0]).toBe("/v1/cognitive-canary/reports/canary-1");
  });

  it("exports JSON evidence packs by report id", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.cognitive-canary", exported_at: "now", format: "json-cognitive-canary-evidence", files: ["canary-report.json", "shadow-plan.json", "response-collector-plan.json", "response-collector-store.json", "response-collector-record.json", "response-collector-pipeline-plan.json", "response-collector-handoff-plan.json", "judge-plan.json", "metrics-plan.json", "rollback-plan.json"], report: { id: "canary-1", results: [] }, shadow_plan: { shadow_plan_ready: true, shadow_traffic_ready: false, response_collector_plan_ready: true, response_collector_ready: false, response_collectors: [], response_collector_summary: { writes_files: false, hash_algorithm: "sha256" } }, response_collector_store: { artifact: "response-collector-store.json", record_count: 1 }, response_collector_records: [{ record_id: "canary-collector-1", writes_response_collector_store: true, response_collector_ready: false }], response_collector_pipeline_plan_ready: true, response_collector_pipeline_plan: { response_collector_pipeline_plan_ready: true, consumes_response_collector_store: true, response_collector_pipeline_ready: false, response_collector_pipeline_plan: { artifact: "response-collector-handoff-plan.json" } } }), { status: 200 }));

    const client = createCognitiveCanaryPackClient();
    const evidence = await client.evidence("canary-1");

    expect(evidence.files).toContain("shadow-plan.json");
    expect(evidence.files).toContain("response-collector-plan.json");
    expect(evidence.files).toContain("response-collector-store.json");
    expect(evidence.files).toContain("response-collector-pipeline-plan.json");
    expect(evidence.response_collector_store?.artifact).toBe("response-collector-store.json");
    expect(evidence.response_collector_records?.[0]?.writes_response_collector_store).toBe(true);
    expect(evidence.response_collector_pipeline_plan_ready).toBe(true);
    expect(evidence.shadow_plan?.response_collector_summary.writes_files).toBe(false);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/cognitive-canary/evidence/canary-1");
  });
});
