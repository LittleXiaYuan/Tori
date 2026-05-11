/** Lightweight project-read SDK facade over the Projects slice. */
import {
  createProjectsClient,
  ProjectsClient,
  ProjectsClientError,
  type Project,
  type ProjectsClientOptions,
  type ProjectsListResponse,
} from "./projects.js";

export type {
  Project,
  ProjectsClientOptions as ProjectReadClientOptions,
  ProjectsListResponse,
};

export { ProjectsClientError as ProjectReadClientError };

export class ProjectReadClient {
  private readonly client: ProjectsClient;

  constructor(options: ProjectsClientOptions) {
    this.client = createProjectsClient(options);
  }

  list(): Promise<ProjectsListResponse> {
    return this.client.list();
  }

  detail(id: string): Promise<Project> {
    return this.client.detail(id);
  }
}

export function createProjectReadClient(options: ProjectsClientOptions): ProjectReadClient {
  return new ProjectReadClient(options);
}
