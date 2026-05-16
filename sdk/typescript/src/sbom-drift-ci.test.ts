import { createSBOMDriftCIClient } from "./sbom-drift-ci";

declare const process: { exitCode?: number };

function assertEqual(actual: unknown, expected: unknown, message?: string): void {
  if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`);
}

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

let failed = false;
try {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSBOMDriftCIClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ plan: {
        status: "ci_workflow_writeback_plan_ready_pending_ci_writer",
        ci_workflow_writeback_plan_ready: true,
        consumes_ci_baseline_store: true,
        writes_ci_workflow: false,
        executes_govulncheck: false,
        blocks_release: false,
        ci_workflow_handoff_plan: { workflow_path: ".github/workflows/sbom-drift.yml", job_name: "sbom-drift-gate", steps: [] },
        release_blocker_plan: { would_block: true, blocks_release: false },
        artifacts: ["ci-workflow-writeback-plan.json"],
      } });
    },
  });

  const result = await client.workflowWritebackPlan({ request_key: "sbom-baseline" });

  assertEqual(result.plan.ci_workflow_writeback_plan_ready, true);
  assertEqual(result.plan.writes_ci_workflow, false);
  assertEqual(result.plan.release_blocker_plan.blocks_release, false);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/sbom-drift/ci-gate/workflow/writeback/plan");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ request_key: "sbom-baseline" }));
  console.log("ok - SBOM Drift CI workflow handoff helper");
} catch (error) {
  failed = true;
  console.error("not ok - SBOM Drift CI workflow handoff helper");
  console.error(error);
}

if (failed) process.exitCode = 1;
