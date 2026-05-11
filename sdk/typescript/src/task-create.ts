/** Lightweight task-create SDK facade over the Tasks slice. */
import {
  createTasksClient,
  TasksClient,
  TasksClientError,
  type CreateTaskRequest,
  type Task,
  type TaskConstraints,
  type TasksClientOptions,
} from "./tasks.js";

export type {
  CreateTaskRequest,
  Task,
  TaskConstraints,
  TasksClientOptions as TaskCreateClientOptions,
};

export { TasksClientError as TaskCreateClientError };

export type TaskCreateDescriptionOptions = Omit<CreateTaskRequest, "description">;

export class TaskCreateClient {
  private readonly client: TasksClient;

  constructor(options: TasksClientOptions) {
    this.client = createTasksClient(options);
  }

  create(request: CreateTaskRequest): Promise<Task> {
    return this.client.create(request);
  }

  createFromDescription(description: string, options: TaskCreateDescriptionOptions = {}): Promise<Task> {
    return this.client.create({ ...options, description });
  }
}

export function createTaskCreateClient(options: TasksClientOptions): TaskCreateClient {
  return new TaskCreateClient(options);
}
