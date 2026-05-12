import { createAgentKit } from "./agent-kit";
import { PluginApiClientError } from "./plugin-api";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("createAgentKit composes state reflect mission parse scheduler and plugin lightweight clients", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const kit = createAgentKit({
    baseUrl: "http://localhost:9090/",
    token: "jwt-token",
    pluginToken: "plugin-token",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      const value = String(url);
      if (value.endsWith("/v1/state/focus")) return jsonResponse({ focus: "sdk" });
      if (value.includes("/v1/reflect/strategies")) return jsonResponse({ strategies: "- keep slices small" });
      if (value.endsWith("/v1/missions/parse")) return jsonResponse({ type: "cron", name: "每日总结", description: "每天总结", config: { cron_expr: "0 8 * * *" }, confidence: 0.9, explanation: "mentions daily schedule" });
      if (value.endsWith("/v1/scheduler/jobs")) return jsonResponse({ jobs: [{ id: "job_1", name: "daily" }], count: 1 });
      if (value.endsWith("/v1/memory/search")) return jsonResponse({ results: [{ key: "pref", value: "喜欢中文" }], count: 1 });
      if (value.endsWith("/v1/graph/stats")) return jsonResponse({ entities: 2, relations: 1 });
      if (value.endsWith("/v1/knowledge/stats")) return jsonResponse({ sources: 2, chunks: 8 });
      if (value.endsWith("/v1/lora/status")) return jsonResponse({ active_model: "adapter-a", rolling_success_rate: 0.8 });
      if (value.endsWith("/v1/workflows")) return jsonResponse({ workflows: [{ id: "wf_1", name: "SDK flow" }], total: 1 });
      if (value.endsWith("/api/connectors")) return jsonResponse({ connectors: [{ id: "github", name: "GitHub", supported: true, status: "connected" }] });
      if (value.endsWith("/api/notify/channels")) return jsonResponse({ channels: [{ id: "feishu-main", type: "feishu", name: "Feishu", enabled: true }] });
      if (value.endsWith("/v1/projects")) return jsonResponse({ projects: [{ id: "p1", name: "云雀", repo_path: "C:/Code/AI/云雀/yunque-agent" }] });
      if (value.endsWith("/v1/market/stats")) return jsonResponse({ total: 3, categories: { coding: 1 } });
      if (value.endsWith("/v1/workers")) return jsonResponse({ workers: [{ id: "w1", type: "cursor", capabilities: ["coding"] }], count: 1 });
      if (value.endsWith("/v1/orchestrator/status")) return jsonResponse({ running: true, adapters: ["cursor"], active_sessions: 1, event_count: 2, policy: { allow_auto_launch: true } });
      if (value.endsWith("/v1/fork/list?session_id=s1")) return jsonResponse({ forks: [{ id: "fork_1", session_id: "s1", messages: [], created_at: "2026-05-12T00:00:00Z" }] });
      if (value.endsWith("/v1/cost/summary")) return jsonResponse({ today_cost: 0.12, month_cost: 1.5 });
      if (value.includes("/v1/plugin-api/search")) return jsonResponse({ results: [{ title: "SDK" }] });
      return jsonResponse({ ok: true });
    },
  });

  assertEqual((await kit.state.focus()).focus, "sdk");
  assert((await kit.reflect.strategies({ tag: "sdk" })).strategies.includes("slices"));
  assertEqual((await kit.missions.parse("每天八点总结昨天的任务")).type, "cron");
  assertEqual((await kit.scheduler.jobs()).count, 1);
  assertEqual((await kit.memory.search({ query: "中文", limit: 1 })).count, 1);
  assertEqual((await kit.graph.stats()).entities, 2);
  assertEqual((await kit.knowledge.stats()).sources, 2);
  assertEqual((await kit.lora.status()).active_model, "adapter-a");
  assertEqual((await kit.workflows.list()).total, 1);
  assertEqual((await kit.connectors.list()).connectors[0]?.id, "github");
  assertEqual((await kit.notify.channels()).channels[0]?.id, "feishu-main");
  assertEqual((await kit.projects.list()).projects[0]?.id, "p1");
  assertEqual((await kit.market.stats()).total, 3);
  assertEqual((await kit.dispatch.workers()).count, 1);
  assertEqual((await kit.orchestrator.status()).running, true);
  assertEqual((await kit.fork.list("s1")).forks[0]?.id, "fork_1");
  assertEqual((await kit.cost.summary()).today_cost, 0.12);
  assertEqual((await kit.plugin.search("sdk", 3)).results.length, 1);
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[2]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[3]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[4]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[5]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[6]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[7]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[8]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[9]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[10]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[11]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[12]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[13]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[14]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[15]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[16]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[17]?.init?.headers).get("authorization"), "Bearer plugin-token");
});

test("createAgentKit can reuse token as plugin token for simple automations", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const kit = createAgentKit({ baseUrl: "http://localhost:9090", token: "shared-token", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ extensions: [] }); } });

  assertEqual((await kit.plugin.extensions()).extensions.length, 0);
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer shared-token");
});

test("createAgentKit requires a token for plugin runtime helpers", () => {
  try {
    createAgentKit({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({}) });
    throw new Error("expected createAgentKit to reject missing token");
  } catch (error) {
    assert(error instanceof Error);
    assertEqual(error.message, "createAgentKit requires pluginToken or token for Plugin API access");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);

