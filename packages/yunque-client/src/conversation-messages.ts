/** Lightweight conversation-messages SDK facade over conversation message reads. */
import {
  ConversationsClient,
  ConversationsClientError,
  createConversationsClient,
  type ConversationContentPart,
  type ConversationMessage,
  type ConversationMessageRole,
  type ConversationMessagesResponse,
  type ConversationsClientOptions,
} from "./conversations.js";

export type {
  ConversationContentPart,
  ConversationMessage,
  ConversationMessageRole,
  ConversationMessagesResponse,
  ConversationsClientOptions as ConversationMessagesClientOptions,
};

export { ConversationsClientError as ConversationMessagesClientError };

export class ConversationMessagesClient {
  private readonly client: ConversationsClient;

  constructor(options: ConversationsClientOptions) { this.client = createConversationsClient(options); }
  list(sessionId: string): Promise<ConversationMessagesResponse> { return this.client.messages(sessionId); }
}

export function createConversationMessagesClient(options: ConversationsClientOptions): ConversationMessagesClient { return new ConversationMessagesClient(options); }
