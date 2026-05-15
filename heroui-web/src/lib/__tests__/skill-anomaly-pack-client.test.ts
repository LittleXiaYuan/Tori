import { afterEach, describe, expect, it, vi } from "vitest";
import { createSkillAnomalyPackClient } from "../skill-anomaly-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("skill-anomaly-pack-client", () => {
  it("reads Skill Anomaly pack status, profiles, and detail through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.skill-anomaly", stage: "pack-shell-before-audit-hook", detector_ready: true, audit_hook_plan_ready: true, audit_hook_ready: false, trust_mutation_plan_ready: true, trust_mutation_ready: false, approval_writeback_ready: false, profile_count: 1, active_profiles: 1, anomaly_count: 0, policy: {}, capabilities: [] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ profiles: [{ skill_slug: "text_processing", observed: 3, action_distrib: {}, param_key_set: {}, success_rate: 1, avg_duration_ms: 100, anomaly_count: 0, updated_at: "now" }], count: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ profile: { skill_slug: "text_processing", window_size: 100, observed: 3, action_distrib: {}, param_key_set: {}, success_rate: 1, avg_duration_ms: 100, anomaly_count: 0, updated_at: "now", recent: [] } }), { status: 200 }));

    const client = createSkillAnomalyPackClient();
    await client.status();
    await client.profiles();
    await client.profile("text_processing");

    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/skill-anomaly/status",
      "/v1/skill-anomaly/profiles",
      "/v1/skill-anomaly/profiles/text_processing",
    ]);
  });

  it("observes, detects, and plans audit hook write-back with method-aware payloads", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ event: { skill_slug: "text_processing" }, result: { score: 0 }, status: "observed" }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ result: { skill_slug: "text_processing", score: 7, severity: "needs_approval", needs_approval: true, block: true } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.skill-anomaly", skill_slug: "text_processing", status: "approval_plan", audit_hook_ready: false, trust_mutation_ready: false, approval_writeback_ready: false, trust_mutation: { delta: -10 } } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ events: [{ skill_slug: "text_processing", action: "read_file" }], count: 1 }), { status: 200 }));

    const client = createSkillAnomalyPackClient();
    await client.observe({ skill_slug: "text_processing", action: "read_file", params: { path: "notes.md" }, success: true });
    await client.detect({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true });
    const plan = await client.auditHookPlan({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true, requested_by: "operator" });
    await client.events({ skill_slug: "text_processing", limit: 10 });

    expect(plan.plan.status).toBe("approval_plan");
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/skill-anomaly/events");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ skill_slug: "text_processing", action: "read_file", params: { path: "notes.md" }, success: true });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/skill-anomaly/detect");
    expect((spy.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/skill-anomaly/audit-hook/plan");
    expect((spy.mock.calls[2]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[2]?.[1] as RequestInit).body))).toEqual({ skill_slug: "text_processing", action: "shell_exec", params: { command: "whoami" }, dry_run: true, requested_by: "operator" });
    expect(spy.mock.calls[3]?.[0]).toBe("/v1/skill-anomaly/events?skill_slug=text_processing&limit=10");
  });

  it("exports JSON evidence packs by skill slug", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.skill-anomaly", exported_at: "now", format: "json-skill-anomaly-evidence", files: ["profile.json", "audit-hook-plan.json", "trust-mutation-plan.json"], profile: { skill_slug: "text_processing" }, events: [], policy: {}, audit_hook_plan: { status: "no_op" }, trust_mutation_plan: { delta: 0 } }), { status: 200 }));

    const client = createSkillAnomalyPackClient();
    const evidence = await client.evidence("text_processing");

    expect(evidence.files).toContain("audit-hook-plan.json");
    expect(evidence.files).toContain("trust-mutation-plan.json");
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/skill-anomaly/evidence/text_processing");
  });
});
