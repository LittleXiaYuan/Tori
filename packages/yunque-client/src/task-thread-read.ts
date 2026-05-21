/** Lightweight task-thread-read SDK facade over task thread listing and messages. */
import {
  createTaskContextClient,
  TaskContextClient,
  TaskContextClientError,
  type TaskContextClientOptions,
  type TaskThreadInfo,
  type TaskThreadMessage,
  type TaskThreadResponse,
  type TaskThreadsResponse,
  type TaskThreadState,
} from "./task-context.js";

export type {
  TaskContextClientOptions as TaskThreadReadClientOptions,
  TaskThreadInfo,
  TaskThreadMessage,
  TaskThreadResponse,
  TaskThreadsResponse,
  TaskThreadState,
};

export { TaskContextClientError as TaskThreadReadClientError };

export class TaskThreadReadClient {
  private readonly client: TaskContextClient;

  constructor(options: TaskContextClientOptions) { this.client = createTaskContextClient(options); }
  list(state?: TaskThreadState): Promise<TaskThreadsResponse> { return this.client.threads(state); }
  get(taskId: string): Promise<TaskThreadResponse> { return this.client.thread(taskId); }
}

export function createTaskThreadReadClient(options: TaskContextClientOptions): TaskThreadReadClient { return new TaskThreadReadClient(options); }
