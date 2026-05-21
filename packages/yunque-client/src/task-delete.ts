/** Lightweight task-delete SDK facade over the Tasks slice. */
import {
  createTasksClient,
  TasksClient,
  TasksClientError,
  type TaskActionResponse,
  type TasksClientOptions,
} from "./tasks.js";

export type {
  TaskActionResponse,
  TasksClientOptions as TaskDeleteClientOptions,
};

export { TasksClientError as TaskDeleteClientError };

export class TaskDeleteClient {
  private readonly client: TasksClient;

  constructor(options: TasksClientOptions) {
    this.client = createTasksClient(options);
  }

  delete(id: string): Promise<TaskActionResponse> {
    return this.client.delete(id);
  }
}

export function createTaskDeleteClient(options: TasksClientOptions): TaskDeleteClient {
  return new TaskDeleteClient(options);
}
