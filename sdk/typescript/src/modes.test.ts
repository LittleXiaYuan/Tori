import { createModesClient, ModesClientError } from "./modes";

declare const process: { exitCode?: number };

function assert(condition: unknown, message?: string): asserts condition {
  if (!condition) throw new Error(message || "assertion failed");
}

function assertEqual(actual: unknown, expected: unknown, message?: string): void {
  if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`);
}

function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void {
  const actualJson = JSON.stringify(actual);
  const expectedJson = JSON.stringify(expected);
  if (actualJson !== expectedJson) throw new Error(message || `expected ${actualJson} to deep equal ${expectedJson}`);
}

const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];

function test(name: string, fn: () => Promise<void> | void): void {
  tests.push({ name, fn });
}

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

test("ModesClient lists modes with auth and query scope", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createModesClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ modes: [{ mode: "coder", name: "Coder", active: true }], total: 1 });
    },
  });

  const result = await client.list({ tenant_id: "tenant-1", session_id: "session-1" });

  assertEqual(result.total, 1);
  assertEqual(result.modes[0]?.mode, "coder");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona/modes?tenant_id=tenant-1&session_id=session-1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ModesClient reads current mode with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createModesClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ mode: "researcher", name: "Researcher", features: ["search"] });
    },
  });

  const result = await client.current({ session_id: "session-2" });

  assertEqual(result.mode, "researcher");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona/mode/current?session_id=session-2");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ModesClient switches mode", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createModesClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ success: true, current_mode: "operator", modes: [{ mode: "operator", active: true }] });
    },
  });

  const result = await client.set({ tenant_id: "tenant-1", session_id: "session-3", mode: "operator" });

  assertEqual(result.success, true);
  assertEqual(result.current_mode, "operator");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona/mode");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ tenant_id: "tenant-1", session_id: "session-3", mode: "operator" }));
});

test("ModesClient throws ModesClientError with parsed body", async () => {
  const client = createModesClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "invalid mode", valid_modes: ["assistant"] }, { status: 400 }),
  });

  try {
    await client.set({ mode: "unknown" });
    throw new Error("expected set to reject");
  } catch (error) {
    assert(error instanceof ModesClientError);
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: "invalid mode", valid_modes: ["assistant"] });
    assertEqual(error.message, "invalid mode");
  }

  const nestedClient = createModesClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested invalid mode" }, valid_modes: ["assistant"] }, { status: 400 }),
  });

  try {
    await nestedClient.set({ mode: "unknown" });
    throw new Error("expected nested set to reject");
  } catch (error) {
    assert(error instanceof ModesClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "nested invalid mode");
  }
});

let failures = 0;
for (const { name, fn } of tests) {
  try {
    await fn();
    console.log(`ok - ${name}`);
  } catch (error) {
    failures += 1;
    console.error(`not ok - ${name}`);
    console.error(error);
  }
}

if (failures > 0) {
  process.exitCode = 1;
} else {
  console.log(`1..${tests.length}`);
}
