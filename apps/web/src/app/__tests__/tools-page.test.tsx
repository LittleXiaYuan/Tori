import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ToolsPage from "../tools/page";

const apiMock = vi.hoisted(() => ({
  toolExec: vi.fn(),
  toolPoll: vi.fn(),
  toolKill: vi.fn(),
}));

const toastMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", () => ({
  api: apiMock,
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: toastMock,
}));

describe("ToolsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiMock.toolExec.mockResolvedValue({ output: "ok" });
  });

  it("keeps the tool recovery surface compact and accessible", async () => {
    render(<ToolsPage />);

    expect(screen.getByRole("heading", { name: "工具执行" })).toBeInTheDocument();
    expect(screen.getByText("验证命令、后台会话和输出。")).toBeInTheDocument();
    const recovery = screen.getByLabelText("工具恢复入口");
    expect(recovery).toHaveTextContent("工具失败？");
    expect(recovery).toHaveTextContent("新会话隔离，直接复现命令。");

    const cwdInput = screen.getByLabelText("工作目录");
    expect(cwdInput).toHaveAttribute("placeholder", "可选路径");

    const commandInput = screen.getByLabelText("命令");
    expect(commandInput).toHaveAttribute("placeholder", "输入命令...");
    fireEvent.click(screen.getByRole("button", { name: "输命令" }));
    expect(commandInput).toHaveFocus();

    const backgroundButton = screen.getByRole("button", { name: "后台" });
    expect(backgroundButton).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(backgroundButton);
    expect(backgroundButton).toHaveAttribute("aria-pressed", "true");

    fireEvent.change(cwdInput, { target: { value: "C:\\Code\\AI\\云雀" } });
    fireEvent.change(commandInput, { target: { value: "pwd" } });
    fireEvent.keyDown(commandInput, { key: "Enter", code: "Enter" });

    await waitFor(() => {
      expect(apiMock.toolExec).toHaveBeenCalledWith("pwd", "C:\\Code\\AI\\云雀", true);
    });
    expect(await screen.findByText(/\$ pwd/)).toBeInTheDocument();
  });

  it("opens additional real tool sessions from the header action", async () => {
    render(<ToolsPage />);

    fireEvent.click(screen.getByRole("button", { name: "新会话" }));

    expect(await screen.findByRole("button", { name: /#1/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /#2/ })).toBeInTheDocument();
  });
});
