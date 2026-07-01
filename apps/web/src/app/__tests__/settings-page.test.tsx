import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SettingsRedirect from "../settings/page";

// /settings is no longer a full page — it redirects to /chat and asks the app
// shell to open the settings modal. These tests pin that contract.

const routerMock = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn() }));
const searchParamsMock = vi.hoisted(() => ({ get: vi.fn(() => null) }));

vi.mock("next/navigation", () => ({
  useRouter: () => routerMock,
  useSearchParams: () => searchParamsMock,
}));

describe("SettingsRedirect", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchParamsMock.get.mockReturnValue(null);
  });

  it("redirects to /chat", async () => {
    render(<SettingsRedirect />);
    await waitFor(() => expect(routerMock.replace).toHaveBeenCalledWith("/chat"));
  });

  it("dispatches the open-settings event after redirecting", async () => {
    const spy = vi.fn();
    window.addEventListener("yunque:open-settings", spy);
    render(<SettingsRedirect />);
    await waitFor(() => expect(spy).toHaveBeenCalled(), { timeout: 1000 });
    window.removeEventListener("yunque:open-settings", spy);
  });

  it("shows a brief opening indicator", () => {
    render(<SettingsRedirect />);
    expect(screen.getByRole("status", { name: "正在打开设置" })).toBeInTheDocument();
  });
});
