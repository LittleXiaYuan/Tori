/** Lightweight scheduler-control SDK facade over the Scheduler slice. */
import {
  SchedulerClient,
  SchedulerClientError,
  createSchedulerClient,
  type SchedulerAddRequest,
  type SchedulerClientOptions,
  type SchedulerJob,
  type SchedulerRemoveResponse,
} from "./scheduler.js";

export type {
  SchedulerAddRequest,
  SchedulerClientOptions as SchedulerControlClientOptions,
  SchedulerJob,
  SchedulerRemoveResponse,
};

export { SchedulerClientError as SchedulerControlClientError };

export class SchedulerControlClient {
  private readonly client: SchedulerClient;

  constructor(options: SchedulerClientOptions) {
    this.client = createSchedulerClient(options);
  }

  add(request: SchedulerAddRequest): Promise<SchedulerJob> {
    return this.client.add(request);
  }

  remove(id: string): Promise<SchedulerRemoveResponse> {
    return this.client.remove(id);
  }
}

export function createSchedulerControlClient(options: SchedulerClientOptions): SchedulerControlClient {
  return new SchedulerControlClient(options);
}
