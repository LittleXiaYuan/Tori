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
        generated_at: "2026-05-17T00:00:00Z",
        sources: ["packs/examples", "packs/private"],
        source_reports: [
          { source: "packs/examples", ok: true, manifest_count: 2, matched_entries: 1 },
          { source: "packs/private", ok: false, manifest_count: 0, matched_entries: 0, errors: ["pack catalog source packs/private: not found"] },
        ],
        count: 1,
        installed: 0,
        enabled: 0,
        downloadable: 1,
        capabilities: 1,
        capability: "browser.intent",
        entries: [{
          manifest_path: "packs/examples/browser-intent-pack/pack.json",
          source: "packs/examples",
          manifest: { id: "yunque.pack.browser-intent", name: "Browser Intent Pack", version: "0.1.0", backend: { capabilities: ["browser.intent"] }, distribution: { packageUrl: "https://packs.yunque.local/browser.tgz", sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" } },
          installed: false,
          enabled: false,
          update_action: "install",
          downloadable: true,
        }],
        install_hints: [],
      }), { status: 200 }))
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
      .mockResolvedValueOnce(new Response(JSON.stringify({
        generated_at: "2026-05-17T00:00:00Z",
        capability: "backup.info",
        found: true,
        enabled: true,
        action: "use",
        preferred: {
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
        },
        entries: [],
        enabled_entries: [],
      }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        generated_at: "2026-05-17T00:00:01Z",
        capability: "backup.info",
        allowed: true,
        action: "use",
        reason: "capability is available through an enabled pack",
        resolution: {
          generated_at: "2026-05-17T00:00:00Z",
          capability: "backup.info",
          found: true,
          enabled: true,
          action: "use",
          entries: [],
          enabled_entries: [],
        },
        route_audit: [{ pack_id: "yunque.pack.backup", enabled: true, status: "ok", declared: true, mounted: true, path: "/v1/backup/info" }],
      }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        generated_at: "2026-05-17T00:00:02Z",
        capabilities: ["backup.info", "browser.intent"],
        allowed: false,
        action: "enable",
        allowed_count: 1,
        blocked_count: 1,
        use_count: 1,
        enable_count: 1,
        install_count: 0,
        route_audit_issue_count: 0,
        gates: [],
        required_packs: [{ capability: "backup.info", pack_id: "yunque.pack.backup", pack_name: "Backup Pack", pack_status: "enabled", enabled: true, optional: true }],
        enable_packs: [{ capability: "browser.intent", pack_id: "yunque.pack.browser-intent", pack_name: "Browser Intent Pack", pack_status: "disabled", enabled: false, optional: true }],
        catalog_install_hints: [{
          manifest_path: "packs/examples/rpa-replay-pack/pack.json",
          source: "packs/examples",
          manifest: { id: "yunque.pack.rpa-replay", name: "RPA Replay Pack", version: "0.1.0", backend: { capabilities: ["rpa.replay.plan"] }, distribution: { packageUrl: "https://packs.yunque.local/rpa.tgz", sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" } },
          installed: false,
          enabled: false,
          update_action: "install",
          downloadable: true,
        }],
        catalog_download_hints: [{
          manifest_path: "packs/examples/rpa-replay-pack/pack.json",
          source: "packs/examples",
          manifest: { id: "yunque.pack.rpa-replay", name: "RPA Replay Pack", version: "0.1.0", backend: { capabilities: ["rpa.replay.plan"] }, distribution: { packageUrl: "https://packs.yunque.local/rpa.tgz", sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" } },
          installed: false,
          enabled: false,
          update_action: "install",
          downloadable: true,
        }],
        catalog_source_reports: [
          { source: "packs/examples", ok: true, manifest_count: 1, matched_entries: 1 },
          { source: "packs/private", ok: false, manifest_count: 0, matched_entries: 0, errors: ["pack catalog source packs/private: not found"] },
        ],
      }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        generated_at: "2026-05-17T00:00:03Z",
        capabilities: ["backup.info", "browser.intent"],
        allowed: false,
        action: "install",
        catalog_source_reports: [
          { source: "packs/examples", ok: true, manifest_count: 1, matched_entries: 1 },
          { source: "packs/private", ok: false, manifest_count: 0, matched_entries: 0, errors: ["pack catalog source packs/private: not found"] },
        ],
        plan: { generated_at: "2026-05-17T00:00:02Z", capabilities: ["backup.info", "browser.intent"], allowed: false, action: "install", allowed_count: 1, blocked_count: 1, use_count: 1, enable_count: 0, install_count: 1, route_audit_issue_count: 0, gates: [], catalog_source_reports: [{ source: "packs/examples", ok: true, manifest_count: 1, matched_entries: 1 }, { source: "packs/private", ok: false, manifest_count: 0, matched_entries: 0, errors: ["pack catalog source packs/private: not found"] }] },
        steps: [{
          action: "download",
          pack_id: "yunque.pack.rpa-replay",
          pack_name: "RPA Replay Pack",
          capability: "rpa.replay.plan",
          manifest_path: "packs/examples/rpa-replay-pack/pack.json",
          package_url: "https://packs.yunque.local/rpa.tgz",
          sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
          installed: false,
          enabled: false,
          downloadable: true,
        }],
        download_steps: [{
          action: "download",
          pack_id: "yunque.pack.rpa-replay",
          package_url: "https://packs.yunque.local/rpa.tgz",
          sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
          installed: false,
          enabled: false,
          downloadable: true,
        }],
        step_count: 1,
        download_count: 1,
        enable_count: 0,
        install_count: 0,
        route_audit_issue_count: 0,
        ready_count: 0,
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
    const catalog = await client.catalog({ capability: "browser.intent" });
    const capabilities = await client.capabilities();
    const resolved = await client.resolveCapability("backup.info");
    const gate = await client.gateCapability("backup.info");
    const plan = await client.planCapabilities(["backup.info", "browser.intent"]);
    const prepare = await client.prepareCapabilities(["backup.info", "browser.intent"]);
    await client.backendModules();
    const audit = await client.backendRouteAudit();

    expect(fetchSpy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/packs/installed",
      "/v1/packs/enabled",
      "/v1/packs/catalog?capability=browser.intent",
      "/v1/packs/capabilities",
      "/v1/packs/capabilities/resolve?capability=backup.info",
      "/v1/packs/capabilities/gate?capability=backup.info",
      "/v1/packs/capabilities/plan?capability=backup.info&capability=browser.intent",
      "/v1/packs/capabilities/prepare?capability=backup.info&capability=browser.intent",
      "/v1/packs/backend-modules",
      "/v1/packs/backend-route-audit",
    ]);
    expect(catalog.sources).toEqual(["packs/examples", "packs/private"]);
    expect(catalog.source_reports?.[0]?.matched_entries).toBe(1);
    expect(catalog.source_reports?.[1]?.ok).toBe(false);
    expect(catalog.source_reports?.[1]?.errors?.[0]).toContain("packs/private");
    expect(catalog.entries[0]?.manifest.id).toBe("yunque.pack.browser-intent");
    expect(catalog.entries[0]?.downloadable).toBe(true);
    expect(capabilities.enabled_capabilities).toBe(1);
    expect(capabilities.entries[0]?.capability).toBe("backup.info");
    expect(resolved.action).toBe("use");
    expect(resolved.preferred?.pack_id).toBe("yunque.pack.backup");
    expect(gate.allowed).toBe(true);
    expect(gate.route_audit?.[0]?.status).toBe("ok");
    expect(plan.action).toBe("enable");
    expect(plan.enable_packs?.[0]?.pack_id).toBe("yunque.pack.browser-intent");
    expect(plan.catalog_install_hints?.[0]?.manifest.id).toBe("yunque.pack.rpa-replay");
    expect(plan.catalog_download_hints?.[0]?.downloadable).toBe(true);
    expect(plan.catalog_source_reports?.[0]?.matched_entries).toBe(1);
    expect(plan.catalog_source_reports?.[1]?.ok).toBe(false);
    expect(prepare.action).toBe("install");
    expect(prepare.catalog_source_reports?.[1]?.errors?.[0]).toContain("packs/private");
    expect(prepare.plan.catalog_source_reports?.[0]?.source).toBe("packs/examples");
    expect(prepare.download_steps?.[0]?.sha256).toBe("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef");
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
