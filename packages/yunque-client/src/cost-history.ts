/** Lightweight cost-history SDK facade over the Cost slice. */
import {
  CostClient,
  CostClientError,
  createCostClient,
  type CostClientOptions,
  type CostHistoryQuery,
} from "./cost.js";

export type {
  CostClientOptions as CostHistoryClientOptions,
  CostHistoryQuery,
};

export { CostClientError as CostHistoryClientError };

export class CostHistoryClient {
  private readonly client: CostClient;

  constructor(options: CostClientOptions) { this.client = createCostClient(options); }
  list(query?: CostHistoryQuery): Promise<unknown> { return this.client.history(query); }
}

export function createCostHistoryClient(options: CostClientOptions): CostHistoryClient { return new CostHistoryClient(options); }
