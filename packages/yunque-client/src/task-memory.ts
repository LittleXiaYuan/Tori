/** Lightweight task-memory SDK facade over task working memory. */
import {
  createTaskContextClient,
  TaskContextClient,
  TaskContextClientError,
  type TaskContextClientOptions,
  type TaskWorkingMemory,
} from "./task-context.js";

export type {
  TaskContextClientOptions as TaskMemoryClientOptions,
  TaskWorkingMemory,
};

export { TaskContextClientError as TaskMemoryClientError };

export class TaskMemoryClient {
  private readonly client: TaskContextClient;

  constructor(options: TaskContextClientOptions) { this.client = createTaskContextClient(options); }
  get(taskId: string): Promise<TaskWorkingMemory> { return this.client.workingMemory(taskId); }
}

export function createTaskMemoryClient(options: TaskContextClientOptions): TaskMemoryClient { return new TaskMemoryClient(options); }
