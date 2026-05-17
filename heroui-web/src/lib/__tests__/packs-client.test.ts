import { afterEach, describe, expect, it, vi } from "vitest";
import { createPacksClient } from "../packs-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("packs-client", () => {
  it("reads pack registries through the pack runtime endpoints", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ packs: [], count: 0 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ packs: [], count: 0 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        generated_at: "2026-05-16T00:00:00Z",
        packs: 1,
        enabled_packs: 1,
        capabilities: 1,
        enabled_capabilities: 1,
        entries: [{
          capability: "backup.info",
          pack_id: "yunque.pack.backup",
          pack_name: "Backup Pack",
          pack_status: "enabled",
          enabled: true,
          optional: true,
          routes: ["/v1/backup/info"],
          permissions: ["backup:read"],
          sdk_typescript: "yunque-client/backup",
          frontend_paths: ["/packs/backup"],
        }],
      }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ modules: [], count: 0 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        generated_at: "2026-05-16T00:00:00Z",
        packs: 0,
        enabled_packs: 0,
        mounted_modules: 0,
        declared_routes: 0,
        mounted_routes: 0,
        ok_routes: 0,
        missing_routes: 0,
        method_mismatches: 0,
        undeclared_routes: 0,
        entries: [],
      }), { status: 200 }));

    const client = createPacksClient();
    await client.installed();
    await client.enabled();
    const capabilities = await client.capabilities();
    await client.backendModules();
    const audit = await client.backendRouteAudit();

    expect(fetchSpy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/packs/installed",
      "/v1/packs/enabled",
      "/v1/packs/capabilities",
      "/v1/packs/backend-modules",
      "/v1/packs/backend-route-audit",
    ]);
    expect(capabilities.enabled_capabilities).toBe(1);
    expect(capabilities.entries[0]?.capability).toBe("backup.info");
    expect(audit.ok_routes).toBe(0);
  });

  it("installs local and remote manifests with optional artifact download", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack: { status: "disabled" }, status: "disabled" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack: { status: "enabled" }, status: "enabled" }), { status: 200 }));

    const client = createPacksClient();
    await client.installLocal("packs/examples/backup-pack/pack.json", "local", false);
    await client.installFromURL("https://packs.example/pack.json", "remote", true);

    expect(fetchSpy.mock.calls[0]?.[0]).toBe("/v1/packs/install");
    expect(JSON.parse(String((fetchSpy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({
      manifest_path: "packs/examples/backup-pack/pack.json",
      source: "local",
      download: false,
    });
    expect(JSON.parse(String((fetchSpy.mock.calls[1]?.[1] as RequestInit).body))).toEqual({
      manifest_url: "https://packs.example/pack.json",
      source: "remote",
      download: true,
    });
  });

  it("mutates pack status and prunes artifacts", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch")
      .mockImplementation(async () => new Response(
        JSON.stringify({ pack: { status: "enabled" }, status: "enabled", removed: [], kept: [], removed_count: 0, kept_count: 0 }),
        { status: 200 },
      ));

    const client = createPacksClient();
    await client.enable("yunque.pack.backup");
    await client.disable("yunque.pack.backup");
    await client.rollback("yunque.pack.backup");
    await client.prune();

    expect(fetchSpy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/packs/enable",
      "/v1/packs/disable",
      "/v1/packs/rollback",
      "/v1/packs/prune",
    ]);
    expect(JSON.parse(String((fetchSpy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ id: "yunque.pack.backup" });
    expect((fetchSpy.mock.calls[3]?.[1] as RequestInit).method).toBe("POST");
  });
});
