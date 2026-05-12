/** Lightweight audit-chain SDK facade over the Audit slice. */
import {
  AuditClient,
  AuditClientError,
  createAuditClient,
  type AuditClientOptions,
  type AuditStatsResponse,
  type AuditTailQuery,
  type AuditTailResponse,
  type AuditVerifyResponse,
} from "./audit.js";

export type {
  AuditClientOptions as AuditChainClientOptions,
  AuditStatsResponse,
  AuditTailQuery,
  AuditTailResponse,
  AuditVerifyResponse,
};

export { AuditClientError as AuditChainClientError };

export class AuditChainClient {
  private readonly client: AuditClient;

  constructor(options: AuditClientOptions) {
    this.client = createAuditClient(options);
  }

  tail(query?: AuditTailQuery): Promise<AuditTailResponse> {
    return this.client.tail(query);
  }

  verify(): Promise<AuditVerifyResponse> {
    return this.client.verify();
  }

  stats(): Promise<AuditStatsResponse> {
    return this.client.stats();
  }
}

export function createAuditChainClient(options: AuditClientOptions): AuditChainClient {
  return new AuditChainClient(options);
}
