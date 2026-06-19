import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import PackSurfaceGuide from "../pack-surface-guide";
import { allGuidedPackIDs, packSurfaceGuide } from "@/lib/pack-surface-guidance";

describe("PackSurfaceGuide", () => {
  it("explains memory as user actions plus supporting capability packs", () => {
    render(<PackSurfaceGuide surface="memory" />);

    expect(screen.getByText("这里承接的能力包")).toBeInTheDocument();
    expect(screen.getByText(/记忆不是一个孤立页面/)).toBeInTheDocument();
    expect(screen.getByText("记忆")).toBeInTheDocument();
    expect(screen.getByText("情绪上下文")).toBeInTheDocument();
    expect(screen.getByText("反思")).toBeInTheDocument();
    expect(screen.getByText("搜索一条记忆")).toBeInTheDocument();
    expect(screen.getByText("查看情感历史")).toBeInTheDocument();
    expect(screen.getByText("把经验带回下一次任务")).toBeInTheDocument();
  });

  it("covers the shared surfaces that otherwise look like generic pages", () => {
    expect(packSurfaceGuide("knowledge").items.map((item) => item.id)).toEqual([
      "yunque.pack.knowledge",
      "yunque.pack.retrieval",
      "yunque.pack.graph",
    ]);
    expect(packSurfaceGuide("missions").items.map((item) => item.id)).toEqual([
      "yunque.pack.missions",
      "yunque.pack.work",
      "yunque.pack.scheduler",
    ]);
    expect(packSurfaceGuide("workers").items.map((item) => item.id)).toEqual([
      "yunque.pack.mcp-dispatch",
      "yunque.pack.orchestrator",
    ]);
  });

  it("keeps the first batch of shared-entry pack IDs explicit", () => {
    expect(allGuidedPackIDs()).toEqual(expect.arrayContaining([
      "yunque.pack.memory",
      "yunque.pack.knowledge",
      "yunque.pack.missions",
      "yunque.pack.work",
      "yunque.pack.skills",
      "yunque.pack.cogni-console",
      "yunque.pack.cogni-kernel",
      "yunque.pack.workspace",
      "yunque.pack.control-plane",
      "yunque.pack.rbac",
    ]));
  });
});
