import { createTrustClient, TrustClientError } from "./trust";

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

test("TrustClient reads scores with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTrustClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ scores: { shell: { score: 80, level: "review" } }, count: 1 });
    },
  });

  const result = await client.scores();

  assertEqual(result.count, 1);
  assertEqual((result.scores.shell as { score?: number }).score, 80);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/trust/scores");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("TrustClient resets and grants trust with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTrustClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/reset")) return jsonResponse({ status: "reset", slug: "shell" });
      return jsonResponse({ status: "granted", slug: "shell", level: "shell" });
    },
  });

  const reset = await client.reset({ slug: "shell" });
  const grant = await client.grant({ slug: "shell" });

  assertEqual(reset.status, "reset");
  assertEqual(grant.status, "granted");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/trust/reset");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ slug: "shell" }));
  assertEqual(calls[1]?.url, "http://localhost:9090/api/trust/grant");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TrustClient grants all skills", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTrustClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ status: "granted_all", upgraded: 3 });
    },
  });

  const result = await client.grantAll();

  assertEqual(result.status, "granted_all");
  assertEqual(result.upgraded, 3);
  assertEqual(calls[0]?.init?.body, JSON.stringify({ slug: "*" }));
});

test("TrustClient reads review status and skill growth patterns", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTrustClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).includes("review/status")) {
        return jsonResponse({ enabled: true, trust_enabled: true, distill_enabled: false });
      }
      return jsonResponse({ patterns: [{ pattern: "repeat-review", count: 4 }], count: 1 });
    },
  });

  const status = await client.reviewStatus();
  const patterns = await client.skillGrowPatterns();

  assertEqual(status.enabled, true);
  assertEqual(status.trust_enabled, true);
  assertEqual(status.distill_enabled, false);
  assertEqual(patterns.patterns[0]?.pattern, "repeat-review");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/review/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/skillgrow/patterns");
});

test("TrustClient throws TrustClientError with parsed and text bodies", async () => {
  const jsonClient = createTrustClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "slug is required" }, { status: 400 }),
  });

  try {
    await jsonClient.reset({ slug: "" });
    throw new Error("expected reset to reject");
  } catch (error) {
    assert(error instanceof TrustClientError);
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: "slug is required" });
    assertEqual(error.message, "slug is required");
  }


  const nestedClient = createTrustClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "trust skill slug is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.reset({ slug: "" });
    throw new Error("expected reset to reject");
  } catch (error) {
    assert(error instanceof TrustClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "trust skill slug is required");
  }

  const textClient = createTrustClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => new Response("POST required", { status: 405 }),
  });

  try {
    await textClient.grant({ slug: "shell" });
    throw new Error("expected grant to reject");
  } catch (error) {
    assert(error instanceof TrustClientError);
    assertEqual(error.status, 405);
    assertEqual(error.body, "POST required");
    assertEqual(error.message, "POST required");
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
