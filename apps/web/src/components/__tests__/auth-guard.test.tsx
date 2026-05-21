import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AuthGuard from "../auth-guard";

const navMocks = vi.hoisted(() => ({
  replace: vi.fn(),
  usePathname: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  usePathname: navMocks.usePathname,
  useRouter: () => ({ replace: navMocks.replace }),
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({ t: (key: string) => key }),
}));

vi.mock("@heroui/react", () => ({
  Spinner: () => <div data-testid="spinner" />,
}));

describe("AuthGuard", () => {
  beforeEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    navMocks.replace.mockReset();
    navMocks.usePathname.mockReset();
    localStorage.clear();
  });

  it("renders public paths without checking auth", () => {
    navMocks.usePathname.mockReturnValue("/login");
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    render(<AuthGuard><div>login form</div></AuthGuard>);

    expect(screen.getByText("login form")).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("unblocks protected pages when auth status is aborted", async () => {
    navMocks.usePathname.mockReturnValue("/dashboard");
    localStorage.setItem("yunque_token", "token");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new DOMException("timeout", "AbortError")),
    );

    render(<AuthGuard><div>protected page</div></AuthGuard>);

    expect(screen.getByTestId("spinner")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("protected page")).toBeInTheDocument());
    expect(navMocks.replace).not.toHaveBeenCalled();
  });

  it("clears invalid tokens and redirects to login", async () => {
    navMocks.usePathname.mockReturnValue("/dashboard");
    localStorage.setItem("yunque_token", "stale");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ authenticated: false, password_set: true }),
      } as Response),
    );

    render(<AuthGuard><div>protected page</div></AuthGuard>);

    await waitFor(() => expect(navMocks.replace).toHaveBeenCalledWith("/login"));
    expect(localStorage.getItem("yunque_token")).toBeNull();
    expect(screen.queryByText("protected page")).toBeNull();
  });
});
