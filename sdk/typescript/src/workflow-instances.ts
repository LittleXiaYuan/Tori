/** Lightweight workflow-instances SDK facade over workflow instance reads. */
import {
  createWorkflowClient,
  WorkflowClient,
  WorkflowClientError,
  type ListWorkflowInstancesResponse,
  type WorkflowClientOptions,
  type WorkflowInstance,
  type WorkflowNodeState,
} from "./workflow.js";

export type {
  ListWorkflowInstancesResponse,
  WorkflowClientOptions as WorkflowInstancesClientOptions,
  WorkflowInstance,
  WorkflowNodeState,
};

export { WorkflowClientError as WorkflowInstancesClientError };

export class WorkflowInstancesClient {
  private readonly client: WorkflowClient;

  constructor(options: WorkflowClientOptions) { this.client = createWorkflowClient(options); }
  list(): Promise<ListWorkflowInstancesResponse> { return this.client.instances(); }
  get(instanceId: string): Promise<WorkflowInstance> { return this.client.getInstance(instanceId); }
}

export function createWorkflowInstancesClient(options: WorkflowClientOptions): WorkflowInstancesClient { return new WorkflowInstancesClient(options); }
