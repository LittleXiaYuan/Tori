/** Lightweight task-threads SDK facade over the Task Context slice. */
import {
  createTaskContextClient,
  TaskContextClient,
  TaskContextClientError,
  type TaskChannelBinding,
  type TaskContextClientOptions,
  type TaskThreadActionResponse,
  type TaskThreadInfo,
  type TaskThreadMessage,
  type TaskThreadResponse,
  type TaskThreadsResponse,
  type TaskThreadState,
} from "./task-context.js";

export type {
  TaskChannelBinding,
  TaskContextClientOptions as TaskThreadsClientOptions,
  TaskThreadActionResponse,
  TaskThreadInfo,
  TaskThreadMessage,
  TaskThreadResponse,
  TaskThreadsResponse,
  TaskThreadState,
};

export { TaskContextClientError as TaskThreadsClientError };

export class TaskThreadsClient {
  private readonly client: TaskContextClient;

  constructor(options: TaskContextClientOptions) {
    this.client = createTaskContextClient(options);
  }

  list(state?: TaskThreadState): Promise<TaskThreadsResponse> {
    return this.client.threads(state);
  }

  get(taskId: string): Promise<TaskThreadResponse> {
    return this.client.thread(taskId);
  }

  postMessage(taskId: string, content: string, channel?: TaskChannelBinding): Promise<TaskThreadActionResponse> {
    return this.client.postThreadMessage(taskId, content, channel);
  }

  updateState(taskId: string, state: TaskThreadState): Promise<TaskThreadActionResponse> {
    return this.client.updateThreadState(taskId, state);
  }
}

export function createTaskThreadsClient(options: TaskContextClientOptions): TaskThreadsClient {
  return new TaskThreadsClient(options);
}
