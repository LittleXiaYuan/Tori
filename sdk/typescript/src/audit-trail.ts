/** Lightweight audit-trail SDK facade over the Audit slice. */
import {
  AuditClient,
  AuditClientError,
  createAuditClient,
  type AuditClientOptions,
  type AuditTrailQuery,
  type AuditTrailResponse,
} from "./audit.js";

export type {
  AuditClientOptions as AuditTrailClientOptions,
  AuditTrailQuery,
  AuditTrailResponse,
};

export { AuditClientError as AuditTrailClientError };

export class AuditTrailClient {
  private readonly client: AuditClient;

  constructor(options: AuditClientOptions) {
    this.client = createAuditClient(options);
  }

  trail(query?: AuditTrailQuery): Promise<AuditTrailResponse> {
    return this.client.trail(query);
  }
}

export function createAuditTrailClient(options: AuditClientOptions): AuditTrailClient {
  return new AuditTrailClient(options);
}
