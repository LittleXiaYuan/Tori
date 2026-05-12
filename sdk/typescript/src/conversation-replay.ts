/** Lightweight conversation-replay SDK facade over conversation replay timelines. */
import {
  ConversationsClient,
  ConversationsClientError,
  createConversationsClient,
  type ConversationPipelinePhase,
  type ConversationReplayOptions,
  type ConversationReplayResponse,
  type ConversationReplayTurn,
  type ConversationsClientOptions,
} from "./conversations.js";

export type {
  ConversationPipelinePhase,
  ConversationReplayOptions,
  ConversationReplayResponse,
  ConversationReplayTurn,
  ConversationsClientOptions as ConversationReplayClientOptions,
};

export { ConversationsClientError as ConversationReplayClientError };

export class ConversationReplayClient {
  private readonly client: ConversationsClient;

  constructor(options: ConversationsClientOptions) { this.client = createConversationsClient(options); }
  get(sessionId: string, options: ConversationReplayOptions = {}): Promise<ConversationReplayResponse> { return this.client.replay(sessionId, options); }
}

export function createConversationReplayClient(options: ConversationsClientOptions): ConversationReplayClient { return new ConversationReplayClient(options); }
