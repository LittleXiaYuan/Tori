import { createMemoryAddClient, MemoryAddClientError } from "./memory-add";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("MemoryAddClient adds memory with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryAddClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ status: "ok" }, { status: 201 });
    },
  });

  const result = await client.add({ layer: "long", content: "用户偏好中文回复", source: "chat" });

  assertEqual(result.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory/add");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ layer: "long", content: "用户偏好中文回复", source: "chat", value: "用户偏好中文回复" }));
});

test("MemoryAddClient remember offers compact write API with API key auth", async () => {
  const client = createMemoryAddClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (_url, init) => {
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      assertEqual(init?.body, JSON.stringify({ layer: "mid", source: "plugin", tags: ["preference"], content: "喜欢短回答", value: "喜欢短回答" }));
      return jsonResponse({ status: "stored" });
    },
  });

  const result = await client.remember("喜欢短回答", { layer: "mid", source: "plugin", tags: ["preference"] });

  assertEqual(result.status, "stored");
});

test("MemoryAddClient exposes memory-add-named nested gateway errors", async () => {
  const client = createMemoryAddClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested memory add failure" } }, { status: 400 }),
  });

  try {
    await client.remember("");
    throw new Error("expected remember to reject");
  } catch (error) {
    assert(error instanceof MemoryAddClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "nested memory add failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
