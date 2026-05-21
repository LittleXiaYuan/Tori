/** Lightweight conversations-read SDK facade over the Conversations slice. */
import {
  ConversationsClient,
  ConversationsClientError,
  createConversationsClient,
  type ConversationContentPart,
  type ConversationMessage,
  type ConversationMessageRole,
  type ConversationMessagesResponse,
  type ConversationPipelinePhase,
  type ConversationReplayOptions,
  type ConversationReplayResponse,
  type ConversationReplayTurn,
  type ConversationSession,
  type ConversationsClientOptions,
  type ConversationsResponse,
} from "./conversations.js";

export type {
  ConversationContentPart,
  ConversationMessage,
  ConversationMessageRole,
  ConversationMessagesResponse,
  ConversationPipelinePhase,
  ConversationReplayOptions,
  ConversationReplayResponse,
  ConversationReplayTurn,
  ConversationSession,
  ConversationsClientOptions as ConversationsReadClientOptions,
  ConversationsResponse,
};

export { ConversationsClientError as ConversationsReadClientError };

export class ConversationsReadClient {
  private readonly client: ConversationsClient;

  constructor(options: ConversationsClientOptions) {
    this.client = createConversationsClient(options);
  }

  list(options: { archived?: boolean } = {}): Promise<ConversationsResponse> {
    return this.client.list(options);
  }

  messages(sessionId: string): Promise<ConversationMessagesResponse> {
    return this.client.messages(sessionId);
  }

  replay(sessionId: string, options: ConversationReplayOptions = {}): Promise<ConversationReplayResponse> {
    return this.client.replay(sessionId, options);
  }
}

export function createConversationsReadClient(options: ConversationsClientOptions): ConversationsReadClient {
  return new ConversationsReadClient(options);
}
