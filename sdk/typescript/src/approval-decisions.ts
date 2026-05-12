/** Lightweight approval-decisions SDK facade over the Approvals slice. */
import {
  ApprovalsClient,
  ApprovalsClientError,
  createApprovalsClient,
  type ApprovalActionResponse,
  type ApprovalDecision,
  type ApprovalsClientOptions,
} from "./approvals.js";

export type {
  ApprovalActionResponse,
  ApprovalDecision,
  ApprovalsClientOptions as ApprovalDecisionsClientOptions,
};

export { ApprovalsClientError as ApprovalDecisionsClientError };

export class ApprovalDecisionsClient {
  private readonly client: ApprovalsClient;

  constructor(options: ApprovalsClientOptions) { this.client = createApprovalsClient(options); }
  approve(id: string): Promise<ApprovalActionResponse> { return this.client.approve(id); }
  deny(id: string, reason?: string): Promise<ApprovalActionResponse> { return this.client.deny(id, reason); }
  decide(id: string, decision: ApprovalDecision): Promise<ApprovalActionResponse> { return this.client.decide(id, decision); }
}

export function createApprovalDecisionsClient(options: ApprovalsClientOptions): ApprovalDecisionsClient { return new ApprovalDecisionsClient(options); }
