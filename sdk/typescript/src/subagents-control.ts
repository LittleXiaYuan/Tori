/** Lightweight subagents-control SDK facade over the Subagents slice. */
import {
  SubagentsClient,
  SubagentsClientError,
  createSubagentsClient,
  type AppendSubagentMessagesResponse,
  type DeleteSubagentResponse,
  type SpawnSubagentRequest,
  type Subagent,
  type SubagentMessage,
  type SubagentsClientOptions,
} from "./subagents.js";

export type {
  AppendSubagentMessagesResponse,
  DeleteSubagentResponse,
  SpawnSubagentRequest,
  Subagent,
  SubagentMessage,
  SubagentsClientOptions as SubagentsControlClientOptions,
};

export { SubagentsClientError as SubagentsControlClientError };

export class SubagentsControlClient {
  private readonly client: SubagentsClient;

  constructor(options: SubagentsClientOptions) {
    this.client = createSubagentsClient(options);
  }

  spawn(request: SpawnSubagentRequest): Promise<Subagent> {
    return this.client.spawn(request);
  }

  destroy(id: string): Promise<DeleteSubagentResponse> {
    return this.client.destroy(id);
  }

  appendMessages(id: string, messages: SubagentMessage[]): Promise<AppendSubagentMessagesResponse> {
    return this.client.appendMessages(id, messages);
  }
}

export function createSubagentsControlClient(options: SubagentsClientOptions): SubagentsControlClient {
  return new SubagentsControlClient(options);
}
