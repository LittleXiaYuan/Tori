import { afterEach, describe, expect, it, vi } from "vitest";
import { createSkillAnomalyPackClient } from "../skill-anomaly-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("skill-anomaly-pack-client", () => {
  it("reads Skill Anomaly pack status, profiles, and detail through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.skill-anomaly", stage: "pack-shell-before-audit-hook", detector_ready: true, audit_hook_plan_ready: true, audit_hook_ready: false, trust_mutation_plan_ready: true, trust_mutation_ready: false, approval_writeback_ready: true, approval_queue_store_ready: true, approval_manager_bridge_plan_ready: true, global_approval_enqueue_ready: false, approval_queue_store: { pack_id: "yunque.pack.skill-anomaly", queue_name: "skill_anomaly_approval", artifact: "approval-queue-store.json", record_count: 1 }, profile_count: 1, active_profiles: 1, anomaly_count: 0, policy: {}, capabilities: ["skill.approval_queue.writeback", "skill.approval_manager.bridge.plan"] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ profiles: [{ skill_slug: "text_processing", observed: 3, action_distrib: {}, param_key_set: {}, success_rate: 1, avg_duration_ms: 100, anomaly_count: 0, updated_at: "now" }], count: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ profile: { skill_slug: "text_processing", window_size: 100, observed: 3, action_distrib: {}, param_key_set: {}, success_rate: 1, avg_duration_ms: 100, anomaly_count: 0, updated_at: "now", recent: [] } }), { status: 200 }));

    const client = createSkillAnomalyPackClient();
    const status = await client.status();
    await client.profiles();
    await client.profile("text_processing");

    expect(status.approval_queue_store_ready).toBe(true);
    expect(status.approval_manager_bridge_plan_ready).toBe(true);
    expect(status.global_approval_enqueue_ready).toBe(false);
    expect(status.approval_queue_store?.artifact).toBe("approval-queue-store.json");
    expect(status.capabilities).toContain("skill.approval_queue.writeback");
    expect(status.capabilities).toContain("skill.approval_manager.bridge.plan");
    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/skill-anomaly/status",
      "/v1/skill-anomaly/profiles",
      "/v1/skill-anomaly/profiles/text_processing",
    ]);
  });

  it("observes, detects, plans audit hook write-back, persists pack-local approval queue records, and plans global approval bridge", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ event: { skill_slug: "text_processing" }, result: { score: 0 }, status: "observed" }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ result: { skill_slug: "text_processing", score: 7, severity: "needs_approval", needs_approval: true, block: true } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.skill-anomaly", skill_slug: "text_processing", status: "approval_plan", audit_hook_ready: false, trust_mutation_ready: false, approval_writeback_ready: false, approval_queue: { queue_name: "skill_anomaly_approval", request_id: "req-1", request_key: "req-key-1", queue_writeback_ready: false, writes_approval_queue: false, writes_queue_store: false, status: "blocked_until_approval_queue_writeback", store_artifact: "approval-queue-store.json" }, trust_mutation: { delta: -10 } } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ writeback: { pack_id: "yunque.pack.skill-anomaly", status: "approval_queue_written_pending_audit_trust_wiring", approval_writeback_ready: true, writes_approval_queue: true, writes_approval_queue_file: true, audit_hook_ready: false, trust_mutation_ready: false, merkle_append_ready: false, action_allowed: false, execution_blocked: true, request_id: "req-1", request_key: "req-key-1", approval_queue_store: { pack_id: "yunque.pack.skill-anomaly", queue_name: "skill_anomaly_approval", artifact: "approval-queue-store.json", record_count: 1 }, approval_queue_record: { request_id: "req-1", request_key: "req-key-1", store_artifact: "approval-queue-store.json", artifacts: ["approval-queue-store.json", "approval-queue-record.json"] }, artifacts: ["approval-queue-store.json", "approval-queue-record.json"], plan_summary: { status: "approval_plan" } } }), { status: 202 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.skill-anomaly", status: "bridge_plan_ready_from_pack_local_queue_record", approval_manager_bridge_plan_ready: true, global_approval_enqueue_ready: false, merkle_append_ready: false, trust_mutation_ready: false, action_release_ready: false, approval_queue_store_ready: true, source_queue_record_persisted: true, request_id: "req-1", request_key: "req-key-1", source_approval_queue_record: { request_id: "req-1", request_key: "req-key-1", store_artifact: "approval-queue-store.json" }, proposed_global_approval_request: { request_id: "req-1", request_key: "req-key-1", queue_name: "global_approval_manager", category: "code_execution", risk_level: "critical", summary: "Review", details: {}, requester: "operator", reason: "review", required_fields: ["request_id"], decision_states: ["pending", "approved", "denied", "expired"], approval_manager_enqueue_ready: false, global_approval_enqueue_ready: false, action_release_ready: false, source_store: "pack-local-json", source_artifact: "approval-queue-record.json", payload: {} }, artifacts: ["approval-manager-bridge-plan.json"], plan_summary: { status: "approval_plan" } } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ events: [{ skill_slug: "text_processing", action: "read_file" }], count: 1 }), { status: 200 }));

    const client = createSkillAnomalyPackClient();
    await client.observe({ skill_slug: "text_processing", action: "read_file", params: { path: "notes.md" }, success: true });
    await client.detect({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true });
    const plan = await client.auditHookPlan({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true, requested_by: "operator" });
    const writeback = await client.approvalQueueWriteback({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true, requested_by: "operator", request_id: "req-1", request_key: "req-key-1" });
    const bridgePlan = await client.approvalManagerBridgePlan({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true, requested_by: "operator", request_id: "req-1", request_key: "req-key-1" });
    await client.events({ skill_slug: "text_processing", limit: 10 });

    expect(plan.plan.status).toBe("approval_plan");
    expect(writeback.writeback.approval_writeback_ready).toBe(true);
    expect(writeback.writeback.writes_approval_queue).toBe(true);
    expect(writeback.writeback.writes_approval_queue_file).toBe(true);
    expect(writeback.writeback.audit_hook_ready).toBe(false);
    expect(writeback.writeback.trust_mutation_ready).toBe(false);
    expect(writeback.writeback.merkle_append_ready).toBe(false);
    expect(writeback.writeback.action_allowed).toBe(false);
    expect(writeback.writeback.execution_blocked).toBe(true);
    expect(writeback.writeback.approval_queue_store.artifact).toBe("approval-queue-store.json");
    expect(writeback.writeback.artifacts).toContain("approval-queue-record.json");
    expect(bridgePlan.plan.approval_manager_bridge_plan_ready).toBe(true);
    expect(bridgePlan.plan.global_approval_enqueue_ready).toBe(false);
    expect(bridgePlan.plan.proposed_global_approval_request.category).toBe("code_execution");
    expect(bridgePlan.plan.proposed_global_approval_request.global_approval_enqueue_ready).toBe(false);
    expect(bridgePlan.plan.artifacts).toContain("approval-manager-bridge-plan.json");
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/skill-anomaly/events");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ skill_slug: "text_processing", action: "read_file", params: { path: "notes.md" }, success: true });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/skill-anomaly/detect");
    expect((spy.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/skill-anomaly/audit-hook/plan");
    expect((spy.mock.calls[2]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[2]?.[1] as RequestInit).body))).toEqual({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true, requested_by: "operator" });
    expect(spy.mock.calls[3]?.[0]).toBe("/v1/skill-anomaly/approval-queue/writeback");
    expect((spy.mock.calls[3]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[3]?.[1] as RequestInit).body))).toEqual({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true, requested_by: "operator", request_id: "req-1", request_key: "req-key-1" });
    expect(spy.mock.calls[4]?.[0]).toBe("/v1/skill-anomaly/approval-queue/bridge/plan");
    expect((spy.mock.calls[4]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[4]?.[1] as RequestInit).body))).toEqual({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true, requested_by: "operator", request_id: "req-1", request_key: "req-key-1" });
    expect(spy.mock.calls[5]?.[0]).toBe("/v1/skill-anomaly/events?skill_slug=text_processing&limit=10");
  });

  it("exports JSON evidence packs by skill slug", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.skill-anomaly", exported_at: "now", format: "json-skill-anomaly-evidence", files: ["profile.json", "audit-hook-plan.json", "trust-mutation-plan.json", "approval-queue-store.json", "approval-queue-record.json", "approval-manager-bridge-plan.json"], profile: { skill_slug: "text_processing" }, events: [], policy: {}, audit_hook_plan: { status: "no_op" }, trust_mutation_plan: { delta: 0 }, approval_queue_store: { artifact: "approval-queue-store.json" }, approval_queue_record: { store_artifact: "approval-queue-store.json" }, approval_manager_bridge_plan: { approval_manager_bridge_plan_ready: true, global_approval_enqueue_ready: false } }), { status: 200 }));

    const client = createSkillAnomalyPackClient();
    const evidence = await client.evidence("text_processing");

    expect(evidence.files).toContain("audit-hook-plan.json");
    expect(evidence.files).toContain("trust-mutation-plan.json");
    expect(evidence.files).toContain("approval-queue-store.json");
    expect(evidence.files).toContain("approval-queue-record.json");
    expect(evidence.files).toContain("approval-manager-bridge-plan.json");
    expect(evidence.approval_queue_store?.artifact).toBe("approval-queue-store.json");
    expect(evidence.approval_manager_bridge_plan?.global_approval_enqueue_ready).toBe(false);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/skill-anomaly/evidence/text_processing");
  });
});
