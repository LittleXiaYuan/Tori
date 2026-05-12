/** Lightweight project-detail SDK facade over the Projects slice. */
import {
  createProjectsClient,
  ProjectsClient,
  ProjectsClientError,
  type Project,
  type ProjectsClientOptions,
} from "./projects.js";

export type {
  Project,
  ProjectsClientOptions as ProjectDetailClientOptions,
};

export { ProjectsClientError as ProjectDetailClientError };

export class ProjectDetailClient {
  private readonly client: ProjectsClient;

  constructor(options: ProjectsClientOptions) {
    this.client = createProjectsClient(options);
  }

  detail(id: string): Promise<Project> {
    return this.client.detail(id);
  }
}

export function createProjectDetailClient(options: ProjectsClientOptions): ProjectDetailClient {
  return new ProjectDetailClient(options);
}
