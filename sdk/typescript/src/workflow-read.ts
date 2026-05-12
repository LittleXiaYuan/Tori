/** Lightweight workflow-read SDK facade over workflow definition reads. */
import {
  createWorkflowClient,
  WorkflowClient,
  WorkflowClientError,
  type ListWorkflowsResponse,
  type WorkflowClientOptions,
  type WorkflowDefinition,
  type WorkflowEdge,
  type WorkflowNode,
  type WorkflowVariable,
} from "./workflow.js";

export type {
  ListWorkflowsResponse as ListWorkflowReadResponse,
  WorkflowClientOptions as WorkflowReadClientOptions,
  WorkflowDefinition,
  WorkflowEdge,
  WorkflowNode,
  WorkflowVariable,
};

export { WorkflowClientError as WorkflowReadClientError };

export class WorkflowReadClient {
  private readonly client: WorkflowClient;

  constructor(options: WorkflowClientOptions) { this.client = createWorkflowClient(options); }
  list(): Promise<ListWorkflowsResponse> { return this.client.list(); }
  get(id: string): Promise<WorkflowDefinition> { return this.client.get(id); }
}

export function createWorkflowReadClient(options: WorkflowClientOptions): WorkflowReadClient { return new WorkflowReadClient(options); }
