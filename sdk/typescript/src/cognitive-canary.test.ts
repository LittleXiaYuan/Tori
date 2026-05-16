import { CognitiveCanaryClientError, createCognitiveCanaryClient } from "./cognitive-canary";

declare const process: { exitCode?: number };

function assert(condition: unknown, message?: string): asserts condition {
  if (!condition) throw new Error(message || "assertion failed");
}

function assertEqual(actual: unknown, expected: unknown, message?: string): void {
  if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`);
}

function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void {
  const actualJson = JSON.stringify(actual);
  const expectedJson = JSON.stringify(expected);
  if (actualJson !== expectedJson) throw new Error(message || `expected ${actualJson} to deep equal ${expectedJson}`);
}

const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];

function test(name: string, fn: () => Promise<void> | void): void {
  tests.push({ name, fn });
}

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

const scenario = {
  id: "runtime-quality-check",
  name: "Runtime quality check",
  category: "planner",
  question: "q",
  stable_response: "stable",
  canary_response: "canary",
  enabled: true,
  weight: 1,
};

test("CognitiveCanaryClient reads status and scenarios with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognitiveCanaryClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/status")) return jsonResponse({ pack_id: "yunque.pack.cognitive-canary", stage: "pack-shell-before-shadow-traffic", shadow_plan_ready: true, shadow_traffic_ready: false, judge_plan_ready: true, judge_pipeline_ready: false, response_collector_plan_ready: true, response_collector_store_ready: true, response_collector_writeback_ready: true, writes_response_collector_store: true, response_collector_ready: false, response_collector_store: { artifact: "response-collector-store.json", record_count: 0 }, metrics_plan_ready: true, prometheus_ready: false, quality_sli_ready: true, auto_rollback_plan_ready: true, auto_rollback_ready: false, scenario_count: 1, report_count: 0, policy: {}, capabilities: ["canary.response_collector.plan", "canary.response_collector.writeback"] });
      return jsonResponse({ scenarios: [scenario], count: 1 });
    },
  });

  const status = await client.status();
  const scenarios = await client.scenarios();

  assertEqual(status.pack_id, "yunque.pack.cognitive-canary");
  assertEqual(status.response_collector_writeback_ready, true);
  assertEqual(status.response_collector_store?.artifact, "response-collector-store.json");
  assertEqual(scenarios.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognitive-canary/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognitive-canary/scenarios");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("CognitiveCanaryClient saves scenarios, evaluates canaries, plans shadow traffic, writes response collector store, and reads report detail", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognitiveCanaryClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/scenarios") && init?.method === "POST") return jsonResponse({ scenarios: [], count: 0, status: "saved" }, { status: 201 });
      if (String(url).endsWith("/evaluate")) return jsonResponse({ report: { id: "canary-1", pack_id: "yunque.pack.cognitive-canary", created_at: "now", stage: "pack-shell-before-shadow-traffic", scenario_count: 1, safety_failure_count: 0, error_count: 0, quality_score: 4.2, safety_pass_rate: 100, delta_score: 0.1, latency_p99_ratio: 1.1, canary_error_rate: 0, gate_status: "pass", promotion_decision: "promote", results: [] }, status: "dry_run" });
      if (String(url).endsWith("/shadow/plan")) return jsonResponse({ plan: { pack_id: "yunque.pack.cognitive-canary", generated_at: "now", status: "shadow_plan", report_id: "canary-1", candidate_version: "1.1.0-rc1", stable_version: "1.0.0", traffic_percent: 5, sample_percent: 5, shadow_plan_ready: true, shadow_traffic_ready: false, judge_plan_ready: true, judge_pipeline_ready: false, response_collector_plan_ready: true, response_collector_ready: false, metrics_plan_ready: true, prometheus_ready: false, auto_rollback_plan_ready: true, auto_rollback_ready: false, quality_score: 4.2, safety_pass_rate: 100, delta_score: 0.1, latency_p99_ratio: 1.1, canary_error_rate: 0, gate_status: "pass", promotion_decision: "promote", shadow_pairs: [], response_collectors: [{ pair_id: "runtime-quality-check-abc", scenario_id: "runtime-quality-check", category: "planner", stable_version: "1.0.0", candidate_version: "1.1.0-rc1", sample_percent: 5, collector_route: "/v1/cognitive-canary/shadow/collect", artifact: "response-collector/runtime-quality-check-abc.json", artifact_sha256: "a".repeat(64), artifact_bytes: 128, writes_files: false, ready: false, labels: { pack_id: "yunque.pack.cognitive-canary" } }], response_collector_summary: { collector_count: 1, artifact_count: 1, writes_files: false, deterministic: true, hash_algorithm: "sha256", ready: false }, judge_batches: [], metrics: [], rollback_actions: [], actions: [] } });
      if (String(url).endsWith("/response-collector/writeback")) return jsonResponse({ writeback: { pack_id: "yunque.pack.cognitive-canary", generated_at: "now", status: "response_collector_store_written_pending_shadow_pipeline", report_id: "canary-1", candidate_version: "1.1.0-rc1", stable_version: "1.0.0", sample_percent: 5, response_collector_store_ready: true, response_collector_writeback_ready: true, writes_response_collector_store: true, response_collector_ready: false, shadow_traffic_ready: false, judge_pipeline_ready: false, prometheus_ready: false, auto_rollback_ready: false, writes_files: false, record_count: 1, records: [{ pack_id: "yunque.pack.cognitive-canary", record_id: "canary-collector-1", record_key: "key", report_id: "canary-1", pair_id: "runtime-quality-check-abc", scenario_id: "runtime-quality-check", category: "planner", stable_version: "1.0.0", candidate_version: "1.1.0-rc1", sample_percent: 5, collector_route: "/v1/cognitive-canary/shadow/collect", artifact: "response-collector/runtime-quality-check-abc.json", artifact_sha256: "a".repeat(64), artifact_bytes: 128, source: "shadow_plan", status: "response_collector_store_written_pending_shadow_pipeline", created_at: "now", updated_at: "now", report_summary: { id: "canary-1", created_at: "now", scenario_count: 1, safety_failure_count: 0, error_count: 0, quality_score: 4.2, safety_pass_rate: 100, delta_score: 0.1, latency_p99_ratio: 1.1, canary_error_rate: 0, gate_status: "pass", promotion_decision: "promote" }, collector_plan: { pair_id: "runtime-quality-check-abc", scenario_id: "runtime-quality-check", category: "planner", stable_version: "1.0.0", candidate_version: "1.1.0-rc1", sample_percent: 5, collector_route: "/v1/cognitive-canary/shadow/collect", artifact: "response-collector/runtime-quality-check-abc.json", artifact_sha256: "a".repeat(64), artifact_bytes: 128, writes_files: false, ready: false }, response_collector_store_ready: true, response_collector_writeback_ready: true, writes_response_collector_store: true, response_collector_ready: false, shadow_traffic_ready: false, judge_pipeline_ready: false, prometheus_ready: false, auto_rollback_ready: false, writes_files: false, artifacts: ["response-collector-record.json"], labels: ["pack-local-store"] }], response_collector_store: { pack_id: "yunque.pack.cognitive-canary", store: "pack-local-json", store_ready: true, record_count: 1, artifact: "response-collector-store.json", response_collector_store_ready: true, response_collector_writeback_ready: true, writes_response_collector_store: true, response_collector_ready: false, shadow_traffic_ready: false, judge_pipeline_ready: false, prometheus_ready: false, auto_rollback_ready: false }, shadow_plan: { pack_id: "yunque.pack.cognitive-canary", generated_at: "now", status: "shadow_plan", traffic_percent: 5, sample_percent: 5, shadow_plan_ready: true, shadow_traffic_ready: false, judge_plan_ready: true, judge_pipeline_ready: false, response_collector_plan_ready: true, response_collector_ready: false, metrics_plan_ready: true, prometheus_ready: false, auto_rollback_plan_ready: true, auto_rollback_ready: false, quality_score: 4.2, safety_pass_rate: 100, delta_score: 0.1, latency_p99_ratio: 1.1, canary_error_rate: 0, gate_status: "pass", promotion_decision: "promote", shadow_pairs: [], response_collectors: [], response_collector_summary: { collector_count: 0, artifact_count: 0, writes_files: false, deterministic: true, hash_algorithm: "sha256", ready: false }, judge_batches: [], metrics: [], rollback_actions: [], actions: [] }, artifacts: ["response-collector-store.json", "response-collector-record.json"], actions: [], labels: [] } });
      return jsonResponse({ report: { id: "canary-1", results: [] } });
    },
  });

  const saved = await client.saveScenarios({ scenarios: [scenario], replace: true });
  const run = await client.evaluate({ scenario_ids: ["runtime-quality-check"], persist: false, candidate_version: "1.1.0-rc1" });
  const plan = await client.shadowPlan({ report_id: "canary-1", traffic_percent: 5, requested_by: "unit" });
  const writeback = await client.responseCollectorWriteback({ report_id: "canary-1", sample_percent: 5, requested_by: "unit" });
  const report = await client.report("canary-1");

  assertEqual(saved.status, "saved");
  assertEqual(run.report.gate_status, "pass");
  assertEqual(plan.plan.shadow_plan_ready, true);
  assertEqual(plan.plan.shadow_traffic_ready, false);
  assertEqual(plan.plan.response_collector_plan_ready, true);
  assertEqual(plan.plan.response_collector_summary.writes_files, false);
  assertEqual(plan.plan.response_collectors[0]?.artifact_sha256.length, 64);
  assertEqual(writeback.writeback.response_collector_writeback_ready, true);
  assertEqual(writeback.writeback.writes_response_collector_store, true);
  assertEqual(writeback.writeback.response_collector_ready, false);
  assertEqual(writeback.writeback.response_collector_store.artifact, "response-collector-store.json");
  assertEqual(report.report.id, "canary-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognitive-canary/scenarios");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognitive-canary/evaluate");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ scenario_ids: ["runtime-quality-check"], persist: false, candidate_version: "1.1.0-rc1" }));
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/cognitive-canary/shadow/plan");
  assertEqual(calls[2]?.init?.method, "POST");
  assertEqual(calls[2]?.init?.body, JSON.stringify({ report_id: "canary-1", traffic_percent: 5, requested_by: "unit" }));
  assertEqual(calls[3]?.url, "http://localhost:9090/v1/cognitive-canary/response-collector/writeback");
  assertEqual(calls[3]?.init?.method, "POST");
  assertEqual(calls[3]?.init?.body, JSON.stringify({ report_id: "canary-1", sample_percent: 5, requested_by: "unit" }));
  assertEqual(calls[4]?.url, "http://localhost:9090/v1/cognitive-canary/reports/canary-1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("CognitiveCanaryClient lists reports and exports evidence packs", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognitiveCanaryClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/reports")) return jsonResponse({ reports: [{ id: "canary-1", created_at: "now", scenario_count: 1, safety_failure_count: 0, error_count: 0, quality_score: 4.2, safety_pass_rate: 100, delta_score: 0.1, latency_p99_ratio: 1.1, canary_error_rate: 0, gate_status: "pass", promotion_decision: "promote" }], count: 1 });
      return jsonResponse({ pack_id: "yunque.pack.cognitive-canary", exported_at: "now", format: "json-cognitive-canary-evidence", files: ["canary-report.json", "shadow-plan.json", "response-collector-plan.json", "response-collector-store.json", "response-collector-record.json", "judge-plan.json", "metrics-plan.json", "rollback-plan.json"], report: { id: "canary-1", results: [] }, shadow_plan: { shadow_plan_ready: true, shadow_traffic_ready: false, response_collector_plan_ready: true, response_collector_ready: false, response_collectors: [], response_collector_summary: { writes_files: false, hash_algorithm: "sha256" } }, response_collector_store: { artifact: "response-collector-store.json", record_count: 1 }, response_collector_records: [{ record_id: "canary-collector-1", writes_response_collector_store: true, response_collector_ready: false }] });
    },
  });

  const reports = await client.reports();
  const evidence = await client.evidence("canary-1");

  assertEqual(reports.count, 1);
  assertEqual(evidence.format, "json-cognitive-canary-evidence");
  assertDeepEqual(evidence.files, ["canary-report.json", "shadow-plan.json", "response-collector-plan.json", "response-collector-store.json", "response-collector-record.json", "judge-plan.json", "metrics-plan.json", "rollback-plan.json"]);
  assertEqual(evidence.shadow_plan?.response_collector_summary.writes_files, false);
  assertEqual(evidence.response_collector_store?.artifact, "response-collector-store.json");
  assertEqual(evidence.response_collector_records?.[0]?.writes_response_collector_store, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognitive-canary/reports");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognitive-canary/evidence/canary-1");
});

test("CognitiveCanaryClient throws CognitiveCanaryClientError with nested gateway messages", async () => {
  const client = createCognitiveCanaryClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "pack route is not enabled" }, { status: 404 }),
  });

  try {
    await client.status();
    throw new Error("expected status to reject");
  } catch (error) {
    assert(error instanceof CognitiveCanaryClientError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "pack route is not enabled" });
    assertEqual(error.message, "pack route is not enabled");
  }

  const nestedClient = createCognitiveCanaryClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_SCENARIO", message: "scenario id is invalid" } }, { status: 400 }),
  });

  try {
    await nestedClient.saveScenarios({ scenarios: [] });
    throw new Error("expected saveScenarios to reject");
  } catch (error) {
    assert(error instanceof CognitiveCanaryClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "scenario id is invalid");
  }
});

let failures = 0;
for (const { name, fn } of tests) {
  try {
    await fn();
    console.log(`ok - ${name}`);
  } catch (error) {
    failures += 1;
    console.error(`not ok - ${name}`);
    console.error(error);
  }
}

if (failures > 0) {
  process.exitCode = 1;
} else {
  console.log(`1..${tests.length}`);
}
