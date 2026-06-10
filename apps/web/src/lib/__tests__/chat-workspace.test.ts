import { describe, expect, it } from "vitest";

import { workspacePathsFromProjects } from "../chat-workspace";
import type { ProjectInfo } from "../api-types";

function project(id: string, repoPath: string): ProjectInfo {
  return {
    id,
    name: id,
    repo_path: repoPath,
    created_at: "2026-06-09T00:00:00Z",
    updated_at: "2026-06-09T00:00:00Z",
  };
}

describe("workspacePathsFromProjects", () => {
  it("deduplicates and preserves project repo paths for agent requests", () => {
    const paths = workspacePathsFromProjects([
      project("campus", " C:\\Users\\Administrator\\Documents\\校园管理 "),
      project("campus-dup", "c:\\users\\administrator\\documents\\校园管理\\"),
      project("agent", "C:\\Code\\AI\\云雀\\yunque-agent"),
      project("empty", " "),
    ]);

    expect(paths).toEqual([
      "C:\\Users\\Administrator\\Documents\\校园管理",
      "C:\\Code\\AI\\云雀\\yunque-agent",
    ]);
  });
});
