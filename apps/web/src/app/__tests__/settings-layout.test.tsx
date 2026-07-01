import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import SettingsLayout from "../settings/layout";

const routerPush = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => "/settings/providers",
  useRouter: () => ({ push: routerPush }),
}));

vi.mock("lucide-react", () => {
  const Icon = () => <svg aria-hidden="true" />;
  return {
    Bell: Icon,
    Cpu: Icon,
    Palette: Icon,
    Plug: Icon,
    Settings: Icon,
  };
});

describe("SettingsLayout", () => {
  it("uses a desktop category nav instead of adding a duplicate settings heading", () => {
    render(
      <SettingsLayout>
        <main>
          <h1>模型提供商</h1>
        </main>
      </SettingsLayout>,
    );

    expect(screen.getByRole("navigation", { name: "设置分类" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /模型提供商/ })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("heading", { level: 1, name: "模型提供商" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 1, name: "设置" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
  });
});
