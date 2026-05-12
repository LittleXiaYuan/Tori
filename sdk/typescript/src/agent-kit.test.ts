import { createAgentKit } from "./agent-kit";
import { PluginApiClientError } from "./plugin-api";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("createAgentKit composes state reflect and plugin lightweight clients", async () => {
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
      if (value.includes("/v1/plugin-api/search")) return jsonResponse({ results: [{ title: "SDK" }] });
      return jsonResponse({ ok: true });
    },
  });

  assertEqual((await kit.state.focus()).focus, "sdk");
  assert((await kit.reflect.strategies({ tag: "sdk" })).strategies.includes("slices"));
  assertEqual((await kit.plugin.search("sdk", 3)).results.length, 1);
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt-token");
  assertEqual(new Headers(calls[2]?.init?.headers).get("authorization"), "Bearer plugin-token");
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

