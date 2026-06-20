import { fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ChatEmptyState } from "../chat-empty-state";
import type { ChatDispatch } from "@/lib/chat-state";
import { I18nProvider } from "@/lib/i18n";

vi.mock("lucide-react", () => ({
  AlertTriangle: () => <svg data-testid="alert-icon" />,
  BookOpen: () => <svg data-testid="book-icon" />,
  Brain: () => <svg data-testid="brain-icon" />,
  ClipboardList: () => <svg data-testid="clipboard-icon" />,
  Cpu: () => <svg data-testid="cpu-icon" />,
  FileText: () => <svg data-testid="file-icon" />,
  MessageCircle: () => <svg data-testid="message-icon" />,
  Monitor: () => <svg data-testid="monitor-icon" />,
  Package: () => <svg data-testid="package-icon" />,
  Search: () => <svg data-testid="search-icon" />,
}));

function renderEmptyState(options?: { setupNeeded?: boolean; dispatch?: ChatDispatch }) {
  const inputRef = createRef<HTMLTextAreaElement>();
  const chatD = options?.dispatch || vi.fn();
  render(
    <I18nProvider>
      <ChatEmptyState
        setupNeeded={options?.setupNeeded ?? false}
        chatD={chatD}
        inputRef={inputRef}
        composer={<textarea ref={inputRef} aria-label="composer" />}
      />
    </I18nProvider>,
  );
  return { chatD, inputRef };
}

describe("ChatEmptyState", () => {
  beforeEach(() => {
    localStorage.setItem("yunque_locale", "zh");
  });

  it("presents a lightweight chat-first empty state without architecture terms", () => {
    renderEmptyState();

    expect(screen.queryByText("选择工作伙伴")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /你说，我在听/ })).toBeInTheDocument();
    expect(screen.getByText("问答、任务、知识和记忆都从这一句话开始。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "解释概念" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "整理知识" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "记住偏好" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "写周报" })).toBeInTheDocument();
    expect(screen.getByText("用通俗的话解释一个概念并举例。")).toBeInTheDocument();
    expect(screen.getByText("把资料沉淀成可复用知识条目。")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "能力扩展" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "电脑使用计划" })).not.toBeInTheDocument();

    const text = document.body.textContent || "";
    expect(text).not.toMatch(/\bPack\b|Cogni|微内核|WASM|DLC/);
  });

  it("fills the composer from compact starter chips", () => {
    const { chatD } = renderEmptyState();

    fireEvent.click(screen.getByRole("button", { name: "整理知识" }));
    expect(chatD).toHaveBeenCalledWith({
      type: "SET_INPUT",
      value: expect.stringContaining("整理成知识库条目"),
    });

    fireEvent.click(screen.getByRole("button", { name: "写周报" }));
    expect(chatD).toHaveBeenLastCalledWith({
      type: "SET_INPUT",
      value: expect.stringContaining("整理成周报"),
    });
  });

  it("keeps first-run setup entry pointed at setup", () => {
    renderEmptyState({ setupNeeded: true });

    expect(screen.getByText("先完成模型配置")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "去配置模型 →" })).toHaveAttribute("href", "/setup");
  });
});
