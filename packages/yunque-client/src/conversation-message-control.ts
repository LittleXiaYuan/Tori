/** Lightweight conversation-message-control SDK facade over conversation message deletion. */
import {
  ConversationsClient,
  ConversationsClientError,
  createConversationsClient,
  type ConversationDeleteResponse,
  type ConversationsClientOptions,
} from "./conversations.js";

export type {
  ConversationDeleteResponse,
  ConversationsClientOptions as ConversationMessageControlClientOptions,
};

export { ConversationsClientError as ConversationMessageControlClientError };

export class ConversationMessageControlClient {
  private readonly client: ConversationsClient;

  constructor(options: ConversationsClientOptions) { this.client = createConversationsClient(options); }
  delete(sessionId: string): Promise<ConversationDeleteResponse> { return this.client.deleteMessages(sessionId); }
}

export function createConversationMessageControlClient(options: ConversationsClientOptions): ConversationMessageControlClient { return new ConversationMessageControlClient(options); }
