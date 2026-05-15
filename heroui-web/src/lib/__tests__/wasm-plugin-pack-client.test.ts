import { afterEach, describe, expect, it, vi } from "vitest";
import { createWASMPluginPackClient } from "../wasm-plugin-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("wasm-plugin-pack-client", () => {
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
      manifest_artifact: "calculator-remote-remote-manifest.json",
      package_artifact: "calculator-remote.tgz",
      cache_key: "cache-key",
    },
    checks: [],
    artifacts: ["remote-install-plan.json", "signature-verification.json"],
    actions: [],
    labels: ["remote-install", "plan-only"],
  };

  it("reads WASM Plugin pack status and plugin metadata through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
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
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
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
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            plugin: {
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
          }),
          { status: 200 },
        ),
      );

    const client = createWASMPluginPackClient();
    const status = await client.status();
    await client.plugins();
    await client.plugin("calculator");

    expect(status.abi_plan_ready).toBe(true);
    expect(status.abi_ready).toBe(false);
    expect(status.remote_install_plan_ready).toBe(true);
    expect(status.remote_install_ready).toBe(false);
    expect(status.capabilities).toContain("wasm.host_abi.plan");
    expect(status.capabilities).toContain("wasm.remote_install.plan");
    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/wasm-plugin/status",
      "/v1/wasm-plugin/plugins",
      "/v1/wasm-plugin/plugins/calculator",
    ]);
  });

  it("installs, loads, unloads, executes, and plans remote signed installs with method-aware payloads", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            plugin: { slug: "calculator" },
            status: "installed",
          }),
          { status: 201 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            plugin: { slug: "calculator", status: "loaded" },
            status: "loaded",
          }),
          { status: 202 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            result: {
              slug: "calculator",
              dry_run: true,
              entrypoint: "plugin_exec",
              success: true,
              exit_code: 0,
              plan: [],
              host_abi_plan: hostABIPlan,
            },
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ plan: remoteInstallPlan }), {
          status: 200,
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            plugin: { slug: "calculator", status: "installed" },
            status: "unloaded",
          }),
          { status: 202 },
        ),
      );

    const client = createWASMPluginPackClient();
    await client.installPlugin({
      slug: "calculator",
      name: "Calculator",
      module_path: "calculator.wasm",
      dry_run: true,
    });
    await client.load("calculator");
    const executed = await client.execute({
      slug: "calculator",
      input: "{}",
      dry_run: true,
    });
    const remotePlan = await client.remoteInstallPlan({
      slug: "calculator-remote",
      name: "Calculator Remote",
      package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
      sha256: "0123456789abcdef",
      signature: "sig",
      public_key_id: "root",
    });
    await client.unload("calculator");

    expect(executed.result.host_abi_plan.plan_ready).toBe(true);
    expect(executed.result.host_abi_plan.enforcement_ready).toBe(false);
    expect(executed.result.host_abi_plan.writes_files).toBe(false);
    expect(remotePlan.plan.remote_install_plan_ready).toBe(true);
    expect(remotePlan.plan.remote_install_ready).toBe(false);
    expect(remotePlan.plan.writes_files).toBe(false);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/wasm-plugin/plugins");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(
      JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body)),
    ).toEqual({
      slug: "calculator",
      name: "Calculator",
      module_path: "calculator.wasm",
      dry_run: true,
    });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/wasm-plugin/plugins/load");
    expect(
      JSON.parse(String((spy.mock.calls[1]?.[1] as RequestInit).body)),
    ).toEqual({ slug: "calculator" });
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/wasm-plugin/execute");
    expect(
      JSON.parse(String((spy.mock.calls[2]?.[1] as RequestInit).body)),
    ).toEqual({ slug: "calculator", input: "{}", dry_run: true });
    expect(spy.mock.calls[3]?.[0]).toBe("/v1/wasm-plugin/remote-install/plan");
    expect((spy.mock.calls[3]?.[1] as RequestInit).method).toBe("POST");
    expect(
      JSON.parse(String((spy.mock.calls[3]?.[1] as RequestInit).body)),
    ).toEqual({
      slug: "calculator-remote",
      name: "Calculator Remote",
      package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
      sha256: "0123456789abcdef",
      signature: "sig",
      public_key_id: "root",
    });
    expect(spy.mock.calls[4]?.[0]).toBe("/v1/wasm-plugin/plugins/unload");
  });

  it("exports JSON evidence packs by plugin slug", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
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
          }),
          { status: 200 },
        ),
      );

    const client = createWASMPluginPackClient();
    const evidence = await client.evidence("calculator");

    expect(evidence.files).toContain("host-abi-plan.json");
    expect(evidence.files).toContain("remote-install-plan.json");
    expect(evidence.host_abi_plan.status).toBe("plan_only");
    expect(evidence.remote_install_plan.downloads).toBe(false);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/wasm-plugin/evidence/calculator");
  });
});
