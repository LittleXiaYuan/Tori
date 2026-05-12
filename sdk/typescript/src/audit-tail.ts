/** Lightweight audit-tail SDK facade over the Audit slice. */
import {
  AuditClient,
  AuditClientError,
  createAuditClient,
  type AuditClientOptions,
  type AuditTailQuery,
  type AuditTailResponse,
} from "./audit.js";

export type {
  AuditClientOptions as AuditTailClientOptions,
  AuditTailQuery,
  AuditTailResponse,
};

export { AuditClientError as AuditTailClientError };

export class AuditTailClient {
  private readonly client: AuditClient;

  constructor(options: AuditClientOptions) { this.client = createAuditClient(options); }
  tail(query?: AuditTailQuery): Promise<AuditTailResponse> { return this.client.tail(query); }
}

export function createAuditTailClient(options: AuditClientOptions): AuditTailClient { return new AuditTailClient(options); }
