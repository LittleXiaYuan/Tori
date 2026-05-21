/** Lightweight realtime-messages SDK facade over the Realtime slice. */
import {
  RealtimeClient,
  createRealtimeClient,
  type RealtimeClientOptions,
  type RealtimeInboundMessage,
  type RealtimeOutboundMessage,
} from "./realtime.js";

export type {
  RealtimeClientOptions as RealtimeMessagesClientOptions,
  RealtimeInboundMessage,
  RealtimeOutboundMessage,
};

export class RealtimeMessagesClient {
  private readonly client: RealtimeClient;

  constructor(options: RealtimeClientOptions) {
    this.client = createRealtimeClient(options);
  }

  ping(extra: Record<string, unknown> = {}): RealtimeOutboundMessage {
    return this.client.ping(extra);
  }

  chat(content: string, options: { session?: string } & Record<string, unknown> = {}): RealtimeOutboundMessage {
    return this.client.chat(content, options);
  }

  send(socket: { send(data: string): void }, message: RealtimeOutboundMessage): void {
    this.client.send(socket, message);
  }

  parse(data: string): RealtimeInboundMessage {
    return this.client.parse(data);
  }
}

export function createRealtimeMessagesClient(options: RealtimeClientOptions): RealtimeMessagesClient {
  return new RealtimeMessagesClient(options);
}
