import { afterEach, describe, expect, it, vi } from "vitest";
import { createChaosProbePackClient } from "../chaos-probe-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("chaos-probe-pack-client", () => {
  it("reads Chaos Probe status, definitions, and reports through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
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
            report_count: 1,
            policy: {},
            capabilities: [],
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
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
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
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
          }),
          { status: 200 },
        ),
      );

    const client = createChaosProbePackClient();
    await client.status();
    await client.probes();
    await client.reports();

    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/chaos-probe/status",
      "/v1/chaos-probe/probes",
      "/v1/chaos-probe/reports",
    ]);
  });

  it("saves definitions and runs probes with method-aware payloads", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ probes: [], count: 0, status: "saved" }),
          { status: 201 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            report: {
              id: "chaos-1",
              gate_status: "pass",
              health_score: 100,
              results: [],
            },
            status: "dry_run",
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ report: { id: "chaos-1", results: [] } }),
          { status: 200 },
        ),
      );

    const client = createChaosProbePackClient();
    await client.saveProbes({
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
    await client.run({ probe_ids: ["runtime-healthz-probe"], persist: false });
    await client.report("chaos-1");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/chaos-probe/probes");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(
      JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body)),
    ).toEqual({
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
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/chaos-probe/run");
    expect((spy.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/chaos-probe/reports/chaos-1");
  });

  it("plans scheduler write-back, writes pack-local degrade state, plans runtime engine handoff, and exports JSON evidence packs by report id", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            plan: {
              pack_id: "yunque.pack.chaos-probe",
              status: "schedule_plan",
              interval: "5m",
              scheduler_plan_ready: true,
              scheduler_ready: false,
              metrics: [],
              actions: [],
            },
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
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
          }),
          { status: 202 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
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
              runtime_handoff_plan: {
                writes_runtime_degrade_state: false,
              },
              audit_append_plan: { merkle_append_ready: false },
              artifacts: ["degrade-engine-plan.json"],
              actions: [],
              labels: [],
            },
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            pack_id: "yunque.pack.chaos-probe",
            exported_at: "now",
            format: "json-chaos-probe-evidence",
            files: [
              "chaos-report.json",
              "scheduler-plan.json",
              "degrade-state-store.json",
              "degrade-state-record.json",
              "degrade-engine-plan.json",
              "runtime-degrade-handoff-plan.json",
              "audit-append-plan.json",
            ],
            report: { id: "chaos-1", results: [] },
          }),
          { status: 200 },
        ),
      );

    const client = createChaosProbePackClient();
    await client.schedulerPlan({ report_id: "chaos-1", interval: "5m" });
    await client.writeDegradeState({
      report_id: "chaos-1",
      requested_by: "unit",
    });
    await client.degradeEnginePlan({
      report_id: "chaos-1",
      requested_by: "unit",
    });
    await client.evidence("chaos-1");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/chaos-probe/scheduler/plan");
    expect(
      JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body)),
    ).toEqual({ report_id: "chaos-1", interval: "5m" });
    expect(spy.mock.calls[1]?.[0]).toBe(
      "/v1/chaos-probe/degrade-state/writeback",
    );
    expect(
      JSON.parse(String((spy.mock.calls[1]?.[1] as RequestInit).body)),
    ).toEqual({ report_id: "chaos-1", requested_by: "unit" });
    expect(spy.mock.calls[2]?.[0]).toBe(
      "/v1/chaos-probe/degrade-state/engine/plan",
    );
    expect(
      JSON.parse(String((spy.mock.calls[2]?.[1] as RequestInit).body)),
    ).toEqual({ report_id: "chaos-1", requested_by: "unit" });
    expect(spy.mock.calls[3]?.[0]).toBe("/v1/chaos-probe/evidence/chaos-1");
  });
});
