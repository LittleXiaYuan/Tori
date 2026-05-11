import { createBrowserClient, BrowserClientError } from "./browser";

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

test("BrowserClient reads status and config with auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBrowserClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/status")) return jsonResponse({ connected: true, state: "extension", version: "1.0.0" });
      return jsonResponse({ mode: "extension", connected: true, headless: false });
    },
  });

  const status = await client.status();
  const config = await client.config();

  assertEqual(status.connected, true);
  assertEqual(config.mode, "extension");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/browser/status");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("BrowserClient navigates, screenshots and extracts content", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBrowserClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/navigate")) return jsonResponse({ url: "https://example.com", title: "Example", screenshot: "abc" });
      if (String(url).endsWith("/ocr")) return jsonResponse({ text: "page text", result: "page text" });
      return jsonResponse({ screenshot: "abc", timestamp: "2026-05-11T00:00:00Z" });
    },
  });

  const nav = await client.navigate("https://example.com");
  const shot = await client.screenshot();
  const latest = await client.latestScreenshot();
  const ocr = await client.ocr();

  assertEqual(nav.title, "Example");
  assertEqual(shot.screenshot, "abc");
  assertEqual(latest.timestamp, "2026-05-11T00:00:00Z");
  assertEqual(ocr.text, "page text");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ url: "https://example.com" }));
});

test("BrowserClient supports OPP pending and decisions", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBrowserClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/pending")) return jsonResponse({ items: [{ problem_id: "opp-1" }], total: 1 });
      return jsonResponse({ status: "resolved", problem_id: "opp-1" });
    },
  });

  const pending = await client.oppPending();
  const decided = await client.oppDecide({ problem_id: "opp-1", decision: "allow_once" });

  assertEqual(pending.total, 1);
  assertEqual(decided.problem_id, "opp-1");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ problem_id: "opp-1", decision: "allow_once" }));
});

test("BrowserClient supports extension session, action and scenarios", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBrowserClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/ext/status")) return jsonResponse({ connected: true, pending: 0 });
      if (String(url).endsWith("/ext/session")) return jsonResponse({ ok: true, ws_url: "ws://localhost:9090/ws/browser", ticket: "t1" });
      if (String(url).endsWith("/ext/action")) return jsonResponse({ ok: true, title: "Example" });
      if (String(url).endsWith("/ext/scenarios")) return jsonResponse({ scenarios: [{ id: "open-page" }] });
      return jsonResponse({ ok: true, scenario: "open-page", results: [{ ok: true }] });
    },
  });

  const status = await client.extensionStatus();
  const session = await client.extensionSession();
  const action = await client.extensionAction({ type: "browser_navigate", url: "https://example.com" });
  const scenarios = await client.scenarios();
  const run = await client.runScenario("open-page");

  assertEqual(status.connected, true);
  assertEqual(session.ticket, "t1");
  assertEqual(action.ok, true);
  assertEqual(scenarios.scenarios[0]?.id, "open-page");
  assertEqual(run.scenario, "open-page");
  assertEqual(calls[2]?.init?.body, JSON.stringify({ type: "browser_navigate", url: "https://example.com" }));
  assertEqual(calls[4]?.init?.body, JSON.stringify({ scenario_id: "open-page" }));
});

test("BrowserClient throws BrowserClientError with parsed body", async () => {
  const client = createBrowserClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "browser extension not connected for current tenant" }, { status: 503 }),
  });

  try {
    await client.extensionAction({ type: "browser_screenshot" });
    throw new Error("expected extensionAction to reject");
  } catch (error) {
    assert(error instanceof BrowserClientError);
    assertEqual(error.status, 503);
    assertDeepEqual(error.body, { error: "browser extension not connected for current tenant" });
    assertEqual(error.message, "browser extension not connected for current tenant");
  }

  const nestedClient = createBrowserClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "browser action type is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.extensionAction({ type: "" });
    throw new Error("expected extensionAction to reject");
  } catch (error) {
    assert(error instanceof BrowserClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "browser action type is required");
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
