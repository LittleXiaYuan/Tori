/** Lightweight workflow-runs SDK facade over the Workflow slice. */
import {
  createWorkflowClient,
  WorkflowClient,
  WorkflowClientError,
  type CancelWorkflowRequest,
  type CancelWorkflowResponse,
  type ListWorkflowInstancesResponse,
  type RunWorkflowRequest,
  type RunWorkflowResponse,
  type WorkflowClientOptions,
  type WorkflowInstance,
} from "./workflow.js";

export type WorkflowRunVariables = Record<string, unknown>;

export type {
  CancelWorkflowRequest,
  CancelWorkflowResponse,
  ListWorkflowInstancesResponse as ListWorkflowRunsResponse,
  RunWorkflowRequest,
  RunWorkflowResponse,
  WorkflowClientOptions as WorkflowRunsClientOptions,
  WorkflowInstance,
};

export { WorkflowClientError as WorkflowRunsClientError };

export class WorkflowRunsClient {
  private readonly client: WorkflowClient;

  constructor(options: WorkflowClientOptions) {
    this.client = createWorkflowClient(options);
  }

  run(request: RunWorkflowRequest): Promise<RunWorkflowResponse> {
    return this.client.run(request);
  }

  runDefinition(definitionId: string, variables?: WorkflowRunVariables): Promise<RunWorkflowResponse> {
    return this.client.run({ definition_id: definitionId, variables });
  }

  list(): Promise<ListWorkflowInstancesResponse> {
    return this.client.instances();
  }

  get(instanceId: string): Promise<WorkflowInstance> {
    return this.client.getInstance(instanceId);
  }

  cancel(instanceId: string): Promise<CancelWorkflowResponse> {
    return this.client.cancel({ instance_id: instanceId });
  }
}

export function createWorkflowRunsClient(options: WorkflowClientOptions): WorkflowRunsClient {
  return new WorkflowRunsClient(options);
}
