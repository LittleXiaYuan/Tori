import type { ProjectInfo } from "@/lib/api-types";

export function workspacePathsFromProjects(projects: ProjectInfo[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];

  for (const project of projects) {
    const path = (project.repo_path || "").trim();
    if (!path) continue;

    const key = path.replace(/[\\/]+$/, "").toLowerCase();
    if (seen.has(key)) continue;

    seen.add(key);
    out.push(path);
  }

  return out;
}
