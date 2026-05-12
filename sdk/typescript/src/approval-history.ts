/** Lightweight approval-history SDK facade over the Approvals slice. */
import {
  ApprovalsClient,
  ApprovalsClientError,
  createApprovalsClient,
  type ApprovalRequest,
  type ApprovalStatus,
  type ApprovalsClientOptions,
  type ListApprovalsOptions,
  type ListApprovalsResponse,
} from "./approvals.js";

export type {
  ApprovalRequest,
  ApprovalStatus,
  ApprovalsClientOptions as ApprovalHistoryClientOptions,
  ListApprovalsOptions as ListApprovalHistoryOptions,
  ListApprovalsResponse as ApprovalHistoryResponse,
};

export { ApprovalsClientError as ApprovalHistoryClientError };

export class ApprovalHistoryClient {
  private readonly client: ApprovalsClient;

  constructor(options: ApprovalsClientOptions) { this.client = createApprovalsClient(options); }
  list(status?: ApprovalStatus | ""): Promise<ListApprovalsResponse> { return this.client.list({ history: true, status }); }
}

export function createApprovalHistoryClient(options: ApprovalsClientOptions): ApprovalHistoryClient { return new ApprovalHistoryClient(options); }
