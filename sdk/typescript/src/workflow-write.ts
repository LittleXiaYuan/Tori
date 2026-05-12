/** Lightweight workflow-write SDK facade over workflow definition mutations. */
import {
  createWorkflowClient,
  WorkflowClient,
  WorkflowClientError,
  type DeleteWorkflowResponse,
  type WorkflowClientOptions,
  type WorkflowDefinition,
  type WorkflowEdge,
  type WorkflowNode,
  type WorkflowVariable,
} from "./workflow.js";

export type {
  DeleteWorkflowResponse,
  WorkflowClientOptions as WorkflowWriteClientOptions,
  WorkflowDefinition,
  WorkflowEdge,
  WorkflowNode,
  WorkflowVariable,
};

export { WorkflowClientError as WorkflowWriteClientError };

export class WorkflowWriteClient {
  private readonly client: WorkflowClient;

  constructor(options: WorkflowClientOptions) { this.client = createWorkflowClient(options); }
  save(definition: WorkflowDefinition): Promise<WorkflowDefinition> { return this.client.save(definition); }
  delete(id: string): Promise<DeleteWorkflowResponse> { return this.client.delete(id); }
}

export function createWorkflowWriteClient(options: WorkflowClientOptions): WorkflowWriteClient { return new WorkflowWriteClient(options); }
