import { createWebChatWidgetClient, WebChatWidgetClientError } from "./webchat-widget";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertIncludes(actual: string, expected: string, message?: string): void { if (!actual.includes(expected)) throw new Error(message || `expected ${JSON.stringify(actual)} to include ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }

test("WebChatWidgetClient builds widget URL and fetches script", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWebChatWidgetClient({ baseUrl: "http://localhost:9090/", headers: { "X-Client": "desktop" }, fetch: async (url, init) => { calls.push({ url: String(url), init }); return new Response("(function(){/* widget */})();", { status: 200, headers: { "Content-Type": "application/javascript" } }); } });
  const script = await client.widgetScript("https://example.com");
  assertEqual(client.widgetUrl(), "http://localhost:9090/v1/webchat/widget.js"); assertIncludes(script, "widget"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/webchat/widget.js"); const headers = new Headers(calls[0]?.init?.headers); assertEqual(headers.get("origin"), "https://example.com"); assertEqual(headers.get("x-client"), "desktop");
});

test("WebChatWidgetClient exposes nested widget errors through alias", async () => {
  const client = createWebChatWidgetClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response(JSON.stringify({ error: { code: "BAD_REQUEST", message: "webchat origin is not allowed" } }), { status: 400, headers: { "Content-Type": "application/json" } }) });
  try { await client.widgetScript("https://blocked.example"); throw new Error("expected widgetScript to reject"); } catch (error) { assert(error instanceof WebChatWidgetClientError); assertEqual(error.name, "WebChatClientError"); assertEqual(error.status, 400); assertEqual(error.message, "webchat origin is not allowed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
