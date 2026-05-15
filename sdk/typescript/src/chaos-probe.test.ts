import { ChaosProbeClientError, createChaosProbeClient } from "./chaos-probe";

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

test("ChaosProbeClient reads status and probes with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createChaosProbeClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/status")) return jsonResponse({ pack_id: "yunque.pack.chaos-probe", stage: "pack-shell-before-scheduler", safe_probe_ready: true, scheduler_ready: false, degrade_engine_ready: false, alert_writeback_ready: false, probe_count: 1, report_count: 0, policy: {}, capabilities: [] });
      return jsonResponse({ probes: [{ id: "runtime-healthz-probe", name: "Runtime healthz probe", category: "network", description: "local", safe: true, enabled: true, interval_seconds: 30, weight: 1 }], count: 1 });
    },
  });

  const status = await client.status();
  const probes = await client.probes();

  assertEqual(status.pack_id, "yunque.pack.chaos-probe");
  assertEqual(probes.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/chaos-probe/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/chaos-probe/probes");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ChaosProbeClient saves probes, runs checks, and reads report detail", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createChaosProbeClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/probes") && init?.method === "POST") return jsonResponse({ probes: [], count: 0, status: "saved" }, { status: 201 });
      if (String(url).endsWith("/run")) return jsonResponse({ report: { id: "chaos-1", pack_id: "yunque.pack.chaos-probe", created_at: "now", stage: "pack-shell-before-scheduler", probe_count: 1, pass_count: 1, degraded_count: 0, fail_count: 0, health_score: 100, degrade_level: 0, gate_status: "pass", results: [] }, status: "dry_run" });
      return jsonResponse({ report: { id: "chaos-1", results: [] } });
    },
  });

  const saved = await client.saveProbes({ probes: [{ id: "runtime-healthz-probe", name: "Runtime healthz probe", category: "network", description: "local", safe: true, enabled: true, interval_seconds: 30, weight: 1 }], replace: true });
  const run = await client.run({ probe_ids: ["runtime-healthz-probe"], persist: false });
  const report = await client.report("chaos-1");

  assertEqual(saved.status, "saved");
  assertEqual(run.report.gate_status, "pass");
  assertEqual(report.report.id, "chaos-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/chaos-probe/probes");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/chaos-probe/run");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ probe_ids: ["runtime-healthz-probe"], persist: false }));
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/chaos-probe/reports/chaos-1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ChaosProbeClient lists reports and exports evidence packs", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createChaosProbeClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/reports")) return jsonResponse({ reports: [{ id: "chaos-1", created_at: "now", probe_count: 1, pass_count: 1, degraded_count: 0, fail_count: 0, health_score: 100, degrade_level: 0, gate_status: "pass" }], count: 1 });
      return jsonResponse({ pack_id: "yunque.pack.chaos-probe", exported_at: "now", format: "json-chaos-probe-evidence", files: ["chaos-report.json"], report: { id: "chaos-1", results: [] } });
    },
  });

  const reports = await client.reports();
  const evidence = await client.evidence("chaos-1");

  assertEqual(reports.count, 1);
  assertEqual(evidence.format, "json-chaos-probe-evidence");
  assertDeepEqual(evidence.files, ["chaos-report.json"]);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/chaos-probe/reports");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/chaos-probe/evidence/chaos-1");
});

test("ChaosProbeClient throws ChaosProbeClientError with nested gateway messages", async () => {
  const client = createChaosProbeClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "pack route is not enabled" }, { status: 404 }),
  });

  try {
    await client.status();
    throw new Error("expected status to reject");
  } catch (error) {
    assert(error instanceof ChaosProbeClientError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "pack route is not enabled" });
    assertEqual(error.message, "pack route is not enabled");
  }

  const nestedClient = createChaosProbeClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_PROBE", message: "probe id is invalid" } }, { status: 400 }),
  });

  try {
    await nestedClient.saveProbes({ probes: [] });
    throw new Error("expected saveProbes to reject");
  } catch (error) {
    assert(error instanceof ChaosProbeClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "probe id is invalid");
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
