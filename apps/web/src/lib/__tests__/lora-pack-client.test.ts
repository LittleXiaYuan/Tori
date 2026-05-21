import { afterEach, describe, expect, it, vi } from "vitest";
import { createLoRAPackClient } from "../lora-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("lora-pack-client", () => {
  it("reads LoRA pack state through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ scheduler: { current_adapter: "a" }, active_model: "base" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ records: [], count: 0 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ summary: { total_runs: 0 } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ state: { rolling_success_rate: 0.8 } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ config: { min_samples: 20 } }), { status: 200 }));

    const client = createLoRAPackClient();
    await client.status();
    await client.history();
    await client.summary();
    await client.evolution();
    await client.config();

    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/lora/status",
      "/v1/lora/history",
      "/v1/lora/summary",
      "/v1/lora/evolution",
      "/v1/lora/config",
    ]);
  });

  it("passes tenant id in preview and trigger requests", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ preview: { ready: true } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "ok", tenant_id: "tenant-1" }), { status: 200 }));

    const client = createLoRAPackClient();
    await client.preview("tenant 1");
    await client.trigger("tenant-1");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/lora/preview?tenant_id=tenant%201");
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/lora/trigger");
    expect((spy.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((spy.mock.calls[1]?.[1] as RequestInit).body))).toEqual({ tenant_id: "tenant-1" });
  });

  it("updates and rolls back LoRA config through method-aware pack routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ config: { min_samples: 30 }, status: "updated" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "ok" }), { status: 200 }));

    const client = createLoRAPackClient();
    await client.updateConfig({ min_samples: 30 });
    await client.rollback();

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/lora/config");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("PUT");
    expect(JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ min_samples: 30 });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/lora/rollback");
    expect((spy.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
  });
});

