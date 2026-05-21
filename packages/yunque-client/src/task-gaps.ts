/** Lightweight task-gaps SDK facade over task gap records. */
import {
  createTaskContextClient,
  TaskContextClient,
  TaskContextClientError,
  type ResolveGapResponse,
  type TaskContextClientOptions,
  type TaskGapRecord,
  type TaskGapStats,
  type TaskGapType,
} from "./task-context.js";

export type {
  ResolveGapResponse,
  TaskContextClientOptions as TaskGapsClientOptions,
  TaskGapRecord,
  TaskGapStats,
  TaskGapType,
};

export { TaskContextClientError as TaskGapsClientError };

export class TaskGapsClient {
  private readonly client: TaskContextClient;

  constructor(options: TaskContextClientOptions) { this.client = createTaskContextClient(options); }
  list(type?: TaskGapType): Promise<TaskGapRecord[]> { return this.client.gaps(type); }
  stats(): Promise<TaskGapStats> { return this.client.gapStats(); }
  resolve(id: string): Promise<ResolveGapResponse> { return this.client.resolveGap(id); }
}

export function createTaskGapsClient(options: TaskContextClientOptions): TaskGapsClient { return new TaskGapsClient(options); }
