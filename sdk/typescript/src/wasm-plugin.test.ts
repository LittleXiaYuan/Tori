import { createWASMPluginClient, WASMPluginClientError } from "./wasm-plugin";

declare const process: { exitCode?: number };

function assert(condition: unknown, message?: string): asserts condition {
  if (!condition) throw new Error(message || "assertion failed");
}

function assertEqual(
  actual: unknown,
  expected: unknown,
  message?: string,
): void {
  if (actual !== expected)
    throw new Error(
      message ||
        `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`,
    );
}

function assertDeepEqual(
  actual: unknown,
  expected: unknown,
  message?: string,
): void {
  const actualJson = JSON.stringify(actual);
  const expectedJson = JSON.stringify(expected);
  if (actualJson !== expectedJson)
    throw new Error(
      message || `expected ${actualJson} to deep equal ${expectedJson}`,
    );
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

const hostABIPlan = {
  plan_ready: true,
  ready: false,
  status: "plan_only",
  enforcement_ready: false,
  writes_files: false,
  network_access: false,
  functions: [],
  summary: {
    function_count: 0,
    enabled_count: 0,
    ledger_kv: false,
    memory_search: false,
    http_fetch: false,
    env_get: false,
    allowed_host_count: 0,
    env_allowlist_count: 0,
  },
  resource_limits: {
    max_memory_mb: 64,
    timeout_seconds: 30,
    allowed_hosts: [],
    env_allowlist: [],
  },
  labels: ["host-abi", "plan-only"],
};

const remoteInstallPlan = {
  pack_id: "yunque.pack.wasm-plugin",
  generated_at: "now",
  status: "plan_only",
  remote_install_plan_ready: true,
  remote_install_ready: false,
  download_ready: false,
  signature_verify_ready: false,
  downloads: false,
  installs_plugin: false,
  writes_files: false,
  network_access: false,
  plugin: {
    slug: "calculator-remote",
    name: "Calculator Remote",
    version: "0.2.0",
    runtime: "wazero",
    entrypoint: "_start",
    module_path: "calculator-remote.wasm",
  },
  package: {
    manifest_url: "https://packs.yunque.local/wasm/calculator-remote.json",
    package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
    expected_sha256: "0123456789abcdef",
    signature: "sig",
    public_key_id: "root",
    manifest_artifact: "calculator-remote-remote-manifest.json",
    package_artifact: "calculator-remote.tgz",
    cache_key: "cache-key",
  },
  checks: [],
  artifacts: ["remote-install-plan.json", "signature-verification.json"],
  actions: [],
  labels: ["remote-install", "plan-only"],
};

test("WASMPluginClient reads status and plugin list with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWASMPluginClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/status"))
        return jsonResponse({
          pack_id: "yunque.pack.wasm-plugin",
          stage: "pack-shell-before-runtime-hosts",
          runtime_ready: true,
          abi_plan_ready: true,
          abi_ready: false,
          remote_install_plan_ready: true,
          remote_install_ready: false,
          plugin_count: 1,
          loaded_count: 0,
          capabilities: ["wasm.host_abi.plan", "wasm.remote_install.plan"],
        });
      return jsonResponse({
        plugins: [
          {
            slug: "calculator",
            name: "Calculator",
            version: "0.1.0",
            runtime: "wazero",
            entrypoint: "plugin_exec",
            module_path: "calculator.wasm",
            status: "installed",
            exec_count: 0,
            permissions: {
              ledger_kv: true,
              memory_search: false,
              http_fetch: false,
              max_memory_mb: 64,
              timeout_seconds: 30,
            },
          },
        ],
        count: 1,
      });
    },
  });

  const status = await client.status();
  const plugins = await client.plugins();

  assertEqual(status.pack_id, "yunque.pack.wasm-plugin");
  assertEqual(status.abi_plan_ready, true);
  assertEqual(status.abi_ready, false);
  assertEqual(status.remote_install_plan_ready, true);
  assertEqual(status.remote_install_ready, false);
  assert(status.capabilities.includes("wasm.host_abi.plan"));
  assert(status.capabilities.includes("wasm.remote_install.plan"));
  assertEqual(plugins.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/wasm-plugin/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/wasm-plugin/plugins");
  assertEqual(
    new Headers(calls[0]?.init?.headers).get("authorization"),
    "Bearer token-123",
  );
});

test("WASMPluginClient installs, loads, executes dry-run, plans remote signed installs, reads detail, and unloads", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWASMPluginClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/plugins") && init?.method === "POST")
        return jsonResponse(
          {
            plugin: { slug: "calculator", name: "Calculator" },
            status: "installed",
          },
          { status: 201 },
        );
      if (String(url).endsWith("/plugins/load"))
        return jsonResponse(
          {
            plugin: { slug: "calculator", status: "loaded" },
            status: "loaded",
          },
          { status: 202 },
        );
      if (String(url).endsWith("/execute"))
        return jsonResponse({
          result: {
            slug: "calculator",
            dry_run: true,
            entrypoint: "plugin_exec",
            success: true,
            exit_code: 0,
            plan: [],
            host_abi_plan: hostABIPlan,
          },
        });
      if (String(url).endsWith("/remote-install/plan"))
        return jsonResponse({ plan: remoteInstallPlan });
      if (String(url).includes("/plugins/calculator"))
        return jsonResponse({
          plugin: { slug: "calculator", status: "loaded" },
        });
      return jsonResponse(
        {
          plugin: { slug: "calculator", status: "installed" },
          status: "unloaded",
        },
        { status: 202 },
      );
    },
  });

  const installed = await client.installPlugin({
    slug: "calculator",
    name: "Calculator",
    module_path: "calculator.wasm",
    dry_run: true,
  });
  const loaded = await client.load("calculator");
  const executed = await client.execute({
    slug: "calculator",
    input: "{}",
    dry_run: true,
  });
  const remotePlanned = await client.remoteInstallPlan({
    slug: "calculator-remote",
    name: "Calculator Remote",
    package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
    sha256: "0123456789abcdef",
    signature: "sig",
    public_key_id: "root",
  });
  const detail = await client.plugin("calculator");
  const unloaded = await client.unload("calculator");

  assertEqual(installed.status, "installed");
  assertEqual(loaded.status, "loaded");
  assertEqual(executed.result.entrypoint, "plugin_exec");
  assertEqual(executed.result.host_abi_plan.plan_ready, true);
  assertEqual(executed.result.host_abi_plan.enforcement_ready, false);
  assertEqual(executed.result.host_abi_plan.writes_files, false);
  assertEqual(remotePlanned.plan.remote_install_plan_ready, true);
  assertEqual(remotePlanned.plan.remote_install_ready, false);
  assertEqual(remotePlanned.plan.writes_files, false);
  assertEqual(detail.plugin.slug, "calculator");
  assertEqual(unloaded.status, "unloaded");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/wasm-plugin/plugins");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(
    calls[0]?.init?.body,
    JSON.stringify({
      slug: "calculator",
      name: "Calculator",
      module_path: "calculator.wasm",
      dry_run: true,
    }),
  );
  assertEqual(
    calls[1]?.url,
    "http://localhost:9090/v1/wasm-plugin/plugins/load",
  );
  assertEqual(calls[1]?.init?.body, JSON.stringify({ slug: "calculator" }));
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/wasm-plugin/execute");
  assertEqual(
    calls[2]?.init?.body,
    JSON.stringify({ slug: "calculator", input: "{}", dry_run: true }),
  );
  assertEqual(
    calls[3]?.url,
    "http://localhost:9090/v1/wasm-plugin/remote-install/plan",
  );
  assertEqual(calls[3]?.init?.method, "POST");
  assertEqual(
    calls[3]?.init?.body,
    JSON.stringify({
      slug: "calculator-remote",
      name: "Calculator Remote",
      package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
      sha256: "0123456789abcdef",
      signature: "sig",
      public_key_id: "root",
    }),
  );
  assertEqual(
    calls[4]?.url,
    "http://localhost:9090/v1/wasm-plugin/plugins/calculator",
  );
  assertEqual(
    calls[5]?.url,
    "http://localhost:9090/v1/wasm-plugin/plugins/unload",
  );
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("WASMPluginClient exports plugin evidence packs", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWASMPluginClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({
        pack_id: "yunque.pack.wasm-plugin",
        exported_at: "now",
        format: "json-wasm-plugin-evidence",
        files: [
          "plugin.json",
          "host-abi-plan.json",
          "remote-install-plan.json",
        ],
        plugin: { slug: "calculator" },
        plan: [],
        host_abi_plan: hostABIPlan,
        remote_install_plan: remoteInstallPlan,
      });
    },
  });

  const evidence = await client.evidence("calculator");

  assertEqual(evidence.format, "json-wasm-plugin-evidence");
  assertDeepEqual(evidence.files, [
    "plugin.json",
    "host-abi-plan.json",
    "remote-install-plan.json",
  ]);
  assertEqual(evidence.host_abi_plan.status, "plan_only");
  assertEqual(evidence.remote_install_plan.downloads, false);
  assertEqual(
    calls[0]?.url,
    "http://localhost:9090/v1/wasm-plugin/evidence/calculator",
  );
});

test("WASMPluginClient throws WASMPluginClientError with nested gateway messages", async () => {
  const client = createWASMPluginClient({
    baseUrl: "http://localhost:9090",
    fetch: async () =>
      jsonResponse({ error: "pack route is not enabled" }, { status: 404 }),
  });

  try {
    await client.status();
    throw new Error("expected status to reject");
  } catch (error) {
    assert(error instanceof WASMPluginClientError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "pack route is not enabled" });
    assertEqual(error.message, "pack route is not enabled");
  }

  const nestedClient = createWASMPluginClient({
    baseUrl: "http://localhost:9090",
    fetch: async () =>
      jsonResponse(
        { error: { code: "BAD_PLUGIN", message: "slug is required" } },
        { status: 400 },
      ),
  });

  try {
    await nestedClient.execute({ slug: "" });
    throw new Error("expected execute to reject");
  } catch (error) {
    assert(error instanceof WASMPluginClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "slug is required");
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
