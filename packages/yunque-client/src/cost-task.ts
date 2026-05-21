/** Lightweight cost-task SDK facade over the Cost slice. */
import {
  CostClient,
  CostClientError,
  createCostClient,
  type CostClientOptions,
} from "./cost.js";

export type {
  CostClientOptions as CostTaskClientOptions,
};

export { CostClientError as CostTaskClientError };

export class CostTaskClient {
  private readonly client: CostClient;

  constructor(options: CostClientOptions) { this.client = createCostClient(options); }
  get(id: string): Promise<unknown> { return this.client.task(id); }
  timeline(id: string): Promise<unknown> { return this.client.taskTimeline(id); }
}

export function createCostTaskClient(options: CostClientOptions): CostTaskClient { return new CostTaskClient(options); }
