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
      if (String(url).endsWith("/status")) return jsonResponse({ pack_id: "yunque.pack.cognitive-canary", stage: "pack-shell-before-shadow-traffic", shadow_traffic_ready: false, judge_pipeline_ready: false, quality_sli_ready: true, auto_rollback_ready: false, scenario_count: 1, report_count: 0, policy: {}, capabilities: [] });
      return jsonResponse({ scenarios: [scenario], count: 1 });
    },
  });

  const status = await client.status();
  const scenarios = await client.scenarios();

  assertEqual(status.pack_id, "yunque.pack.cognitive-canary");
  assertEqual(scenarios.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognitive-canary/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognitive-canary/scenarios");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("CognitiveCanaryClient saves scenarios, evaluates canaries, and reads report detail", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognitiveCanaryClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/scenarios") && init?.method === "POST") return jsonResponse({ scenarios: [], count: 0, status: "saved" }, { status: 201 });
      if (String(url).endsWith("/evaluate")) return jsonResponse({ report: { id: "canary-1", pack_id: "yunque.pack.cognitive-canary", created_at: "now", stage: "pack-shell-before-shadow-traffic", scenario_count: 1, safety_failure_count: 0, error_count: 0, quality_score: 4.2, safety_pass_rate: 100, delta_score: 0.1, latency_p99_ratio: 1.1, canary_error_rate: 0, gate_status: "pass", promotion_decision: "promote", results: [] }, status: "dry_run" });
      return jsonResponse({ report: { id: "canary-1", results: [] } });
    },
  });

  const saved = await client.saveScenarios({ scenarios: [scenario], replace: true });
  const run = await client.evaluate({ scenario_ids: ["runtime-quality-check"], persist: false, candidate_version: "1.1.0-rc1" });
  const report = await client.report("canary-1");

  assertEqual(saved.status, "saved");
  assertEqual(run.report.gate_status, "pass");
  assertEqual(report.report.id, "canary-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognitive-canary/scenarios");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognitive-canary/evaluate");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ scenario_ids: ["runtime-quality-check"], persist: false, candidate_version: "1.1.0-rc1" }));
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/cognitive-canary/reports/canary-1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("CognitiveCanaryClient lists reports and exports evidence packs", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognitiveCanaryClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/reports")) return jsonResponse({ reports: [{ id: "canary-1", created_at: "now", scenario_count: 1, safety_failure_count: 0, error_count: 0, quality_score: 4.2, safety_pass_rate: 100, delta_score: 0.1, latency_p99_ratio: 1.1, canary_error_rate: 0, gate_status: "pass", promotion_decision: "promote" }], count: 1 });
      return jsonResponse({ pack_id: "yunque.pack.cognitive-canary", exported_at: "now", format: "json-cognitive-canary-evidence", files: ["canary-report.json"], report: { id: "canary-1", results: [] } });
    },
  });

  const reports = await client.reports();
  const evidence = await client.evidence("canary-1");

  assertEqual(reports.count, 1);
  assertEqual(evidence.format, "json-cognitive-canary-evidence");
  assertDeepEqual(evidence.files, ["canary-report.json"]);
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
