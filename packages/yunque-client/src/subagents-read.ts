/** Lightweight subagents-read SDK facade over the Subagents slice. */
import {
  SubagentsClient,
  SubagentsClientError,
  createSubagentsClient,
  type Subagent,
  type SubagentMessage,
  type SubagentsClientOptions,
  type SubagentsResponse,
} from "./subagents.js";

export type {
  Subagent,
  SubagentMessage,
  SubagentsClientOptions as SubagentsReadClientOptions,
  SubagentsResponse,
};

export { SubagentsClientError as SubagentsReadClientError };

export class SubagentsReadClient {
  private readonly client: SubagentsClient;

  constructor(options: SubagentsClientOptions) {
    this.client = createSubagentsClient(options);
  }

  list(parentId?: string): Promise<SubagentsResponse> {
    return this.client.list(parentId);
  }

  get(id: string): Promise<Subagent> {
    return this.client.get(id);
  }
}

export function createSubagentsReadClient(options: SubagentsClientOptions): SubagentsReadClient {
  return new SubagentsReadClient(options);
}
