import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import CognisPage from "../cognis/page";

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
}));

const cogniPackMock = vi.hoisted(() => ({
  list: vi.fn().mockResolvedValue({ cognis: [], health: {} }),
  alerts: vi.fn().mockResolvedValue({ alerts: [], count: 0 }),
  runtimePackState: vi.fn().mockResolvedValue({
    pack_status: "enabled",
    runtime_loop_running: true,
    active_bus_cognis: 0,
    experience_store_count: 0,
  }),
}));

vi.mock("@/lib/cogni-kernel-pack-client", () => ({
  createCogniKernelPackClient: () => cogniPackMock,
}));

vi.mock("yunque-client/cognis", () => ({
  createCognisClient: () => ({
    exportBundle: vi.fn(),
  }),
}));

vi.mock("@/lib/sdk-client", () => ({
  createYunqueSDKClientOptions: () => ({
    baseUrl: "http://localhost",
    fetch: vi.fn(),
  }),
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

describe("CognisPage", () => {
  it("states the formal delivery boundary for Cogni", async () => {
    render(<CognisPage />);

    expect(await screen.findByText("Cogni 的正式交付口径")).toBeInTheDocument();
    expect(screen.getByText(/模型能力组织层/)).toBeInTheDocument();
    expect(screen.getByText("现在可稳定交付")).toBeInTheDocument();
    expect(screen.getByText("Beta 继续观察")).toBeInTheDocument();
    expect(screen.getByText("暂不作为稳定承诺")).toBeInTheDocument();
    expect(screen.getByText("创建、导入、导出、启用和停用 Cogni 声明。")).toBeInTheDocument();
    expect(screen.getByText("Planner 自动选择 Cogni 的命中质量。")).toBeInTheDocument();
    expect(screen.getByText("自主执行高风险动作或本机电脑控制。")).toBeInTheDocument();
    expect(screen.getByText("云雀如何使用它")).toBeInTheDocument();
    expect(screen.getByText("能力包扩展云雀底座：提供路由、界面、权限、WASM/DLC 或后端能力。")).toBeInTheDocument();
    expect(screen.getByText("Cogni 增设模型侧能力声明：告诉模型何时选择 Skill、MCP、能力包、记忆和工作流。")).toBeInTheDocument();
    expect(screen.getByText("如果某个能力包被禁用，Cogni 只能看到受限状态，不能绕过 Pack Runtime。")).toBeInTheDocument();
  });
});
