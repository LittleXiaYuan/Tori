/** Lightweight audit-verify SDK facade over the Audit slice. */
import {
  AuditClient,
  AuditClientError,
  createAuditClient,
  type AuditClientOptions,
  type AuditStatsResponse,
  type AuditVerifyResponse,
} from "./audit.js";

export type {
  AuditClientOptions as AuditVerifyClientOptions,
  AuditStatsResponse,
  AuditVerifyResponse,
};

export { AuditClientError as AuditVerifyClientError };

export class AuditVerifyClient {
  private readonly client: AuditClient;

  constructor(options: AuditClientOptions) { this.client = createAuditClient(options); }
  verify(): Promise<AuditVerifyResponse> { return this.client.verify(); }
  stats(): Promise<AuditStatsResponse> { return this.client.stats(); }
}

export function createAuditVerifyClient(options: AuditClientOptions): AuditVerifyClient { return new AuditVerifyClient(options); }
