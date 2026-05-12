/** Lightweight webchat-widget SDK facade over the WebChat slice. */
import {
  WebChatClient,
  WebChatClientError,
  createWebChatClient,
  type WebChatWidgetOptions,
} from "./webchat.js";

export type { WebChatWidgetOptions };
export { WebChatClientError as WebChatWidgetClientError };

export class WebChatWidgetClient {
  private readonly client: WebChatClient;

  constructor(options: WebChatWidgetOptions) {
    this.client = createWebChatClient(options);
  }

  widgetUrl(): string {
    return this.client.widgetUrl();
  }

  widgetScript(origin?: string): Promise<string> {
    return this.client.widgetScript(origin);
  }
}

export function createWebChatWidgetClient(options: WebChatWidgetOptions): WebChatWidgetClient {
  return new WebChatWidgetClient(options);
}
