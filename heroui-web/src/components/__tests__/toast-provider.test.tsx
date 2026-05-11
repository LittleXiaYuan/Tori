import { act, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { showErrorToast, showToast, Toaster } from "../toast-provider";

describe("showErrorToast", () => {
  it("formats low-level errors before showing the toast", async () => {
    render(<Toaster />);

    await act(async () => {
      showErrorToast(new Error("handoff agent execution failed: context deadline exceeded"));
    });

    expect(screen.getByText("响应暂时超时，已保留现场，可稍后重试或继续。")).toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded|handoff agent|execution failed/)).toBeNull();
  });

  it("formats direct error toasts from legacy callers", async () => {
    render(<Toaster />);

    await act(async () => {
      showToast("all fallback LLM clients failed (FC): EOF", "error");
    });

    expect(screen.getByText("任务暂时没有完成，已保留现场，可切换策略或稍后继续。")).toBeInTheDocument();
    expect(screen.queryByText(/fallback|EOF/)).toBeNull();
  });
});
