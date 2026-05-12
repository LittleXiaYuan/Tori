/** Lightweight chat-stream SDK facade over the Chat slice. */
import {
  ChatClient,
  ChatClientError,
  createChatClient,
  type ChatClientOptions,
  type ChatRequest,
  type ChatStreamItem,
} from "./chat.js";

export type {
  ChatClientOptions as ChatStreamClientOptions,
  ChatRequest,
  ChatStreamItem,
};

export { ChatClientError as ChatStreamClientError };

export class ChatStreamClient {
  private readonly client: ChatClient;

  constructor(options: ChatClientOptions) {
    this.client = createChatClient(options);
  }

  stream(body: ChatRequest): AsyncGenerator<ChatStreamItem> {
    return this.client.stream(body);
  }

  parseStream(body: ReadableStream<Uint8Array>): AsyncGenerator<ChatStreamItem> {
    return this.client.parseStream(body);
  }
}

export function createChatStreamClient(options: ChatClientOptions): ChatStreamClient {
  return new ChatStreamClient(options);
}
