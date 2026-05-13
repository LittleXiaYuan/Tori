import { createAgentKit } from "./agent-kit";
import { PluginApiClientError } from "./plugin-api";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }
async function collect<T>(iterable: AsyncIterable<T>): Promise<T[]> { const out: T[] = []; for await (const item of iterable) out.push(item); return out; }

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
      if (value.endsWith("/api/skillhub/installed")) return jsonResponse({ skills: [{ slug: "browser", version: "1.0.0" }], count: 1 });
      if (value.endsWith("/v1/plugins")) return jsonResponse({ plugins: [{ name: "demo", enabled: true }] });
      if (value.endsWith("/v1/plugins/ui")) return jsonResponse({ tabs: [{ id: "demo-tab" }] });
      if (value.endsWith("/v1/skills")) return jsonResponse({ skills: [{ name: "web.search", description: "search" }], count: 1 });
      if (value.endsWith("/v1/skills/scan")) return jsonResponse({ status: "scanned", skills_loaded: 2 });
      if (value.endsWith("/v1/skill-suggestions?session_id=s1")) return jsonResponse({ suggestions: [{ name: "summarize" }] });
      if (value.endsWith("/v1/skills/dynamic")) return jsonResponse({ skills: [{ name: "draft_doc", approval_status: "pending" }] });
      if (value.endsWith("/v1/workers")) return jsonResponse({ workers: [{ id: "w1", type: "cursor", capabilities: ["coding"] }], count: 1 });
      if (value.endsWith("/v1/orchestrator/status")) return jsonResponse({ running: true, adapters: ["cursor"], active_sessions: 1, event_count: 2, policy: { allow_auto_launch: true } });
      if (value.endsWith("/v1/fork/list?session_id=s1")) return jsonResponse({ forks: [{ id: "fork_1", session_id: "s1", messages: [], created_at: "2026-05-12T00:00:00Z" }] });
      if (value.endsWith("/v1/cost/summary")) return jsonResponse({ today_cost: 0.12, month_cost: 1.5 });
      if (value.endsWith("/api/providers")) return jsonResponse({ providers: [{ id: "deepseek", model: "deepseek-chat" }], mode: "hybrid" });
      if (value.endsWith("/v1/models")) return jsonResponse({ models: [{ id: "kimi", model_id: "moonshot-v1-8k" }] });
      if (value.endsWith("/v1/cognis")) return jsonResponse({ cognis: [{ id: "reviewer", name: "Code Reviewer" }], count: 1 });
      if (value.endsWith("/v1/trace/recent?limit=1")) return jsonResponse({ events: [{ trace_id: "tr-1" }], count: 1 });
      if (value.endsWith("/v1/heartbeat")) return jsonResponse({ running: true });
      if (value.endsWith("/v1/events/stream")) return new Response(new TextEncoder().encode("event: connected\ndata: {\"client_id\":\"sse-1\"}\n\n"), { status: 200, headers: { "Content-Type": "text/event-stream" } });
      if (value.endsWith("/v1/reverie/stats")) return jsonResponse({ total: 2, delivered: 1 });
      if (value.endsWith("/v1/chat")) return jsonResponse({ reply: "hello from chat" });
      if (value.endsWith("/v1/conversations")) return jsonResponse({ sessions: [{ id: "s1" }], count: 1 });
      if (value.endsWith("/v1/approvals?status=pending")) return jsonResponse({ approvals: [{ id: "ap1", status: "pending" }], total: 1 });
      if (value.endsWith("/v1/rbac/my-roles")) return jsonResponse({ subject_id: "u1", roles: [{ id: "operator", name: "Operator", permissions: [] }], total: 1 });
      if (value.endsWith("/api/files?path=artifacts")) return jsonResponse({ files: [{ name: "report.md", path: "artifacts/report.md", size: 12, is_dir: false }] });
      if (value.endsWith("/v1/browser/status")) return jsonResponse({ connected: true, state: "extension" });
      if (value.endsWith("/v1/sessions/queue")) return jsonResponse({ queues: { s1: 1 } });
      if (value.endsWith("/v1/subagent?parent_id=task-1")) return jsonResponse({ subagents: [{ id: "sa-1", name: "reviewer" }] });
      if (value.endsWith("/v1/tools/list")) return jsonResponse({ sessions: [{ id: "tool-1", command: "npm test", state: "running" }] });
      if (value.endsWith("/v1/audit/verify")) return jsonResponse({ valid: true, checked: 1 });
      if (value.endsWith("/api/trust/scores")) return jsonResponse({ scores: { shell: { score: 80 } }, count: 1 });
      if (value.endsWith("/api/skillgrow/patterns")) return jsonResponse({ patterns: [{ pattern: "retry_then_fix", count: 2 }], count: 1 });
      if (value.endsWith("/api/review/status")) return jsonResponse({ enabled: true, trust_enabled: true, distill_enabled: false });
      if (value.endsWith("/api/iterate/proposals?status=pending")) return jsonResponse({ proposals: [{ id: "it-1", status: "pending" }], count: 1 });
      if (value.endsWith("/v1/persona")) return jsonResponse({ identity: "Tori", soul: "careful", skills: [{ name: "review" }] });
      if (value.endsWith("/v1/persona/mode/current")) return jsonResponse({ mode: "coder", name: "Coder" });
      if (value.endsWith("/v1/emotion/history?session_id=s1&limit=1")) return jsonResponse({ entries: [{ emotion: "happy" }], total: 1 });
      if (value.endsWith("/v1/emotion/stickers")) return jsonResponse({ telegram: { happy: [{ package_id: "p1", sticker_id: "s1" }] } });
      if (value.endsWith("/v1/instructions?category=style")) return jsonResponse({ instructions: [{ instruction_id: "ins-1", content: "保持简洁" }], total: 1 });
      if (value.endsWith("/v1/react")) return jsonResponse({ status: "ok" });
      if (value.endsWith("/v1/rbac/check")) return jsonResponse({ allowed: true, subject_id: "u1", resource: "knowledge", action: "read" });
      if (value.endsWith("/v1/backup/info")) return jsonResponse({ file_count: 1, total_bytes: 12, files: { "memory.json": 12 } });
      if (value.endsWith("/v1/upload")) return jsonResponse({ filename: "note.txt", size: 4, path: "note.txt", parse: { status: "parsed" } });
      if (value.endsWith("/api/settings/check")) return jsonResponse({ setup_needed: false });
      if (value.endsWith("/healthz")) return jsonResponse({ status: "ok", version: "dev" });
      if (value.endsWith("/v1/auth/status")) return jsonResponse({ password_set: true, authenticated: true });
      if (value.endsWith("/v1/tasks?id=task-1")) return jsonResponse({ id: "task-1", status: "running" });
      if (value.endsWith("/v1/search?q=agent&limit=1")) return jsonResponse({ results: [{ title: "Discovery" }] });
      if (value.endsWith("/v1/search/providers")) return jsonResponse({ enabled: true, providers: ["local"] });
      if (value.endsWith("/v1/identity/profiles")) return jsonResponse({ profiles: [{ unified_id: "wechat:u1" }] });
      if (value.endsWith("/v1/embeddings")) return jsonResponse({ providers: ["local"] });
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
  assertEqual((await kit.skillhub.installed()).count, 1);
  assertEqual((await kit.plugins.list()).plugins[0]?.name, "demo");
  assertEqual(((await kit.pluginUI.ui()).tabs[0] as { id?: string })?.id, "demo-tab");
  assertEqual((await kit.skills.list()).skills[0]?.name, "web.search");
  assertEqual((await kit.skillsCatalog.list()).skills[0]?.name, "web.search");
  assertEqual((await kit.skillsScan.scan()).skills_loaded, 2);
  assertEqual((await kit.skillsSuggestions.suggestions("s1")).suggestions[0]?.name, "summarize");
  assertEqual((await kit.skillsDynamic.list()).skills[0]?.approval_status, "pending");
  assertEqual((await kit.dispatch.workers()).count, 1);
  assertEqual((await kit.orchestrator.status()).running, true);
  assertEqual((await kit.fork.list("s1")).forks[0]?.id, "fork_1");
  assertEqual((await kit.cost.summary()).today_cost, 0.12);
  assertEqual((await kit.providers.listProviders()).providers[0]?.id, "deepseek");
  assertEqual((await kit.models.listModels()).models[0]?.model_id, "moonshot-v1-8k");
  assertEqual((await kit.cognis.list()).cognis?.[0]?.id, "reviewer");
  assertEqual((await kit.trace.recent({ limit: 1 })).events[0]?.trace_id, "tr-1");
  assertEqual((await kit.heartbeat.status()).running, true);
  const [event] = await collect(kit.events.stream<{ client_id: string }>());
  assertEqual(event.data?.client_id, "sse-1");
  assertEqual((await kit.reverie.stats()).total, 2);
  assert(kit.realtime.wsUrl().startsWith("ws://localhost:9090/v1/ws?access_token=jwt-token"));
  assertEqual(kit.realtime.parse(JSON.stringify(kit.realtime.chat("你好", { session: "s1" }))).session, "s1");
  assertEqual((await kit.chat.send({ messages: [{ role: "user", content: "hi" }] })).reply, "hello from chat");
  assertEqual((await kit.conversations.list()).sessions[0]?.id, "s1");
  assertEqual((await kit.approvals.pending()).approvals[0]?.id, "ap1");
  assertEqual((await kit.rbac.myRoles()).roles[0]?.id, "operator");
  assertEqual((await kit.files.list("artifacts")).files[0]?.name, "report.md");
  assertEqual((await kit.browser.status()).connected, true);
  assertEqual((await kit.runtime.queues()).queues?.s1, 1);
  assertEqual((await kit.runtimeQueue.overview()).queues?.s1, 1);
  assertEqual((await kit.subagents.list("task-1")).subagents[0]?.id, "sa-1");
  assertEqual((await kit.tools.list()).sessions[0]?.id, "tool-1");
  assertEqual((await kit.audit.verify()).valid, true);
  assertEqual(((await kit.trust.scores()).scores.shell as { score?: number }).score, 80);
  assertEqual((await kit.skillgrow.patterns()).patterns[0]?.pattern, "retry_then_fix");
  assertEqual((await kit.review.status()).enabled, true);
  assertEqual((await kit.iterate.pendingProposals()).proposals[0]?.id, "it-1");
  assertEqual((await kit.persona.get()).identity, "Tori");
  assertEqual((await kit.modes.current()).mode, "coder");
  assertEqual((await kit.emotion.history({ sessionId: "s1", limit: 1 })).entries[0]?.emotion, "happy");
  assertEqual((await kit.interactions.stickers()).telegram?.happy?.[0]?.sticker_id, "s1");
  assertEqual((await kit.instructions.list("style")).instructions[0]?.instruction_id, "ins-1");
  assertEqual((await kit.reactions.react({ channel_type: "wechat", target: "u1", message_id: "m1", emoji: "👍" })).status, "ok");
  assertEqual((await kit.permissions.check({ subject_id: "u1", resource: "knowledge", action: "read" })).allowed, true);
  assertEqual((await kit.backup.info()).file_count, 1);
  assertEqual((await kit.upload.file(new Blob(["note"]), "note.txt")).parse?.status, "parsed");
  assertEqual((await kit.settings.check()).setup_needed, false);
  assertEqual((await kit.system.health()).status, "ok");
  assertEqual((await kit.auth.status()).authenticated, true);
  assertEqual((await kit.tasks.get("task-1")).status, "running");
  assertEqual(((await kit.discovery.search("agent", { limit: 1 })).results as Array<{ title?: string }>)[0]?.title, "Discovery");
  assertEqual((await kit.identity.profiles()).profiles[0]?.unified_id, "wechat:u1");
  assertEqual((await kit.embeddings.providers()).providers[0], "local");
  assertEqual((await kit.search.providers()).providers[0], "local");
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
  assertEqual(new Headers(calls[17]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[18]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[19]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[20]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[21]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[22]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[23]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[24]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[25]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[26]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[27]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[28]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[29]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[30]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[31]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[32]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[33]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[34]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[35]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[36]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[37]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[38]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[39]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[40]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[41]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[42]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[43]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[44]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[45]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[46]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[57]?.init?.headers).get("authorization"), "Bearer plugin-token");
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

