/** Lightweight approval-queue SDK facade over the Approvals slice. */
import {
  ApprovalsClient,
  ApprovalsClientError,
  createApprovalsClient,
  type ApprovalActionResponse,
  type ApprovalDecision,
  type ApprovalRequest,
  type ApprovalStatus,
  type ApprovalsClientOptions,
  type ListApprovalsOptions,
  type ListApprovalsResponse,
} from "./approvals.js";

export type {
  ApprovalActionResponse,
  ApprovalDecision,
  ApprovalRequest,
  ApprovalStatus,
  ApprovalsClientOptions as ApprovalQueueClientOptions,
  ListApprovalsOptions as ListApprovalQueueOptions,
  ListApprovalsResponse as ApprovalQueueResponse,
};

export { ApprovalsClientError as ApprovalQueueClientError };

export class ApprovalQueueClient {
  private readonly client: ApprovalsClient;

  constructor(options: ApprovalsClientOptions) {
    this.client = createApprovalsClient(options);
  }

  list(options?: ListApprovalsOptions): Promise<ListApprovalsResponse> {
    return this.client.list(options);
  }

  pending(): Promise<ListApprovalsResponse> {
    return this.client.pending();
  }

  history(): Promise<ListApprovalsResponse> {
    return this.client.history();
  }

  approve(id: string): Promise<ApprovalActionResponse> {
    return this.client.approve(id);
  }

  deny(id: string, reason?: string): Promise<ApprovalActionResponse> {
    return this.client.deny(id, reason);
  }

  decide(id: string, decision: ApprovalDecision): Promise<ApprovalActionResponse> {
    return this.client.decide(id, decision);
  }
}

export function createApprovalQueueClient(options: ApprovalsClientOptions): ApprovalQueueClient {
  return new ApprovalQueueClient(options);
}
