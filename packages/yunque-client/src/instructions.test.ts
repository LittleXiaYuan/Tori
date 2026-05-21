import { createInstructionsClient, InstructionsClientError } from "./instructions";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("InstructionsClient lists and creates user instructions through instructions facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createInstructionsClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ instruction_id: "ins-1", content: "保持简洁" }); return jsonResponse({ instructions: [{ instruction_id: "ins-1", content: "保持简洁" }], total: 1 }); } });

  const list = await client.list("style");
  const created = await client.create({ category: "style", content: "保持简洁" });

  assertEqual(list.total, 1);
  assertEqual(created.instruction_id, "ins-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/instructions?category=style");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/instructions");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ category: "style", content: "保持简洁" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("InstructionsClient updates deletes and reorders without generated SDK", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createInstructionsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "updated" }); } });

  await client.update({ instruction_id: "ins-1", content: "更新" });
  await client.delete("ins-1");
  await client.reorder(["ins-2", "ins-1"]);

  assertEqual(calls[0]?.init?.method, "PUT");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/instructions?id=ins-1");
  assertEqual(calls[1]?.init?.method, "DELETE");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/instructions/reorder");
  assertEqual(calls[2]?.init?.body, JSON.stringify({ ids: ["ins-2", "ins-1"] }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("InstructionsClient exposes instructions-named errors", async () => {
  const client = createInstructionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "instruction rejected" } }, { status: 400 }) });

  try {
    await client.list();
    throw new Error("expected list to reject");
  } catch (error) {
    assert(error instanceof InstructionsClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "instruction rejected");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
