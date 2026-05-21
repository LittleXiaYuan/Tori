/** Lightweight task-read SDK facade over the Tasks slice. */
import {
  createTasksClient,
  TasksClient,
  TasksClientError,
  type Task,
  type TasksClientOptions,
} from "./tasks.js";

export type {
  Task,
  TasksClientOptions as TaskReadClientOptions,
};

export { TasksClientError as TaskReadClientError };

export class TaskReadClient {
  private readonly client: TasksClient;

  constructor(options: TasksClientOptions) {
    this.client = createTasksClient(options);
  }

  list(): Promise<Task[]> {
    return this.client.list();
  }

  get(id: string): Promise<Task> {
    return this.client.get(id);
  }
}

export function createTaskReadClient(options: TasksClientOptions): TaskReadClient {
  return new TaskReadClient(options);
}
