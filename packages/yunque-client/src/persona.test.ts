import { createPersonaClient, PersonaClientError } from "./persona";

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

test("PersonaClient reads persona state with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPersonaClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ identity: "Tori", soul: "careful", skills: [{ name: "review" }] });
    },
  });

  const result = await client.get();

  assertEqual(result.identity, "Tori");
  assertEqual(result.skills?.[0]?.name, "review");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("PersonaClient updates persona identity and soul with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPersonaClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ status: "ok" });
    },
  });

  const result = await client.update({ identity: "云雀", soul: "warm" });

  assertEqual(result.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona");
  assertEqual(calls[0]?.init?.method, "PUT");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ identity: "云雀", soul: "warm" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("PersonaClient manages skills", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPersonaClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (init?.method === "GET") return jsonResponse({ skills: [{ name: "planner", content: "plan" }] });
      return jsonResponse({ status: "ok" }, { status: init?.method === "POST" ? 201 : 200 });
    },
  });

  const listed = await client.skills();
  const added = await client.addSkill({ name: "planner", description: "Plan", content: "plan" });
  const deleted = await client.deleteSkill({ name: "planner" });

  assertEqual(listed.skills[0]?.name, "planner");
  assertEqual(added.status, "ok");
  assertEqual(deleted.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona/skills");
  assertEqual(calls[1]?.init?.method, "POST");
  assertEqual(calls[2]?.init?.method, "DELETE");
  assertEqual(calls[2]?.init?.body, JSON.stringify({ name: "planner" }));
});

test("PersonaClient manages modes, presets and feature flags", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPersonaClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).includes("/v1/persona/modes")) return jsonResponse({ modes: [{ mode: "study" }, { mode: "focus" }], total: 2 });
      if (String(url).includes("/v1/persona/mode/current")) return jsonResponse({ mode: "study", name: "Study", description: "Study mode" });
      if (init?.method === "GET") return jsonResponse({ presets: [{ id: "default", name: "Default" }], active: "default" });
      if (String(url).endsWith("/custom") && init?.method === "POST") return jsonResponse({ status: "ok", id: "studio" });
      if (String(url).endsWith("/presets") && init?.method === "POST") return jsonResponse({ status: "ok", active: "studio" });
      return jsonResponse({ status: "ok" });
    },
  });

  const presets = await client.presets();
  const modes = await client.modes({ tenant_id: "tenant-1", session_id: "session-1" });
  const currentMode = await client.currentMode({ tenant_id: "tenant-1" });
  const switched = await client.switchPreset({ id: "studio" });
  const added = await client.addCustomPreset({ id: "studio", name: "Studio", features: { emotion: true } });
  const featureUpdated = await client.updatePresetFeatures({ id: "studio", features: { sticker: false } });
  const deleted = await client.deleteCustomPreset({ id: "studio" });

  assertEqual(presets.active, "default");
  assertEqual(modes.total, 2);
  assertEqual(currentMode.mode, "study");
  assertEqual(switched.active, "studio");
  assertEqual(added.id, "studio");
  assertEqual(featureUpdated.status, "ok");
  assertEqual(deleted.status, "ok");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/persona/modes?tenant_id=tenant-1&session_id=session-1");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/persona/mode/current?tenant_id=tenant-1");
  assertEqual(calls[3]?.url, "http://localhost:9090/v1/persona/presets");
  assertEqual(calls[4]?.url, "http://localhost:9090/v1/persona/presets/custom");
  assertEqual(calls[5]?.url, "http://localhost:9090/v1/persona/presets/features");
  assertEqual(calls[5]?.init?.method, "PUT");
  assertEqual(calls[6]?.url, "http://localhost:9090/v1/persona/presets/custom");
  assertEqual(calls[6]?.init?.method, "DELETE");
});

test("PersonaClient throws PersonaClientError with parsed and text bodies", async () => {
  const jsonClient = createPersonaClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "id required" }, { status: 400 }),
  });

  try {
    await jsonClient.switchPreset({ id: "" });
    throw new Error("expected switchPreset to reject");
  } catch (error) {
    assert(error instanceof PersonaClientError);
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: "id required" });
    assertEqual(error.message, "id required");
  }


  const nestedClient = createPersonaClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "persona preset id is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.switchPreset({ id: "" });
    throw new Error("expected switchPreset to reject");
  } catch (error) {
    assert(error instanceof PersonaClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "persona preset id is required");
  }

  const textClient = createPersonaClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => new Response("method not allowed", { status: 405 }),
  });

  try {
    await textClient.get();
    throw new Error("expected get to reject");
  } catch (error) {
    assert(error instanceof PersonaClientError);
    assertEqual(error.status, 405);
    assertEqual(error.body, "method not allowed");
    assertEqual(error.message, "method not allowed");
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
