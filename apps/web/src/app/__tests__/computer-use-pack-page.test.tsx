import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ComputerUsePackPage from "../packs/computer-use/page";

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: React.ReactNode;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

const fetchMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/sdk-client", () => ({
  createYunqueSDKClientOptions: () => ({
    baseUrl: "http://localhost",
    fetch: fetchMock,
  }),
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status });
}

describe("ComputerUsePackPage", () => {
  beforeEach(() => {
    fetchMock.mockReset();
    fetchMock.mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith("/v1/computer/status")) {
        return Promise.resolve(jsonResponse({
          execution_ready: false,
          capabilities: ["computer.status", "computer.intent.plan", "computer.screenshot.browser"],
          surfaces: {
            browser: { available: true, connected: true, status: "connected" },
            desktop_sandbox: { available: true, running: false, status: "configured" },
            local_desktop: { available: false, status: "not_supported_in_beta" },
          },
          safety: {
            direct_local_control: false,
            executes_local_commands: false,
          },
        }));
      }
      if (url.endsWith("/v1/computer/screenshot?surface=browser")) {
        return Promise.resolve(jsonResponse({
          surface: "browser",
          screenshot: "abc123",
          timestamp: "2026-06-19T00:00:00Z",
          safety: { read_only: true },
        }));
      }
      if (url.endsWith("/v1/computer/intent/plan") && init?.method === "POST") {
        return Promise.resolve(jsonResponse({
          plan: {
            goal: "检查当前浏览器页面",
            surface: "browser",
            status: "plan_ready_pending_policy_runtime",
            plan_ready: true,
            execution_ready: false,
            required_permissions: ["computer:read", "browser:read"],
            blocked_by: ["computer-use-executor-not-wired"],
            steps: [{
              index: 1,
              action: "inspect_surface",
              surface: "browser",
              read_only: true,
              permission: "browser:read",
              executor: "browser.inspect",
              description: "Inspect current surface readiness and collect non-destructive context.",
            }],
            gates: [{
              gate: "computer.permission.policy_gate",
              ready: true,
              allows_execute: false,
              human_approval: true,
              policy_enforced: false,
              blocked_by: ["permission-policy-not-enforced"],
            }],
            notes: ["plan-only"],
          },
        }));
      }
      return Promise.resolve(jsonResponse({ error: "unexpected" }, 404));
    });
  });

  it("explains computer use as plan-first and separates browser, cloud desktop and local desktop surfaces", async () => {
    render(<ComputerUsePackPage />);

    expect(await screen.findByText("这个能力包现在能做什么")).toBeInTheDocument();
    expect(screen.getByText("1. 先规划")).toBeInTheDocument();
    expect(screen.getByText("2. 再取证")).toBeInTheDocument();
    expect(screen.getByText("3. 后审批")).toBeInTheDocument();
    expect(screen.getByText("浏览器")).toBeInTheDocument();
    expect(screen.getByText("云桌面")).toBeInTheDocument();
    expect(screen.getByText("本机桌面")).toBeInTheDocument();
    expect(screen.getByText("Beta 关闭")).toBeInTheDocument();
  });

  it("connects computer use plans back to Chat, tasks, trace and Pack Studio", async () => {
    render(<ComputerUsePackPage />);

    expect(await screen.findByText("从电脑使用计划到可验证任务")).toBeInTheDocument();
    expect(screen.getByText("2. 带回 Chat")).toBeInTheDocument();
    expect(screen.getByText("4. 继续交给小羽改")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /带回 Chat/ })).toHaveAttribute(
      "href",
      expect.stringContaining("/chat?q="),
    );
    expect(screen.getByRole("link", { name: /看任务/ })).toHaveAttribute(
      "href",
      "/missions",
    );
    expect(screen.getByRole("link", { name: "核对执行轨迹" })).toHaveAttribute(
      "href",
      "/trace",
    );
    expect(screen.getByRole("link", { name: "让小羽继续改" })).toHaveAttribute(
      "href",
      "/packs/studio?packId=yunque.pack.computer-use",
    );
  });

  it("captures browser screenshot as read-only evidence", async () => {
    render(<ComputerUsePackPage />);
    await screen.findByText("浏览器截图证据");

    fireEvent.click(screen.getByRole("button", { name: /读取截图/ }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost/v1/computer/screenshot?surface=browser",
      expect.any(Object),
    ));
    expect(await screen.findByAltText("浏览器截图证据")).toBeInTheDocument();
    expect(screen.getByText(/2026-06-19T00:00:00Z · 只读证据/)).toBeInTheDocument();
  });

  it("renders generated plans as non-executing review output", async () => {
    render(<ComputerUsePackPage />);
    await screen.findByRole("button", { name: /生成计划/ });

    fireEvent.click(screen.getByRole("button", { name: /生成计划/ }));

    expect(await screen.findByText("计划结果")).toBeInTheDocument();
    expect(screen.getByText("仅计划")).toBeInTheDocument();
    expect(screen.getByText("为什么还不能自动执行")).toBeInTheDocument();
    expect(screen.getByText("computer-use-executor-not-wired")).toBeInTheDocument();
    expect(screen.getByText("权限策略")).toBeInTheDocument();
    expect(screen.getByText("暂不执行")).toBeInTheDocument();
  });
});
