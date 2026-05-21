/** Lightweight project-list SDK facade over the Projects slice. */
import {
  createProjectsClient,
  ProjectsClient,
  ProjectsClientError,
  type ProjectsClientOptions,
  type ProjectsListResponse,
} from "./projects.js";

export type {
  ProjectsClientOptions as ProjectListClientOptions,
  ProjectsListResponse,
};

export { ProjectsClientError as ProjectListClientError };

export class ProjectListClient {
  private readonly client: ProjectsClient;

  constructor(options: ProjectsClientOptions) {
    this.client = createProjectsClient(options);
  }

  list(): Promise<ProjectsListResponse> {
    return this.client.list();
  }
}

export function createProjectListClient(options: ProjectsClientOptions): ProjectListClient {
  return new ProjectListClient(options);
}
