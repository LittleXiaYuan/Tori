/** Lightweight heartbeat-observe SDK facade over the Heartbeat slice. */
import {
  HeartbeatClient,
  HeartbeatClientError,
  createHeartbeatClient,
  type HeartbeatClientOptions,
  type HeartbeatLogEntry,
  type HeartbeatLogsQuery,
  type HeartbeatStatusResponse,
} from "./heartbeat.js";

export type {
  HeartbeatClientOptions as HeartbeatObserveClientOptions,
  HeartbeatLogEntry,
  HeartbeatLogsQuery,
  HeartbeatStatusResponse,
};

export { HeartbeatClientError as HeartbeatObserveClientError };

export class HeartbeatObserveClient {
  private readonly client: HeartbeatClient;

  constructor(options: HeartbeatClientOptions) {
    this.client = createHeartbeatClient(options);
  }

  status(): Promise<HeartbeatStatusResponse> {
    return this.client.status();
  }

  logs(query?: HeartbeatLogsQuery): Promise<HeartbeatLogEntry[]> {
    return this.client.logs(query);
  }
}

export function createHeartbeatObserveClient(options: HeartbeatClientOptions): HeartbeatObserveClient {
  return new HeartbeatObserveClient(options);
}
