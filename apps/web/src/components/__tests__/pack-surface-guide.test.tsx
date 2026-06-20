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
    expect(screen.getAllByRole("link", { name: /查看详情/ })[0]).toHaveAttribute("href", "/packs/detail?id=yunque.pack.memory");
    expect(screen.getAllByRole("link", { name: /回中心/ })[0]).toHaveAttribute("href", "/packs?q=yunque.pack.memory");
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
      "yunque.pack.cron",
      "yunque.pack.planner-recovery",
      "yunque.pack.session-queue",
      "yunque.pack.state",
      "yunque.pack.subagents",
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

  it("covers the remaining infrastructure packs through their real user surfaces", () => {
    expect(allGuidedPackIDs()).toEqual(expect.arrayContaining([
      "yunque.pack.documents",
      "yunque.pack.files",
      "yunque.pack.speech",
      "yunque.pack.forks",
      "yunque.pack.persona-modes",
      "yunque.pack.channels",
      "yunque.pack.cost",
      "yunque.pack.plugin-api",
      "yunque.pack.desktop",
      "yunque.pack.identity",
      "yunque.pack.instructions",
      "yunque.pack.persona",
      "yunque.pack.connectors",
      "yunque.pack.notifications",
      "yunque.pack.tori",
      "yunque.pack.sandbox",
      "yunque.pack.ide",
      "yunque.pack.federation",
      "yunque.pack.trace",
      "yunque.pack.triggers",
      "yunque.pack.cron",
      "yunque.pack.planner-recovery",
      "yunque.pack.session-queue",
      "yunque.pack.state",
      "yunque.pack.subagents",
    ]));
  });

  it("renders compact mode for dense surfaces such as chat and inbox", () => {
    render(<PackSurfaceGuide surface="chat" compact />);

    expect(screen.getByText("对话里可直接触发的能力包")).toBeInTheDocument();
    expect(screen.getByText("文档生成")).toBeInTheDocument();
    expect(screen.getByText("产物文件")).toBeInTheDocument();
    expect(screen.getByText("语音")).toBeInTheDocument();
    expect(screen.getByText(/没有必要单独占一个页面/)).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /详情/ })[0]).toHaveAttribute("href", "/packs/detail?id=yunque.pack.documents");
    expect(screen.getAllByRole("link", { name: /中心/ })[0]).toHaveAttribute("href", "/packs?q=yunque.pack.documents");
  });
});
