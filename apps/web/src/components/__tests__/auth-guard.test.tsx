import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AuthGuard from "../auth-guard";
import { setApiKey } from "@/lib/api-core";

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
    setApiKey("");
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

  it("uses stored API key credentials on protected pages", async () => {
    navMocks.usePathname.mockReturnValue("/settings");
    localStorage.setItem("yunque_api_key", "dev-key");
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ authenticated: true, password_set: true }),
    } as Response);
    vi.stubGlobal("fetch", fetchMock);

    render(<AuthGuard><div>settings page</div></AuthGuard>);

    await waitFor(() => expect(screen.getByText("settings page")).toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledWith("/v1/auth/status", expect.objectContaining({
      headers: { "X-API-Key": "dev-key" },
    }));
    expect(navMocks.replace).not.toHaveBeenCalled();
  });

  it("bootstraps a desktop JWT before redirecting to login", async () => {
    navMocks.usePathname.mockReturnValue("/settings");
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ token: "desktop-jwt" }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ authenticated: true, password_set: true }),
      } as Response);
    vi.stubGlobal("fetch", fetchMock);

    render(<AuthGuard><div>settings page</div></AuthGuard>);

    await waitFor(() => expect(screen.getByText("settings page")).toBeInTheDocument());
    expect(fetchMock).toHaveBeenNthCalledWith(1, "/v1/auth/desktop-bootstrap", expect.objectContaining({ method: "POST" }));
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/v1/auth/status", expect.objectContaining({
      headers: { Authorization: "Bearer desktop-jwt" },
    }));
    expect(localStorage.getItem("yunque_token")).toBe("desktop-jwt");
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
