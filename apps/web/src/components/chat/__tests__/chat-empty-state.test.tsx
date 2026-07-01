import { render, screen } from "@testing-library/react";
import { createRef } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ChatEmptyState } from "../chat-empty-state";
import { I18nProvider } from "@/lib/i18n";

vi.mock("lucide-react", () => ({
  AlertTriangle: () => <svg data-testid="alert-icon" />,
  Pencil: () => <svg data-testid="pencil-icon" />,
}));

// The empty state was reworked into a centered, Gemini-style screen: a single
// hero greeting + the composer, no starter chips, no architecture terms. These
// tests pin that contract.

function renderEmptyState(setupNeeded = false) {
  const inputRef = createRef<HTMLTextAreaElement>();
  render(
    <I18nProvider>
      <ChatEmptyState
        setupNeeded={setupNeeded}
        composer={<textarea ref={inputRef} aria-label="composer" />}
      />
    </I18nProvider>,
  );
  return { inputRef };
}

describe("ChatEmptyState", () => {
  beforeEach(() => {
    localStorage.setItem("yunque_locale", "zh");
    localStorage.removeItem("yunque_user_nickname");
  });

  it("renders a centered chat-first hero with the composer and no architecture terms", () => {
    renderEmptyState();

    expect(screen.getByLabelText("composer")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "整理知识" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "能力扩展" })).not.toBeInTheDocument();

    const text = document.body.textContent || "";
    expect(text).not.toMatch(/\bPack\b|Cogni|微内核|WASM|DLC/);
  });

  it("offers a nickname-edit affordance", () => {
    renderEmptyState();
    expect(screen.getByRole("button", { name: "设置称呼" })).toBeInTheDocument();
  });

  it("keeps first-run setup entry pointed at setup", () => {
    renderEmptyState(true);

    expect(screen.getByText("先完成模型配置")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "去配置模型 →" })).toHaveAttribute("href", "/setup");
  });
});
