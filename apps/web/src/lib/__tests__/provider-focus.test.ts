import { describe, expect, it } from "vitest";
import { providerFocusHrefFromText, providerIdFromText } from "../provider-focus";

describe("provider focus helpers", () => {
  it("extracts explicit provider ids from retained failure text", () => {
    expect(providerIdFromText(["provider_id=qwen-backup returned 403"])).toBe("qwen-backup");
    expect(providerFocusHrefFromText(["provider: openai-main quota exceeded"])).toBe("/settings/providers?focus=openai-main");
  });

  it("keeps generic provider text on the provider settings tab", () => {
    expect(providerIdFromText(["OpenAI provider returned 403 forbidden"])).toBe("");
    expect(providerFocusHrefFromText(["OpenAI provider returned 403 forbidden"])).toBe("/settings/providers?tab=providers");
    expect(providerIdFromText(["request failed at https://api.moonshot.ai/v1"])).toBe("");
  });
});
