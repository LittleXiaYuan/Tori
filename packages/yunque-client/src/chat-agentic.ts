/** Lightweight chat-agentic SDK facade over the Chat slice. */
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
  ChatClientOptions as ChatAgenticClientOptions,
  ChatMessage,
  ChatPlanStep,
  ChatRequest,
  ChatResponse,
  ChatRole,
};

export { ChatClientError as ChatAgenticClientError };

export class ChatAgenticClient {
  private readonly client: ChatClient;

  constructor(options: ChatClientOptions) {
    this.client = createChatClient(options);
  }

  agentic(body: ChatRequest): Promise<ChatResponse> {
    return this.client.agentic(body);
  }
}

export function createChatAgenticClient(options: ChatClientOptions): ChatAgenticClient {
  return new ChatAgenticClient(options);
}
