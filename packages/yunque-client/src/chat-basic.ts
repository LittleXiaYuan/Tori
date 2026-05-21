/** Lightweight chat-basic SDK facade over the Chat slice. */
import {
  ChatClient,
  ChatClientError,
  createChatClient,
  type ChatClientOptions,
  type ChatMessage,
  type ChatPlanStep,
  type ChatRequest,
  type ChatResponse,
  type ChatRole,
} from "./chat.js";

export type {
  ChatClientOptions as ChatBasicClientOptions,
  ChatMessage,
  ChatPlanStep,
  ChatRequest,
  ChatResponse,
  ChatRole,
};

export { ChatClientError as ChatBasicClientError };

export class ChatBasicClient {
  private readonly client: ChatClient;

  constructor(options: ChatClientOptions) {
    this.client = createChatClient(options);
  }

  send(body: ChatRequest): Promise<ChatResponse> {
    return this.client.send(body);
  }
}

export function createChatBasicClient(options: ChatClientOptions): ChatBasicClient {
  return new ChatBasicClient(options);
}
