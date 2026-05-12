/** Lightweight workflow-run SDK facade over workflow start/cancel actions. */
import {
  createWorkflowClient,
  WorkflowClient,
  WorkflowClientError,
  type CancelWorkflowRequest,
  type CancelWorkflowResponse,
  type RunWorkflowRequest,
  type RunWorkflowResponse,
  type WorkflowClientOptions,
} from "./workflow.js";

export type WorkflowRunVariables = Record<string, unknown>;

export type {
  CancelWorkflowRequest,
  CancelWorkflowResponse,
  RunWorkflowRequest,
  RunWorkflowResponse,
  WorkflowClientOptions as WorkflowRunClientOptions,
};

export { WorkflowClientError as WorkflowRunClientError };

export class WorkflowRunClient {
  private readonly client: WorkflowClient;

  constructor(options: WorkflowClientOptions) { this.client = createWorkflowClient(options); }
  run(request: RunWorkflowRequest): Promise<RunWorkflowResponse> { return this.client.run(request); }
  runDefinition(definitionId: string, variables?: WorkflowRunVariables): Promise<RunWorkflowResponse> { return this.client.run({ definition_id: definitionId, variables }); }
  cancel(instanceId: string): Promise<CancelWorkflowResponse> { return this.client.cancel({ instance_id: instanceId }); }
}

export function createWorkflowRunClient(options: WorkflowClientOptions): WorkflowRunClient { return new WorkflowRunClient(options); }
