import { createSkillAnomalyClient, SkillAnomalyClientError } from "./skill-anomaly";

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

test("SkillAnomalyClient reads status and profiles with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillAnomalyClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/status")) return jsonResponse({ pack_id: "yunque.pack.skill-anomaly", stage: "pack-shell-before-audit-hook", detector_ready: true, audit_hook_ready: false, approval_writeback_ready: true, approval_queue_store_ready: true, approval_queue_store: { artifact: "approval-queue-store.json", record_count: 1 }, profile_count: 1, active_profiles: 1, anomaly_count: 0, policy: {}, capabilities: ["skill.approval_queue.writeback"] });
      return jsonResponse({ profiles: [{ skill_slug: "text_processing", observed: 3, action_distrib: {}, param_key_set: {}, success_rate: 1, avg_duration_ms: 100, anomaly_count: 0, updated_at: "now" }], count: 1 });
    },
  });

  const status = await client.status();
  const profiles = await client.profiles();

  assertEqual(status.pack_id, "yunque.pack.skill-anomaly");
  assertEqual(status.approval_queue_store_ready, true);
  assertEqual(status.approval_queue_store?.artifact, "approval-queue-store.json");
  assertEqual(profiles.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/skill-anomaly/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/skill-anomaly/profiles");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("SkillAnomalyClient observes, detects, lists events, and reads profile detail", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillAnomalyClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/events") && init?.method === "POST") return jsonResponse({ event: { skill_slug: "text_processing" }, result: { score: 0 }, status: "observed" }, { status: 201 });
      if (String(url).endsWith("/detect")) return jsonResponse({ result: { skill_slug: "text_processing", score: 7, severity: "needs_approval", needs_approval: true, block: true } });
      if (String(url).endsWith("/audit-hook/plan")) return jsonResponse({ plan: { pack_id: "yunque.pack.skill-anomaly", skill_slug: "text_processing", status: "approval_plan", audit_hook_ready: false, trust_mutation_ready: false, approval_writeback_ready: false, approval_queue: { queue_name: "skill_anomaly_approval", request_id: "req-1", request_key: "req-key-1", queue_writeback_ready: false, writes_approval_queue: false, writes_queue_store: false, status: "blocked_until_approval_queue_writeback", store_artifact: "approval-queue-store.json" }, trust_mutation: { delta: -10 } } });
      if (String(url).endsWith("/approval-queue/writeback")) return jsonResponse({ writeback: { pack_id: "yunque.pack.skill-anomaly", status: "approval_queue_written_pending_audit_trust_wiring", approval_writeback_ready: true, writes_approval_queue: true, writes_approval_queue_file: true, audit_hook_ready: false, trust_mutation_ready: false, merkle_append_ready: false, action_allowed: false, execution_blocked: true, request_id: "req-1", request_key: "req-key-1", approval_queue_store: { pack_id: "yunque.pack.skill-anomaly", queue_name: "skill_anomaly_approval", artifact: "approval-queue-store.json", record_count: 1 }, approval_queue_record: { request_id: "req-1", request_key: "req-key-1", store_artifact: "approval-queue-store.json", artifacts: ["approval-queue-store.json", "approval-queue-record.json"] }, artifacts: ["approval-queue-store.json", "approval-queue-record.json"], plan_summary: { status: "approval_plan" } } }, { status: 202 });
      if (String(url).includes("/profiles/text_processing")) return jsonResponse({ profile: { skill_slug: "text_processing", recent: [] } });
      return jsonResponse({ events: [{ skill_slug: "text_processing", action: "read_file" }], count: 1 });
    },
  });

  const observed = await client.observe({ skill_slug: "text_processing", action: "read_file", params: { path: "notes.md" }, success: true });
  const detected = await client.detect({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true });
  const auditPlan = await client.auditHookPlan({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true, requested_by: "operator" });
  const writeback = await client.approvalQueueWriteback({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true, requested_by: "operator", request_id: "req-1", request_key: "req-key-1" });
  const profile = await client.profile("text_processing");
  const events = await client.events({ skill_slug: "text_processing", limit: 10 });

  assertEqual(observed.status, "observed");
  assertEqual(detected.result.severity, "needs_approval");
  assertEqual(auditPlan.plan.status, "approval_plan");
  assertEqual(writeback.writeback.approval_writeback_ready, true);
  assertEqual(writeback.writeback.writes_approval_queue, true);
  assertEqual(writeback.writeback.writes_approval_queue_file, true);
  assertEqual(writeback.writeback.audit_hook_ready, false);
  assertEqual(writeback.writeback.trust_mutation_ready, false);
  assertEqual(writeback.writeback.merkle_append_ready, false);
  assertEqual(writeback.writeback.action_allowed, false);
  assertEqual(writeback.writeback.execution_blocked, true);
  assertEqual(writeback.writeback.approval_queue_store.artifact, "approval-queue-store.json");
  assertEqual(profile.profile.skill_slug, "text_processing");
  assertEqual(events.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/skill-anomaly/events");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ skill_slug: "text_processing", action: "read_file", params: { path: "notes.md" }, success: true }));
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/skill-anomaly/detect");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/skill-anomaly/audit-hook/plan");
  assertEqual(calls[2]?.init?.method, "POST");
  assertEqual(calls[2]?.init?.body, JSON.stringify({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true, requested_by: "operator" }));
  assertEqual(calls[3]?.url, "http://localhost:9090/v1/skill-anomaly/approval-queue/writeback");
  assertEqual(calls[3]?.init?.method, "POST");
  assertEqual(calls[3]?.init?.body, JSON.stringify({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true, requested_by: "operator", request_id: "req-1", request_key: "req-key-1" }));
  assertEqual(calls[4]?.url, "http://localhost:9090/v1/skill-anomaly/profiles/text_processing");
  assertEqual(calls[5]?.url, "http://localhost:9090/v1/skill-anomaly/events?skill_slug=text_processing&limit=10");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("SkillAnomalyClient exports profile evidence packs", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillAnomalyClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ pack_id: "yunque.pack.skill-anomaly", exported_at: "now", format: "json-skill-anomaly-evidence", files: ["profile.json", "audit-hook-plan.json", "trust-mutation-plan.json", "approval-queue-store.json", "approval-queue-record.json"], profile: { skill_slug: "text_processing" }, events: [], policy: {}, audit_hook_plan: { status: "no_op" }, trust_mutation_plan: { delta: 0 }, approval_queue_store: { artifact: "approval-queue-store.json" }, approval_queue_record: { store_artifact: "approval-queue-store.json" } });
    },
  });

  const evidence = await client.evidence("text_processing");

  assertEqual(evidence.format, "json-skill-anomaly-evidence");
  assertDeepEqual(evidence.files, ["profile.json", "audit-hook-plan.json", "trust-mutation-plan.json", "approval-queue-store.json", "approval-queue-record.json"]);
  assert(evidence.audit_hook_plan);
  assertEqual(evidence.approval_queue_store?.artifact, "approval-queue-store.json");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/skill-anomaly/evidence/text_processing");
});

test("SkillAnomalyClient throws SkillAnomalyClientError with nested gateway messages", async () => {
  const client = createSkillAnomalyClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "pack route is not enabled" }, { status: 404 }),
  });

  try {
    await client.status();
    throw new Error("expected status to reject");
  } catch (error) {
    assert(error instanceof SkillAnomalyClientError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "pack route is not enabled" });
    assertEqual(error.message, "pack route is not enabled");
  }

  const nestedClient = createSkillAnomalyClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_EVENT", message: "action is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.detect({ skill_slug: "text_processing", action: "" });
    throw new Error("expected detect to reject");
  } catch (error) {
    assert(error instanceof SkillAnomalyClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "action is required");
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
