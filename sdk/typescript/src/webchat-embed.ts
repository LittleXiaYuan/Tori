/** Lightweight webchat-embed SDK facade over the WebChat snippet helpers. */
import {
  WebChatClient,
  buildWebChatEmbedSnippet,
  createWebChatClient,
  type WebChatEmbedOptions,
  type WebChatPosition,
  type WebChatTheme,
  type WebChatWidgetOptions,
} from "./webchat.js";

export type {
  WebChatEmbedOptions,
  WebChatPosition,
  WebChatTheme,
  WebChatWidgetOptions as WebChatEmbedClientOptions,
};

export { buildWebChatEmbedSnippet };

export class WebChatEmbedClient {
  private readonly client: WebChatClient;

  constructor(options: WebChatWidgetOptions) {
    this.client = createWebChatClient(options);
  }

  embedSnippet(options: WebChatEmbedOptions): string {
    return this.client.embedSnippet(options);
  }
}

export function createWebChatEmbedClient(options: WebChatWidgetOptions): WebChatEmbedClient {
  return new WebChatEmbedClient(options);
}
