import { describe, expect, it } from "vitest";
import { connectorRecoveryHint } from "../connector-recovery";
import type { ConnectorView } from "../api-types";

function connector(overrides: Partial<ConnectorView>): ConnectorView {
  return {
    id: "github",
    name: "GitHub",
    description: "Repository connector",
    icon: "github",
    category: "developer",
    auth_type: "token",
    supported: true,
    status: "connected",
    action_count: 7,
    ...overrides,
  };
}

describe("connectorRecoveryHint", () => {
  it("classifies expired credentials as an auth recovery", () => {
    const hint = connectorRecoveryHint(connector({
      last_event: {
        kind: "execute",
        connector_id: "github",
        action_id: "create_issue",
        status: "error",
        message: "token expired",
        at: "2026-06-24T00:00:00Z",
      },
    }));

    expect(hint?.kind).toBe("auth");
    expect(hint?.severity).toBe("danger");
    expect(hint?.title).toBe("GitHub 凭据需要重新授权");
    expect(hint?.href).toBe("/settings/connectors?focus=github");
  });

  it("routes browser pairing failures to the browser pack", () => {
    const hint = connectorRecoveryHint(connector({
      id: "browser",
      name: "My Browser",
      auth_type: "pairing",
      status: "error",
      error: "browser extension not paired",
    }));

    expect(hint?.kind).toBe("browser");
    expect(hint?.href).toBe("/packs/browser");
    expect(hint?.actionLabel).toBe("打开浏览器包");
  });

  it("does not route ordinary disconnected app failures to the browser pack", () => {
    const hint = connectorRecoveryHint(connector({
      status: "error",
      error: "connector github is not connected",
    }));

    expect(hint?.kind).toBe("generic");
    expect(hint?.href).toBe("/settings/connectors?focus=github");
  });

  it("classifies allowlist failures without pretending credentials are broken", () => {
    const hint = connectorRecoveryHint(connector({
      last_event: {
        kind: "execute",
        connector_id: "github",
        action_id: "delete_repo",
        status: "error",
        message: "action delete_repo is not allowed by allowlist",
        at: "2026-06-24T00:00:00Z",
      },
    }));

    expect(hint?.kind).toBe("allowlist");
    expect(hint?.title).toBe("GitHub 动作未在 Allowlist 中");
    expect(hint?.summary).toContain("动作边界");
  });
});
