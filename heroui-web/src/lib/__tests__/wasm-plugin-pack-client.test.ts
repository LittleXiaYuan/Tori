import { afterEach, describe, expect, it, vi } from "vitest";
import { createWASMPluginPackClient } from "../wasm-plugin-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("wasm-plugin-pack-client", () => {
  it("reads WASM Plugin pack status and plugin metadata through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.wasm-plugin", stage: "pack-shell-before-runtime-hosts", runtime_ready: true, abi_ready: false, plugin_count: 1, loaded_count: 0, capabilities: [] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ plugins: [{ slug: "calculator", name: "Calculator", version: "0.1.0", runtime: "wazero", entrypoint: "plugin_exec", module_path: "calculator.wasm", status: "installed", exec_count: 0, permissions: { ledger_kv: true, memory_search: false, http_fetch: false, max_memory_mb: 64, timeout_seconds: 30 } }], count: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ plugin: { slug: "calculator", name: "Calculator", version: "0.1.0", runtime: "wazero", entrypoint: "plugin_exec", module_path: "calculator.wasm", status: "installed", exec_count: 0, permissions: { ledger_kv: true, memory_search: false, http_fetch: false, max_memory_mb: 64, timeout_seconds: 30 } } }), { status: 200 }));

    const client = createWASMPluginPackClient();
    await client.status();
    await client.plugins();
    await client.plugin("calculator");

    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/wasm-plugin/status",
      "/v1/wasm-plugin/plugins",
      "/v1/wasm-plugin/plugins/calculator",
    ]);
  });

  it("installs, loads, unloads, and executes with method-aware payloads", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ plugin: { slug: "calculator" }, status: "installed" }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ plugin: { slug: "calculator", status: "loaded" }, status: "loaded" }), { status: 202 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ result: { slug: "calculator", dry_run: true, entrypoint: "plugin_exec", success: true, exit_code: 0, plan: [] } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ plugin: { slug: "calculator", status: "installed" }, status: "unloaded" }), { status: 202 }));

    const client = createWASMPluginPackClient();
    await client.installPlugin({ slug: "calculator", name: "Calculator", module_path: "calculator.wasm", dry_run: true });
    await client.load("calculator");
    await client.execute({ slug: "calculator", input: "{}", dry_run: true });
    await client.unload("calculator");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/wasm-plugin/plugins");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ slug: "calculator", name: "Calculator", module_path: "calculator.wasm", dry_run: true });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/wasm-plugin/plugins/load");
    expect(JSON.parse(String((spy.mock.calls[1]?.[1] as RequestInit).body))).toEqual({ slug: "calculator" });
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/wasm-plugin/execute");
    expect(JSON.parse(String((spy.mock.calls[2]?.[1] as RequestInit).body))).toEqual({ slug: "calculator", input: "{}", dry_run: true });
    expect(spy.mock.calls[3]?.[0]).toBe("/v1/wasm-plugin/plugins/unload");
  });

  it("exports JSON evidence packs by plugin slug", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.wasm-plugin", exported_at: "now", format: "json-wasm-plugin-evidence", files: ["plugin.json"], plugin: { slug: "calculator" }, plan: [] }), { status: 200 }));

    const client = createWASMPluginPackClient();
    await client.evidence("calculator");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/wasm-plugin/evidence/calculator");
  });
});
