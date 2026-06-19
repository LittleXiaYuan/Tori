import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import PacksCognisPage from "../packs/cognis/page";

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

describe("PacksCognisPage", () => {
  it("explains the Cogni kernel as an infrastructure pack with a clear user path", () => {
    render(<PacksCognisPage />);

    expect(screen.getByText("Cogni 内核")).toBeInTheDocument();
    expect(screen.getByText("这个能力包现在能做什么")).toBeInTheDocument();
    expect(screen.getByText(/它不是一个单独给用户日常操作的应用/)).toBeInTheDocument();
    expect(screen.getByText("管理 Cogni 声明")).toBeInTheDocument();
    expect(screen.getByText("观察路由与健康")).toBeInTheDocument();
    expect(screen.getByText("连接能力包状态")).toBeInTheDocument();
    expect(screen.getByText("当前边界")).toBeInTheDocument();
    expect(screen.getByText("不会替代能力包本身的安装、权限授权或启用流程。")).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /打开 Cogni 管理/ })[0]).toHaveAttribute("href", "/cognis");
  });
});
