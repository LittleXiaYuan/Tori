/** Lightweight cost-alerts SDK facade over the Cost slice. */
import {
  CostClient,
  CostClientError,
  createCostClient,
  type CostAlertsResponse,
  type CostClientOptions,
} from "./cost.js";

export type {
  CostAlertsResponse,
  CostClientOptions as CostAlertsClientOptions,
};

export { CostClientError as CostAlertsClientError };

export class CostAlertsClient {
  private readonly client: CostClient;

  constructor(options: CostClientOptions) { this.client = createCostClient(options); }
  list(): Promise<CostAlertsResponse> { return this.client.alerts(); }
}

export function createCostAlertsClient(options: CostClientOptions): CostAlertsClient { return new CostAlertsClient(options); }
