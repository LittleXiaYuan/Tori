/** Lightweight cognis-workflows SDK facade over the Cognis slice. */
import {
  CognisClient,
  CognisClientError,
  createCognisClient,
  type CogniWorkflowRunRequest,
  type CognisClientOptions,
} from "./cognis.js";

export type {
  CogniWorkflowRunRequest,
  CognisClientOptions as CognisWorkflowsClientOptions,
};

export { CognisClientError as CognisWorkflowsClientError };

export class CognisWorkflowsClient {
  private readonly client: CognisClient;

  constructor(options: CognisClientOptions) {
    this.client = createCognisClient(options);
  }

  list(id: string): Promise<Record<string, unknown>> {
    return this.client.workflows(id);
  }

  run(id: string, workflow: string, request: CogniWorkflowRunRequest = {}): Promise<Record<string, unknown>> {
    return this.client.runWorkflow(id, workflow, request);
  }
}

export function createCognisWorkflowsClient(options: CognisClientOptions): CognisWorkflowsClient {
  return new CognisWorkflowsClient(options);
}
