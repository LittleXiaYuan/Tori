import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import TracePage from "../trace/page";
import { createTracePackClient } from "@/lib/trace-pack-client";

const traceClient = vi.hoisted(() => ({
  recent: vi.fn(),
  byTask: vi.fn(),
  byTrace: vi.fn(),
}));
let queryString = "";

vi.mock("@/lib/trace-pack-client", () => ({
  createTracePackClient: vi.fn(() => traceClient),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(queryString),
}));

vi.mock("@/components/execution-trace", () => ({
  default: ({ events }: { events: Array<{ summary: string }> }) => (
    <div data-testid="execution-trace">
      {events.map((event) => event.summary).join(" | ")}
    </div>
  ),
}));

function traceEvent(id: string, summary: string) {
  return {
    id,
    trace_id: "trace-1",
    ts: "2026-06-19T01:00:00Z",
    domain: "planner",
    type: "thinking",
    summary,
    meta: { task_id: "task-1" },
  };
}

describe("TracePage", () => {
  beforeEach(() => {
    queryString = "";
    vi.mocked(createTracePackClient).mockClear();
    traceClient.recent.mockReset();
    traceClient.byTask.mockReset();
    traceClient.byTrace.mockReset();
    traceClient.recent.mockResolvedValue({
      count: 1,
      raw: false,
      events: [traceEvent("evt-1", "正在拆解任务")],
    });
    traceClient.byTask.mockResolvedValue({
      count: 1,
      raw: false,
      task_id: "task-42",
      events: [traceEvent("evt-task", "任务步骤已完成")],
    });
    traceClient.byTrace.mockResolvedValue({
      count: 1,
      raw: true,
      trace_id: "trace-42",
      events: [traceEvent("evt-trace", "Trace 已回放")],
    });
  });

  it("loads recent execution trace events by default", async () => {
    render(<TracePage />);

    await waitFor(() => expect(traceClient.recent).toHaveBeenCalledWith(50));
    expect(await screen.findByTestId("execution-trace")).toHaveTextContent("正在拆解任务");
    expect(screen.getAllByText("执行轨迹").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("事件")).toBeInTheDocument();
  });

  it("queries trace events by task id", async () => {
    render(<TracePage />);
    await screen.findByText("正在拆解任务");

    fireEvent.click(screen.getByText("按任务 ID"));
    fireEvent.change(screen.getByLabelText("按任务 ID"), { target: { value: "task-42" } });
    fireEvent.click(screen.getByText("查询"));

    await waitFor(() => expect(traceClient.byTask).toHaveBeenCalledWith("task-42"));
    expect(await screen.findByText(/任务步骤已完成/)).toBeInTheDocument();
  });

  it("queries trace events by trace id", async () => {
    render(<TracePage />);
    await screen.findByText("正在拆解任务");

    fireEvent.click(screen.getByText("按 Trace ID"));
    fireEvent.change(screen.getByLabelText("按 Trace ID"), { target: { value: "trace-42" } });
    fireEvent.click(screen.getByText("查询"));

    await waitFor(() => expect(traceClient.byTrace).toHaveBeenCalledWith("trace-42"));
    expect(await screen.findByText(/Trace 已回放/)).toBeInTheDocument();
    expect(screen.getByText("Raw")).toBeInTheDocument();
  });

  it("loads task trace directly from the task query parameter", async () => {
    queryString = "task=task-42";

    render(<TracePage />);

    await waitFor(() => expect(traceClient.byTask).toHaveBeenCalledWith("task-42"));
    expect(await screen.findByText(/任务步骤已完成/)).toBeInTheDocument();
    expect(traceClient.recent).not.toHaveBeenCalled();
  });
});
