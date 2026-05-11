import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { NLConfigCard } from "../nl-config-card";

describe("NLConfigCard", () => {
  it("formats low-level apply errors", async () => {
    render(
      <NLConfigCard
        changes={[{ category: "模型", field: "主模型", fromValue: "A", toValue: "B" }]}
        onApply={vi.fn().mockRejectedValue(new Error("handoff agent execution failed: context deadline exceeded"))}
        onCancel={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "应用配置" }));

    await waitFor(() => {
      expect(screen.getByText("响应暂时超时，已保留现场，可稍后重试或继续。")).toBeInTheDocument();
    });
    expect(screen.queryByText(/context deadline exceeded|handoff agent|execution failed/)).toBeNull();
  });
});
