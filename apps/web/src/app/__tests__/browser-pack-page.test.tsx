import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import BrowserIntentPackPage from "../packs/browser/page";

if (typeof Element !== "undefined" && typeof Element.prototype.getAnimations !== "function") {
  Object.defineProperty(Element.prototype, "getAnimations", {
    configurable: true,
    value: () => [],
  });
}

const browserClient = vi.hoisted(() => ({
  status: vi.fn(),
  oppPending: vi.fn(),
  config: vi.fn(),
  screenshotLatest: vi.fn(),
  extensionStatus: vi.fn(),
  scenarios: vi.fn(),
  desktopStatus: vi.fn(),
  browserActPlan: vi.fn(),
}));

vi.mock("@/lib/browser-intent-pack-client", () => ({
  createBrowserIntentPackClient: vi.fn(() => browserClient),
}));

vi.mock("@/components/browser-session-card", () => ({
  BrowserSessionCard: () => null,
}));

vi.mock("@/lib/use-browser-bridge", () => ({
  useBrowserBridge: () => ({
    bridgeState: null,
    bridgeActionPending: null,
    bridgeNotice: null,
    lastArtifact: null,
    sendBridgeAction: vi.fn(),
  }),
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

describe("BrowserIntentPackPage", () => {
  beforeEach(() => {
    browserClient.status.mockReset().mockResolvedValue({ connected: false });
    browserClient.oppPending.mockReset().mockResolvedValue({ items: [], total: 0 });
    browserClient.config.mockReset().mockResolvedValue({ mode: "extension" });
    browserClient.screenshotLatest.mockReset().mockResolvedValue({});
    browserClient.extensionStatus.mockReset().mockResolvedValue({ connected: false });
    browserClient.scenarios.mockReset().mockResolvedValue({ scenarios: [] });
    browserClient.desktopStatus.mockReset().mockResolvedValue({ ok: true, running: false });
    browserClient.browserActPlan.mockReset().mockResolvedValue({
      plan: {
        pack_id: "yunque.pack.browser-intent",
        generated_at: "2026-06-19T00:00:00Z",
        stage: "plan",
        status: "blocked",
        dry_run: true,
        intent: "open_url",
        browser_act_plan_ready: true,
        browser_act_ready: false,
        permission_gate_ready: true,
        runtime_skill_gate_ready: true,
        opp_gate_ready: true,
        consumes_browser_session: false,
        executes_browser_actions: false,
        writes_browser_state: false,
        writes_files: false,
        network_access: false,
        requires_human_approval: true,
        action_count: 1,
        planned_actions: [{
          index: 0,
          intent: "open_url",
          executor_action: "navigate",
          target_url: "https://example.com",
          selector: "button[data-action=export]",
          requires_permission: "browser.intent",
          requires_runtime_skill: "browser.act",
          requires_opp_gate: true,
          consumes_browser_session: false,
          executes_browser_action: false,
          writes_browser_state: false,
          network_access: false,
        }],
        permission_gate: { gate: "permission", blocked_by: [] },
        runtime_skill_gate: { gate: "runtime", blocked_by: [] },
        opp_gate: { gate: "opp", blocked_by: [] },
        artifacts: ["browser-act-plan.json"],
        actions: ["open_url"],
        blocked_by: ["executor-not-wired"],
        labels: ["plan-only"],
      },
    });
  });

  it("explains the browser pack as a user-facing ability before technical details", async () => {
    render(<BrowserIntentPackPage />);

    expect(await screen.findByText("浏览器能力包能做什么")).toBeInTheDocument();
    expect(screen.getByText("看见网页现场")).toBeInTheDocument();
    expect(screen.getByText("提取页面信息")).toBeInTheDocument();
    expect(screen.getByText("运行审核过的场景")).toBeInTheDocument();
    expect(screen.getByText("生成动作计划")).toBeInTheDocument();
    expect(screen.getByText("当前不执行本机桌面控制")).toBeInTheDocument();
  });

  it("renders browser_act output as an actionable plan with honest boundaries", async () => {
    render(<BrowserIntentPackPage />);
    await screen.findByText("浏览器能力包能做什么");

    fireEvent.click(screen.getByText("动作计划"));
    fireEvent.click(screen.getByText("生成计划"));

    await waitFor(() => expect(browserClient.browserActPlan).toHaveBeenCalledOnce());
    expect(await screen.findByText("计划步骤")).toBeInTheDocument();
    expect(screen.getByText("权限检查")).toBeInTheDocument();
    expect(screen.getByText("运行能力")).toBeInTheDocument();
    expect(screen.getByText("人工审批")).toBeInTheDocument();
    expect(screen.getByText("真实执行")).toBeInTheDocument();
    expect(screen.getByText("为什么还不能自动执行")).toBeInTheDocument();
    expect(screen.getAllByText("executor-not-wired").length).toBeGreaterThan(0);
    expect(screen.getByText("当前不执行")).toBeInTheDocument();
  });
});
