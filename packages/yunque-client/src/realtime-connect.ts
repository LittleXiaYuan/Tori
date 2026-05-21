/** Lightweight realtime-connect SDK facade over the Realtime slice. */
import {
  RealtimeClient,
  createRealtimeClient,
  type RealtimeClientOptions,
  type RealtimeConnectOptions,
} from "./realtime.js";

export type {
  RealtimeClientOptions as RealtimeConnectClientOptions,
  RealtimeConnectOptions,
};

export class RealtimeConnectClient {
  private readonly client: RealtimeClient;

  constructor(options: RealtimeClientOptions) {
    this.client = createRealtimeClient(options);
  }

  wsUrl(options: RealtimeConnectOptions = {}): string {
    return this.client.wsUrl(options);
  }

  connect(options: RealtimeConnectOptions = {}): WebSocket {
    return this.client.connect(options);
  }
}

export function createRealtimeConnectClient(options: RealtimeClientOptions): RealtimeConnectClient {
  return new RealtimeConnectClient(options);
}
