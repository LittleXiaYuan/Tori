/** Lightweight Usage/Quota SDK facade over the cost slice. */
import {
  CostClient,
  CostClientError,
  createCostClient,
  type CostClientOptions,
  type SetQuotaRequest,
  type SetQuotaResponse,
  type UsageRecord,
} from "./cost.js";

export type {
  CostClientOptions as UsageClientOptions,
  SetQuotaRequest,
  SetQuotaResponse,
  UsageRecord,
};

export { CostClientError as UsageClientError };

export class UsageClient {
  private readonly client: CostClient;

  constructor(options: CostClientOptions) {
    this.client = createCostClient(options);
  }

  usage(): Promise<UsageRecord> {
    return this.client.usage();
  }

  setQuota(body: SetQuotaRequest): Promise<SetQuotaResponse> {
    return this.client.setQuota(body);
  }
}

export function createUsageClient(options: CostClientOptions): UsageClient {
  return new UsageClient(options);
}
