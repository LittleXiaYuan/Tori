/** Lightweight task-templates SDK facade over the Task Context slice. */
import {
  createTaskContextClient,
  TaskContextClient,
  TaskContextClientError,
  type DeleteTaskTemplateResponse,
  type TaskContextClientOptions,
  type TaskContextTask,
  type TaskTemplate,
  type TaskTemplateStep,
  type TaskTemplatesResponse,
  type TaskTemplateVariable,
  type TaskTemplateVariables,
} from "./task-context.js";

export type {
  DeleteTaskTemplateResponse,
  TaskContextClientOptions as TaskTemplatesClientOptions,
  TaskContextTask,
  TaskTemplate,
  TaskTemplatesResponse,
  TaskTemplateStep,
  TaskTemplateVariable,
  TaskTemplateVariables,
};

export { TaskContextClientError as TaskTemplatesClientError };

export class TaskTemplatesClient {
  private readonly client: TaskContextClient;

  constructor(options: TaskContextClientOptions) {
    this.client = createTaskContextClient(options);
  }

  list(): Promise<TaskTemplatesResponse> {
    return this.client.templates();
  }

  get(id: string): Promise<TaskTemplate> {
    return this.client.template(id);
  }

  create(template: TaskTemplate): Promise<TaskTemplate> {
    return this.client.createTemplate(template);
  }

  delete(id: string): Promise<DeleteTaskTemplateResponse> {
    return this.client.deleteTemplate(id);
  }

  instantiate(templateId: string, variables: TaskTemplateVariables = {}): Promise<TaskContextTask> {
    return this.client.instantiateTemplate(templateId, variables);
  }
}

export function createTaskTemplatesClient(options: TaskContextClientOptions): TaskTemplatesClient {
  return new TaskTemplatesClient(options);
}
