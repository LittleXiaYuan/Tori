/** Lightweight task-thread-control SDK facade over task thread message/state mutations. */
import {
  createTaskContextClient,
  TaskContextClient,
  TaskContextClientError,
  type TaskChannelBinding,
  type TaskContextClientOptions,
  type TaskThreadActionResponse,
  type TaskThreadState,
} from "./task-context.js";

export type {
  TaskChannelBinding,
  TaskContextClientOptions as TaskThreadControlClientOptions,
  TaskThreadActionResponse,
  TaskThreadState,
};

export { TaskContextClientError as TaskThreadControlClientError };

export class TaskThreadControlClient {
  private readonly client: TaskContextClient;

  constructor(options: TaskContextClientOptions) { this.client = createTaskContextClient(options); }
  postMessage(taskId: string, content: string, channel?: TaskChannelBinding): Promise<TaskThreadActionResponse> { return this.client.postThreadMessage(taskId, content, channel); }
  updateState(taskId: string, state: TaskThreadState): Promise<TaskThreadActionResponse> { return this.client.updateThreadState(taskId, state); }
}

export function createTaskThreadControlClient(options: TaskContextClientOptions): TaskThreadControlClient { return new TaskThreadControlClient(options); }
