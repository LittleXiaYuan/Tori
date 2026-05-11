import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ErrorPage from "../error";

describe("App error page", () => {
  it("formats low-level errors before rendering them", () => {
    render(<ErrorPage error={new Error("handoff agent execution failed: context deadline exceeded")} reset={vi.fn()} />);

    expect(screen.getByText("响应暂时超时，已保留现场，可稍后重试或继续。")).toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded|handoff agent|execution failed/)).toBeNull();
  });
});
