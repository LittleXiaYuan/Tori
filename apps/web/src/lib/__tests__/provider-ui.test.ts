import { describe, expect, it } from "vitest";
import {
  capabilityOverflow,
  normalizeProviderTestResult,
  orderedCapabilities,
  providerColor,
  providerInitial,
  providerTitle,
  searchMatch,
  statusTone,
  type ProviderLike,
} from "../provider-ui";

function provider(overrides: Partial<ProviderLike> = {}): ProviderLike {
  return {
    id: "openai-gpt-5.4-mini",
    display_name: "OpenAI Demo",
    type: "chat",
    model: "gpt-5.4-mini",
    base_url: "https://api.openai.com/v1",
    enabled: true,
    tier: "smart",
    priority: 0,
    capabilities: ["function_calling"],
    key_count: 1,
    breaker_state: "closed",
    ...overrides,
  };
}

describe("provider-ui", () => {
  it("derives stable provider labels and brand accents", () => {
    expect(providerTitle(provider({ display_name: "Tori Router" }))).toBe("Tori Router");
    expect(providerInitial(" 月之暗面")).toBe("月");
    expect(providerColor("qwen-wanx2.1-t2i-plus")).toBe("#6236ff");
    expect(providerColor("unknown")).toBe("#6b7280");
  });

  it("maps runtime status to concise UI tone labels", () => {
    expect(statusTone(provider({ enabled: false })).label).toBe("已停用");
    expect(statusTone(provider({ breaker_state: "open" })).label).toBe("熔断");
    expect(statusTone(provider({ breaker_state: "half-open" })).label).toBe("半开");
    expect(statusTone(provider()).label).toBe("启用");
  });

  it("prioritizes demo-relevant capabilities while keeping overflow accurate", () => {
    const caps = ["custom_cap", "image_gen", "vision", "mcp", "streaming"];
    expect(orderedCapabilities(caps, 3)).toEqual(["image_gen", "vision", "mcp"]);
    expect(capabilityOverflow(caps, 3)).toBe(2);
  });

  it("matches search tokens across nested provider and preset fields", () => {
    expect(searchMatch(["OpenAI", ["gpt-image-1", ["image_gen"]]], "image")).toBe(true);
    expect(searchMatch(["通义千问", ["wanx2.1-t2i-plus", ["image_gen"]]], "wanx qwen")).toBe(true);
    expect(searchMatch(["Moonshot", "kimi-k2.5"], "flux")).toBe(false);
  });

  it("normalizes legacy provider test payloads from backend handlers", () => {
    expect(normalizeProviderTestResult({ success: true })).toMatchObject({ status: "ok", latency_ms: 0 });
    expect(normalizeProviderTestResult({ status: "ok", latency_ms: 12 })).toMatchObject({ status: "ok", latency_ms: 12 });
    expect(normalizeProviderTestResult({ success: false, error: "401 Unauthorized" })).toMatchObject({
      status: "error",
      error: "401 Unauthorized",
    });
  });
});
