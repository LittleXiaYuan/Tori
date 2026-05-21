import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import EmptyState from "../empty-state";

vi.mock("@heroui/react", () => ({
  Button: ({ children, onPress, ...props }: { children: React.ReactNode; onPress?: () => void; [k: string]: unknown }) => (
    <button onClick={onPress} {...props}>{children}</button>
  ),
}));

vi.mock("lucide-react", () => ({
  MessageCircle: (props: Record<string, unknown>) => <svg data-testid="message-circle-icon" {...props} />,
}));

describe("EmptyState", () => {
  it("renders icon, title and description", () => {
    render(
      <EmptyState
        icon={<span data-testid="test-icon">📦</span>}
        title="No items"
        description="Start by creating one"
      />,
    );

    expect(screen.getByTestId("test-icon")).toBeInTheDocument();
    expect(screen.getByText("No items")).toBeInTheDocument();
    expect(screen.getByText("Start by creating one")).toBeInTheDocument();
  });

  it("renders with role='status' for accessibility", () => {
    const { container } = render(
      <EmptyState icon={<span>🔍</span>} title="Empty" />,
    );
    expect(container.querySelector("[role='status']")).toBeInTheDocument();
  });

  it("omits description when not provided", () => {
    render(<EmptyState icon={<span>📭</span>} title="Nothing here" />);
    expect(screen.getByText("Nothing here")).toBeInTheDocument();
    expect(screen.queryByText("Start by creating one")).toBeNull();
  });

  it("renders action button and fires callback on click", () => {
    const onAction = vi.fn();
    render(
      <EmptyState
        icon={<span>➕</span>}
        title="Empty"
        actionLabel="Create"
        onAction={onAction}
      />,
    );

    const btn = screen.getByText("Create");
    expect(btn).toBeInTheDocument();
    fireEvent.click(btn);
    expect(onAction).toHaveBeenCalledOnce();
  });

  it("does not render action button when actionLabel is missing", () => {
    render(<EmptyState icon={<span>📭</span>} title="Nope" />);
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("renders NL hint and fires onNlHint with hint text", () => {
    const onNlHint = vi.fn();
    render(
      <EmptyState
        icon={<span>💡</span>}
        title="Try this"
        nlHint="帮我创建一个新项目"
        onNlHint={onNlHint}
      />,
    );

    const hintBtn = screen.getByText(/帮我创建一个新项目/);
    expect(hintBtn).toBeInTheDocument();
    fireEvent.click(hintBtn.closest("button")!);
    expect(onNlHint).toHaveBeenCalledWith("帮我创建一个新项目");
  });
});
