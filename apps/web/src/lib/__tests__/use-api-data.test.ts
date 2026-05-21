import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useApiData } from "../use-api-data";

describe("useApiData", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("starts in loading state with initial value", () => {
    const fetcher = vi.fn(() => new Promise<string[]>(() => {}));
    const { result } = renderHook(() => useApiData(fetcher, [], []));

    expect(result.current.loading).toBe(true);
    expect(result.current.data).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it("resolves data and clears loading", async () => {
    const fetcher = vi.fn().mockResolvedValue(["a", "b"]);
    const { result } = renderHook(() => useApiData(fetcher, [], []));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toEqual(["a", "b"]);
    expect(result.current.error).toBeNull();
  });

  it("captures errors", async () => {
    const fetcher = vi.fn().mockRejectedValue(new Error("network down"));
    const { result } = renderHook(() => useApiData(fetcher, "fallback", []));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error!.message).toBe("network down");
  });

  it("normalizes known low-level failures before exposing error.message", async () => {
    const fetcher = vi.fn().mockRejectedValue(
      new Error('task did not finish; execution failed while preparing handoff'),
    );
    const { result } = renderHook(() => useApiData(fetcher, "fallback", []));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error!.message).toBe("任务暂时没有完成，已保留现场，可切换策略或稍后继续。");
    expect(result.current.error!.message).not.toContain("execution failed");
    expect(result.current.error!.message).not.toContain("handoff");
  });

  it("refresh re-triggers the fetcher", async () => {
    let callCount = 0;
    const fetcher = vi.fn(async () => {
      callCount++;
      return callCount;
    });

    const { result } = renderHook(() => useApiData(fetcher, 0, []));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toBe(1);

    act(() => { result.current.refresh(); });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toBe(2);
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("ignores stale responses when refresh races", async () => {
    let resolvers: Array<(v: string) => void> = [];
    const fetcher = vi.fn(() => new Promise<string>((resolve) => {
      resolvers.push(resolve);
    }));

    const { result } = renderHook(() => useApiData(fetcher, "", []));

    act(() => { result.current.refresh(); });

    expect(resolvers).toHaveLength(2);

    // Resolve second (newer) request first
    act(() => { resolvers[1]("fresh"); });
    await waitFor(() => expect(result.current.data).toBe("fresh"));

    // Now resolve stale first request — should be ignored
    act(() => { resolvers[0]("stale"); });
    expect(result.current.data).toBe("fresh");
  });
});
