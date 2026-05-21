/** Lightweight conversation-sessions SDK facade over conversation session listing. */
import {
  ConversationsClient,
  ConversationsClientError,
  createConversationsClient,
  type ConversationSession,
  type ConversationsClientOptions,
  type ConversationsResponse,
} from "./conversations.js";

export type {
  ConversationSession,
  ConversationsClientOptions as ConversationSessionsClientOptions,
  ConversationsResponse,
};

export { ConversationsClientError as ConversationSessionsClientError };

export class ConversationSessionsClient {
  private readonly client: ConversationsClient;

  constructor(options: ConversationsClientOptions) { this.client = createConversationsClient(options); }
  list(options: { archived?: boolean } = {}): Promise<ConversationsResponse> { return this.client.list(options); }
}

export function createConversationSessionsClient(options: ConversationsClientOptions): ConversationSessionsClient { return new ConversationSessionsClient(options); }
