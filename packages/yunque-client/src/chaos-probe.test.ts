import { ChaosProbeClientError, createChaosProbeClient } from "./chaos-probe";

declare const process: { exitCode?: number };

function assert(condition: unknown, message?: string): asserts condition {
  if (!condition) throw new Error(message || "assertion failed");
}

function assertEqual(
  actual: unknown,
  expected: unknown,
  message?: string,
): void {
  if (actual !== expected)
    throw new Error(
      message ||
        `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`,
    );
}

function assertDeepEqual(
  actual: unknown,
  expected: unknown,
  message?: string,
): void {
  const actualJson = JSON.stringify(actual);
  const expectedJson = JSON.stringify(expected);
  if (actualJson !== expectedJson)
    throw new Error(
      message || `expected ${actualJson} to deep equal ${expectedJson}`,
    );
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
      if (String(url).endsWith("/status"))
        return jsonResponse({
          pack_id: "yunque.pack.chaos-probe",
          stage: "pack-shell-before-scheduler",
          safe_probe_ready: true,
          scheduler_plan_ready: true,
          scheduler_ready: false,
          metrics_plan_ready: true,
          prometheus_ready: false,
          degrade_writeback_plan_ready: true,
          degrade_writeback_ready: true,
          degrade_state_store_ready: true,
          writes_degrade_state_store: true,
          degrade_engine_plan_ready: true,
          audit_append_plan_ready: true,
          merkle_append_ready: false,
          consumes_degrade_state_store: true,
          writes_runtime_degrade_state: false,
          runtime_degrade_state_ready: false,
          degrade_engine_ready: false,
          alert_writeback_plan_ready: true,
          alert_writeback_ready: false,
          probe_count: 1,
          report_count: 0,
          policy: {},
          capabilities: [],
        });
      return jsonResponse({
        probes: [
          {
            id: "runtime-healthz-probe",
            name: "Runtime healthz probe",
            category: "network",
            description: "local",
            safe: true,
            enabled: true,
            interval_seconds: 30,
            weight: 1,
          },
        ],
        count: 1,
      });
    },
  });

  const status = await client.status();
  const probes = await client.probes();

  assertEqual(status.pack_id, "yunque.pack.chaos-probe");
  assertEqual(probes.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/chaos-probe/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/chaos-probe/probes");
  assertEqual(
    new Headers(calls[0]?.init?.headers).get("authorization"),
    "Bearer token-123",
  );
});

test("ChaosProbeClient saves probes, runs checks, and reads report detail", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createChaosProbeClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/probes") && init?.method === "POST")
        return jsonResponse(
          { probes: [], count: 0, status: "saved" },
          { status: 201 },
        );
      if (String(url).endsWith("/run"))
        return jsonResponse({
          report: {
            id: "chaos-1",
            pack_id: "yunque.pack.chaos-probe",
            created_at: "now",
            stage: "pack-shell-before-scheduler",
            probe_count: 1,
            pass_count: 1,
            degraded_count: 0,
            fail_count: 0,
            health_score: 100,
            degrade_level: 0,
            gate_status: "pass",
            results: [],
          },
          status: "dry_run",
        });
      return jsonResponse({ report: { id: "chaos-1", results: [] } });
    },
  });

  const saved = await client.saveProbes({
    probes: [
      {
        id: "runtime-healthz-probe",
        name: "Runtime healthz probe",
        category: "network",
        description: "local",
        safe: true,
        enabled: true,
        interval_seconds: 30,
        weight: 1,
      },
    ],
    replace: true,
  });
  const run = await client.run({
    probe_ids: ["runtime-healthz-probe"],
    persist: false,
  });
  const report = await client.report("chaos-1");

  assertEqual(saved.status, "saved");
  assertEqual(run.report.gate_status, "pass");
  assertEqual(report.report.id, "chaos-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/chaos-probe/probes");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/chaos-probe/run");
  assertEqual(
    calls[1]?.init?.body,
    JSON.stringify({ probe_ids: ["runtime-healthz-probe"], persist: false }),
  );
  assertEqual(
    calls[2]?.url,
    "http://localhost:9090/v1/chaos-probe/reports/chaos-1",
  );
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ChaosProbeClient lists reports, plans scheduler, writes pack-local degrade state, plans runtime engine handoff, and exports evidence packs", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createChaosProbeClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/reports"))
        return jsonResponse({
          reports: [
            {
              id: "chaos-1",
              created_at: "now",
              probe_count: 1,
              pass_count: 1,
              degraded_count: 0,
              fail_count: 0,
              health_score: 100,
              degrade_level: 0,
              gate_status: "pass",
            },
          ],
          count: 1,
        });
      if (String(url).includes("/scheduler/plan"))
        return jsonResponse({
          plan: {
            pack_id: "yunque.pack.chaos-probe",
            generated_at: "now",
            status: "schedule_plan",
            report_id: "chaos-1",
            interval: "5m",
            scheduler_plan_ready: true,
            scheduler_ready: false,
            metrics_plan_ready: true,
            prometheus_ready: false,
            degrade_writeback_plan_ready: true,
            degrade_engine_ready: false,
            alert_writeback_plan_ready: true,
            alert_writeback_ready: false,
            health_score: 100,
            degrade_level: 0,
            gate_status: "pass",
            metrics: [],
            actions: [],
          },
        });
      if (String(url).includes("/degrade-state/writeback"))
        return jsonResponse({
          writeback: {
            pack_id: "yunque.pack.chaos-probe",
            generated_at: "now",
            status: "pack_local_degrade_state_written_pending_runtime_engine",
            report_id: "chaos-1",
            target: "runtime.degrade_state",
            level: 1,
            gate_status: "warn",
            health_score: 80,
            degrade_state_store_ready: true,
            degrade_writeback_plan_ready: true,
            degrade_writeback_ready: true,
            writes_degrade_state_store: true,
            runtime_degrade_state_ready: false,
            degrade_engine_ready: false,
            scheduler_ready: false,
            prometheus_ready: false,
            alert_writeback_ready: false,
            record_id: "chaos-degrade-1",
            record_key: "key",
            degrade_state_record: { record_id: "chaos-degrade-1" },
            degrade_state_store: { record_count: 1 },
            plan_summary: {},
            artifacts: ["degrade-state-store.json"],
            actions: [],
            labels: [],
          },
        });
      if (String(url).includes("/degrade-state/engine/plan"))
        return jsonResponse({
          plan: {
            pack_id: "yunque.pack.chaos-probe",
            generated_at: "now",
            status: "degrade_engine_handoff_plan",
            report_id: "chaos-1",
            record_id: "chaos-degrade-1",
            record_key: "key",
            target: "runtime.degrade_state",
            level: 1,
            gate_status: "warn",
            health_score: 80,
            degrade_engine_plan_ready: true,
            runtime_degrade_handoff_plan_ready: true,
            runtime_degrade_state_ready: false,
            degrade_engine_ready: false,
            audit_append_plan_ready: true,
            merkle_append_ready: false,
            consumes_degrade_state_store: true,
            writes_runtime_degrade_state: false,
            degrade_state_store_ready: true,
            degrade_writeback_ready: true,
            scheduler_ready: false,
            prometheus_ready: false,
            alert_writeback_ready: false,
            degrade_state_record: { record_id: "chaos-degrade-1" },
            degrade_state_store: { record_count: 1 },
            runtime_handoff_plan: { writes_runtime_degrade_state: false },
            audit_append_plan: { merkle_append_ready: false },
            artifacts: ["degrade-engine-plan.json"],
            actions: [],
            labels: [],
          },
        });
      return jsonResponse({
        pack_id: "yunque.pack.chaos-probe",
        exported_at: "now",
        format: "json-chaos-probe-evidence",
        files: ["chaos-report.json"],
        report: { id: "chaos-1", results: [] },
      });
    },
  });

  const reports = await client.reports();
  const plan = await client.schedulerPlan({
    report_id: "chaos-1",
    interval: "5m",
  });
  const writeback = await client.writeDegradeState({
    report_id: "chaos-1",
    requested_by: "unit",
  });
  const enginePlan = await client.degradeEnginePlan({
    report_id: "chaos-1",
    requested_by: "unit",
  });
  const evidence = await client.evidence("chaos-1");

  assertEqual(reports.count, 1);
  assertEqual(plan.plan.scheduler_ready, false);
  assertEqual(writeback.writeback.runtime_degrade_state_ready, false);
  assertEqual(enginePlan.plan.degrade_engine_plan_ready, true);
  assertEqual(enginePlan.plan.writes_runtime_degrade_state, false);
  assertEqual(evidence.format, "json-chaos-probe-evidence");
  assertDeepEqual(evidence.files, ["chaos-report.json"]);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/chaos-probe/reports");
  assertEqual(
    calls[1]?.url,
    "http://localhost:9090/v1/chaos-probe/scheduler/plan",
  );
  assertEqual(
    calls[1]?.init?.body,
    JSON.stringify({ report_id: "chaos-1", interval: "5m" }),
  );
  assertEqual(
    calls[2]?.url,
    "http://localhost:9090/v1/chaos-probe/degrade-state/writeback",
  );
  assertEqual(
    calls[2]?.init?.body,
    JSON.stringify({ report_id: "chaos-1", requested_by: "unit" }),
  );
  assertEqual(
    calls[3]?.url,
    "http://localhost:9090/v1/chaos-probe/degrade-state/engine/plan",
  );
  assertEqual(
    calls[3]?.init?.body,
    JSON.stringify({ report_id: "chaos-1", requested_by: "unit" }),
  );
  assertEqual(
    calls[4]?.url,
    "http://localhost:9090/v1/chaos-probe/evidence/chaos-1",
  );
});

test("ChaosProbeClient throws ChaosProbeClientError with nested gateway messages", async () => {
  const client = createChaosProbeClient({
    baseUrl: "http://localhost:9090",
    fetch: async () =>
      jsonResponse({ error: "pack route is not enabled" }, { status: 404 }),
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
    fetch: async () =>
      jsonResponse(
        { error: { code: "BAD_PROBE", message: "probe id is invalid" } },
        { status: 400 },
      ),
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
