/** Lightweight project-write SDK facade over the Projects slice. */
import {
  createProjectsClient,
  ProjectsClient,
  ProjectsClientError,
  type CreateProjectRequest,
  type DeleteProjectResponse,
  type Project,
  type ProjectsClientOptions,
  type UpdateProjectRequest,
} from "./projects.js";

export type {
  CreateProjectRequest,
  DeleteProjectResponse,
  Project,
  ProjectsClientOptions as ProjectWriteClientOptions,
  UpdateProjectRequest,
};

export { ProjectsClientError as ProjectWriteClientError };

export class ProjectWriteClient {
  private readonly client: ProjectsClient;

  constructor(options: ProjectsClientOptions) {
    this.client = createProjectsClient(options);
  }

  create(request: CreateProjectRequest): Promise<Project> {
    return this.client.create(request);
  }

  update(id: string, patch: UpdateProjectRequest): Promise<Project> {
    return this.client.update(id, patch);
  }

  remove(id: string): Promise<DeleteProjectResponse> {
    return this.client.remove(id);
  }
}

export function createProjectWriteClient(options: ProjectsClientOptions): ProjectWriteClient {
  return new ProjectWriteClient(options);
}
