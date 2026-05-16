import { afterEach, describe, expect, it, vi } from "vitest";
import { createSBOMDriftPackClient } from "../sbom-drift-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("sbom-drift-pack-client", () => {
  it("reads SBOM Drift pack status and snapshots through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.sbom-drift", stage: "pack-shell-before-ci", scanner_ready: true, cyclonedx_ready: true, ci_gate_plan_ready: true, ci_baseline_store_ready: true, ci_baseline_writeback_ready: true, ci_workflow_writeback_plan_ready: true, ci_workflow_writeback_ready: false, consumes_ci_baseline_store: false, ci_gate_ready: false, vulnerability_ready: false, govulncheck_plan_ready: true, govulncheck_ready: false, writes_ci_baseline_store: false, writes_ci_workflow: false, snapshot_count: 1, capabilities: ["sbom.govulncheck.plan", "sbom.ci_baseline.writeback", "sbom.ci_workflow.writeback_plan"] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ snapshots: [{ id: "baseline", source: "unit", created_at: "now", component_count: 1, ecosystems: { gomod: 1 } }], count: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ snapshot: { id: "baseline", source: "unit", created_at: "now", component_count: 1, ecosystems: { gomod: 1 }, components: [] } }), { status: 200 }));

    const client = createSBOMDriftPackClient();
    const status = await client.status();
    await client.snapshots();
    await client.snapshot("baseline");

    expect(status.govulncheck_plan_ready).toBe(true);
    expect(status.ci_baseline_writeback_ready).toBe(true);
    expect(status.ci_workflow_writeback_plan_ready).toBe(true);
    expect(status.govulncheck_ready).toBe(false);
    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/sbom-drift/status",
      "/v1/sbom-drift/snapshots",
      "/v1/sbom-drift/snapshots/baseline",
    ]);
  });

  it("creates snapshots and diffs with method-aware payloads", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ snapshot: { id: "baseline", components: [] }, status: "created" }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ diff: { base: { id: "baseline" }, target: { id: "current" }, added: [], removed: [], changed: [], risk_level: "none" } }), { status: 200 }));

    const client = createSBOMDriftPackClient();
    await client.createSnapshot({ id: "baseline", source: "manual" });
    await client.diff({ base_id: "baseline", target_current: true });

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/sbom-drift/snapshots");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ id: "baseline", source: "manual" });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/sbom-drift/diff");
    expect(JSON.parse(String((spy.mock.calls[1]?.[1] as RequestInit).body))).toEqual({ base_id: "baseline", target_current: true });
  });

  it("exports JSON evidence packs by snapshot id", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ bom: { bomFormat: "CycloneDX", specVersion: "1.5", version: 1, metadata: {}, components: [] }, snapshot: { id: "baseline" } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.sbom-drift", blocked: false, ci_gate_plan_ready: true, ci_gate_ready: false, govulncheck_plan_ready: true, govulncheck_ready: false, govulncheck_plan: { plan_ready: true, ready: false, command: "govulncheck -json ./...", report_artifact: "govulncheck-report.json", executes: false, writes_files: false, package_count: 1, module_count: 1, packages: [] }, artifacts: ["dist/sbom.cdx.json", "govulncheck-plan.json"], commands: [], actions: [], diff: { risk_level: "none" } } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.sbom-drift", exported_at: "now", format: "json-sbom-drift-evidence", files: ["snapshot.json", "sbom.cdx.json", "ci-gate-plan.json", "govulncheck-plan.json"], snapshot: { id: "baseline" }, govulncheck_plan: { writes_files: false } }), { status: 200 }));

    const client = createSBOMDriftPackClient();
    await client.cycloneDX("baseline");
    const plan = await client.ciGatePlan({ base_id: "baseline", target_current: true, fail_on_risk: "high" });
    const evidence = await client.evidence("baseline");

    expect(plan.plan.govulncheck_plan_ready).toBe(true);
    expect(plan.plan.govulncheck_plan.writes_files).toBe(false);
    expect(evidence.files).toContain("govulncheck-plan.json");
    expect(evidence.govulncheck_plan?.writes_files).toBe(false);
    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/sbom-drift/cyclonedx/baseline",
      "/v1/sbom-drift/ci-gate/plan",
      "/v1/sbom-drift/evidence/baseline",
    ]);
    expect(JSON.parse(String((spy.mock.calls[1]?.[1] as RequestInit).body))).toEqual({ base_id: "baseline", target_current: true, fail_on_risk: "high" });
  });

  it("writes pack-local CI baseline gate handoff records without executing scanners", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ writeback: {
        pack_id: "yunque.pack.sbom-drift",
        status: "ci_baseline_gate_record_stored_pending_ci_wiring",
        request_key: "sbom-baseline",
        ci_baseline_store_ready: true,
        ci_baseline_writeback_ready: true,
        writes_ci_baseline_store: true,
        ci_gate_plan_ready: true,
        ci_gate_ready: false,
        govulncheck_plan_ready: true,
        govulncheck_ready: false,
        vulnerability_ready: false,
        writes_ci_workflow: false,
        executes_govulncheck: false,
        blocks_release: false,
        ci_baseline_store: { record_count: 1 },
        ci_baseline_record: { request_key: "sbom-baseline" },
        artifacts: ["ci-baseline-store.json", "ci-baseline-record.json"],
      } }), { status: 200 }));

    const client = createSBOMDriftPackClient();
    const writeback = await client.ciBaselineWriteback({ base_id: "baseline", target_current: true, fail_on_risk: "high", request_key: "sbom-baseline" });

    expect(writeback.writeback.ci_baseline_writeback_ready).toBe(true);
    expect(writeback.writeback.writes_ci_baseline_store).toBe(true);
    expect(writeback.writeback.writes_ci_workflow).toBe(false);
    expect(writeback.writeback.executes_govulncheck).toBe(false);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/sbom-drift/ci-gate/baseline/writeback");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ base_id: "baseline", target_current: true, fail_on_risk: "high", request_key: "sbom-baseline" });
  });

  it("plans CI workflow write-back handoffs without mutating workflow files", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: {
        pack_id: "yunque.pack.sbom-drift",
        status: "ci_workflow_writeback_plan_ready_pending_ci_writer",
        stage: "workflow-plan-before-ci-write",
        request_key: "sbom-baseline",
        ci_baseline_store_ready: true,
        ci_baseline_writeback_ready: true,
        ci_workflow_writeback_plan_ready: true,
        ci_workflow_writeback_ready: false,
        consumes_ci_baseline_store: true,
        ci_gate_plan_ready: true,
        ci_gate_ready: false,
        govulncheck_plan_ready: true,
        govulncheck_ready: false,
        vulnerability_ready: false,
        writes_ci_baseline_store: false,
        writes_ci_workflow: false,
        executes_govulncheck: false,
        blocks_release: false,
        ci_workflow_handoff_plan: { workflow_path: ".github/workflows/security.yml", job_name: "sbom-drift-gate", consumes_ci_baseline_store: true, steps: [{ name: "govulncheck-plan", writes_files: false, executes_govulncheck: false }] },
        release_blocker_plan: { would_block: true, blocks_release: false },
        artifacts: ["ci-workflow-writeback-plan.json", "ci-workflow-handoff-plan.json", "release-blocker-plan.json"],
      } }), { status: 200 }));

    const client = createSBOMDriftPackClient();
    const plan = await client.ciWorkflowWritebackPlan({ request_key: "sbom-baseline", workflow_path: ".github/workflows/security.yml" });

    expect(plan.plan.ci_workflow_writeback_plan_ready).toBe(true);
    expect(plan.plan.consumes_ci_baseline_store).toBe(true);
    expect(plan.plan.writes_ci_workflow).toBe(false);
    expect(plan.plan.executes_govulncheck).toBe(false);
    expect(plan.plan.release_blocker_plan.blocks_release).toBe(false);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/sbom-drift/ci-gate/workflow/writeback/plan");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ request_key: "sbom-baseline", workflow_path: ".github/workflows/security.yml" });
  });
});
