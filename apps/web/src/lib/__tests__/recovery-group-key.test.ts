import { describe, expect, it } from "vitest";
import { recoveryGroupKeyForTarget } from "../recovery-group-key";

describe("recovery group keys", () => {
  it("normalizes concrete recovery targets into stable grouping keys", () => {
    expect(recoveryGroupKeyForTarget("provider", "/settings/providers?focus=qwen-backup")).toBe(
      "provider|/settings/providers?focus=qwen-backup",
    );
    expect(recoveryGroupKeyForTarget("browser", "/packs/browser")).toBe("connector|/packs/browser");
    expect(recoveryGroupKeyForTarget("model", "/settings/providers?tab=providers")).toBe(
      "provider|/settings/providers?tab=providers",
    );
    expect(recoveryGroupKeyForTarget("dependency", "/planner-checkpoint?plan_id=p1#dependency-view")).toBe(
      "dependency|/planner-checkpoint?plan_id=p1#dependency-view",
    );
  });

  it("can infer categories from known recovery hrefs", () => {
    expect(recoveryGroupKeyForTarget(undefined, "/settings/connectors?focus=github")).toBe(
      "connector|/settings/connectors?focus=github",
    );
    expect(recoveryGroupKeyForTarget(undefined, "/approvals")).toBe("approval|/approvals");
    expect(recoveryGroupKeyForTarget(undefined, "/tools")).toBe("tool|/tools");
  });

  it("does not invent keys for unknown targets", () => {
    expect(recoveryGroupKeyForTarget(undefined, "/custom")).toBeUndefined();
    expect(recoveryGroupKeyForTarget("custom", "")).toBeUndefined();
  });
});
