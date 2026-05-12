/** Lightweight scheduler-read SDK facade over the Scheduler slice. */
import {
  SchedulerClient,
  SchedulerClientError,
  createSchedulerClient,
  type SchedulerClientOptions,
  type SchedulerJob,
  type SchedulerJobsResponse,
} from "./scheduler.js";

export type {
  SchedulerClientOptions as SchedulerReadClientOptions,
  SchedulerJob,
  SchedulerJobsResponse,
};

export { SchedulerClientError as SchedulerReadClientError };

export class SchedulerReadClient {
  private readonly client: SchedulerClient;

  constructor(options: SchedulerClientOptions) {
    this.client = createSchedulerClient(options);
  }

  jobs(): Promise<SchedulerJobsResponse> {
    return this.client.jobs();
  }
}

export function createSchedulerReadClient(options: SchedulerClientOptions): SchedulerReadClient {
  return new SchedulerReadClient(options);
}
