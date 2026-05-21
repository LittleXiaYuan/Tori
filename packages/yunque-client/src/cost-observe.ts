/** Lightweight cost-observe SDK facade over the Cost slice. */
import {
  CostClient,
  CostClientError,
  createCostClient,
  type CostBreakdownResponse,
  type CostClientOptions,
  type CostHistoryQuery,
} from "./cost.js";

export type {
  CostBreakdownResponse,
  CostClientOptions as CostObserveClientOptions,
  CostHistoryQuery,
};

export { CostClientError as CostObserveClientError };

export class CostObserveClient {
  private readonly client: CostClient;

  constructor(options: CostClientOptions) {
    this.client = createCostClient(options);
  }

  task(id: string): Promise<unknown> {
    return this.client.task(id);
  }

  taskTimeline(id: string): Promise<unknown> {
    return this.client.taskTimeline(id);
  }

  breakdown(): Promise<CostBreakdownResponse> {
    return this.client.breakdown();
  }

  history(query?: CostHistoryQuery): Promise<unknown> {
    return this.client.history(query);
  }
}

export function createCostObserveClient(options: CostClientOptions): CostObserveClient {
  return new CostObserveClient(options);
}
