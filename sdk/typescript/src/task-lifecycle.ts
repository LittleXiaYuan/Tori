/** Lightweight task lifecycle SDK facade over the Tasks slice. */
import {
  createTasksClient,
  TasksClient,
  TasksClientError,
  type TaskActionResponse,
  type TasksClientOptions,
} from "./tasks.js";

export type {
  TaskActionResponse,
  TasksClientOptions as TaskLifecycleClientOptions,
};

export { TasksClientError as TaskLifecycleClientError };

export type TaskLifecycleAction = "run" | "pause" | "resume" | "restart" | "cancel";

export class TaskLifecycleClient {
  private readonly client: TasksClient;

  constructor(options: TasksClientOptions) {
    this.client = createTasksClient(options);
  }

  run(id: string): Promise<TaskActionResponse> {
    return this.client.run(id);
  }

  pause(id: string): Promise<TaskActionResponse> {
    return this.client.pause(id);
  }

  resume(id: string): Promise<TaskActionResponse> {
    return this.client.resume(id);
  }

  restart(id: string): Promise<TaskActionResponse> {
    return this.client.restart(id);
  }

  cancel(id: string): Promise<TaskActionResponse> {
    return this.client.cancel(id);
  }

  action(action: TaskLifecycleAction, id: string): Promise<TaskActionResponse> {
    switch (action) {
      case "run": return this.run(id);
      case "pause": return this.pause(id);
      case "resume": return this.resume(id);
      case "restart": return this.restart(id);
      case "cancel": return this.cancel(id);
    }
  }
}

export function createTaskLifecycleClient(options: TasksClientOptions): TaskLifecycleClient {
  return new TaskLifecycleClient(options);
}
