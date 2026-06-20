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

  it("presents a chat-first scene selector without architecture terms", () => {
    renderEmptyState();

    expect(screen.getByText("选择工作伙伴")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "今天想让云雀怎么帮你？" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "通用云雀" })).toHaveAttribute("aria-current", "true");
    expect(screen.getByRole("button", { name: "任务执行" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "知识整理" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "记忆沉淀" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "能力扩展" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "电脑使用计划" })).toBeInTheDocument();

    const text = document.body.textContent || "";
    expect(text).not.toMatch(/\bPack\b|Cogni|微内核|WASM|DLC/);
  });

  it("switches scene prompts and keeps computer use plan-only", () => {
    const { chatD } = renderEmptyState();

    fireEvent.click(screen.getByRole("button", { name: "能力扩展" }));
    expect(screen.getByText("描述现有能力哪里不够，先生成可验收的补强方案。")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "补强能力" }));
    expect(chatD).toHaveBeenCalledWith({
      type: "SET_INPUT",
      value: expect.stringContaining("可验收的改进清单"),
    });

    fireEvent.click(screen.getByRole("button", { name: "电脑使用计划" }));
    expect(screen.getByText("先规划浏览器或桌面动作，当前不会直接控制本机。")).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "电脑使用计划" })[1]);
    expect(chatD).toHaveBeenLastCalledWith({
      type: "SET_INPUT",
      value: expect.stringContaining("暂时不要执行本机控制"),
    });
  });

  it("keeps first-run setup entry pointed at setup", () => {
    renderEmptyState({ setupNeeded: true });

    expect(screen.getByText("先完成模型配置")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "去配置模型 →" })).toHaveAttribute("href", "/setup");
  });
});
