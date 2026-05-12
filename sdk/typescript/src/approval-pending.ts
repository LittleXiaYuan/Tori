/** Lightweight approval-pending SDK facade over the Approvals slice. */
import {
  ApprovalsClient,
  ApprovalsClientError,
  createApprovalsClient,
  type ApprovalActionResponse,
  type ApprovalDecision,
  type ApprovalRequest,
  type ApprovalStatus,
  type ApprovalsClientOptions,
  type ListApprovalsResponse,
} from "./approvals.js";

export type {
  ApprovalActionResponse,
  ApprovalDecision,
  ApprovalRequest,
  ApprovalStatus,
  ApprovalsClientOptions as ApprovalPendingClientOptions,
  ListApprovalsResponse as ApprovalPendingResponse,
};

export { ApprovalsClientError as ApprovalPendingClientError };

export class ApprovalPendingClient {
  private readonly client: ApprovalsClient;

  constructor(options: ApprovalsClientOptions) { this.client = createApprovalsClient(options); }
  list(): Promise<ListApprovalsResponse> { return this.client.pending(); }
  approve(id: string): Promise<ApprovalActionResponse> { return this.client.approve(id); }
  deny(id: string, reason?: string): Promise<ApprovalActionResponse> { return this.client.deny(id, reason); }
  decide(id: string, decision: ApprovalDecision): Promise<ApprovalActionResponse> { return this.client.decide(id, decision); }
}

export function createApprovalPendingClient(options: ApprovalsClientOptions): ApprovalPendingClient { return new ApprovalPendingClient(options); }
