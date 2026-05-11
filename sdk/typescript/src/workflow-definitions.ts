/** Lightweight workflow-definitions SDK facade over the Workflow slice. */
import {
  createWorkflowClient,
  WorkflowClient,
  WorkflowClientError,
  type DeleteWorkflowResponse,
  type ListWorkflowsResponse,
  type WorkflowClientOptions,
  type WorkflowDefinition,
  type WorkflowEdge,
  type WorkflowNode,
  type WorkflowVariable,
} from "./workflow.js";

export type {
  DeleteWorkflowResponse,
  ListWorkflowsResponse as ListWorkflowDefinitionsResponse,
  WorkflowClientOptions as WorkflowDefinitionsClientOptions,
  WorkflowDefinition,
  WorkflowEdge,
  WorkflowNode,
  WorkflowVariable,
};

export { WorkflowClientError as WorkflowDefinitionsClientError };

export class WorkflowDefinitionsClient {
  private readonly client: WorkflowClient;

  constructor(options: WorkflowClientOptions) {
    this.client = createWorkflowClient(options);
  }

  list(): Promise<ListWorkflowsResponse> {
    return this.client.list();
  }

  get(id: string): Promise<WorkflowDefinition> {
    return this.client.get(id);
  }

  save(definition: WorkflowDefinition): Promise<WorkflowDefinition> {
    return this.client.save(definition);
  }

  delete(id: string): Promise<DeleteWorkflowResponse> {
    return this.client.delete(id);
  }
}

export function createWorkflowDefinitionsClient(options: WorkflowClientOptions): WorkflowDefinitionsClient {
  return new WorkflowDefinitionsClient(options);
}
