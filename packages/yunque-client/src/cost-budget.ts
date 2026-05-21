/** Lightweight cost-budget SDK facade over the Cost slice. */
import {
  CostClient,
  CostClientError,
  createCostClient,
  type CostAlertsResponse,
  type CostBudget,
  type CostClientOptions,
  type CostSummaryResponse,
  type SetCostBudgetResponse,
} from "./cost.js";

export type {
  CostAlertsResponse,
  CostBudget,
  CostClientOptions as CostBudgetClientOptions,
  CostSummaryResponse,
  SetCostBudgetResponse,
};

export { CostClientError as CostBudgetClientError };

export class CostBudgetClient {
  private readonly client: CostClient;

  constructor(options: CostClientOptions) {
    this.client = createCostClient(options);
  }

  summary(): Promise<CostSummaryResponse> {
    return this.client.summary();
  }

  setBudget(budget: CostBudget): Promise<SetCostBudgetResponse> {
    return this.client.setBudget(budget);
  }

  alerts(): Promise<CostAlertsResponse> {
    return this.client.alerts();
  }
}

export function createCostBudgetClient(options: CostClientOptions): CostBudgetClient {
  return new CostBudgetClient(options);
}
