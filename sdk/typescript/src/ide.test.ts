import { createIDEClient, IDEClientError } from "./ide";

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

test("IDEClient reads status with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIDEClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ version: "0.1.0", connected: true, capabilities: ["review", "sse"], skills_count: 3 });
    },
  });

  const result = await client.status();

  assertEqual(result.connected, true);
  assertEqual(result.capabilities?.[0], "review");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/ide/status");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("IDEClient reviews code with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIDEClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ summary: "代码可读", issues: [], score: 9, improvements: ["补测试"] });
    },
  });

  const result = await client.review({
    file_path: "main.go",
    content: "package main\nfunc main(){}",
    language: "go",
    mode: "full",
  });

  assertEqual(result.score, 9);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/ide/review");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
  assertEqual(
    calls[0]?.init?.body,
    JSON.stringify({ file_path: "main.go", content: "package main\nfunc main(){}", language: "go", mode: "full" }),
  );
});

test("IDEClient provides review helpers", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIDEClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ summary: "ok", issues: [], score: 8 });
    },
  });

  await client.reviewDiff({ file_path: "main.go", diff: "+fmt.Println(1)", language: "go" });
  await client.reviewQuick({ file_path: "main.ts", content: "console.log(1)", language: "ts" });
  await client.reviewFull({ file_path: "main.py", content: "print(1)", language: "py" });

  assertEqual(JSON.parse(String(calls[0]?.init?.body)).mode, "diff");
  assertEqual(JSON.parse(String(calls[1]?.init?.body)).mode, "quick");
  assertEqual(JSON.parse(String(calls[2]?.init?.body)).mode, "full");
});

test("IDEClient throws IDEClientError with text body", async () => {
  const client = createIDEClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => new Response("content or diff required", { status: 400 }),
  });

  try {
    await client.review({});
    throw new Error("expected review to reject");
  } catch (error) {
    assert(error instanceof IDEClientError);
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, "content or diff required");
    assertEqual(error.message, "content or diff required");
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
