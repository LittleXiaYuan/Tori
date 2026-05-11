/** Lightweight approval-rules SDK facade over the Approvals slice. */
import {
  ApprovalsClient,
  ApprovalsClientError,
  createApprovalsClient,
  type ApprovalActionResponse,
  type ApprovalRule,
  type ApprovalRulesResponse,
  type ApprovalsClientOptions,
} from "./approvals.js";

export type {
  ApprovalActionResponse,
  ApprovalRule,
  ApprovalRulesResponse,
  ApprovalsClientOptions as ApprovalRulesClientOptions,
};

export { ApprovalsClientError as ApprovalRulesClientError };

export class ApprovalRulesClient {
  private readonly client: ApprovalsClient;

  constructor(options: ApprovalsClientOptions) {
    this.client = createApprovalsClient(options);
  }

  list(): Promise<ApprovalRulesResponse> {
    return this.client.rules();
  }

  add(rule: ApprovalRule): Promise<ApprovalActionResponse> {
    return this.client.addRule(rule);
  }

  delete(id: string): Promise<ApprovalActionResponse> {
    return this.client.deleteRule(id);
  }
}

export function createApprovalRulesClient(options: ApprovalsClientOptions): ApprovalRulesClient {
  return new ApprovalRulesClient(options);
}
