/** Lightweight conversations-control SDK facade over the Conversations slice. */
import {
  ConversationsClient,
  ConversationsClientError,
  createConversationsClient,
  type ConversationDeleteResponse,
  type ConversationMessage,
  type ConversationSession,
  type ConversationsClientOptions,
  type ManageConversationRequest,
  type ManageConversationResponse,
} from "./conversations.js";

export type {
  ConversationDeleteResponse,
  ConversationMessage,
  ConversationSession,
  ConversationsClientOptions as ConversationsControlClientOptions,
  ManageConversationRequest,
  ManageConversationResponse,
};

export { ConversationsClientError as ConversationsControlClientError };

export class ConversationsControlClient {
  private readonly client: ConversationsClient;

  constructor(options: ConversationsClientOptions) {
    this.client = createConversationsClient(options);
  }

  deleteMessages(sessionId: string): Promise<ConversationDeleteResponse> {
    return this.client.deleteMessages(sessionId);
  }

  manage(request: ManageConversationRequest): Promise<ManageConversationResponse> {
    return this.client.manage(request);
  }

  rename(sessionId: string, name: string): Promise<ManageConversationResponse> {
    return this.client.rename(sessionId, name);
  }

  pin(sessionId: string, pinned = true): Promise<ManageConversationResponse> {
    return this.client.pin(sessionId, pinned);
  }

  archive(sessionId: string, archive = true): Promise<ManageConversationResponse> {
    return this.client.archive(sessionId, archive);
  }
}

export function createConversationsControlClient(options: ConversationsClientOptions): ConversationsControlClient {
  return new ConversationsControlClient(options);
}
