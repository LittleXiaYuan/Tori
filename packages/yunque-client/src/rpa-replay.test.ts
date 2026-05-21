import { createRPAReplayClient, RPAReplayClientError } from "./rpa-replay";

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

test("RPAReplayClient reads status and trace list with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRPAReplayClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/status")) return jsonResponse({ pack_id: "yunque.pack.rpa-replay", stage: "pack-shell-before-executor", executor_plan_ready: true, executor_ready: false, action_tracer_plan_ready: true, action_tracer_ready: false, browser_intent_gate_plan_ready: true, browser_intent_ready: false, consumes_browser_intent: false, executes_browser_actions: false, writes_browser_state: false, writes_files: false, network_access: false, trace_count: 1, active_recordings: 0, capabilities: ["rpa.executor.plan"] });
      return jsonResponse({ traces: [{ slug: "export-report", name: "Export", recorded_at: "now", step_count: 1 }], count: 1 });
    },
  });

  const status = await client.status();
  const traces = await client.traces();

  assertEqual(status.pack_id, "yunque.pack.rpa-replay");
  assertEqual(status.executor_plan_ready, true);
  assertEqual(status.executes_browser_actions, false);
  assertEqual(traces.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/rpa-replay/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/rpa-replay/traces");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("RPAReplayClient plans Browser Intent executor handoffs without executing browser actions", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRPAReplayClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ plan: {
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
        executor_handoff_plan: { target: "rpa.replay.executor.browser_intent" },
        browser_intent_gate_plan: { target: "yunque.pack.browser-intent" },
        action_tracer_handoff_plan: { target: "rpa.action_tracer.trace_sink" },
        artifacts: ["executor-handoff-plan.json", "browser-intent-gate-plan.json", "action-tracer-plan.json"],
      } });
    },
  });

  const plan = await client.executorPlan({ slug: "export-report", params: { month: "2026-05" }, dry_run: true });

  assertEqual(plan.plan.executor_plan_ready, true);
  assertEqual(plan.plan.executor_ready, false);
  assertEqual(plan.plan.executes_browser_actions, false);
  assertEqual(plan.plan.writes_browser_state, false);
  assertEqual(plan.plan.network_access, false);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/rpa-replay/executor/plan");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ slug: "export-report", params: { month: "2026-05" }, dry_run: true }));
});

test("RPAReplayClient creates and reads traces", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRPAReplayClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ trace: { slug: "export-report", name: "Export", type: "rpa-replay", recorded_at: "now", steps: [{ action: "navigate", value: "{{month}}" }] }, status: "created" }, { status: init?.method === "POST" ? 201 : 200 });
    },
  });

  const created = await client.createTrace({ slug: "export-report", name: "Export", steps: [{ action: "navigate", value: "{{month}}" }] });
  const detail = await client.trace("export report");

  assertEqual(created.status, "created");
  assertEqual(detail.trace.slug, "export-report");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/rpa-replay/traces");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ slug: "export-report", name: "Export", steps: [{ action: "navigate", value: "{{month}}" }] }));
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/rpa-replay/traces/export%20report");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("RPAReplayClient manages recording shell and dry-run replay", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRPAReplayClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/recordings/start")) return jsonResponse({ session: { id: "rec-1", status: "recording", started_at: "now" }, status: "recording", note: "shell" }, { status: 202 });
      if (String(url).endsWith("/recordings/stop")) return jsonResponse({ trace: { slug: "fill-form", name: "Fill", type: "rpa-replay", recorded_at: "now", steps: [] }, status: "recorded" }, { status: 201 });
      return jsonResponse({ result: { success: true, dry_run: true, steps_run: 1, failed_step: -1, duration_ms: 0, planned_steps: [{ action: "click", selector: "#submit" }] }, trace: "fill-form" });
    },
  });

  const started = await client.startRecording({ slug: "fill-form", name: "Fill" });
  const stopped = await client.stopRecording({ session_id: "rec-1", steps: [{ action: "click", selector: "#submit" }] });
  const replay = await client.replay({ slug: "fill-form", params: { month: "2026-05" }, dry_run: true });

  assertEqual(started.session.id, "rec-1");
  assertEqual(stopped.status, "recorded");
  assertEqual(replay.result.steps_run, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/rpa-replay/recordings/start");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/rpa-replay/recordings/stop");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/rpa-replay/replay");
  assertEqual(calls[2]?.init?.body, JSON.stringify({ slug: "fill-form", params: { month: "2026-05" }, dry_run: true }));
});

test("RPAReplayClient exports evidence packs", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRPAReplayClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ pack_id: "yunque.pack.rpa-replay", exported_at: "now", format: "json-evidence-pack", files: ["trace.json", "executor-handoff-plan.json"], trace: { slug: "export-report", name: "Export", type: "rpa-replay", recorded_at: "now", steps: [] }, executor_plan: { executor_plan_ready: true, executor_ready: false } });
    },
  });

  const evidence = await client.evidence("export-report");

  assertEqual(evidence.format, "json-evidence-pack");
  assertDeepEqual(evidence.files, ["trace.json", "executor-handoff-plan.json"]);
  assertEqual(evidence.executor_plan?.executor_ready, false);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/rpa-replay/evidence/export-report");
});

test("RPAReplayClient throws RPAReplayClientError with parsed body", async () => {
  const client = createRPAReplayClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "pack route is not enabled" }, { status: 404 }),
  });

  try {
    await client.status();
    throw new Error("expected status to reject");
  } catch (error) {
    assert(error instanceof RPAReplayClientError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "pack route is not enabled" });
    assertEqual(error.message, "pack route is not enabled");
  }

  const nestedClient = createRPAReplayClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_TRACE", message: "slug is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.replay({ slug: "" });
    throw new Error("expected replay to reject");
  } catch (error) {
    assert(error instanceof RPAReplayClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "slug is required");
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
