import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ProvidersPage from "../settings/providers/page";

// ProvidersPanel is now a minimal custom-model manager (add / list / toggle /
// delete). These tests pin that simplified contract.

const routerMock = vi.hoisted(() => ({ push: vi.fn(), query: "" }));

const apiMock = vi.hoisted(() => ({
  providerList: vi.fn(),
  execProvider: vi.fn(),
  providerRegister: vi.fn(),
  providerDelete: vi.fn(),
  providerEnable: vi.fn(),
  providerDisable: vi.fn(),
  providerTest: vi.fn(),
  setExecProvider: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => routerMock,
  useSearchParams: () => new URLSearchParams(routerMock.query),
}));

vi.mock("@/lib/api", () => ({ api: apiMock }));
vi.mock("@/components/toast-provider", () => ({ showToast: vi.fn() }));

const provider = {
  id: "my-gpt-abcd",
  display_name: "我的 GPT",
  type: "chat",
  source: "manual",
  model: "gpt-4.1",
  base_url: "https://api.openai.com/v1",
  enabled: true,
  priority: 10,
  key_count: 1,
  breaker_state: "closed",
};

describe("ProvidersPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiMock.providerList.mockResolvedValue({ providers: [provider], count: 1 });
    apiMock.execProvider.mockResolvedValue({ exec_provider: "my-gpt-abcd", available_providers: ["my-gpt-abcd"] });
    apiMock.providerRegister.mockResolvedValue({ ok: true, provider_id: "x" });
    apiMock.providerDelete.mockResolvedValue({ ok: true });
  });

  it("lists registered custom models", async () => {
    render(<ProvidersPage />);
    expect(await screen.findByText("我的 GPT")).toBeInTheDocument();
    expect(screen.getByText(/gpt-4\.1 ·/)).toBeInTheDocument();
    expect(screen.getByText("主模型")).toBeInTheDocument();
  });

  it("shows an add form and registers a custom model", async () => {
    render(<ProvidersPage />);
    fireEvent.click(await screen.findByRole("button", { name: /添加模型/ }));

    fireEvent.change(screen.getByPlaceholderText("https://api.openai.com/v1"), { target: { value: "https://custom.api/v1" } });
    fireEvent.change(screen.getByPlaceholderText(/gpt-4\.1 \/ deepseek-chat/), { target: { value: "my-model" } });
    fireEvent.click(screen.getByRole("button", { name: "添加" }));

    await waitFor(() => expect(apiMock.providerRegister).toHaveBeenCalled());
    const arg = apiMock.providerRegister.mock.calls[0][0];
    expect(arg.base_url).toBe("https://custom.api/v1");
    expect(arg.model).toBe("my-model");
    expect(typeof arg.id).toBe("string");
  });

  it("deletes a model", async () => {
    render(<ProvidersPage />);
    await screen.findByText("我的 GPT");
    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    await waitFor(() => expect(apiMock.providerDelete).toHaveBeenCalledWith("my-gpt-abcd"));
  });
});
