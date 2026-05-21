/** Lightweight conversation-manage SDK facade over conversation metadata mutations. */
import {
  ConversationsClient,
  ConversationsClientError,
  createConversationsClient,
  type ConversationSession,
  type ConversationsClientOptions,
  type ManageConversationRequest,
  type ManageConversationResponse,
} from "./conversations.js";

export type {
  ConversationSession,
  ConversationsClientOptions as ConversationManageClientOptions,
  ManageConversationRequest,
  ManageConversationResponse,
};

export { ConversationsClientError as ConversationManageClientError };

export class ConversationManageClient {
  private readonly client: ConversationsClient;

  constructor(options: ConversationsClientOptions) { this.client = createConversationsClient(options); }
  update(request: ManageConversationRequest): Promise<ManageConversationResponse> { return this.client.manage(request); }
  rename(sessionId: string, name: string): Promise<ManageConversationResponse> { return this.client.rename(sessionId, name); }
  pin(sessionId: string, pinned = true): Promise<ManageConversationResponse> { return this.client.pin(sessionId, pinned); }
  archive(sessionId: string, archive = true): Promise<ManageConversationResponse> { return this.client.archive(sessionId, archive); }
}

export function createConversationManageClient(options: ConversationsClientOptions): ConversationManageClient { return new ConversationManageClient(options); }
