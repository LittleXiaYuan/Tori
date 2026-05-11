/** Lightweight task-observe SDK facade over the Task Context slice. */
import {
  createTaskContextClient,
  TaskContextClient,
  TaskContextClientError,
  type TaskContextClientOptions,
  type TaskGapRecord,
  type TaskGapStats,
  type TaskGapType,
  type TaskWorkingMemory,
} from "./task-context.js";

export type {
  TaskContextClientOptions as TaskObserveClientOptions,
  TaskGapRecord,
  TaskGapStats,
  TaskGapType,
  TaskWorkingMemory,
};

export { TaskContextClientError as TaskObserveClientError };

export class TaskObserveClient {
  private readonly client: TaskContextClient;

  constructor(options: TaskContextClientOptions) {
    this.client = createTaskContextClient(options);
  }

  gaps(type?: TaskGapType): Promise<TaskGapRecord[]> {
    return this.client.gaps(type);
  }

  gapStats(): Promise<TaskGapStats> {
    return this.client.gapStats();
  }

  workingMemory(taskId: string): Promise<TaskWorkingMemory> {
    return this.client.workingMemory(taskId);
  }
}

export function createTaskObserveClient(options: TaskContextClientOptions): TaskObserveClient {
  return new TaskObserveClient(options);
}
